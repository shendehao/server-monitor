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

const AgentVersion = "2.3.0"

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

func loadConfigFile() map[string]string {
	cfg := make(map[string]string)
	// 读取可执行文件同目录下的 agent.conf
	selfPath, err := os.Executable()
	if err != nil {
		return cfg
	}
	selfPath, _ = filepath.EvalSymlinks(selfPath)
	confPath := filepath.Join(filepath.Dir(selfPath), "agent.conf")
	data, err := os.ReadFile(confPath)
	if err != nil {
		return cfg
	}
	// 去除 UTF-8 BOM（Windows PowerShell 写入 UTF8 时会带 BOM）
	content := string(data)
	content = strings.TrimPrefix(content, "\xEF\xBB\xBF")
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			cfg[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	log.Printf("已加载配置文件: %s", confPath)
	return cfg
}

func saveConfigFile(serverURL, token, interval string) {
	selfPath, err := os.Executable()
	if err != nil {
		return
	}
	selfPath, _ = filepath.EvalSymlinks(selfPath)
	confPath := filepath.Join(filepath.Dir(selfPath), "agent.conf")
	content := fmt.Sprintf("SERVER_URL=%s\nAGENT_TOKEN=%s\nINTERVAL=%s\n", serverURL, token, interval)
	os.WriteFile(confPath, []byte(content), 0600)
}

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
	resp, err := http.Post(registerURL, "application/json", bytes.NewReader(reqBody))
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
	// 初始化日志（GUI 模式下无控制台，必须写到文件）
	setupLogging()

	// 看门狗模式：如果是 AGENT_ROLE=watchdog，则只运行监控循环
	if os.Getenv("AGENT_ROLE") == "watchdog" {
		spawnWatchdog()
		return
	}

	// 主进程：启动看门狗子进程用于互相监控
	spawnWatchdog()

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
		fmt.Println("用法: 设置 SERVER_URL 后启动")
		fmt.Println("  SERVER_URL=http://监控服务器地址:5000 ./agentlinux")
		fmt.Println("  或在 agent.conf 中配置 SERVER_URL=...")
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

	// 自动保存配置文件，确保重启后不丢失
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

// wsLoop 保持 WebSocket 长连接，接收服务端下发的命令并执行（指数退避重连）
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

	backoff := time.Second * 2
	const maxBackoff = time.Minute * 2

	for {
		log.Printf("WebSocket 连接中: %s", wsURL)
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
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
		backoff = time.Second * 2 // 连接成功，重置退避
		log.Printf("WebSocket 已连接")

		// writeMu 保护并发写， gorilla/websocket 不支持并发 Write
		var writeMu sync.Mutex
		var wg sync.WaitGroup // 跟踪执行中的命令 goroutine

		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				log.Printf("WebSocket 断开: %v, 重连中...", err)
				break
			}

			var msg AgentMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}

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
			case "update":
				go handleSelfUpdate(conn, &writeMu, msg)
			case "ping":
				resp, _ := json.Marshal(AgentMessage{Type: "pong", ID: msg.ID})
				writeMu.Lock()
				conn.WriteMessage(websocket.TextMessage, resp)
				writeMu.Unlock()
			}
		}

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
		time.Sleep(3 * time.Second)
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
		Type:    "exec_result",
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
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
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

	if req.URL == "" {
		sendUpdateResult(conn, writeMu, msg.ID, false, "缺少下载地址")
		return
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

	// 备份旧版本
	backupPath := selfPath + ".bak"
	os.Remove(backupPath)

	if runtime.GOOS == "windows" {
		// Windows: 运行中的 exe 可以被 rename，但不能被覆盖
		// 先把当前 exe rename 为 .bak，再把新 exe rename 为原名
		if err := os.Rename(selfPath, backupPath); err != nil {
			// Windows 下如果 rename 失败，尝试 copy 覆盖方式
			log.Printf("Windows rename 失败，尝试 copy 方式: %v", err)
			if cpErr := copyFile(tmpPath, selfPath); cpErr != nil {
				os.Remove(tmpPath)
				sendUpdateResult(conn, writeMu, msg.ID, false, "Windows 替换失败: "+cpErr.Error())
				return
			}
			os.Remove(tmpPath)
		} else {
			if err := os.Rename(tmpPath, selfPath); err != nil {
				os.Rename(backupPath, selfPath) // 恢复
				os.Remove(tmpPath)
				sendUpdateResult(conn, writeMu, msg.ID, false, "替换失败: "+err.Error())
				return
			}
		}
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

	// 重启自身
	cmd := exec.Command(selfPath)
	cmd.Dir = selfDir
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()

	os.Exit(0)
}

// downloadWithTimeout 带超时的文件下载
func downloadWithTimeout(url, destPath string, timeout time.Duration) (int64, error) {
	client := &http.Client{Timeout: timeout}
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

func sendUpdateResult(conn *websocket.Conn, writeMu *sync.Mutex, id string, success bool, message string) {
	payload, _ := json.Marshal(map[string]interface{}{
		"success": success,
		"message": message,
	})
	resp, _ := json.Marshal(AgentMessage{
		Type:    "update_result",
		ID:      id,
		Payload: payload,
	})
	writeMu.Lock()
	conn.WriteMessage(websocket.TextMessage, resp)
	writeMu.Unlock()
}
