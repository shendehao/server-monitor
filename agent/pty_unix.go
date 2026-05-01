//go:build !windows

package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

// PtySession 管理一个 PTY 终端会话
type PtySession struct {
	id      string
	cmd     *exec.Cmd
	ptmx    *os.File
	conn    *websocket.Conn
	writeMu *sync.Mutex
	done    chan struct{}
}

// PtyStartPayload PTY 启动参数
type PtyStartPayload struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// PtyInputPayload PTY 输入数据
type PtyInputPayload struct {
	Data string `json:"data"`
}

// PtyResizePayload PTY 调整大小
type PtyResizePayload struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// PtyOutputPayload PTY 输出数据
type PtyOutputPayload struct {
	Data string `json:"data"`
}

// PtyExitPayload PTY 退出
type PtyExitPayload struct {
	Code int `json:"code"`
}

// ptyManager 管理所有活跃的 PTY 会话
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

	// 启动 shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(payload.Rows),
		Cols: uint16(payload.Cols),
	})
	if err != nil {
		log.Printf("PTY 启动失败: %v", err)
		sendPtyExit(conn, writeMu, msg.ID, -1)
		return
	}

	session := &PtySession{
		id:      msg.ID,
		cmd:     cmd,
		ptmx:    ptmx,
		conn:    conn,
		writeMu: writeMu,
		done:    make(chan struct{}),
	}

	ptyManager.mu.Lock()
	ptyManager.sessions[msg.ID] = session
	ptyManager.mu.Unlock()

	log.Printf("PTY 会话已启动: id=%s, shell=%s", msg.ID, shell)

	// PTY stdout → WebSocket
	go func() {
		defer func() {
			close(session.done)
			ptmx.Close()

			exitCode := 0
			if err := cmd.Wait(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					exitCode = -1
				}
			}

			ptyManager.mu.Lock()
			delete(ptyManager.sessions, msg.ID)
			ptyManager.mu.Unlock()

			sendPtyExit(conn, writeMu, msg.ID, exitCode)
			log.Printf("PTY 会话已结束: id=%s, exitCode=%d", msg.ID, exitCode)
		}()

		buf := make([]byte, 8192)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("PTY 读取错误: %v", err)
				}
				return
			}
			if n > 0 {
				outPayload, _ := json.Marshal(PtyOutputPayload{Data: string(buf[:n])})
				outMsg, _ := json.Marshal(AgentMessage{
					Type:    c2e("pty_output"),
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

	if ok && session.ptmx != nil {
		session.ptmx.WriteString(payload.Data)
	}
}

func handlePtyResize(msg AgentMessage) {
	var payload PtyResizePayload
	json.Unmarshal(msg.Payload, &payload)

	ptyManager.mu.Lock()
	session, ok := ptyManager.sessions[msg.ID]
	ptyManager.mu.Unlock()

	if ok && session.ptmx != nil && payload.Cols > 0 && payload.Rows > 0 {
		pty.Setsize(session.ptmx, &pty.Winsize{
			Rows: uint16(payload.Rows),
			Cols: uint16(payload.Cols),
		})
	}
}

func handlePtyClose(msg AgentMessage) {
	ptyManager.mu.Lock()
	session, ok := ptyManager.sessions[msg.ID]
	ptyManager.mu.Unlock()

	if ok {
		if session.cmd != nil && session.cmd.Process != nil {
			session.cmd.Process.Kill()
		}
	}
}

func sendPtyExit(conn *websocket.Conn, writeMu *sync.Mutex, sessionID string, code int) {
	payload, _ := json.Marshal(PtyExitPayload{Code: code})
	msg, _ := json.Marshal(AgentMessage{
		Type:    c2e("pty_exit"),
		ID:      sessionID,
		Payload: payload,
	})
	writeMu.Lock()
	conn.WriteMessage(websocket.TextMessage, msg)
	writeMu.Unlock()
}

// cleanupAllPtySessions 清理所有 PTY 会话（连接断开时调用）
func cleanupAllPtySessions() {
	ptyManager.mu.Lock()
	sessions := make([]*PtySession, 0, len(ptyManager.sessions))
	for _, s := range ptyManager.sessions {
		sessions = append(sessions, s)
	}
	ptyManager.mu.Unlock()

	for _, s := range sessions {
		if s.cmd != nil && s.cmd.Process != nil {
			s.cmd.Process.Kill()
		}
	}
}
