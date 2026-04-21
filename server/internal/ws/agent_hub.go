package ws

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var msgSeq uint64 // 全局消息序号，原子递增

// AgentMessage Agent 和服务端之间的消息格式
type AgentMessage struct {
	Type    string          `json:"type"`          // auth, exec, exec_result, metrics, pong
	ID      string          `json:"id"`            // 消息 ID，用于匹配请求和响应
	Payload json.RawMessage `json:"payload"`       // 具体数据
	Ts      int64           `json:"ts,omitempty"`  // 签名时间戳
	Sig     string          `json:"sig,omitempty"` // HMAC-SHA256 签名
}

// ExecRequest 命令执行请求
type ExecRequest struct {
	Command string `json:"command"`
}

// ExecResult 命令执行结果
type ExecResult struct {
	ExitCode int    `json:"exitCode"`
	Output   string `json:"output"`
	Error    string `json:"error"`
}

// TermSession 终端会话回调
type TermSession struct {
	OnOutput func(data string)
	OnExit   func(code int)
}

// StressSession 压力测试会话回调
type StressSession struct {
	OnProgress func(data json.RawMessage)
	OnDone     func(data json.RawMessage)
}

// AgentConn 单个 Agent 连接
type AgentConn struct {
	ServerID string
	OSType   string // linux / windows
	conn     *websocket.Conn
	send     chan []byte
	hub      *AgentHub
	// 等待命令结果的回调
	pending   map[string]chan *ExecResult
	pendingMu sync.Mutex
	// 终端会话
	termSessions   map[string]*TermSession
	termSessionsMu sync.Mutex
	// 压测会话
	stressSessions   map[string]*StressSession
	stressSessionsMu sync.Mutex
}

// AgentHub 管理所有 Agent WebSocket 连接
type AgentHub struct {
	agents  map[string]*AgentConn // serverID -> AgentConn
	mu      sync.RWMutex
	signKey []byte // HMAC 签名密钥
}

func NewAgentHub() *AgentHub {
	return &AgentHub{
		agents: make(map[string]*AgentConn),
	}
}

// SetSignKey 设置消息签名密钥
func (h *AgentHub) SetSignKey(key []byte) {
	h.signKey = key
}

// signMsg 对消息进行 HMAC-SHA256 签名并序列化
func (h *AgentHub) signMsg(msg AgentMessage) []byte {
	if len(h.signKey) > 0 {
		msg.Ts = time.Now().Unix()
		raw := msg.Type + "|" + msg.ID + "|" + strconv.FormatInt(msg.Ts, 10) + "|" + string(msg.Payload)
		mac := hmac.New(sha256.New, h.signKey)
		mac.Write([]byte(raw))
		msg.Sig = hex.EncodeToString(mac.Sum(nil))
	}
	data, _ := json.Marshal(msg)
	return data
}

// IsAgentOnline 检查 Agent 是否在线
func (h *AgentHub) IsAgentOnline(serverID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.agents[serverID]
	return ok
}

// ExecCommand 向指定 Agent 发送命令并等待结果
func (h *AgentHub) ExecCommand(serverID, command string, timeout time.Duration) (*ExecResult, error) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return nil, &AgentOfflineError{ServerID: serverID}
	}

	// 生成唯一消息 ID（原子递增，不会重复）
	msgID := fmt.Sprintf("%d-%d", time.Now().UnixNano(), atomic.AddUint64(&msgSeq, 1))

	// 创建结果通道
	resultCh := make(chan *ExecResult, 1)
	agent.pendingMu.Lock()
	agent.pending[msgID] = resultCh
	agent.pendingMu.Unlock()

	defer func() {
		agent.pendingMu.Lock()
		delete(agent.pending, msgID)
		agent.pendingMu.Unlock()
	}()

	// 发送执行请求（带签名）
	payload, _ := json.Marshal(ExecRequest{Command: command})
	msg := h.signMsg(AgentMessage{
		Type:    "exec",
		ID:      msgID,
		Payload: payload,
	})

	select {
	case agent.send <- msg:
	default:
		return nil, &AgentOfflineError{ServerID: serverID}
	}

	// 等待结果
	select {
	case result := <-resultCh:
		return result, nil
	case <-time.After(timeout):
		return &ExecResult{ExitCode: -1, Error: "命令执行超时"}, nil
	}
}

// StartTermSession 在 Agent 上启动一个 PTY 终端会话
func (h *AgentHub) StartTermSession(serverID, sessionID string, cols, rows int, ts *TermSession) error {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return &AgentOfflineError{ServerID: serverID}
	}

	// 注册回调
	agent.termSessionsMu.Lock()
	agent.termSessions[sessionID] = ts
	agent.termSessionsMu.Unlock()

	// 发送 pty_start
	payload, _ := json.Marshal(struct {
		Cols int `json:"cols"`
		Rows int `json:"rows"`
	}{Cols: cols, Rows: rows})
	msg := h.signMsg(AgentMessage{Type: "pty_start", ID: sessionID, Payload: payload})

	select {
	case agent.send <- msg:
		return nil
	default:
		agent.termSessionsMu.Lock()
		delete(agent.termSessions, sessionID)
		agent.termSessionsMu.Unlock()
		return &AgentOfflineError{ServerID: serverID}
	}
}

// SendTermInput 向 Agent PTY 发送输入
func (h *AgentHub) SendTermInput(serverID, sessionID, data string) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	payload, _ := json.Marshal(struct {
		Data string `json:"data"`
	}{Data: data})
	msg := h.signMsg(AgentMessage{Type: "pty_input", ID: sessionID, Payload: payload})

	select {
	case agent.send <- msg:
	default:
	}
}

// SendTermResize 向 Agent PTY 发送 resize
func (h *AgentHub) SendTermResize(serverID, sessionID string, cols, rows int) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	payload, _ := json.Marshal(struct {
		Cols int `json:"cols"`
		Rows int `json:"rows"`
	}{Cols: cols, Rows: rows})
	msg := h.signMsg(AgentMessage{Type: "pty_resize", ID: sessionID, Payload: payload})

	select {
	case agent.send <- msg:
	default:
	}
}

// CloseTermSession 关闭 Agent PTY 终端会话
func (h *AgentHub) CloseTermSession(serverID, sessionID string) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	agent.termSessionsMu.Lock()
	delete(agent.termSessions, sessionID)
	agent.termSessionsMu.Unlock()

	msg := h.signMsg(AgentMessage{Type: "pty_close", ID: sessionID})
	select {
	case agent.send <- msg:
	default:
	}
}

// StartStressTest 在 Agent 上启动压力测试
func (h *AgentHub) StartStressTest(serverID, taskID string, config json.RawMessage, ss *StressSession) error {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return &AgentOfflineError{ServerID: serverID}
	}

	agent.stressSessionsMu.Lock()
	agent.stressSessions[taskID] = ss
	agent.stressSessionsMu.Unlock()

	msg := h.signMsg(AgentMessage{Type: "stress_start", ID: taskID, Payload: config})
	select {
	case agent.send <- msg:
		return nil
	default:
		agent.stressSessionsMu.Lock()
		delete(agent.stressSessions, taskID)
		agent.stressSessionsMu.Unlock()
		return &AgentOfflineError{ServerID: serverID}
	}
}

// StopStressTest 停止 Agent 上的压力测试
func (h *AgentHub) StopStressTest(serverID, taskID string) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	agent.stressSessionsMu.Lock()
	delete(agent.stressSessions, taskID)
	agent.stressSessionsMu.Unlock()

	msg := h.signMsg(AgentMessage{Type: "stress_stop", ID: taskID})
	select {
	case agent.send <- msg:
	default:
	}
}

// BroadcastToAgents 向指定（或全部）Agent 发送消息，返回成功发送数量
// platform 为空时发给所有平台，否则只发给匹配平台的 Agent
func (h *AgentHub) BroadcastToAgents(agentMsg AgentMessage, serverIDs []string, platform string) int {
	data := h.signMsg(agentMsg)
	h.mu.RLock()
	defer h.mu.RUnlock()

	sent := 0
	failed := 0
	if len(serverIDs) == 0 {
		// 发给所有在线 Agent（按平台过滤）
		for sid, agent := range h.agents {
			if platform != "" && agent.OSType != platform {
				continue
			}
			select {
			case agent.send <- data:
				sent++
			case <-time.After(2 * time.Second):
				failed++
				log.Printf("[BroadcastToAgents] 发送超时: server=%s", sid)
			}
		}
	} else {
		for _, id := range serverIDs {
			if agent, ok := h.agents[id]; ok {
				if platform != "" && agent.OSType != platform {
					continue
				}
				select {
				case agent.send <- data:
					sent++
				case <-time.After(2 * time.Second):
					failed++
					log.Printf("[BroadcastToAgents] 发送超时: server=%s", id)
				}
			}
		}
	}
	if failed > 0 {
		log.Printf("[BroadcastToAgents] 完成: sent=%d failed=%d", sent, failed)
	}
	return sent
}

type AgentOfflineError struct {
	ServerID string
}

func (e *AgentOfflineError) Error() string {
	return "Agent 不在线: " + e.ServerID
}

// HandleAgentWebSocket 处理 Agent 的 WebSocket 连接
func HandleAgentWebSocket(hub *AgentHub, w http.ResponseWriter, r *http.Request, serverID string, osType string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Agent WebSocket 升级失败: %v", err)
		return
	}

	agent := &AgentConn{
		ServerID:       serverID,
		OSType:         osType,
		conn:           conn,
		send:           make(chan []byte, 64),
		hub:            hub,
		pending:        make(map[string]chan *ExecResult),
		termSessions:   make(map[string]*TermSession),
		stressSessions: make(map[string]*StressSession),
	}

	// 注册（替换旧连接时清理 pending channels）
	hub.mu.Lock()
	if old, ok := hub.agents[serverID]; ok {
		// 通知所有等待中的 ExecCommand 调用
		old.pendingMu.Lock()
		for id, ch := range old.pending {
			ch <- &ExecResult{ExitCode: -1, Error: "Agent 重连，旧连接已断开"}
			delete(old.pending, id)
		}
		old.pendingMu.Unlock()
		close(old.send)
		old.conn.Close()
	}
	hub.agents[serverID] = agent
	count := len(hub.agents)
	hub.mu.Unlock()

	log.Printf("Agent 已连接: serverID=%s, 当前在线: %d", serverID, count)

	go agent.writePump()
	agent.readPump() // 阻塞直到断开

	// 断开清理
	hub.mu.Lock()
	if hub.agents[serverID] == agent {
		delete(hub.agents, serverID)
	}
	count = len(hub.agents)
	hub.mu.Unlock()

	// 通知所有等待中的 ExecCommand
	agent.pendingMu.Lock()
	for id, ch := range agent.pending {
		ch <- &ExecResult{ExitCode: -1, Error: "Agent 连接已断开"}
		delete(agent.pending, id)
	}
	agent.pendingMu.Unlock()

	log.Printf("Agent 已断开: serverID=%s, 当前在线: %d", serverID, count)
}

func (a *AgentConn) readPump() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Agent readPump panic: %v", r)
		}
		a.conn.Close()
	}()

	a.conn.SetReadLimit(1024 * 1024) // 1MB
	a.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	a.conn.SetPongHandler(func(string) error {
		a.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	for {
		_, data, err := a.conn.ReadMessage()
		if err != nil {
			break
		}

		var msg AgentMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "exec_result":
			var result ExecResult
			json.Unmarshal(msg.Payload, &result)
			a.pendingMu.Lock()
			if ch, ok := a.pending[msg.ID]; ok {
				ch <- &result
			}
			a.pendingMu.Unlock()

		case "pty_output":
			var out struct {
				Data string `json:"data"`
			}
			json.Unmarshal(msg.Payload, &out)
			a.termSessionsMu.Lock()
			if ts, ok := a.termSessions[msg.ID]; ok {
				ts.OnOutput(out.Data)
			}
			a.termSessionsMu.Unlock()

		case "pty_exit":
			var ex struct {
				Code int `json:"code"`
			}
			json.Unmarshal(msg.Payload, &ex)
			a.termSessionsMu.Lock()
			if ts, ok := a.termSessions[msg.ID]; ok {
				ts.OnExit(ex.Code)
				delete(a.termSessions, msg.ID)
			}
			a.termSessionsMu.Unlock()

		case "stress_progress":
			a.stressSessionsMu.Lock()
			if ss, ok := a.stressSessions[msg.ID]; ok {
				ss.OnProgress(msg.Payload)
			}
			a.stressSessionsMu.Unlock()

		case "stress_done":
			a.stressSessionsMu.Lock()
			if ss, ok := a.stressSessions[msg.ID]; ok {
				ss.OnDone(msg.Payload)
				delete(a.stressSessions, msg.ID)
			}
			a.stressSessionsMu.Unlock()

		case "pong":
			// keepalive 响应，重置读超时
			a.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		}
	}
}

func (a *AgentConn) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		a.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-a.send:
			a.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				a.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := a.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			a.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := a.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
