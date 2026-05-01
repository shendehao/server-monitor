//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash"
	"hash/fnv"
	"image"
	"image/jpeg"
	"log"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/gorilla/websocket"
	"github.com/kbinani/screenshot"
)

var (
	modUser32                = syscall.NewLazyDLL("user32.dll")
	procOpenWindowStationW   = modUser32.NewProc("OpenWindowStationW")
	procSetProcessWinStation = modUser32.NewProc("SetProcessWindowStation")
	procOpenDesktopW         = modUser32.NewProc("OpenDesktopW")
	procSetThreadDesktop     = modUser32.NewProc("SetThreadDesktop")
	procCloseDesktop         = modUser32.NewProc("CloseDesktop")
	desktopAttachMu          sync.Mutex
	desktopAttachDone        bool
)

// attachToInteractiveDesktop 将当前进程/线程绑定到交互式桌面 WinSta0\Default
// 计划任务或服务启动的进程默认不在交互式桌面，导致 GetDC(0) + BitBlt 失败
func attachToInteractiveDesktop() {
	wsName, _ := syscall.UTF16PtrFromString("WinSta0")
	hWinSta, _, _ := procOpenWindowStationW.Call(
		uintptr(unsafe.Pointer(wsName)),
		0,
		0x37F, // WINSTA_ALL_ACCESS
	)
	if hWinSta != 0 {
		procSetProcessWinStation.Call(hWinSta)
	}

	deskName, _ := syscall.UTF16PtrFromString("Default")
	hDesktop, _, _ := procOpenDesktopW.Call(
		uintptr(unsafe.Pointer(deskName)),
		0, 0,
		0x01FF, // DESKTOP_ALL_ACCESS (GENERIC_ALL)
	)
	if hDesktop != 0 {
		procSetThreadDesktop.Call(hDesktop)
	}
	log.Printf("attachToInteractiveDesktop: winsta=%v desktop=%v", hWinSta, hDesktop)
}

// Windows 桌面截图流式传输（优化版）
// - 帧哈希比较，画面未变化时不发送（节省 80-90% 带宽）
// - 直接发送 JPEG 二进制，不做 base64（节省 33% 体积）
// - 复用缓冲区减少 GC 压力

type ScreenSession struct {
	id      string
	conn    *websocket.Conn
	writeMu *sync.Mutex
	done    chan struct{}
	stopCh  chan struct{}
}

type ScreenStartPayload struct {
	FPS     int `json:"fps"`
	Quality int `json:"quality"`
	Scale   int `json:"scale"`
}

var screenManager struct {
	mu       sync.Mutex
	sessions map[string]*ScreenSession
}

func init() {
	screenManager.sessions = make(map[string]*ScreenSession)
}

func handleScreenStart(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	var payload ScreenStartPayload
	json.Unmarshal(msg.Payload, &payload)

	if payload.FPS <= 0 || payload.FPS > 30 {
		payload.FPS = 2
	}
	if payload.Quality <= 0 || payload.Quality > 100 {
		payload.Quality = 50
	}
	if payload.Scale <= 0 || payload.Scale > 100 {
		payload.Scale = 50
	}

	screenManager.mu.Lock()
	if old, ok := screenManager.sessions[msg.ID]; ok {
		close(old.stopCh)
		delete(screenManager.sessions, msg.ID)
	}
	screenManager.mu.Unlock()

	session := &ScreenSession{
		id:      msg.ID,
		conn:    conn,
		writeMu: writeMu,
		done:    make(chan struct{}),
		stopCh:  make(chan struct{}),
	}

	screenManager.mu.Lock()
	screenManager.sessions[msg.ID] = session
	screenManager.mu.Unlock()

	log.Printf("桌面截图会话已启动: id=%s, fps=%d, quality=%d, scale=%d%%", msg.ID, payload.FPS, payload.Quality, payload.Scale)

	go screenCaptureLoop(session, payload)
}

func screenCaptureLoop(session *ScreenSession, cfg ScreenStartPayload) {
	defer func() {
		close(session.done)
		screenManager.mu.Lock()
		delete(screenManager.sessions, session.id)
		screenManager.mu.Unlock()
		log.Printf("桌面截图会话已结束: id=%s", session.id)
	}()

	// SYSTEM 身份（计划任务/服务启动）直接使用子进程截图，不尝试直接 BitBlt
	if shouldUseHelper() {
		log.Printf("SYSTEM 身份，直接使用子进程截图模式")
		screenCaptureLoopSession0(session, cfg)
		return
	}

	interval := time.Second / time.Duration(cfg.FPS)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastHash uint64 = 0xFFFFFFFFFFFFFFFF // 哨兵值，确保第一帧一定发送
	jpegBuf := bytes.NewBuffer(make([]byte, 0, 256*1024))
	hasher := fnv.New64a()    // 复用哈希器
	var frameBuf []byte       // 复用帧发送缓冲区
	var scaledImg *image.RGBA // 复用缩放图像缓冲区
	var errCount int
	var sending int32          // 原子标志，防止帧堆积（0=空闲，1=发送中）
	const maxDirectRetries = 5 // Win10 上 DXGI ACCESS_LOST 可能连续几次，给够重试次数

	for {
		select {
		case <-session.stopCh:
			return
		case <-ticker.C:
			if atomic.LoadInt32(&sending) != 0 {
				continue // 上一帧还没发完，跳过本帧
			}
			jpegBuf.Reset()
			w, h, hash, err := captureScreenReuse(jpegBuf, hasher, &scaledImg, cfg.Quality, cfg.Scale)
			if err != nil {
				errCount++
				log.Printf("截图失败(%d/%d): %v", errCount, maxDirectRetries, err)
				if errCount <= 2 {
					// 前两次失败：重置桌面绑定 + DXGI，下一帧重试
					resetDesktopAttach()
					resetDXGI()
					continue
				}
				if errCount < maxDirectRetries {
					// 继续重试直接截图（给 Win10 DXGI 恢复更多时间）
					time.Sleep(500 * time.Millisecond)
					continue
				}
				// 连续失败超过阈值，切换到 Session 0 子进程模式（带重试）
				log.Printf("直接截图连续失败%d次，切换到子进程截图模式", errCount)
				ticker.Stop()
				screenCaptureLoopSession0(session, cfg)
				return
			}
			errCount = 0
			// 画面没变化，跳过
			if hash == lastHash {
				continue
			}
			lastHash = hash
			// 复制数据后异步发送，不阻塞采集
			needed := jpegBuf.Len()
			if cap(frameBuf) < needed {
				frameBuf = make([]byte, needed)
			} else {
				frameBuf = frameBuf[:needed]
			}
			copy(frameBuf, jpegBuf.Bytes())
			frame := make([]byte, needed)
			copy(frame, frameBuf)
			fw, fh := w, h
			atomic.StoreInt32(&sending, 1)
			go func() {
				sendScreenFrameBinary(session, frame, fw, fh)
				atomic.StoreInt32(&sending, 0)
			}()
		}
	}
}

// sendScreenError 发送截图错误信息到前端
func sendScreenError(session *ScreenSession, errMsg string) {
	msg, _ := json.Marshal(AgentMessage{
		Type: c2e("screen_error"),
		ID:   session.id,
		Payload: func() json.RawMessage {
			p, _ := json.Marshal(map[string]string{"error": errMsg})
			return p
		}(),
	})
	session.writeMu.Lock()
	session.conn.WriteMessage(websocket.TextMessage, msg)
	session.writeMu.Unlock()
}

// tryAttachDesktop 尝试绑定交互式桌面（可重试，非 sync.Once）
func tryAttachDesktop() {
	desktopAttachMu.Lock()
	defer desktopAttachMu.Unlock()
	if !desktopAttachDone {
		attachToInteractiveDesktop()
		desktopAttachDone = true
	}
}

// resetDesktopAttach 重置桌面绑定标志，下次截图时重新绑定
func resetDesktopAttach() {
	desktopAttachMu.Lock()
	desktopAttachDone = false
	desktopAttachMu.Unlock()
}

// ─── 直接截图（DXGI 优先，BitBlt 兜底） ───

var (
	directDXGI     *DXGICapturer
	directDXGIMu   sync.Mutex
	directDXGIFail bool // DXGI 初始化失败过，不再重试
)

// resetDXGI 释放当前 DXGI 截图器，下次截图时重新初始化
// Win10 遇到 ACCESS_LOST 后需要完全重建 DXGI 资源
func resetDXGI() {
	directDXGIMu.Lock()
	if directDXGI != nil {
		directDXGI.Close()
		directDXGI = nil
	}
	directDXGIFail = false // 允许重新尝试 DXGI
	directDXGIMu.Unlock()
}

// captureScreenReuse 截图并输出 JPEG，复用缩放缓冲区和哈希器减少 GC
// 优先 DXGI Desktop Duplication（GPU 加速），失败则回退 BitBlt
func captureScreenReuse(buf *bytes.Buffer, hasher hash.Hash64, scaledBuf **image.RGBA, quality, scale int) (int, int, uint64, error) {
	tryAttachDesktop()

	var img *image.RGBA
	var err error

	// 尝试 DXGI
	directDXGIMu.Lock()
	if !directDXGIFail && directDXGI == nil {
		directDXGI, err = NewDXGICapturer()
		if err != nil {
			directDXGIFail = true
			log.Printf("DXGI 初始化失败，使用 BitBlt: %v", err)
		}
	}
	dxgi := directDXGI
	directDXGIMu.Unlock()

	if dxgi != nil {
		img, err = dxgi.CaptureFrame()
		if err != nil {
			if err != ErrNoNewFrame {
				// 真正的 DXGI 错误（ACCESS_LOST 等），重置以便下次重建
				log.Printf("DXGI 截图错误，重置: %v", err)
				resetDXGI()
			}
			img = nil
		}
	}

	// BitBlt 兜底
	if img == nil {
		n := screenshot.NumActiveDisplays()
		if n == 0 {
			return 0, 0, 0, fmt.Errorf("no active displays")
		}
		bounds := screenshot.GetDisplayBounds(0)
		img, err = screenshot.CaptureRect(bounds)
		if err != nil {
			return 0, 0, 0, err
		}
	}

	origW := img.Bounds().Dx()
	origH := img.Bounds().Dy()

	var target image.Image = img
	outW, outH := origW, origH

	if scale < 100 {
		outW = origW * scale / 100
		outH = origH * scale / 100
		scaleImageReuse(img, outW, outH, scaledBuf)
		target = *scaledBuf
	}

	jpeg.Encode(buf, target, &jpeg.Options{Quality: quality})

	hasher.Reset()
	hasher.Write(buf.Bytes())

	return outW, outH, hasher.Sum64(), nil
}

// scaleImageReuse 快速最近邻缩放，复用目标图像缓冲区（每帧节省 ~2MB 分配）
func scaleImageReuse(src *image.RGBA, newW, newH int, dstPtr **image.RGBA) {
	srcW := src.Bounds().Dx()
	srcH := src.Bounds().Dy()
	srcStride := src.Stride
	srcPix := src.Pix

	dst := *dstPtr
	// 尺寸变化时才重新分配
	if dst == nil || dst.Bounds().Dx() != newW || dst.Bounds().Dy() != newH {
		dst = image.NewRGBA(image.Rect(0, 0, newW, newH))
		*dstPtr = dst
	}
	dstStride := dst.Stride
	dstPix := dst.Pix

	for y := 0; y < newH; y++ {
		srcY := y * srcH / newH
		srcRow := srcY * srcStride
		dstRow := y * dstStride
		for x := 0; x < newW; x++ {
			srcX := x * srcW / newW
			si := srcRow + srcX*4
			di := dstRow + x*4
			dstPix[di] = srcPix[si]
			dstPix[di+1] = srcPix[si+1]
			dstPix[di+2] = srcPix[si+2]
			dstPix[di+3] = srcPix[si+3]
		}
	}
}

// sendScreenFrameBinary 发送二进制帧（JSON header + 二进制 JPEG）
// 返回 error 以便调用方感知 WS 写入失败
func sendScreenFrameBinary(session *ScreenSession, jpegData []byte, w, h int) error {
	header, _ := json.Marshal(AgentMessage{
		Type: c2e("screen_frame"),
		ID:   session.id,
		Payload: func() json.RawMessage {
			p, _ := json.Marshal(map[string]interface{}{
				"width":  w,
				"height": h,
				"size":   len(jpegData),
				"ts":     time.Now().UnixMilli(),
			})
			return p
		}(),
	})

	session.writeMu.Lock()
	err := session.conn.WriteMessage(websocket.TextMessage, header)
	if err == nil {
		err = session.conn.WriteMessage(websocket.BinaryMessage, jpegData)
	}
	session.writeMu.Unlock()
	return err
}

func handleScreenStop(msg AgentMessage) {
	screenManager.mu.Lock()
	session, ok := screenManager.sessions[msg.ID]
	if ok {
		close(session.stopCh)
		delete(screenManager.sessions, msg.ID)
	}
	screenManager.mu.Unlock()
}

func cleanupAllScreenSessions() {
	screenManager.mu.Lock()
	sessions := make([]*ScreenSession, 0, len(screenManager.sessions))
	for _, s := range screenManager.sessions {
		sessions = append(sessions, s)
	}
	screenManager.mu.Unlock()

	for _, s := range sessions {
		select {
		case <-s.stopCh:
		default:
			close(s.stopCh)
		}
	}
}
