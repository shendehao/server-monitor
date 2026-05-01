//go:build windows

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/gorilla/websocket"
)

// ═══ Media Foundation 摄像头采集 ═══
// DirectShow (devenum.dll) 在某些机器上不可用 (0x80040154)
// Media Foundation 是 Windows 7+ 的现代替代，使用 PnP 枚举设备
// 不依赖 DirectShow COM 注册

var (
	modMFPlat      = syscall.NewLazyDLL("mfplat.dll")
	modMF          = syscall.NewLazyDLL("mf.dll")
	modMFReadWrite = syscall.NewLazyDLL("mfreadwrite.dll")

	procMFStartup                           = modMFPlat.NewProc("MFStartup")
	procMFShutdown                          = modMFPlat.NewProc("MFShutdown")
	procMFCreateAttributes                  = modMFPlat.NewProc("MFCreateAttributes")
	procMFCreateMediaType                   = modMFPlat.NewProc("MFCreateMediaType")
	procMFEnumDeviceSources                 = modMF.NewProc("MFEnumDeviceSources")
	procMFCreateSourceReaderFromMediaSource = modMFReadWrite.NewProc("MFCreateSourceReaderFromMediaSource")
)

// MF GUIDs
var (
	guidMF_DEVSOURCE_ATTRIBUTE_SOURCE_TYPE        = _GUID{0xc60ac5fe, 0x252a, 0x478f, [8]byte{0xa0, 0xef, 0xbc, 0x8f, 0xa5, 0xf7, 0xca, 0xd3}}
	guidMF_DEVSOURCE_ATTRIBUTE_SOURCE_TYPE_VIDCAP = _GUID{0x8ac3587a, 0x4ae7, 0x42d8, [8]byte{0x99, 0xe0, 0x0a, 0x60, 0x13, 0xee, 0xf9, 0x0f}}
	guidMFMediaType_Video                         = _GUID{0x73646976, 0x0000, 0x0010, [8]byte{0x80, 0x00, 0x00, 0xAA, 0x00, 0x38, 0x9B, 0x71}}
	guidMFVideoFormat_RGB32                       = _GUID{0x00000016, 0x0000, 0x0010, [8]byte{0x80, 0x00, 0x00, 0xAA, 0x00, 0x38, 0x9B, 0x71}}
	guidMF_MT_MAJOR_TYPE                          = _GUID{0x48eba18e, 0xf8c9, 0x4687, [8]byte{0xbf, 0x11, 0x0a, 0x74, 0xc9, 0xf9, 0x6a, 0x8f}}
	guidMF_MT_SUBTYPE                             = _GUID{0xf7e34c9a, 0x42e8, 0x4714, [8]byte{0xb7, 0x4b, 0xcb, 0x29, 0xd7, 0x2c, 0x35, 0xe5}}
	guidMF_MT_FRAME_SIZE                          = _GUID{0x1652c33d, 0xd6b2, 0x4012, [8]byte{0xb8, 0x34, 0x72, 0x03, 0x08, 0x49, 0xa3, 0x7d}}
	guidIID_IMFMediaSource                        = _GUID{0x279a808d, 0xaec7, 0x40c8, [8]byte{0x9c, 0x6b, 0xa6, 0xb4, 0x92, 0xc7, 0x8a, 0x66}}
)

const (
	mfVersion                      = 0x00020070 // MF_VERSION
	mfStartupNoSocket              = 0x1
	mfSourceReaderFirstVideoStream = 0xFFFFFFFC
)

// ═══ MF vtable 索引常量 ═══
const (
	// IMFAttributes (继承 IUnknown 0-2)
	mfAttr_GetUINT64 = 8
	mfAttr_SetGUID   = 24
	// IMFActivate (继承 IMFAttributes 0-32)
	mfActivate_ActivateObject = 33
	// IMFSourceReader
	mfReader_GetCurrentMediaType = 6
	mfReader_SetCurrentMediaType = 7
	mfReader_ReadSample          = 9
	// IMFSample (继承 IMFAttributes 0-32)
	mfSample_ConvertToContiguousBuffer = 46
	// IMFMediaBuffer
	mfBuffer_Lock   = 3
	mfBuffer_Unlock = 4
)

// mfAvailable 检查 Media Foundation DLL 是否存在
func mfAvailable() bool {
	return modMFPlat.Load() == nil && modMF.Load() == nil && modMFReadWrite.Load() == nil
}

// ═══ MF 采集管线 ═══

type mfPipeline struct {
	pReader uintptr // IMFSourceReader
	width   int
	height  int
}

func (p *mfPipeline) close() {
	if p.pReader != 0 {
		comRelease(p.pReader)
		p.pReader = 0
	}
}

// mfInitPipeline 初始化 MF 采集管线：枚举设备 → 创建 SourceReader → 配置 RGB32 输出
func mfInitPipeline() (*mfPipeline, error) {
	// 1. 初始化 MF
	if err := procMFStartup.Find(); err != nil {
		return nil, fmt.Errorf("mfplat.dll不可用")
	}
	r, _, _ := procMFStartup.Call(mfVersion, mfStartupNoSocket)
	if int32(r) < 0 {
		return nil, fmt.Errorf("MFStartup failed: 0x%08x", uint32(r))
	}

	// 2. 创建属性，设置设备类型为视频捕获
	var pAttr uintptr
	r, _, _ = procMFCreateAttributes.Call(uintptr(unsafe.Pointer(&pAttr)), 1)
	if int32(r) < 0 {
		procMFShutdown.Call()
		return nil, fmt.Errorf("MFCreateAttributes failed: 0x%08x", uint32(r))
	}
	defer comRelease(pAttr)

	// IMFAttributes::SetGUID(MF_DEVSOURCE_ATTRIBUTE_SOURCE_TYPE, VIDCAP)
	r = comCall(pAttr, mfAttr_SetGUID,
		uintptr(unsafe.Pointer(&guidMF_DEVSOURCE_ATTRIBUTE_SOURCE_TYPE)),
		uintptr(unsafe.Pointer(&guidMF_DEVSOURCE_ATTRIBUTE_SOURCE_TYPE_VIDCAP)))
	if int32(r) < 0 {
		procMFShutdown.Call()
		return nil, fmt.Errorf("SetGUID source type failed: 0x%08x", uint32(r))
	}

	// 3. 枚举视频设备
	var ppDevices uintptr
	var deviceCount uint32
	r, _, _ = procMFEnumDeviceSources.Call(
		pAttr,
		uintptr(unsafe.Pointer(&ppDevices)),
		uintptr(unsafe.Pointer(&deviceCount)))
	if int32(r) < 0 || deviceCount == 0 {
		procMFShutdown.Call()
		return nil, fmt.Errorf("MF: 未检测到摄像头设备")
	}

	// 获取第一个设备
	pActivate := *(*uintptr)(unsafe.Pointer(ppDevices))

	// 释放数组（稍后释放 activate 对象）
	defer func() {
		for i := uint32(1); i < deviceCount; i++ {
			p := *(*uintptr)(unsafe.Pointer(ppDevices + uintptr(i)*unsafe.Sizeof(uintptr(0))))
			comRelease(p)
		}
		procCoTaskMemFree.Call(ppDevices)
	}()

	// 4. 激活设备 → IMFMediaSource
	var pSource uintptr
	r = comCall(pActivate, mfActivate_ActivateObject,
		uintptr(unsafe.Pointer(&guidIID_IMFMediaSource)),
		uintptr(unsafe.Pointer(&pSource)))
	comRelease(pActivate) // 激活后释放
	if int32(r) < 0 || pSource == 0 {
		procMFShutdown.Call()
		return nil, fmt.Errorf("MF ActivateObject failed: 0x%08x", uint32(r))
	}
	defer comRelease(pSource)

	// 5. 创建 SourceReader
	var pReader uintptr
	r, _, _ = procMFCreateSourceReaderFromMediaSource.Call(
		pSource, 0, uintptr(unsafe.Pointer(&pReader)))
	if int32(r) < 0 || pReader == 0 {
		procMFShutdown.Call()
		return nil, fmt.Errorf("MF CreateSourceReader failed: 0x%08x", uint32(r))
	}

	// 6. 配置输出格式为 RGB32
	var pMediaType uintptr
	r, _, _ = procMFCreateMediaType.Call(uintptr(unsafe.Pointer(&pMediaType)))
	if int32(r) >= 0 && pMediaType != 0 {
		comCall(pMediaType, mfAttr_SetGUID,
			uintptr(unsafe.Pointer(&guidMF_MT_MAJOR_TYPE)),
			uintptr(unsafe.Pointer(&guidMFMediaType_Video)))
		comCall(pMediaType, mfAttr_SetGUID,
			uintptr(unsafe.Pointer(&guidMF_MT_SUBTYPE)),
			uintptr(unsafe.Pointer(&guidMFVideoFormat_RGB32)))
		comCall(pReader, mfReader_SetCurrentMediaType,
			mfSourceReaderFirstVideoStream, 0, pMediaType)
		comRelease(pMediaType)
	}

	// 7. 读取当前媒体类型获取实际分辨率
	width, height := 640, 480
	var pActualType uintptr
	r = comCall(pReader, mfReader_GetCurrentMediaType,
		mfSourceReaderFirstVideoStream,
		uintptr(unsafe.Pointer(&pActualType)))
	if int32(r) >= 0 && pActualType != 0 {
		var frameSize uint64
		hr := comCall(pActualType, mfAttr_GetUINT64,
			uintptr(unsafe.Pointer(&guidMF_MT_FRAME_SIZE)),
			uintptr(unsafe.Pointer(&frameSize)))
		if int32(hr) >= 0 {
			w := int(frameSize >> 32)
			h := int(frameSize & 0xFFFFFFFF)
			if w > 0 && h > 0 {
				width, height = w, h
			}
		}
		comRelease(pActualType)
	}

	return &mfPipeline{pReader: pReader, width: width, height: height}, nil
}

// mfReadFrame 从 SourceReader 读取一帧并转为 JPEG
func (p *mfPipeline) mfReadFrame(quality int) ([]byte, int, int, error) {
	var streamIdx uint32
	var flags uint32
	var timestamp int64
	var pSample uintptr

	r := comCall(p.pReader, mfReader_ReadSample,
		mfSourceReaderFirstVideoStream, 0,
		uintptr(unsafe.Pointer(&streamIdx)),
		uintptr(unsafe.Pointer(&flags)),
		uintptr(unsafe.Pointer(&timestamp)),
		uintptr(unsafe.Pointer(&pSample)))
	if int32(r) < 0 {
		return nil, 0, 0, fmt.Errorf("ReadSample failed: 0x%08x", uint32(r))
	}
	if pSample == 0 {
		return nil, 0, 0, fmt.Errorf("ReadSample: empty sample")
	}
	defer comRelease(pSample)

	// ConvertToContiguousBuffer → IMFMediaBuffer
	var pBuffer uintptr
	r = comCall(pSample, mfSample_ConvertToContiguousBuffer,
		uintptr(unsafe.Pointer(&pBuffer)))
	if int32(r) < 0 || pBuffer == 0 {
		return nil, 0, 0, fmt.Errorf("ConvertToContiguousBuffer failed")
	}
	defer comRelease(pBuffer)

	// Lock buffer
	var pData uintptr
	var maxLen, curLen uint32
	r = comCall(pBuffer, mfBuffer_Lock,
		uintptr(unsafe.Pointer(&pData)),
		uintptr(unsafe.Pointer(&maxLen)),
		uintptr(unsafe.Pointer(&curLen)))
	if int32(r) < 0 {
		return nil, 0, 0, fmt.Errorf("Lock buffer failed")
	}
	defer comCall(pBuffer, mfBuffer_Unlock)

	if curLen == 0 || pData == 0 {
		return nil, 0, 0, fmt.Errorf("empty buffer")
	}

	// RGB32 (BGRA) → JPEG
	w, h := p.width, p.height
	stride := w * 4
	pixels := (*[1 << 28]byte)(unsafe.Pointer(pData))[:curLen:curLen]

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		srcOff := y * stride
		if srcOff+stride > int(curLen) {
			break
		}
		for x := 0; x < w; x++ {
			off := srcOff + x*4
			if off+4 > int(curLen) {
				break
			}
			img.SetRGBA(x, y, color.RGBA{R: pixels[off+2], G: pixels[off+1], B: pixels[off], A: 255})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), w, h, nil
}

// ═══ 公共接口 ═══

// mfCaptureFrame 使用 Media Foundation 采集单帧
func mfCaptureFrame() ([]byte, int, int, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	attachToInteractiveDesktop()

	hr, _, _ := procCoInitializeEx.Call(0, 0x2)
	if int32(hr) < 0 && hr != 1 {
		procCoInitializeEx.Call(0, 0x0)
	}
	defer procCoUninitialize.Call()

	pipe, err := mfInitPipeline()
	if err != nil {
		return nil, 0, 0, err
	}
	defer func() {
		pipe.close()
		procMFShutdown.Call()
	}()

	// 预热：读几帧丢弃（摄像头曝光需要时间）
	for i := 0; i < 5; i++ {
		pipe.mfReadFrame(50)
		time.Sleep(100 * time.Millisecond)
	}

	// 抓实际帧
	for attempt := 0; attempt < 10; attempt++ {
		jpgData, w, h, err := pipe.mfReadFrame(75)
		if err == nil && len(jpgData) > 0 {
			return jpgData, w, h, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, 0, 0, fmt.Errorf("MF: 10次抓帧均失败")
}

// tryMFWebcamStream 使用 Media Foundation 持续流式采集
func tryMFWebcamStream(conn *websocket.Conn, writeMu *sync.Mutex, stopCh chan struct{}) bool {
	if !mfAvailable() {
		return false
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	attachToInteractiveDesktop()

	hr, _, _ := procCoInitializeEx.Call(0, 0x2)
	if int32(hr) < 0 && hr != 1 {
		procCoInitializeEx.Call(0, 0x0)
	}
	defer procCoUninitialize.Call()

	pipe, err := mfInitPipeline()
	if err != nil {
		return false
	}
	defer func() {
		pipe.close()
		procMFShutdown.Call()
	}()

	// 预热
	for i := 0; i < 3; i++ {
		pipe.mfReadFrame(50)
		time.Sleep(100 * time.Millisecond)
	}

	for {
		select {
		case <-stopCh:
			return true
		default:
		}

		jpgData, w, h, err := pipe.mfReadFrame(65)
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		b64 := base64.StdEncoding.EncodeToString(jpgData)
		framePayload, _ := json.Marshal(map[string]interface{}{
			"image":  b64,
			"width":  w,
			"height": h,
			"size":   len(jpgData),
		})
		resp, _ := json.Marshal(AgentMessage{Type: c2e("webcam_frame"), Payload: framePayload})
		writeMu.Lock()
		conn.WriteMessage(websocket.TextMessage, resp)
		writeMu.Unlock()

		time.Sleep(100 * time.Millisecond) // ~10 FPS
	}
}
