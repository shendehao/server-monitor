//go:build windows

package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/gorilla/websocket"
)

// Windows 使用管道式终端（PowerShell）

type PtySession struct {
	id      string
	cmd     *exec.Cmd
	stdin   io.WriteCloser
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

	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NoExit")
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("PTY stdin 管道失败: %v", err)
		sendPtyExit(conn, writeMu, msg.ID, -1)
		return
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("PTY stdout 管道失败: %v", err)
		sendPtyExit(conn, writeMu, msg.ID, -1)
		return
	}

	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		log.Printf("PTY 启动失败: %v", err)
		sendPtyExit(conn, writeMu, msg.ID, -1)
		return
	}

	session := &PtySession{
		id:      msg.ID,
		cmd:     cmd,
		stdin:   stdin,
		conn:    conn,
		writeMu: writeMu,
		done:    make(chan struct{}),
	}

	ptyManager.mu.Lock()
	ptyManager.sessions[msg.ID] = session
	ptyManager.mu.Unlock()

	log.Printf("PTY 会话已启动: id=%s, shell=powershell.exe", msg.ID)

	// stdout → WebSocket
	go func() {
		defer func() {
			close(session.done)

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
			n, err := stdout.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("PTY 读取错误: %v", err)
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

	if ok && session.stdin != nil {
		session.stdin.Write([]byte(payload.Data))
	}
}

func handlePtyResize(msg AgentMessage) {
	// Windows 管道模式不支持 resize，忽略
}

func handlePtyClose(msg AgentMessage) {
	ptyManager.mu.Lock()
	session, ok := ptyManager.sessions[msg.ID]
	ptyManager.mu.Unlock()

	if ok {
		if session.stdin != nil {
			session.stdin.Close()
		}
		if session.cmd != nil && session.cmd.Process != nil {
			session.cmd.Process.Kill()
		}
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
		if s.stdin != nil {
			s.stdin.Close()
		}
		if s.cmd != nil && s.cmd.Process != nil {
			s.cmd.Process.Kill()
		}
	}
}
