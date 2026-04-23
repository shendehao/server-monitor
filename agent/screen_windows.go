//go:build windows

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/jpeg"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kbinani/screenshot"
)

// Windows 桌面截图流式传输

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
	Scale   int `json:"scale"` // 缩放百分比 (10-100)
}

type ScreenFramePayload struct {
	Data   string `json:"data"` // base64 JPEG
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Ts     int64  `json:"ts"`
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

	// 默认值
	if payload.FPS <= 0 || payload.FPS > 15 {
		payload.FPS = 2
	}
	if payload.Quality <= 0 || payload.Quality > 100 {
		payload.Quality = 50
	}
	if payload.Scale <= 0 || payload.Scale > 100 {
		payload.Scale = 50
	}

	// 关闭已有的同 ID 会话
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

	for {
		select {
		case <-session.stopCh:
			return
		case <-ticker.C:
			frame, w, h, err := captureScreen(cfg.Quality, cfg.Scale)
			if err != nil {
				log.Printf("截图失败: %v", err)
				continue
			}
			sendScreenFrame(session, frame, w, h)
		}
	}
}

func captureScreen(quality, scale int) (string, int, int, error) {
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return "", 0, 0, nil
	}

	bounds := screenshot.GetDisplayBounds(0)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return "", 0, 0, err
	}

	// 缩放
	origW := img.Bounds().Dx()
	origH := img.Bounds().Dy()
	newW := origW * scale / 100
	newH := origH * scale / 100

	// 简单最近邻缩放
	if scale < 100 {
		scaled := scaleImage(img, newW, newH)
		var buf bytes.Buffer
		jpeg.Encode(&buf, scaled, &jpeg.Options{Quality: quality})
		return base64.StdEncoding.EncodeToString(buf.Bytes()), newW, newH, nil
	}

	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	return base64.StdEncoding.EncodeToString(buf.Bytes()), origW, origH, nil
}

// scaleImage 最近邻缩放
func scaleImage(src *image.RGBA, newW, newH int) *image.RGBA {
	srcW := src.Bounds().Dx()
	srcH := src.Bounds().Dy()
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := y * srcH / newH
		for x := 0; x < newW; x++ {
			srcX := x * srcW / newW
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

func sendScreenFrame(session *ScreenSession, data string, w, h int) {
	payload, _ := json.Marshal(ScreenFramePayload{
		Data:   data,
		Width:  w,
		Height: h,
		Ts:     time.Now().UnixMilli(),
	})
	msg, _ := json.Marshal(AgentMessage{
		Type:    "screen_frame",
		ID:      session.id,
		Payload: payload,
	})
	session.writeMu.Lock()
	session.conn.WriteMessage(websocket.TextMessage, msg)
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
