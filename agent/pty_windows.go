//go:build windows

package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

// Windows 终端：ConPTY 优先，不可用时自动降级为管道模式

type PtySession struct {
	id      string
	cpty    *hiddenConPTY // ConPTY 模式
	cmd     *exec.Cmd     // 管道模式
	stdin   io.WriteCloser
	conn    *websocket.Conn
	writeMu *sync.Mutex
	done    chan struct{}
	isPipe  bool // true = 管道降级模式
	pipeBuf []rune
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

	// Session 0（服务会话）下 ConPTY 虽不报错但无法产生输出，直接使用管道模式
	if getProcessSessionId() == 0 {
		log.Printf("Session 0 环境，跳过 ConPTY，使用管道模式: id=%s", msg.ID)
		startPipeMode(conn, writeMu, msg)
		return
	}

	// 优先使用隐藏 ConPTY（原生 CreateProcess，无窗口闪烁，有真实 PTY 提示符）
	if safeStartConPTYMode(conn, writeMu, msg, payload) {
		return
	}
	// ConPTY 不可用时降级到管道模式
	log.Printf("ConPTY 不可用，降级到管道模式: id=%s", msg.ID)
	startPipeMode(conn, writeMu, msg)
}

// readConPTY 读取 ConPTY 输出并转发到 WebSocket
func readConPTY(session *PtySession, cpty *hiddenConPTY, conn *websocket.Conn, writeMu *sync.Mutex, sessionID string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("ConPTY 会话 panic: id=%s err=%v", sessionID, r)
		}
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

func safeStartConPTYMode(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage, payload PtyStartPayload) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("隐藏 ConPTY 启动 panic: id=%s err=%v", msg.ID, r)
			ok = false
		}
	}()
	return startConPTYMode(conn, writeMu, msg, payload)
}

func startConPTYMode(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage, payload PtyStartPayload) bool {
	commandLine := "powershell.exe -NoLogo -NoProfile"
	cpty, err := startHiddenConPTY(commandLine, payload.Cols, payload.Rows, "", os.Environ())
	if err != nil {
		log.Printf("隐藏 ConPTY 启动失败: %v", err)
		return false
	}

	session := &PtySession{
		id:      msg.ID,
		cpty:    cpty,
		conn:    conn,
		writeMu: writeMu,
		done:    make(chan struct{}),
		isPipe:  false,
	}

	ptyManager.mu.Lock()
	ptyManager.sessions[msg.ID] = session
	ptyManager.mu.Unlock()

	log.Printf("隐藏 ConPTY 会话已启动: id=%s", msg.ID)
	sendPtyStarted(conn, writeMu, msg.ID, "conpty")
	go readConPTY(session, cpty, conn, writeMu, msg.ID)
	return true
}

// startPipeMode 管道降级模式
func startPipeMode(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NoExit")
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}

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

	// 管道模式下 PowerShell 不显示提示符，注入自定义 prompt 函数
	go func() {
		time.Sleep(500 * time.Millisecond)
		if session.stdin != nil {
			// 定义 prompt 函数（用 Write-Host 保证非交互模式也输出）+ cls 清掉初始化命令 + 显示首个提示符
			init := "function prompt { Write-Host -NoNewline ('PS ' + $pwd.Path + '> '); return '' }\r\ncls\r\nprompt\r\n"
			session.stdin.Write([]byte(init))
		}
	}()

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
			handlePipeInput(session, payload.Data)
		}
	} else if session.cpty != nil {
		session.cpty.Write([]byte(payload.Data))
	}
}

func handlePipeInput(session *PtySession, data string) {
	if data == "" || session.stdin == nil {
		return
	}
	if strings.HasPrefix(data, "\x1b") {
		return
	}
	data = strings.ReplaceAll(data, "\r\n", "\n")
	data = strings.ReplaceAll(data, "\r", "\n")
	for _, ch := range data {
		switch ch {
		case '\n':
			line := string(session.pipeBuf)
			session.pipeBuf = session.pipeBuf[:0]
			session.stdin.Write([]byte(line + "\r\n"))
			s := session.stdin
			go func() {
				time.Sleep(800 * time.Millisecond)
				s.Write([]byte("prompt\r\n"))
			}()
		case '\x7f', '\x08':
			if len(session.pipeBuf) > 0 {
				session.pipeBuf = session.pipeBuf[:len(session.pipeBuf)-1]
			}
		case '\x03':
			session.pipeBuf = session.pipeBuf[:0]
		default:
			if ch >= ' ' {
				session.pipeBuf = append(session.pipeBuf, ch)
			}
		}
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
		Type:    c2e("pty_started"),
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
		Type:    c2e("pty_output"),
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
		Type:    c2e("pty_exit"),
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
