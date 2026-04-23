package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"server-monitor/internal/model"
	"server-monitor/internal/ws"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AgentUpdateHandler struct {
	agentHub    *ws.AgentHub
	db          *gorm.DB
	storagePath string // agent 二进制存储路径
}

func NewAgentUpdateHandler(agentHub *ws.AgentHub, db *gorm.DB) *AgentUpdateHandler {
	dir := "data/agent-bin"
	os.MkdirAll(dir, 0755)
	return &AgentUpdateHandler{
		agentHub:    agentHub,
		db:          db,
		storagePath: dir,
	}
}

func (h *AgentUpdateHandler) agentBinPath() string {
	return filepath.Join(h.storagePath, "agentlinux")
}

func (h *AgentUpdateHandler) agentWinBinPath() string {
	return filepath.Join(h.storagePath, "agent-windows.exe")
}

// Upload 接收上传的 agent 二进制（通过 platform 参数区分 linux/windows）
func (h *AgentUpdateHandler) Upload(c *gin.Context) {
	platform := c.DefaultQuery("platform", "linux")
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "未选择文件"})
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "打开文件失败"})
		return
	}
	defer src.Close()

	targetPath := h.agentBinPath()
	if platform == "windows" {
		targetPath = h.agentWinBinPath()
	}

	dst, err := os.Create(targetPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "保存失败"})
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "写入失败"})
		return
	}

	os.Chmod(targetPath, 0755)

	log.Printf("Agent [%s] 二进制已上传: %s, 大小: %.2f MB", platform, targetPath, float64(written)/1024/1024)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"size":     written,
			"filename": file.Filename,
			"platform": platform,
		},
	})
}

// blockBrowser 拦截浏览器直接访问，允许 curl/wget/PowerShell 等命令行工具
func (h *AgentUpdateHandler) blockBrowser(c *gin.Context) bool {
	ua := c.GetHeader("User-Agent")
	// 先放行已知命令行工具（PowerShell 默认 UA 含 Mozilla，必须优先判断）
	uaLower := strings.ToLower(ua)
	if strings.Contains(uaLower, "powershell") || strings.Contains(uaLower, "curl") || strings.Contains(uaLower, "wget") {
		return true
	}
	// 拦截浏览器
	if strings.Contains(ua, "Mozilla") || strings.Contains(ua, "Chrome") || strings.Contains(ua, "Safari") || strings.Contains(ua, "Edge") {
		c.String(http.StatusForbidden, "Access denied")
		return false
	}
	return true
}

// Download 提供 agent 二进制下载
func (h *AgentUpdateHandler) Download(c *gin.Context) {
	if !h.blockBrowser(c) {
		return
	}
	binPath := h.agentBinPath()
	info, err := os.Stat(binPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Agent 二进制不存在，请先上传"})
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=agentlinux")
	c.Header("Content-Length", fmt.Sprintf("%d", info.Size()))
	c.File(binPath)
}

// DownloadWin 提供 Windows agent 二进制下载
func (h *AgentUpdateHandler) DownloadWin(c *gin.Context) {
	if !h.blockBrowser(c) {
		return
	}
	binPath := h.agentWinBinPath()
	info, err := os.Stat(binPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Windows Agent 二进制不存在，请先上传"})
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=agent-windows.exe")
	c.Header("Content-Length", fmt.Sprintf("%d", info.Size()))
	c.File(binPath)
}

// Info 返回当前 agent 二进制信息（Linux + Windows）
func (h *AgentUpdateHandler) Info(c *gin.Context) {
	result := gin.H{}

	// Linux
	if info, err := os.Stat(h.agentBinPath()); err == nil {
		result["linux"] = gin.H{
			"exists":   true,
			"size":     info.Size(),
			"modified": info.ModTime().Format(time.RFC3339),
		}
	} else {
		result["linux"] = gin.H{"exists": false}
	}

	// Windows
	if info, err := os.Stat(h.agentWinBinPath()); err == nil {
		result["windows"] = gin.H{
			"exists":   true,
			"size":     info.Size(),
			"modified": info.ModTime().Format(time.RFC3339),
		}
	} else {
		result["windows"] = gin.H{"exists": false}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// InstallScript 生成一键安装脚本（拦截浏览器访问）
func (h *AgentUpdateHandler) InstallScript(c *gin.Context) {
	if !h.blockBrowser(c) {
		return
	}
	token := c.Query("token")
	serverURL := fmt.Sprintf("http://%s", c.Request.Host)
	downloadURL := serverURL + "/api/agent/download"

	// 获取签名密钥
	signKey := model.GetSignKey(h.db)

	// 配置文件内容：有 token 写入，无 token 则只写 SERVER_URL，Agent 启动时自动注册
	confLines := fmt.Sprintf("SERVER_URL=%s\nINTERVAL=10\nSIGN_KEY=%s", serverURL, signKey)
	modeHint := "自动注册模式（无需 Token，首次启动自动注册）"
	if token != "" {
		confLines = fmt.Sprintf("SERVER_URL=%s\nAGENT_TOKEN=%s\nINTERVAL=10\nSIGN_KEY=%s", serverURL, token, signKey)
		modeHint = "指定 Token 模式"
	}

	_ = modeHint // 已移除不必要的输出
	script := fmt.Sprintf(`#!/bin/bash
set -e
exec 3>&1
INSTALL_DIR="/opt/server-monitor-agent"
BINARY_NAME="agentlinux"
{
mkdir -p "$INSTALL_DIR"
} >/dev/null 2>&1

# 下载 Agent（显示进度条）
curl -#fSL -o "$INSTALL_DIR/$BINARY_NAME" "%s"
{
chmod +x "$INSTALL_DIR/$BINARY_NAME"

cat > "$INSTALL_DIR/agent.conf" <<EOF
%s
EOF

pkill -f "$BINARY_NAME" || true
sleep 1

if command -v systemctl >/dev/null 2>&1; then
    cat > /etc/systemd/system/monitor-agent.service <<EOF
[Unit]
Description=Server Monitor Agent
After=network.target

[Service]
Type=simple
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/$BINARY_NAME
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable monitor-agent
    systemctl start monitor-agent
else
    cd "$INSTALL_DIR"
    nohup "./$BINARY_NAME" > agent.log 2>&1 &
fi
} >/dev/null 2>&1

echo "部署完成" >&3
`, downloadURL, confLines)

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, script)
}

// InstallScriptWin 生成 Windows 一键安装 PowerShell 脚本（拦截浏览器访问）
func (h *AgentUpdateHandler) InstallScriptWin(c *gin.Context) {
	if !h.blockBrowser(c) {
		return
	}
	token := c.Query("token")
	serverURL := fmt.Sprintf("http://%s", c.Request.Host)
	downloadURL := serverURL + "/api/agent/download-win"

	// 获取签名密钥
	signKeyWin := model.GetSignKey(h.db)

	// 配置文件行（PowerShell 数组格式，每个元素是一行）
	confArray := fmt.Sprintf("\"SERVER_URL=%s\",\"INTERVAL=10\",\"SIGN_KEY=%s\"", serverURL, signKeyWin)
	modeHint := "自动注册模式"
	if token != "" {
		confArray = fmt.Sprintf("\"SERVER_URL=%s\",\"AGENT_TOKEN=%s\",\"INTERVAL=10\",\"SIGN_KEY=%s\"", serverURL, token, signKeyWin)
		modeHint = "指定 Token 模式"
	}

	_ = modeHint // 已移除不必要的输出
	// 使用双引号 Go 字符串拼接，避免 raw string 中出现 backtick
	script := "$ErrorActionPreference = \"Stop\"\r\n"
	script += "$InstallDir = \"$env:ProgramData\\ServerMonitorAgent\"\r\n"
	script += "$BinaryName = \"agent-windows.exe\"\r\n"
	script += "if (!(Test-Path $InstallDir)) { New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null }\r\n"
	script += "[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12\r\n"
	// 用 WebClient 下载并显示进度条
	script += "$wc = New-Object Net.WebClient\r\n"
	script += "$wc.Headers.Add(\"User-Agent\",\"PowerShell\")\r\n"
	script += "$target = \"$InstallDir\\$BinaryName\"\r\n"
	script += "Register-ObjectEvent -InputObject $wc -EventName DownloadProgressChanged -Action { Write-Host -NoNewline (\"`r下载中: {0}% ({1:N1}/{2:N1} MB)  \" -f $EventArgs.ProgressPercentage, ($EventArgs.BytesReceived/1MB), ($EventArgs.TotalBytesToReceive/1MB)) } | Out-Null\r\n"
	script += "$done = Register-ObjectEvent -InputObject $wc -EventName DownloadFileCompleted -Action { } -MessageData @{}\r\n"
	script += fmt.Sprintf("$wc.DownloadFileAsync(\"%s\", $target)\r\n", downloadURL)
	script += "while ($wc.IsBusy) { Start-Sleep -Milliseconds 200 }\r\n"
	script += "Write-Host \"\"\r\n"
	script += "Get-EventSubscriber | Unregister-Event -Force | Out-Null\r\n"
	script += "$wc.Dispose()\r\n"
	// 配置文件（无 BOM）
	script += fmt.Sprintf("$utf8NoBom = New-Object System.Text.UTF8Encoding($false); [System.IO.File]::WriteAllLines(\"$InstallDir\\agent.conf\", @(%s), $utf8NoBom)\r\n", confArray)
	// 停止旧进程
	script += "Get-Process -Name \"agent-windows\" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue\r\n"
	script += "Start-Sleep -Seconds 1\r\n"
	// 管理员判断 + 安装
	script += "$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)\r\n"
	script += "if ($isAdmin) {\r\n"
	script += "    $taskName = \"ServerMonitorAgent\"\r\n"
	script += "    $wdTaskName = \"ServerMonitorAgentWatchdog\"\r\n"
	// 清理旧任务
	script += "    Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue | Unregister-ScheduledTask -Confirm:$false -ErrorAction SilentlyContinue\r\n"
	script += "    Get-ScheduledTask -TaskName $wdTaskName -ErrorAction SilentlyContinue | Unregister-ScheduledTask -Confirm:$false -ErrorAction SilentlyContinue\r\n"
	// 主任务：用户登录时启动（交互式会话，支持桌面截图）
	script += "    $action = New-ScheduledTaskAction -Execute \"$InstallDir\\$BinaryName\" -WorkingDirectory $InstallDir\r\n"
	script += "    $trigger = New-ScheduledTaskTrigger -AtLogOn -User \"$env:USERDOMAIN\\$env:USERNAME\"\r\n"
	script += "    $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -RestartCount 9999 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit (New-TimeSpan -Days 3650)\r\n"
	script += "    $principal = New-ScheduledTaskPrincipal -UserId \"$env:USERDOMAIN\\$env:USERNAME\" -LogonType Interactive -RunLevel Highest\r\n"
	script += "    Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Description \"Server Monitor Agent\" | Out-Null\r\n"
	script += "    Start-ScheduledTask -TaskName $taskName\r\n"
	// 看门狗：写 watchdog.ps1 脚本文件，避免嵌套引号问题
	script += "    $wdPs1 = 'if (!(Get-Process -Name \"agent-windows\" -ErrorAction SilentlyContinue)) { Start-Process -FilePath \"' + $InstallDir + '\\' + $BinaryName + '\" -WorkingDirectory \"' + $InstallDir + '\" -WindowStyle Hidden }'\r\n"
	script += "    [System.IO.File]::WriteAllText(\"$InstallDir\\watchdog.ps1\", $wdPs1)\r\n"
	script += "    $wdAction = New-ScheduledTaskAction -Execute \"powershell.exe\" -Argument ('-NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File \"' + $InstallDir + '\\watchdog.ps1\"') -WorkingDirectory $InstallDir\r\n"
	script += "    $wdTrigger = New-ScheduledTaskTrigger -Once -At (Get-Date) -RepetitionInterval (New-TimeSpan -Minutes 2) -RepetitionDuration (New-TimeSpan -Days 3650)\r\n"
	script += "    Register-ScheduledTask -TaskName $wdTaskName -Action $wdAction -Trigger $wdTrigger -Settings $settings -Principal $principal -Description \"Agent Watchdog\" | Out-Null\r\n"
	script += "} else {\r\n"
	// 非管理员：用 VBS 无窗口启动 + 启动目录快捷方式
	script += "    $vbsPath = \"$InstallDir\\start-hidden.vbs\"\r\n"
	script += "    $vbsContent = 'CreateObject(\"WScript.Shell\").Run \"' + $InstallDir + '\\' + $BinaryName + '\", 0, False'\r\n"
	script += "    [System.IO.File]::WriteAllText($vbsPath, $vbsContent)\r\n"
	script += "    $startupDir = [Environment]::GetFolderPath('Startup')\r\n"
	script += "    $shortcutPath = Join-Path $startupDir \"ServerMonitorAgent.lnk\"\r\n"
	script += "    $ws = New-Object -ComObject WScript.Shell\r\n"
	script += "    $sc = $ws.CreateShortcut($shortcutPath)\r\n"
	script += "    $sc.TargetPath = \"wscript.exe\"\r\n"
	script += "    $sc.Arguments = \"$vbsPath\"\r\n"
	script += "    $sc.WorkingDirectory = $InstallDir\r\n"
	script += "    $sc.WindowStyle = 7\r\n"
	script += "    $sc.Save()\r\n"
	// 用 VBS 启动（完全无窗口）
	script += "    Start-Process -FilePath \"wscript.exe\" -ArgumentList $vbsPath -WindowStyle Hidden\r\n"
	script += "}\r\n"
	script += "Write-Host \"部署完成\" -ForegroundColor Green\r\n"
	_ = serverURL

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, script)
}

// PushUpdate 向在线 Agent 推送更新指令
func (h *AgentUpdateHandler) PushUpdate(c *gin.Context) {
	var req struct {
		ServerIDs []string `json:"serverIds"` // 空则更新所有
		Platform  string   `json:"platform"`  // linux / windows，默认 linux
	}
	c.ShouldBindJSON(&req)

	if req.Platform == "" {
		req.Platform = "linux"
	}

	// 检查对应平台的二进制是否存在
	binPath := h.agentBinPath()
	downloadURL := fmt.Sprintf("http://%s/api/agent/download", c.Request.Host)
	if req.Platform == "windows" {
		binPath = h.agentWinBinPath()
		downloadURL = fmt.Sprintf("http://%s/api/agent/download-win", c.Request.Host)
	}

	if _, err := os.Stat(binPath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("%s Agent 二进制不存在，请先上传", req.Platform)})
		return
	}

	payload, _ := json.Marshal(map[string]string{
		"url": downloadURL,
	})
	msg := ws.AgentMessage{
		Type:    "update",
		ID:      fmt.Sprintf("update-%s-%d", req.Platform, time.Now().UnixMilli()),
		Payload: payload,
	}

	sent := h.agentHub.BroadcastToAgents(msg, req.ServerIDs, req.Platform)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"sent":     sent,
			"platform": req.Platform,
		},
	})
}
