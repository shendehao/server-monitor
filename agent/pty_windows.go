//go:build windows

package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/UserExistsError/conpty"
	"github.com/gorilla/websocket"
)

// Windows 终端：ConPTY 优先，不可用时自动降级为管道模式

type PtySession struct {
	id      string
	cpty    *conpty.ConPty // ConPTY 模式
	cmd     *exec.Cmd      // 管道模式
	stdin   io.WriteCloser
	conn    *websocket.Conn
	writeMu *sync.Mutex
	done    chan struct{}
	isPipe  bool // true = 管道降级模式
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

	// 优先尝试 ConPTY
	if conpty.IsConPtyAvailable() {
		cpty, err := conpty.Start("powershell.exe -NoLogo -NoProfile", conpty.ConPtyDimensions(payload.Cols, payload.Rows))
		if err == nil {
			session := &PtySession{
				id: msg.ID, cpty: cpty, conn: conn, writeMu: writeMu,
				done: make(chan struct{}), isPipe: false,
			}
			ptyManager.mu.Lock()
			ptyManager.sessions[msg.ID] = session
			ptyManager.mu.Unlock()
			log.Printf("ConPTY 会话已启动: id=%s", msg.ID)
			sendPtyStarted(conn, writeMu, msg.ID, "conpty")
			go readConPTY(session, cpty, conn, writeMu, msg.ID)
			return
		}
		log.Printf("ConPTY 启动失败，降级管道模式: %v", err)
	}

	// 降级：管道模式
	startPipeMode(conn, writeMu, msg)
}

// readConPTY 读取 ConPTY 输出并转发到 WebSocket
func readConPTY(session *PtySession, cpty *conpty.ConPty, conn *websocket.Conn, writeMu *sync.Mutex, sessionID string) {
	defer func() {
		close(session.done)
		code, _ := cpty.Wait(context.Background())
		exitCode := int(code)
		cpty.Close()

		ptyManager.mu.Lock()
		delete(ptyManager.sessions, sessionID)
		ptyManager.mu.Unlock()

		sendPtyExit(conn, writeMu, sessionID, exitCode)
		log.Printf("ConPTY 会话已结束: id=%s, exitCode=%d", sessionID, exitCode)
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
			sendPtyOutput(conn, writeMu, sessionID, buf[:n])
		}
	}
}

// startPipeMode 管道降级模式
func startPipeMode(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NoExit")
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("管道 stdin 失败: %v", err)
		sendPtyExit(conn, writeMu, msg.ID, -1)
		return
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("管道 stdout 失败: %v", err)
		sendPtyExit(conn, writeMu, msg.ID, -1)
		return
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		log.Printf("管道模式启动失败: %v", err)
		sendPtyExit(conn, writeMu, msg.ID, -1)
		return
	}

	session := &PtySession{
		id: msg.ID, cmd: cmd, stdin: stdin, conn: conn, writeMu: writeMu,
		done: make(chan struct{}), isPipe: true,
	}

	ptyManager.mu.Lock()
	ptyManager.sessions[msg.ID] = session
	ptyManager.mu.Unlock()

	log.Printf("管道模式会话已启动: id=%s", msg.ID)
	sendPtyStarted(conn, writeMu, msg.ID, "pipe")

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
			log.Printf("管道模式会话已结束: id=%s, exitCode=%d", msg.ID, exitCode)
		}()

		buf := make([]byte, 8192)
		for {
			n, err := stdout.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("管道读取错误: %v", err)
				}
				return
			}
			if n > 0 {
				sendPtyOutput(conn, writeMu, msg.ID, buf[:n])
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

	if !ok {
		return
	}
	if session.isPipe {
		if session.stdin != nil {
			session.stdin.Write([]byte(payload.Data))
		}
	} else if session.cpty != nil {
		session.cpty.Write([]byte(payload.Data))
	}
}

func handlePtyResize(msg AgentMessage) {
	var payload PtyResizePayload
	json.Unmarshal(msg.Payload, &payload)

	ptyManager.mu.Lock()
	session, ok := ptyManager.sessions[msg.ID]
	ptyManager.mu.Unlock()

	// 管道模式不支持 resize，仅 ConPTY 支持
	if ok && !session.isPipe && session.cpty != nil && payload.Cols > 0 && payload.Rows > 0 {
		session.cpty.Resize(payload.Cols, payload.Rows)
	}
}

func handlePtyClose(msg AgentMessage) {
	ptyManager.mu.Lock()
	session, ok := ptyManager.sessions[msg.ID]
	ptyManager.mu.Unlock()

	if !ok {
		return
	}
	if session.isPipe {
		if session.stdin != nil {
			session.stdin.Close()
		}
		if session.cmd != nil && session.cmd.Process != nil {
			session.cmd.Process.Kill()
		}
	} else if session.cpty != nil {
		session.cpty.Close()
	}
}

// sendPtyStarted 通知服务端 PTY 模式（conpty 或 pipe）
func sendPtyStarted(conn *websocket.Conn, writeMu *sync.Mutex, sessionID string, mode string) {
	payload, _ := json.Marshal(struct {
		Mode string `json:"mode"`
	}{Mode: mode})
	msg, _ := json.Marshal(AgentMessage{
		Type:    "pty_started",
		ID:      sessionID,
		Payload: payload,
	})
	writeMu.Lock()
	conn.WriteMessage(websocket.TextMessage, msg)
	writeMu.Unlock()
}

// pipeEcho 管道模式下将用户输入转为回显字符串
func pipeEcho(data string) string {
	var out []byte
	for i := 0; i < len(data); i++ {
		ch := data[i]
		switch {
		case ch == '\r' || ch == '\n':
			out = append(out, '\r', '\n')
		case ch == '\x7f' || ch == '\x08': // backspace / delete
			out = append(out, '\b', ' ', '\b')
		case ch == '\x03': // Ctrl+C
			out = append(out, '^', 'C', '\r', '\n')
		case ch >= 32 && ch < 127: // 可打印 ASCII
			out = append(out, ch)
		case ch >= 0xC0: // UTF-8 多字节起始，直接透传整个字符
			out = append(out, ch)
		case ch >= 0x80 && ch <= 0xBF: // UTF-8 续字节
			out = append(out, ch)
		}
		// 其他控制字符不回显
	}
	return string(out)
}

func sendPtyOutput(conn *websocket.Conn, writeMu *sync.Mutex, sessionID string, data []byte) {
	outPayload, _ := json.Marshal(PtyOutputPayload{Data: string(data)})
	outMsg, _ := json.Marshal(AgentMessage{
		Type:    "pty_output",
		ID:      sessionID,
		Payload: outPayload,
	})
	writeMu.Lock()
	conn.WriteMessage(websocket.TextMessage, outMsg)
	writeMu.Unlock()
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
		if s.isPipe {
			if s.stdin != nil {
				s.stdin.Close()
			}
			if s.cmd != nil && s.cmd.Process != nil {
				s.cmd.Process.Kill()
			}
		} else if s.cpty != nil {
			s.cpty.Close()
		}
	}
}
