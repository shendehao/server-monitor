package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"server-monitor/internal/model"
	"server-monitor/internal/ws"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	gorillaws "github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

var termSessionSeq uint64
var screenSessionSeq uint64
var micSessionSeq uint64

type TerminalHandler struct {
	db       *gorm.DB
	agentHub *ws.AgentHub
}

func NewTerminalHandler(db *gorm.DB, agentHub *ws.AgentHub) *TerminalHandler {
	return &TerminalHandler{db: db, agentHub: agentHub}
}

// TerminalResize 终端尺寸调整消息
type TerminalResize struct {
	Type string `json:"type"` // "resize"
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

var termUpgrader = gorillaws.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  8192,
	WriteBufferSize: 8192,
}

// HandleTerminal 处理交互式终端 WebSocket，根据 connectMethod 选择 SSH 或 Agent 通道
func (h *TerminalHandler) HandleTerminal(w http.ResponseWriter, r *http.Request, server model.Server) {
	wsConn, err := termUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("终端 WebSocket 升级失败: %v", err)
		return
	}
	defer wsConn.Close()

	switch server.ConnectMethod {
	case "agent", "plugin":
		h.handleAgentTerminal(wsConn, server)
	default:
		h.handleSSHTerminal(wsConn, server)
	}
}

// handleAgentTerminal 通过 Agent WebSocket 通道建立 PTY 终端
func (h *TerminalHandler) handleAgentTerminal(wsConn *gorillaws.Conn, server model.Server) {
	serverID := server.ID
	sessionID := fmt.Sprintf("term-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&termSessionSeq, 1))

	if !h.agentHub.IsAgentOnline(serverID) {
		sendTermError(wsConn, "Agent 不在线，无法连接终端")
		return
	}

	done := make(chan struct{})
	var wsMu sync.Mutex // 保护 wsConn 并发写

	// 注册终端会话回调
	ts := &ws.TermSession{
		OnOutput: func(data string) {
			wsMu.Lock()
			wsConn.WriteMessage(gorillaws.TextMessage, []byte(data))
			wsMu.Unlock()
		},
		OnExit: func(code int) {
			msg := fmt.Sprintf("\r\n\033[33m终端已退出 (code: %d)\033[0m\r\n", code)
			wsMu.Lock()
			wsConn.WriteMessage(gorillaws.TextMessage, []byte(msg))
			wsMu.Unlock()
			select {
			case <-done:
			default:
				close(done)
			}
		},
		OnMode: func(mode string) {
			modeMsg, _ := json.Marshal(map[string]string{"type": "pty_mode", "mode": mode})
			wsMu.Lock()
			wsConn.WriteMessage(gorillaws.TextMessage, modeMsg)
			wsMu.Unlock()
		},
	}

	// 启动 Agent PTY（默认 120x30，前端会马上发 resize）
	if err := h.agentHub.StartTermSession(serverID, sessionID, 120, 30, ts); err != nil {
		sendTermError(wsConn, fmt.Sprintf("启动终端失败: %v", err))
		return
	}

	log.Printf("Agent 终端会话已启动: server=%s session=%s", server.Name, sessionID)

	// 前端 WebSocket → Agent PTY
	go func() {
		defer func() {
			h.agentHub.CloseTermSession(serverID, sessionID)
			select {
			case <-done:
			default:
				close(done)
			}
		}()
		for {
			msgType, data, err := wsConn.ReadMessage()
			if err != nil {
				return
			}
			if msgType == gorillaws.TextMessage {
				if len(data) > 0 && data[0] == '{' {
					var resize TerminalResize
					if json.Unmarshal(data, &resize) == nil && resize.Type == "resize" {
						if resize.Cols > 0 && resize.Rows > 0 {
							h.agentHub.SendTermResize(serverID, sessionID, resize.Cols, resize.Rows)
						}
						continue
					}
				}
				h.agentHub.SendTermInput(serverID, sessionID, string(data))
			} else if msgType == gorillaws.BinaryMessage {
				h.agentHub.SendTermInput(serverID, sessionID, string(data))
			}
		}
	}()

	<-done
	log.Printf("Agent 终端会话已结束: server=%s session=%s", server.Name, sessionID)
}

// handleSSHTerminal 通过 SSH PTY 建立终端
func (h *TerminalHandler) handleSSHTerminal(wsConn *gorillaws.Conn, server model.Server) {
	// 建立 SSH 连接
	sshConfig := &ssh.ClientConfig{
		User:            server.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	switch server.AuthType {
	case "key":
		signer, err := ssh.ParsePrivateKey([]byte(server.AuthValue))
		if err != nil {
			sendTermError(wsConn, fmt.Sprintf("解析私钥失败: %v", err))
			return
		}
		sshConfig.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	default:
		sshConfig.Auth = []ssh.AuthMethod{ssh.Password(server.AuthValue)}
	}

	addr := fmt.Sprintf("%s:%d", server.Host, server.Port)
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		sendTermError(wsConn, fmt.Sprintf("SSH 连接失败: %v", err))
		return
	}
	defer sshClient.Close()

	session, err := sshClient.NewSession()
	if err != nil {
		sendTermError(wsConn, fmt.Sprintf("创建 SSH 会话失败: %v", err))
		return
	}
	defer session.Close()

	// 请求 PTY
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := session.RequestPty("xterm-256color", 30, 120, modes); err != nil {
		sendTermError(wsConn, fmt.Sprintf("请求 PTY 失败: %v", err))
		return
	}

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		sendTermError(wsConn, fmt.Sprintf("获取 stdin 失败: %v", err))
		return
	}

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		sendTermError(wsConn, fmt.Sprintf("获取 stdout 失败: %v", err))
		return
	}

	stderrPipe, err := session.StderrPipe()
	if err != nil {
		sendTermError(wsConn, fmt.Sprintf("获取 stderr 失败: %v", err))
		return
	}

	if err := session.Shell(); err != nil {
		sendTermError(wsConn, fmt.Sprintf("启动 Shell 失败: %v", err))
		return
	}

	// 标记连接是否关闭
	done := make(chan struct{})

	// SSH stdout → WebSocket
	go func() {
		defer func() {
			select {
			case <-done:
			default:
				close(done)
			}
		}()
		buf := make([]byte, 8192)
		for {
			n, err := stdoutPipe.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				data := sanitizeUTF8(buf[:n])
				wsConn.WriteMessage(gorillaws.TextMessage, data)
			}
		}
	}()

	// SSH stderr → WebSocket
	go func() {
		buf := make([]byte, 8192)
		for {
			n, err := stderrPipe.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				data := sanitizeUTF8(buf[:n])
				wsConn.WriteMessage(gorillaws.TextMessage, data)
			}
		}
	}()

	// WebSocket → SSH stdin（处理输入和 resize）
	go func() {
		defer func() {
			select {
			case <-done:
			default:
				close(done)
			}
		}()
		for {
			msgType, data, err := wsConn.ReadMessage()
			if err != nil {
				return
			}

			if msgType == gorillaws.TextMessage {
				// 检查是否是 resize 消息
				if len(data) > 0 && data[0] == '{' {
					var resize TerminalResize
					if json.Unmarshal(data, &resize) == nil && resize.Type == "resize" {
						if resize.Cols > 0 && resize.Rows > 0 {
							session.WindowChange(resize.Rows, resize.Cols)
						}
						continue
					}
				}
				// 普通输入
				stdinPipe.Write(data)
			} else if msgType == gorillaws.BinaryMessage {
				stdinPipe.Write(data)
			}
		}
	}()

	// 等待连接关闭
	select {
	case <-done:
	}

	log.Printf("SSH 终端会话已结束: server=%s", server.Name)
}

func sendTermError(conn *gorillaws.Conn, msg string) {
	errMsg := fmt.Sprintf("\r\n\033[31m%s\033[0m\r\n", msg)
	conn.WriteMessage(gorillaws.TextMessage, []byte(errMsg))
}

// sanitizeUTF8 确保发送给 WebSocket 的是有效 UTF-8
func sanitizeUTF8(data []byte) []byte {
	if utf8.Valid(data) {
		return data
	}
	// 替换无效字节
	var clean []byte
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && size == 1 {
			clean = append(clean, '?')
			data = data[1:]
		} else {
			clean = append(clean, data[:size]...)
			data = data[size:]
		}
	}
	return clean
}

// HandleScreen 处理桌面实时查看 WebSocket
func (h *TerminalHandler) HandleScreen(w http.ResponseWriter, r *http.Request, server model.Server) {
	wsConn, err := termUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("桌面查看 WebSocket 升级失败: %v", err)
		return
	}
	defer wsConn.Close()

	serverID := server.ID
	sessionID := fmt.Sprintf("screen-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&screenSessionSeq, 1))

	if !h.agentHub.IsAgentOnline(serverID) {
		wsConn.WriteMessage(gorillaws.TextMessage, []byte(`{"type":"error","message":"Agent 不在线"}`))
		return
	}

	done := make(chan struct{})
	var wsMu sync.Mutex

	ss := &ws.ScreenSession{
		OnFrame: func(data json.RawMessage) {
			// 包装为前端期望的格式: {"type":"screen_frame","payload":{...}}
			wrapped := make([]byte, 0, len(data)+40)
			wrapped = append(wrapped, `{"type":"screen_frame","payload":`...)
			wrapped = append(wrapped, data...)
			wrapped = append(wrapped, '}')
			wsMu.Lock()
			wsConn.WriteMessage(gorillaws.TextMessage, wrapped)
			wsMu.Unlock()
		},
		OnBinary: func(data []byte) {
			// JPEG/H264 二进制帧
			wsMu.Lock()
			wsConn.WriteMessage(gorillaws.BinaryMessage, data)
			wsMu.Unlock()
		},
		OnClose: func(message string) {
			errPayload, _ := json.Marshal(map[string]string{
				"type":    "error",
				"message": message,
			})
			wsMu.Lock()
			wsConn.WriteMessage(gorillaws.TextMessage, errPayload)
			wsMu.Unlock()
			select {
			case <-done:
			default:
				close(done)
			}
		},
	}

	// 浏览器 WebSocket keepalive（防止反代/NAT 超时断开）
	wsConn.SetReadDeadline(time.Now().Add(90 * time.Second))
	wsConn.SetPongHandler(func(string) error {
		wsConn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})
	pingTicker := time.NewTicker(30 * time.Second)
	go func() {
		defer pingTicker.Stop()
		for {
			select {
			case <-done:
				return
			case <-pingTicker.C:
				wsMu.Lock()
				err := wsConn.WriteMessage(gorillaws.PingMessage, nil)
				wsMu.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()

	// 默认参数（与前端默认值一致）
	fps, quality, scale := 10, 70, 100

	if err := h.agentHub.StartScreenSession(serverID, sessionID, fps, quality, scale, ss); err != nil {
		wsConn.WriteMessage(gorillaws.TextMessage, []byte(`{"type":"error","message":"启动截图失败"}`))
		return
	}

	// 读取浏览器消息（控制命令）
	go func() {
		defer func() {
			h.agentHub.StopScreenSession(serverID, sessionID)
			select {
			case <-done:
			default:
				close(done)
			}
		}()
		for {
			_, data, err := wsConn.ReadMessage()
			if err != nil {
				return
			}
			// 解析控制消息
			var ctrl struct {
				Type    string `json:"type"`
				FPS     int    `json:"fps"`
				Quality int    `json:"quality"`
				Scale   int    `json:"scale"`
			}
			if json.Unmarshal(data, &ctrl) == nil {
				if ctrl.Type == "config" {
					// 重启截图会话并更新参数
					h.agentHub.StopScreenSession(serverID, sessionID)
					newID := fmt.Sprintf("screen-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&screenSessionSeq, 1))
					if ctrl.FPS > 0 {
						fps = ctrl.FPS
					}
					if ctrl.Quality > 0 {
						quality = ctrl.Quality
					}
					if ctrl.Scale > 0 {
						scale = ctrl.Scale
					}
					sessionID = newID
					h.agentHub.StartScreenSession(serverID, sessionID, fps, quality, scale, ss)
				} else if ctrl.Type == "input" {
					// 转发远程桌面输入到 Agent（鼠标/键盘）
					go h.agentHub.ScreenInput(serverID, data, 5*time.Second)
				}
			}
		}
	}()

	<-done
	log.Printf("桌面查看会话已结束: server=%s session=%s", server.Name, sessionID)
}

// HandleWebcam 处理摄像头实时流 WebSocket（二进制推送）
func (h *TerminalHandler) HandleWebcam(w http.ResponseWriter, r *http.Request, server model.Server) {
	wsConn, err := termUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("摄像头 WebSocket 升级失败: %v", err)
		return
	}
	defer wsConn.Close()

	serverID := server.ID
	sessionID := fmt.Sprintf("webcam-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&screenSessionSeq, 1))
	codec := strings.ToLower(r.URL.Query().Get("codec"))
	if codec != "jpeg" {
		codec = "h264"
	}

	if !h.agentHub.IsAgentOnline(serverID) {
		wsConn.WriteMessage(gorillaws.TextMessage, []byte(`{"type":"error","message":"Agent 不在线"}`))
		return
	}

	done := make(chan struct{})
	var wsMu sync.Mutex

	wcs := &ws.WebcamSession{
		Codec: codec,
		OnFrame: func(data json.RawMessage) {
			wrapped := make([]byte, 0, len(data)+40)
			wrapped = append(wrapped, `{"type":"webcam_frame","payload":`...)
			wrapped = append(wrapped, data...)
			wrapped = append(wrapped, '}')
			wsMu.Lock()
			wsConn.WriteMessage(gorillaws.TextMessage, wrapped)
			wsMu.Unlock()
		},
		OnBinary: func(data []byte) {
			wsMu.Lock()
			wsConn.WriteMessage(gorillaws.BinaryMessage, data)
			wsMu.Unlock()
		},
		OnClose: func(message string) {
			wsMu.Lock()
			wsConn.WriteMessage(gorillaws.TextMessage, []byte(`{"type":"error","message":"`+message+`"}`))
			wsMu.Unlock()
			select {
			case <-done:
			default:
				close(done)
			}
		},
	}

	// keepalive
	wsConn.SetReadDeadline(time.Now().Add(90 * time.Second))
	wsConn.SetPongHandler(func(string) error {
		wsConn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})
	pingTicker := time.NewTicker(30 * time.Second)
	go func() {
		defer pingTicker.Stop()
		for {
			select {
			case <-done:
				return
			case <-pingTicker.C:
				wsMu.Lock()
				err := wsConn.WriteMessage(gorillaws.PingMessage, nil)
				wsMu.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()

	if err := h.agentHub.StartWebcamSession(serverID, sessionID, wcs); err != nil {
		wsConn.WriteMessage(gorillaws.TextMessage, []byte(`{"type":"error","message":"启动摄像头失败"}`))
		return
	}

	// 读取客户端消息（保持连接 + 检测断开）
	go func() {
		defer func() {
			h.agentHub.StopWebcamSession(serverID, sessionID)
			select {
			case <-done:
			default:
				close(done)
			}
		}()
		for {
			_, _, err := wsConn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	<-done
}

// HandleMic 处理麦克风实时流 WebSocket
func (h *TerminalHandler) HandleMic(w http.ResponseWriter, r *http.Request, server model.Server) {
	wsConn, err := termUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("麦克风 WebSocket 升级失败: %v", err)
		return
	}
	defer wsConn.Close()

	serverID := server.ID
	sessionID := fmt.Sprintf("mic-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&micSessionSeq, 1))

	if !h.agentHub.IsAgentOnline(serverID) {
		wsConn.WriteMessage(gorillaws.TextMessage, []byte(`{"error":"Agent 不在线"}`))
		return
	}

	done := make(chan struct{})
	var wsMu sync.Mutex

	ms := &ws.MicSession{
		OnFrame: func(data json.RawMessage) {
			wsMu.Lock()
			wsConn.WriteMessage(gorillaws.TextMessage, data)
			wsMu.Unlock()
		},
		OnClose: func(message string) {
			wsMu.Lock()
			wsConn.WriteMessage(gorillaws.TextMessage, []byte(`{"error":"`+message+`"}`))
			wsMu.Unlock()
			select {
			case <-done:
			default:
				close(done)
			}
		},
	}

	if err := h.agentHub.StartMicSession(serverID, sessionID, ms); err != nil {
		wsConn.WriteMessage(gorillaws.TextMessage, []byte(`{"error":"注册麦克风流失败"}`))
		return
	}

	// keepalive
	wsConn.SetReadDeadline(time.Now().Add(90 * time.Second))
	wsConn.SetPongHandler(func(string) error {
		wsConn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})
	pingTicker := time.NewTicker(30 * time.Second)
	go func() {
		defer pingTicker.Stop()
		for {
			select {
			case <-done:
				return
			case <-pingTicker.C:
				wsMu.Lock()
				err := wsConn.WriteMessage(gorillaws.PingMessage, nil)
				wsMu.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()

	// 读取客户端消息（保持连接活跃 + 检测断开）
	go func() {
		defer func() {
			h.agentHub.StopMicSession(serverID, sessionID)
			select {
			case <-done:
			default:
				close(done)
			}
		}()
		for {
			_, _, err := wsConn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	<-done
	log.Printf("麦克风流会话已结束: server=%s session=%s", server.Name, sessionID)
}

// ValidateAndGetServer 验证 JWT 并获取服务器信息（用于 WebSocket 端点）
func (h *TerminalHandler) ValidateAndGetServer(r *http.Request, serverID string) (*model.Server, error) {
	// 从 query 获取 token
	token := r.URL.Query().Get("token")
	if token == "" {
		// 也尝试从 header 获取
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		}
	}

	if token == "" {
		return nil, fmt.Errorf("未提供认证令牌")
	}

	claims, err := parseToken(token)
	if err != nil {
		return nil, fmt.Errorf("令牌无效")
	}

	if claims.Exp < time.Now().Unix() {
		return nil, fmt.Errorf("令牌已过期")
	}

	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		return nil, fmt.Errorf("服务器不存在")
	}

	return &server, nil
}
