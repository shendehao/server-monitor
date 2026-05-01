package handler

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net/http"
	"server-monitor/internal/model"
	"server-monitor/internal/service"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func generateAgentToken() string {
	b := make([]byte, 32)
	cryptorand.Read(b)
	return hex.EncodeToString(b)
}

type ServerHandler struct {
	db        *gorm.DB
	collector *service.Collector
}

func NewServerHandler(db *gorm.DB, collector *service.Collector) *ServerHandler {
	return &ServerHandler{db: db, collector: collector}
}

type serverListItem struct {
	model.Server
	IsOnline     bool    `json:"isOnline"`
	LastReportAt *string `json:"lastReportAt,omitempty"`
	DeployId     string  `json:"deployId,omitempty"`
}

func (h *ServerHandler) List(c *gin.Context) {
	var servers []model.Server
	h.db.Order("sort_order ASC, created_at ASC").Find(&servers)

	items := make([]serverListItem, 0, len(servers))
	for _, s := range servers {
		item := serverListItem{Server: s, IsOnline: h.collector.IsOnline(s.ID)}
		if t := h.collector.GetLastReportAt(s.ID); t != nil {
			ts := t.Format(time.RFC3339)
			item.LastReportAt = &ts
		}
		item.DeployId = h.collector.GetDeployId(s.ID)
		items = append(items, item)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

func (h *ServerHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}

	detail := model.ServerDetail{
		Server:        server,
		IsOnline:      h.collector.IsOnline(id),
		LatestMetrics: h.collector.GetLatestMetric(id),
	}
	if detail.LatestMetrics != nil {
		detail.Uptime = detail.LatestMetrics.Uptime
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": detail})
}

func (h *ServerHandler) Create(c *gin.Context) {
	var form model.ServerForm
	if err := c.ShouldBindJSON(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "参数错误: " + err.Error()})
		return
	}

	port := form.Port
	if port == 0 {
		port = 22
	}
	authType := form.AuthType
	if authType == "" {
		authType = "password"
	}
	osType := form.OSType
	if osType == "" {
		osType = "linux"
	}

	connectMethod := form.ConnectMethod
	if connectMethod == "" {
		connectMethod = "ssh"
	}

	server := model.Server{
		ID:            fmt.Sprintf("s%07d", rand.Intn(10000000)),
		Name:          form.Name,
		Host:          form.Host,
		Port:          port,
		Username:      form.Username,
		AuthType:      authType,
		AuthValue:     form.AuthValue,
		ConnectMethod: connectMethod,
		AgentToken:    generateAgentToken(),
		OSType:        osType,
		Group:         form.Group,
		SortOrder:     form.SortOrder,
		IsActive:      true,
	}

	if err := h.db.Create(&server).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "创建失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": server})
}

func (h *ServerHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}

	var form model.ServerForm
	if err := c.ShouldBindJSON(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "参数错误: " + err.Error()})
		return
	}

	server.Name = form.Name
	server.Host = form.Host
	if form.Port > 0 {
		server.Port = form.Port
	}
	server.Username = form.Username
	if form.AuthType != "" {
		server.AuthType = form.AuthType
	}
	if form.AuthValue != "" {
		server.AuthValue = form.AuthValue
	}
	if form.ConnectMethod != "" {
		server.ConnectMethod = form.ConnectMethod
	}
	if form.OSType != "" {
		server.OSType = form.OSType
	}
	server.Group = form.Group
	server.SortOrder = form.SortOrder

	h.db.Save(&server)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": server})
}

func (h *ServerHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	result := h.db.Delete(&model.Server{}, "id = ?", id)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	// 同时删除相关指标和告警
	h.db.Delete(&model.Metric{}, "server_id = ?", id)
	h.db.Delete(&model.Alert{}, "server_id = ?", id)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ServerHandler) TestConnection(c *gin.Context) {
	// 测试已有服务器的连接
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": model.TestResult{
			Connected: true,
			Latency:   45,
			Message:   "连接成功（模拟）",
			ServerInfo: map[string]interface{}{
				"os":     "Ubuntu 22.04 LTS",
				"kernel": "5.15.0-91-generic",
				"arch":   "x86_64",
			},
		},
	})
}

func (h *ServerHandler) TestNewConnection(c *gin.Context) {
	// 测试新连接参数
	var form model.ServerForm
	if err := c.ShouldBindJSON(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "参数错误"})
		return
	}

	// 模拟连接测试
	time.Sleep(500 * time.Millisecond)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": model.TestResult{
			Connected: true,
			Latency:   32,
			Message:   "连接成功（模拟）",
			ServerInfo: map[string]interface{}{
				"os":     "CentOS 7.9",
				"kernel": "3.10.0-1160.el7.x86_64",
				"arch":   "x86_64",
			},
		},
	})
}
