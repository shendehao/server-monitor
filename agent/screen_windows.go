//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"image"
	"image/jpeg"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kbinani/screenshot"
)

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

	if payload.FPS <= 0 || payload.FPS > 15 {
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

	interval := time.Second / time.Duration(cfg.FPS)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastHash uint64 = 0xFFFFFFFFFFFFFFFF // 哨兵值，确保第一帧一定发送
	var jpegBuf bytes.Buffer
	var errCount int

	for {
		select {
		case <-session.stopCh:
			return
		case <-ticker.C:
			jpegBuf.Reset()
			w, h, hash, err := captureScreenWithFallback(&jpegBuf, cfg.Quality, cfg.Scale)
			if err != nil {
				errCount++
				log.Printf("截图失败: %v", err)
				if errCount <= 3 {
					sendScreenError(session, fmt.Sprintf("截图失败: %v", err))
				}
				// 连续失败5次，自动切换到 Session 0 helper 模式
				if errCount >= 5 {
					log.Printf("直接截图连续失败%d次，切换到子进程截图模式", errCount)
					sendScreenError(session, "正在切换到子进程截图模式...")
					ticker.Stop()
					screenCaptureLoopSession0(session, cfg)
					return
				}
				continue
			}
			errCount = 0
			// 画面没变化，跳过
			if hash == lastHash {
				continue
			}
			lastHash = hash
			sendScreenFrameBinary(session, jpegBuf.Bytes(), w, h)
		}
	}
}

// sendScreenError 发送截图错误信息到前端
func sendScreenError(session *ScreenSession, errMsg string) {
	msg, _ := json.Marshal(AgentMessage{
		Type: "screen_error",
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

// captureScreenBinary 截图并直接输出 JPEG 到缓冲区，返回尺寸和哈希
func captureScreenBinary(buf *bytes.Buffer, quality, scale int) (int, int, uint64, error) {
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return 0, 0, 0, fmt.Errorf("no active displays")
	}

	bounds := screenshot.GetDisplayBounds(0)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return 0, 0, 0, err
	}

	origW := img.Bounds().Dx()
	origH := img.Bounds().Dy()

	var target image.Image = img
	outW, outH := origW, origH

	if scale < 100 {
		outW = origW * scale / 100
		outH = origH * scale / 100
		target = scaleImageFast(img, outW, outH)
	}

	jpeg.Encode(buf, target, &jpeg.Options{Quality: quality})

	// FNV hash of JPEG bytes for change detection
	h := fnv.New64a()
	h.Write(buf.Bytes())

	return outW, outH, h.Sum64(), nil
}

// scaleImageFast 快速最近邻缩放（直接操作像素数组）
func scaleImageFast(src *image.RGBA, newW, newH int) *image.RGBA {
	srcW := src.Bounds().Dx()
	srcH := src.Bounds().Dy()
	srcStride := src.Stride
	srcPix := src.Pix

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
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
	return dst
}

// sendScreenFrameBinary 发送二进制帧（JSON header + 二进制 JPEG）
func sendScreenFrameBinary(session *ScreenSession, jpegData []byte, w, h int) {
	// 用 JSON 包装元数据 + JPEG 数据通过同一条消息发送
	// 格式: AgentMessage { type: "screen_frame", payload: { width, height, size, ts } }
	// 紧跟一条二进制消息包含 JPEG 数据
	header, _ := json.Marshal(AgentMessage{
		Type: "screen_frame",
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
	session.conn.WriteMessage(websocket.TextMessage, header)
	session.conn.WriteMessage(websocket.BinaryMessage, jpegData)
	session.writeMu.Unlock()
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
