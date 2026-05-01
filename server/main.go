package main

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"server-monitor/internal/config"
	"server-monitor/internal/handler"
	"server-monitor/internal/model"
	"server-monitor/internal/service"
	"server-monitor/internal/ws"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置
	cfg := config.Load()

	// 自动迁移旧 data.db → data/data.db
	if cfg.DBPath == "data/data.db" {
		if _, err := os.Stat("data.db"); err == nil {
			if _, err2 := os.Stat("data/data.db"); os.IsNotExist(err2) {
				os.MkdirAll("data", 0755)
				if err3 := os.Rename("data.db", "data/data.db"); err3 == nil {
					log.Println("已将 data.db 迁移到 data/data.db")
				}
			}
		}
	}

	// 初始化数据库
	db := model.InitDB(cfg.DBPath)

	// 初始化种子数据
	model.SeedServers(db)
	model.BackfillAgentTokens(db)

	// 初始化安全模块（用户表、黑名单表、JWT Secret 等）
	model.InitSecurity(db)

	// 初始化 WebSocket Hub
	hub := ws.NewHub()
	go hub.Run()

	// 初始化 Agent Hub（远程执行）
	agentHub := ws.NewAgentHub()
	agentHub.SetSignKey([]byte(model.GetSignKey(db)))

	// 初始化采集服务
	collector := service.NewCollector(db, hub, cfg)
	collector.Start()

	// 定期清理旧指标数据（保留 7 天）
	go model.StartMetricCleanup(db)

	// 初始化 Gin
	if cfg.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()

	// CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// 注册路由
	handler.RegisterRoutes(r, db, hub, agentHub, collector)

	// WebSocket
	r.GET("/ws", func(c *gin.Context) {
		ws.HandleWebSocket(hub, c.Writer, c.Request)
	})

	// 静态文件服务（前端）— 优先外部目录，否则使用内嵌
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		exePath, _ := os.Executable()
		staticDir = filepath.Join(filepath.Dir(exePath), "..", "web", "dist")
	}

	if info, err := os.Stat(staticDir); err == nil && info.IsDir() {
		// 外部静态文件目录
		log.Printf("前端静态文件目录(外部): %s\n", staticDir)
		r.Static("/assets", filepath.Join(staticDir, "assets"))
		r.StaticFile("/favicon.ico", filepath.Join(staticDir, "favicon.ico"))

		indexHTML := filepath.Join(staticDir, "index.html")
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if !strings.HasPrefix(path, "/api") && !strings.HasPrefix(path, "/ws") {
				c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
				c.Header("Pragma", "no-cache")
				c.File(indexHTML)
				return
			}
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "not found"})
		})
	} else {
		// 使用内嵌前端资源
		subFS, err := fs.Sub(frontendFS, "frontend")
		if err != nil {
			log.Printf("内嵌前端资源加载失败: %v，仅提供 API 服务\n", err)
		} else {
			log.Println("前端静态文件: 使用内嵌资源")
			fileServer := http.FileServer(http.FS(subFS))
			r.GET("/assets/*filepath", gin.WrapH(fileServer))
			r.GET("/favicon.ico", gin.WrapH(fileServer))

			// SPA fallback
			r.NoRoute(func(c *gin.Context) {
				path := c.Request.URL.Path
				if !strings.HasPrefix(path, "/api") && !strings.HasPrefix(path, "/ws") {
					indexData, err := fs.ReadFile(subFS, "index.html")
					if err != nil {
						c.JSON(500, gin.H{"error": "index.html not found"})
						return
					}
					c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
					c.Header("Pragma", "no-cache")
					c.Data(200, "text/html; charset=utf-8", indexData)
					return
				}
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "not found"})
			})
		}
	}

	// 加载 TLS 配置
	tlsCfg := config.LoadTLS(filepath.Dir(cfg.DBPath))

	// 启动服务
	addr := fmt.Sprintf(":%d", cfg.Port)
	if tlsCfg.Enabled {
		log.Printf("服务器启动在 https://localhost%s (TLS)\n", addr)
		if err := http.ListenAndServeTLS(addr, tlsCfg.CertFile, tlsCfg.KeyFile, r); err != nil {
			log.Fatalf("启动失败: %v", err)
		}
	} else {
		log.Printf("服务器启动在 http://localhost%s\n", addr)
		if err := http.ListenAndServe(addr, r); err != nil {
			log.Fatalf("启动失败: %v", err)
		}
	}
}
