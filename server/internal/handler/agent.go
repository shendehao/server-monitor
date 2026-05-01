package handler

import (
	"fmt"
	"math/rand"
	"net/http"
	"server-monitor/internal/model"
	"server-monitor/internal/service"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Agent 注册频率限制：每个 IP 每 10 分钟最多 5 次
var (
	registerLimitMap   = make(map[string]*registerLimit)
	registerLimitMapMu sync.Mutex
)

type registerLimit struct {
	Count  int
	Window time.Time
}

const (
	registerMaxPerIP  = 5
	registerWindowDur = 10 * time.Minute
)

type AgentHandler struct {
	db        *gorm.DB
	collector *service.Collector
}

func NewAgentHandler(db *gorm.DB, collector *service.Collector) *AgentHandler {
	return &AgentHandler{db: db, collector: collector}
}

// AgentReport 是 Agent 上报的指标结构
type AgentReport struct {
	Token        string  `json:"token" binding:"required"`
	Version      string  `json:"version"`
	CPUUsage     float64 `json:"cpuUsage"`
	MemTotal     int64   `json:"memTotal"`
	MemUsed      int64   `json:"memUsed"`
	MemUsage     float64 `json:"memUsage"`
	DiskTotal    int64   `json:"diskTotal"`
	DiskUsed     int64   `json:"diskUsed"`
	DiskUsage    float64 `json:"diskUsage"`
	NetIn        int64   `json:"netIn"`
	NetOut       int64   `json:"netOut"`
	Load1m       float64 `json:"load1m"`
	Load5m       float64 `json:"load5m"`
	Load15m      float64 `json:"load15m"`
	ProcessCount int     `json:"processCount"`
	Uptime       string  `json:"uptime"`
	DeployId     string  `json:"deployId"`
}

// Register 自动注册 Agent（无需预先创建服务器）
func (h *AgentHandler) Register(c *gin.Context) {
	// IP 频率限制
	clientIP := c.ClientIP()
	registerLimitMapMu.Lock()
	rl, exists := registerLimitMap[clientIP]
	now := time.Now()
	if !exists || now.Sub(rl.Window) > registerWindowDur {
		registerLimitMap[clientIP] = &registerLimit{Count: 1, Window: now}
		registerLimitMapMu.Unlock()
	} else {
		rl.Count++
		if rl.Count > registerMaxPerIP {
			registerLimitMapMu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{"success": false, "error": "注册请求过于频繁"})
			return
		}
		registerLimitMapMu.Unlock()
	}

	var req struct {
		Hostname string `json:"hostname"`
		OS       string `json:"os"` // linux / windows
		IP       string `json:"ip"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "参数错误"})
		return
	}

	if req.Hostname == "" {
		req.Hostname = "未命名主机"
	}
	if req.OS == "" {
		req.OS = "linux"
	}
	if req.IP == "" {
		req.IP = c.ClientIP()
	}

	// 检查是否已存在同主机名/系统的 agent 服务器
	var existing model.Server
	if err := h.db.Where("name = ? AND os_type = ? AND connect_method = ?", req.Hostname, req.OS, "agent").First(&existing).Error; err == nil {
		updates := map[string]interface{}{}
		if existing.Host != req.IP {
			updates["host"] = req.IP
		}
		if !existing.IsActive {
			updates["is_active"] = true
		}
		if len(updates) > 0 {
			h.db.Model(&existing).Updates(updates)
		}
		// 已存在，返回已有 token
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"token":    existing.AgentToken,
				"serverId": existing.ID,
				"message":  "已注册",
			},
		})
		return
	}

	// 创建新服务器
	token := generateAgentToken()
	server := model.Server{
		ID:            fmt.Sprintf("s%07d", rand.Intn(10000000)),
		Name:          req.Hostname,
		Host:          req.IP,
		Port:          0,
		Username:      "agent",
		ConnectMethod: "agent",
		AgentToken:    token,
		OSType:        req.OS,
		IsActive:      true,
	}

	if err := h.db.Create(&server).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "注册失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"token":    token,
			"serverId": server.ID,
			"message":  "注册成功",
		},
	})
}

// Report 接收 Agent 上报的指标数据
func (h *AgentHandler) Report(c *gin.Context) {
	var report AgentReport
	if err := c.ShouldBindJSON(&report); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "参数错误"})
		return
	}

	// 通过 token 查找服务器
	var server model.Server
	if err := h.db.Where("agent_token = ? AND is_active = ?", report.Token, true).First(&server).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "无效的 Agent Token"})
		return
	}

	// 构建指标
	metric := &model.Metric{
		ServerID:     server.ID,
		CPUUsage:     report.CPUUsage,
		MemTotal:     report.MemTotal,
		MemUsed:      report.MemUsed,
		MemUsage:     report.MemUsage,
		DiskTotal:    report.DiskTotal,
		DiskUsed:     report.DiskUsed,
		DiskUsage:    report.DiskUsage,
		NetIn:        report.NetIn,
		NetOut:       report.NetOut,
		Load1m:       report.Load1m,
		Load5m:       report.Load5m,
		Load15m:      report.Load15m,
		ProcessCount: report.ProcessCount,
		Uptime:       report.Uptime,
		CollectedAt:  time.Now(),
	}

	// 写入 collector 缓存和数据库
	h.collector.IngestAgentMetric(server, metric)

	// 存储 Agent 版本号
	if report.Version != "" {
		h.collector.SetAgentVersion(server.ID, report.Version)
	}

	// 存储 deployId（用于前端判定本次部署是否成功）
	if report.DeployId != "" {
		h.collector.SetDeployId(server.ID, report.DeployId)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
