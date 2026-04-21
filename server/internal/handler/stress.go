package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"server-monitor/internal/model"
	"server-monitor/internal/service"
	"server-monitor/internal/ws"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	gorillaws "github.com/gorilla/websocket"
	"gorm.io/gorm"
)

var stressTaskSeq uint64

// StressHandler 压力测试处理器
type StressHandler struct {
	db        *gorm.DB
	agentHub  *ws.AgentHub
	collector *service.Collector
	// 当前活跃任务
	tasks   map[string]*StressTask
	tasksMu sync.RWMutex
	// 前端 WebSocket 订阅
	subscribers   map[string]*gorillaws.Conn // taskID -> wsConn
	subscribersMu sync.RWMutex
}

// StressTask 一次压测任务
type StressTask struct {
	ID        string          `json:"id"`
	URL       string          `json:"url"`
	Method    string          `json:"method"`
	ServerIDs []string        `json:"serverIds"`
	Config    json.RawMessage `json:"config"`
	StartTime time.Time       `json:"startTime"`
	Running   bool            `json:"running"`
	// 各 Agent 的最新进度
	Progress   map[string]*AgentProgress `json:"progress"`
	progressMu sync.RWMutex
	// 已完成的 Agent 数
	doneCount  int32
	totalCount int32
}

// AgentProgress 单个 Agent 的压测进度
type AgentProgress struct {
	ServerID   string  `json:"serverId"`
	ServerName string  `json:"serverName"`
	Sent       int64   `json:"sent"`
	Success    int64   `json:"success"`
	Errors     int64   `json:"errors"`
	RPS        float64 `json:"rps"`
	AvgLatency float64 `json:"avgLatency"`
	MinLatency float64 `json:"minLatency"`
	MaxLatency float64 `json:"maxLatency"`
	BytesSent  int64   `json:"bytesSent"`
	BytesRecv  int64   `json:"bytesRecv"`
	MbpsSent   float64 `json:"mbpsSent"`
	MbpsRecv   float64 `json:"mbpsRecv"`
	ActiveConn int64   `json:"activeConn"`
	Running    bool    `json:"running"`
}

// StressStartRequest 前端请求体
type StressStartRequest struct {
	URL         string            `json:"url" binding:"required"`
	Method      string            `json:"method"`
	Mode        string            `json:"mode"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	Concurrency int               `json:"concurrency"`
	Duration    int               `json:"duration"`
	TotalReqs   int               `json:"totalReqs"`
	KeepAlive   bool              `json:"keepAlive"`
	BodySize    int               `json:"bodySize"`
	ServerIDs   []string          `json:"serverIds" binding:"required"`
}

func NewStressHandler(db *gorm.DB, agentHub *ws.AgentHub, collector *service.Collector) *StressHandler {
	return &StressHandler{
		db:          db,
		agentHub:    agentHub,
		collector:   collector,
		tasks:       make(map[string]*StressTask),
		subscribers: make(map[string]*gorillaws.Conn),
	}
}

// Start 启动压力测试
func (h *StressHandler) Start(c *gin.Context) {
	var req StressStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if req.Method == "" {
		req.Method = "GET"
	}
	if req.Concurrency <= 0 {
		req.Concurrency = 10
	}
	if req.Duration <= 0 && req.TotalReqs <= 0 {
		req.Duration = 30
	}

	// 验证服务器
	var servers []model.Server
	if err := h.db.Where("id IN ?", req.ServerIDs).Find(&servers).Error; err != nil {
		c.JSON(500, gin.H{"error": "查询服务器失败"})
		return
	}

	if len(servers) == 0 {
		c.JSON(400, gin.H{"error": "未找到指定服务器"})
		return
	}

	// 检查 Agent 在线状态
	onlineServers := make([]model.Server, 0)
	for _, s := range servers {
		if h.agentHub.IsAgentOnline(s.ID) {
			onlineServers = append(onlineServers, s)
		}
	}
	if len(onlineServers) == 0 {
		c.JSON(400, gin.H{"error": "所有选中的服务器 Agent 均不在线"})
		return
	}

	taskID := fmt.Sprintf("stress-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&stressTaskSeq, 1))

	config, _ := json.Marshal(struct {
		URL         string            `json:"url"`
		Method      string            `json:"method"`
		Mode        string            `json:"mode"`
		Headers     map[string]string `json:"headers"`
		Body        string            `json:"body"`
		Concurrency int               `json:"concurrency"`
		Duration    int               `json:"duration"`
		TotalReqs   int               `json:"totalReqs"`
		KeepAlive   bool              `json:"keepAlive"`
		BodySize    int               `json:"bodySize"`
	}{
		URL:         req.URL,
		Method:      req.Method,
		Mode:        req.Mode,
		Headers:     req.Headers,
		Body:        req.Body,
		Concurrency: req.Concurrency,
		Duration:    req.Duration,
		TotalReqs:   req.TotalReqs,
		KeepAlive:   req.KeepAlive,
		BodySize:    req.BodySize,
	})

	task := &StressTask{
		ID:         taskID,
		URL:        req.URL,
		Method:     req.Method,
		ServerIDs:  make([]string, len(onlineServers)),
		Config:     config,
		StartTime:  time.Now(),
		Running:    true,
		Progress:   make(map[string]*AgentProgress),
		totalCount: int32(len(onlineServers)),
	}

	for i, s := range onlineServers {
		task.ServerIDs[i] = s.ID
		task.Progress[s.ID] = &AgentProgress{
			ServerID:   s.ID,
			ServerName: s.Name,
			Running:    true,
		}
	}

	h.tasksMu.Lock()
	h.tasks[taskID] = task
	h.tasksMu.Unlock()

	// 向每个在线 Agent 下发压测命令
	for _, s := range onlineServers {
		serverID := s.ID
		ss := &ws.StressSession{
			OnProgress: func(data json.RawMessage) {
				h.handleProgress(taskID, serverID, data)
			},
			OnDone: func(data json.RawMessage) {
				h.handleDone(taskID, serverID, data)
			},
		}
		if err := h.agentHub.StartStressTest(serverID, taskID, config, ss); err != nil {
			log.Printf("向 Agent %s 下发压测失败: %v", serverID, err)
		}
	}

	log.Printf("压力测试已启动: taskID=%s url=%s servers=%d concurrency=%d duration=%ds",
		taskID, req.URL, len(onlineServers), req.Concurrency, req.Duration)

	c.JSON(200, gin.H{
		"taskId":  taskID,
		"servers": len(onlineServers),
		"message": fmt.Sprintf("压力测试已启动，%d 台服务器参与", len(onlineServers)),
	})
}

// Stop 停止压力测试
func (h *StressHandler) Stop(c *gin.Context) {
	taskID := c.Param("id")

	h.tasksMu.RLock()
	task, ok := h.tasks[taskID]
	h.tasksMu.RUnlock()

	if !ok {
		c.JSON(404, gin.H{"error": "任务不存在"})
		return
	}

	for _, serverID := range task.ServerIDs {
		h.agentHub.StopStressTest(serverID, taskID)
	}

	task.Running = false
	c.JSON(200, gin.H{"message": "已发送停止指令"})
}

// handleProgress 处理 Agent 进度上报
func (h *StressHandler) handleProgress(taskID, serverID string, data json.RawMessage) {
	h.tasksMu.RLock()
	task, ok := h.tasks[taskID]
	h.tasksMu.RUnlock()
	if !ok {
		return
	}

	var progress AgentProgress
	json.Unmarshal(data, &progress)
	progress.ServerID = serverID
	progress.Running = true

	task.progressMu.Lock()
	if existing, ok := task.Progress[serverID]; ok {
		progress.ServerName = existing.ServerName
	}
	task.Progress[serverID] = &progress
	task.progressMu.Unlock()

	h.broadcastProgress(taskID, task)
}

// handleDone 处理 Agent 完成上报
func (h *StressHandler) handleDone(taskID, serverID string, data json.RawMessage) {
	h.tasksMu.RLock()
	task, ok := h.tasks[taskID]
	h.tasksMu.RUnlock()
	if !ok {
		return
	}

	var progress AgentProgress
	json.Unmarshal(data, &progress)
	progress.ServerID = serverID
	progress.Running = false

	task.progressMu.Lock()
	if existing, ok := task.Progress[serverID]; ok {
		progress.ServerName = existing.ServerName
	}
	task.Progress[serverID] = &progress
	task.progressMu.Unlock()

	done := atomic.AddInt32(&task.doneCount, 1)
	if done >= task.totalCount {
		task.Running = false
		// 5分钟后自动清理任务
		go func() {
			time.Sleep(5 * time.Minute)
			h.tasksMu.Lock()
			delete(h.tasks, taskID)
			h.tasksMu.Unlock()
		}()
	}

	h.broadcastProgress(taskID, task)
}

// broadcastProgress 向订阅者推送进度
func (h *StressHandler) broadcastProgress(taskID string, task *StressTask) {
	task.progressMu.RLock()
	progresses := make([]*AgentProgress, 0, len(task.Progress))
	var totalSent, totalSuccess, totalErrors, totalBytesSent, totalBytesRecv, totalActiveConn int64
	var totalRPS, totalMbpsSent, totalMbpsRecv float64
	for _, p := range task.Progress {
		progresses = append(progresses, p)
		totalSent += p.Sent
		totalSuccess += p.Success
		totalErrors += p.Errors
		totalRPS += p.RPS
		totalBytesSent += p.BytesSent
		totalBytesRecv += p.BytesRecv
		totalMbpsSent += p.MbpsSent
		totalMbpsRecv += p.MbpsRecv
		totalActiveConn += p.ActiveConn
	}
	task.progressMu.RUnlock()

	msg, _ := json.Marshal(gin.H{
		"taskId":          taskID,
		"running":         task.Running,
		"agents":          progresses,
		"totalSent":       totalSent,
		"totalSuccess":    totalSuccess,
		"totalErrors":     totalErrors,
		"totalRPS":        totalRPS,
		"totalBytesSent":  totalBytesSent,
		"totalBytesRecv":  totalBytesRecv,
		"totalMbpsSent":   totalMbpsSent,
		"totalMbpsRecv":   totalMbpsRecv,
		"totalActiveConn": totalActiveConn,
	})

	h.subscribersMu.Lock()
	for id, conn := range h.subscribers {
		conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if err := conn.WriteMessage(gorillaws.TextMessage, msg); err != nil {
			conn.Close()
			delete(h.subscribers, id)
		}
	}
	h.subscribersMu.Unlock()
}

// HandleWS 处理前端 WebSocket 订阅压测进度
func (h *StressHandler) HandleWS(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	if token == "" {
		http.Error(w, "未认证", 401)
		return
	}
	claims, err := parseToken(token)
	if err != nil || claims.Exp < time.Now().Unix() {
		http.Error(w, "令牌无效", 401)
		return
	}

	conn, err := termUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	connID := fmt.Sprintf("sub-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&stressTaskSeq, 1))

	h.subscribersMu.Lock()
	h.subscribers[connID] = conn
	h.subscribersMu.Unlock()

	defer func() {
		h.subscribersMu.Lock()
		delete(h.subscribers, connID)
		h.subscribersMu.Unlock()
	}()

	// 保持连接，读取客户端消息（主要用于 keepalive）
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// GetOnlineAgents 获取在线的 Agent 服务器列表（用于前端选择）
func (h *StressHandler) GetOnlineAgents(c *gin.Context) {
	var servers []model.Server
	h.db.Where("connect_method IN ?", []string{"agent", "plugin"}).Find(&servers)

	type AgentInfo struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Online bool   `json:"online"`
	}

	result := make([]AgentInfo, 0, len(servers))
	for _, s := range servers {
		result = append(result, AgentInfo{
			ID:     s.ID,
			Name:   s.Name,
			Online: h.collector.IsOnline(s.ID),
		})
	}

	c.JSON(200, result)
}
