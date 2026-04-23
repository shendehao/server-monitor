//go:build windows

package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"sync"

	"github.com/UserExistsError/conpty"
	"github.com/gorilla/websocket"
)

// Windows 使用 ConPTY（真正的伪终端），支持退格/删除/箭头键/Tab补全/resize

type PtySession struct {
	id      string
	cpty    *conpty.ConPty
	conn    *websocket.Conn
	writeMu *sync.Mutex
	done    chan struct{}
}

type PtyStartPayload struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

type PtyInputPayload struct {
	Data string `json:"data"`
}

type PtyResizePayload struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

type PtyOutputPayload struct {
	Data string `json:"data"`
}

type PtyExitPayload struct {
	Code int `json:"code"`
}

var ptyManager = struct {
	sessions map[string]*PtySession
	mu       sync.Mutex
}{
	sessions: make(map[string]*PtySession),
}

func handlePtyStart(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	var payload PtyStartPayload
	json.Unmarshal(msg.Payload, &payload)

	if payload.Cols <= 0 {
		payload.Cols = 120
	}
	if payload.Rows <= 0 {
		payload.Rows = 30
	}

	// 使用 ConPTY 启动 PowerShell
	cpty, err := conpty.Start("powershell.exe -NoLogo -NoProfile", conpty.ConPtyDimensions(payload.Cols, payload.Rows))
	if err != nil {
		log.Printf("ConPTY 启动失败: %v", err)
		sendPtyExit(conn, writeMu, msg.ID, -1)
		return
	}

	session := &PtySession{
		id:      msg.ID,
		cpty:    cpty,
		conn:    conn,
		writeMu: writeMu,
		done:    make(chan struct{}),
	}

	ptyManager.mu.Lock()
	ptyManager.sessions[msg.ID] = session
	ptyManager.mu.Unlock()

	log.Printf("ConPTY 会话已启动: id=%s", msg.ID)

	// ConPTY output → WebSocket
	go func() {
		defer func() {
			close(session.done)

			code, _ := cpty.Wait(context.Background())
			exitCode := int(code)
			cpty.Close()

			ptyManager.mu.Lock()
			delete(ptyManager.sessions, msg.ID)
			ptyManager.mu.Unlock()

			sendPtyExit(conn, writeMu, msg.ID, exitCode)
			log.Printf("ConPTY 会话已结束: id=%s, exitCode=%d", msg.ID, exitCode)
		}()

		buf := make([]byte, 8192)
		for {
			n, err := cpty.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("ConPTY 读取错误: %v", err)
				}
				return
			}
			if n > 0 {
				outPayload, _ := json.Marshal(PtyOutputPayload{Data: string(buf[:n])})
				outMsg, _ := json.Marshal(AgentMessage{
					Type:    "pty_output",
					ID:      msg.ID,
					Payload: outPayload,
				})
				writeMu.Lock()
				conn.WriteMessage(websocket.TextMessage, outMsg)
				writeMu.Unlock()
			}
		}
	}()
}

func handlePtyInput(msg AgentMessage) {
	var payload PtyInputPayload
	json.Unmarshal(msg.Payload, &payload)

	ptyManager.mu.Lock()
	session, ok := ptyManager.sessions[msg.ID]
	ptyManager.mu.Unlock()

	if ok && session.cpty != nil {
		session.cpty.Write([]byte(payload.Data))
	}
}

func handlePtyResize(msg AgentMessage) {
	var payload PtyResizePayload
	json.Unmarshal(msg.Payload, &payload)

	ptyManager.mu.Lock()
	session, ok := ptyManager.sessions[msg.ID]
	ptyManager.mu.Unlock()

	if ok && session.cpty != nil && payload.Cols > 0 && payload.Rows > 0 {
		session.cpty.Resize(payload.Cols, payload.Rows)
	}
}

func handlePtyClose(msg AgentMessage) {
	ptyManager.mu.Lock()
	session, ok := ptyManager.sessions[msg.ID]
	ptyManager.mu.Unlock()

	if ok && session.cpty != nil {
		session.cpty.Close()
	}
}

func sendPtyExit(conn *websocket.Conn, writeMu *sync.Mutex, sessionID string, code int) {
	payload, _ := json.Marshal(PtyExitPayload{Code: code})
	msg, _ := json.Marshal(AgentMessage{
		Type:    "pty_exit",
		ID:      sessionID,
		Payload: payload,
	})
	writeMu.Lock()
	conn.WriteMessage(websocket.TextMessage, msg)
	writeMu.Unlock()
}

func cleanupAllPtySessions() {
	ptyManager.mu.Lock()
	sessions := make([]*PtySession, 0, len(ptyManager.sessions))
	for _, s := range ptyManager.sessions {
		sessions = append(sessions, s)
	}
	ptyManager.mu.Unlock()

	for _, s := range sessions {
		if s.cpty != nil {
			s.cpty.Close()
		}
	}
}
