package handler

import (
	"net/http"
	"server-monitor/internal/model"
	"server-monitor/internal/service"
	"server-monitor/internal/ws"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ExecHandler struct {
	db       *gorm.DB
	agentHub *ws.AgentHub
}

func NewExecHandler(db *gorm.DB, agentHub *ws.AgentHub) *ExecHandler {
	return &ExecHandler{db: db, agentHub: agentHub}
}

type ExecForm struct {
	Command string `json:"command" binding:"required"`
}

// Exec 向指定服务器发送命令并等待结果（支持 SSH 和 Agent 两种方式）
func (h *ExecHandler) Exec(c *gin.Context) {
	serverID := c.Param("id")

	var form ExecForm
	if err := c.ShouldBindJSON(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供命令"})
		return
	}

	// 查找服务器
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}

	switch server.ConnectMethod {
	case "ssh", "":
		h.execViaSSH(c, server, form.Command)
	case "agent", "plugin":
		h.execViaAgent(c, server, form.Command)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "不支持的连接方式: " + server.ConnectMethod})
	}
}

// execViaSSH 通过 SSH 执行命令
func (h *ExecHandler) execViaSSH(c *gin.Context, server model.Server, command string) {
	client := service.NewSSHClient(
		server.Host, server.Port,
		server.Username, server.AuthType, server.AuthValue,
		10*time.Second,
	)

	output, err := client.Run(command)
	result := gin.H{
		"exitCode": 0,
		"output":   output,
		"error":    "",
	}
	if err != nil {
		result["error"] = err.Error()
		result["exitCode"] = 1
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// execViaAgent 通过 Agent WebSocket 执行命令
func (h *ExecHandler) execViaAgent(c *gin.Context, server model.Server, command string) {
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}

	result, err := h.agentHub.ExecCommand(server.ID, command, 30*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// AgentStatus 获取远程执行是否可用
func (h *ExecHandler) AgentStatus(c *gin.Context) {
	serverID := c.Param("id")

	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"online": false}})
		return
	}

	switch server.ConnectMethod {
	case "ssh", "":
		// SSH 服务器只要有配置就认为可用（实际连接时再判断）
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"online": server.Host != "" && server.IsActive}})
	case "agent", "plugin":
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"online": h.agentHub.IsAgentOnline(serverID)}})
	default:
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"online": false}})
	}
}

// HandleAgentWS 处理 Agent WebSocket 连接（通过 token 认证）
func (h *ExecHandler) HandleAgentWS(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少 token"})
		return
	}

	var server model.Server
	if err := h.db.Where("agent_token = ? AND is_active = ?", token, true).First(&server).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的 Agent Token"})
		return
	}

	ws.HandleAgentWebSocket(h.agentHub, c.Writer, c.Request, server.ID, server.OSType)
}
