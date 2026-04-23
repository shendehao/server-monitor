package handler

import (
	"server-monitor/internal/model"
	"server-monitor/internal/service"
	"server-monitor/internal/ws"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.Engine, db *gorm.DB, hub *ws.Hub, agentHub *ws.AgentHub, collector *service.Collector) {
	// 初始化安全模块
	InitAuth(db)

	// 全局安全中间件
	r.Use(IPBlacklistMiddleware())
	r.Use(RateLimitMiddleware())

	// 公开接口
	r.POST("/api/login", Login)

	// Agent 上报和自动注册接口（不需要 JWT）
	agentHandler := NewAgentHandler(db, collector)
	r.POST("/api/agent/report", agentHandler.Report)
	r.POST("/api/agent/register", agentHandler.Register)

	// Agent 二进制下载和一键安装脚本（公开）
	agentUpdateHandler := NewAgentUpdateHandler(agentHub, db)
	r.GET("/api/agent/download", agentUpdateHandler.Download)
	r.GET("/api/agent/download-win", agentUpdateHandler.DownloadWin)
	r.GET("/api/agent/install.sh", agentUpdateHandler.InstallScript)
	r.GET("/api/agent/install.ps1", agentUpdateHandler.InstallScriptWin)

	// Agent WebSocket（用 AgentToken 认证）
	execHandler := NewExecHandler(db, agentHub)
	r.GET("/ws/agent", execHandler.HandleAgentWS)

	// 压力测试 WebSocket（JWT 通过 query param 认证）
	stressHandler := NewStressHandler(db, agentHub, collector)
	r.GET("/ws/stress", func(c *gin.Context) {
		stressHandler.HandleWS(c.Writer, c.Request)
	})

	// 交互式终端 WebSocket（JWT 通过 query param 认证）
	termHandler := NewTerminalHandler(db, agentHub)
	r.GET("/ws/terminal/:id", func(c *gin.Context) {
		server, err := termHandler.ValidateAndGetServer(c.Request, c.Param("id"))
		if err != nil {
			c.JSON(401, gin.H{"error": err.Error()})
			return
		}
		termHandler.HandleTerminal(c.Writer, c.Request, *server)
	})

	// 桌面实时查看 WebSocket（JWT 通过 query param 认证）
	r.GET("/ws/screen/:id", func(c *gin.Context) {
		server, err := termHandler.ValidateAndGetServer(c.Request, c.Param("id"))
		if err != nil {
			c.JSON(401, gin.H{"error": err.Error()})
			return
		}
		termHandler.HandleScreen(c.Writer, c.Request, *server)
	})

	api := r.Group("/api")
	api.Use(AuthMiddleware())

	// 用户信息
	api.GET("/user/info", GetUserInfo)
	api.PUT("/user/password", ChangePassword)

	// 服务器管理
	serverHandler := NewServerHandler(db, collector)
	api.GET("/servers", serverHandler.List)
	api.GET("/servers/:id", serverHandler.GetByID)
	api.POST("/servers", serverHandler.Create)
	api.PUT("/servers/:id", serverHandler.Update)
	api.DELETE("/servers/:id", serverHandler.Delete)
	api.POST("/servers/:id/test", serverHandler.TestConnection)
	api.POST("/servers/test", serverHandler.TestNewConnection)

	// 远程执行
	api.POST("/servers/:id/exec", execHandler.Exec)
	api.GET("/servers/:id/agent-status", execHandler.AgentStatus)

	// 指标查询
	metricHandler := NewMetricHandler(db, collector)
	api.GET("/metrics/overview", metricHandler.Overview)
	api.GET("/metrics/realtime", metricHandler.Realtime)
	api.GET("/metrics/:serverId", metricHandler.History)

	// 告警管理
	alertHandler := NewAlertHandler(db)
	api.GET("/alerts", alertHandler.List)
	api.GET("/alerts/count", alertHandler.Count)
	api.PUT("/alerts/:id/resolve", alertHandler.Resolve)
	api.PUT("/alerts/batch-resolve", alertHandler.BatchResolve)

	// 告警规则
	api.GET("/alert-rules", alertHandler.ListRules)
	api.PUT("/alert-rules/:id", alertHandler.UpdateRule)

	// 压力测试
	api.POST("/stress/start", stressHandler.Start)
	api.POST("/stress/stop/:id", stressHandler.Stop)
	api.GET("/stress/agents", stressHandler.GetOnlineAgents)

	// 安全管理
	api.GET("/security/blacklist", ListBlacklist)
	api.POST("/security/blacklist", AddBlacklist)
	api.DELETE("/security/blacklist/:id", RemoveBlacklist)
	api.GET("/security/logs", ListSecurityLogs)
	api.GET("/security/login-attempts", ListLoginAttempts)

	// 通知推送配置
	notifyHandler := NewNotifyHandler(db, collector)
	api.GET("/notify/config", notifyHandler.GetConfig)
	api.PUT("/notify/config", notifyHandler.UpdateConfig)
	api.POST("/notify/test", notifyHandler.TestNotify)

	// Agent 更新管理
	api.POST("/agent/upload", agentUpdateHandler.Upload)
	api.GET("/agent/info", agentUpdateHandler.Info)
	api.POST("/agent/push-update", agentUpdateHandler.PushUpdate)
	api.POST("/agent/force-update-win", agentUpdateHandler.ForceUpdateWin)

	// 系统配置
	api.GET("/config", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"success": true,
			"data": gin.H{
				"collectInterval": 10,
				"sshTimeout":      5,
				"retryCount":      2,
				"version":         "2.8.0",
				"installKey":      model.GetSignKey(db),
			},
		})
	})
}
