//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
)

// fixScheduledTaskIfSystem 检测当前是否以 SYSTEM 身份运行，
// 如果是则自动修改计划任务为当前登录用户的交互式会话，
// 这样 agent 重启后就能截图了。
func fixScheduledTaskIfSystem() {
	u, err := user.Current()
	if err != nil {
		return
	}
	// 只有 SYSTEM 才需要修复
	if !strings.HasSuffix(strings.ToUpper(u.Username), "SYSTEM") {
		return
	}

	log.Println("检测到以 SYSTEM 身份运行，尝试修复计划任务为交互式会话...")

	// 获取当前登录的交互式用户
	activeUser := getActiveConsoleUser()
	if activeUser == "" {
		log.Println("未检测到交互式登录用户，跳过计划任务修复")
		return
	}

	log.Printf("检测到交互式用户: %s，正在修复计划任务...", activeUser)

	taskName := "ServerMonitorAgent"
	wdTaskName := "ServerMonitorAgentWatchdog"

	// 获取 agent 路径
	selfPath, err := getSelfPath()
	if err != nil {
		log.Printf("获取自身路径失败: %v", err)
		return
	}
	installDir := selfPath[:strings.LastIndex(selfPath, "\\")]

	// 删除旧任务
	runPS(fmt.Sprintf(`Get-ScheduledTask -TaskName "%s" -ErrorAction SilentlyContinue | Unregister-ScheduledTask -Confirm:$false -ErrorAction SilentlyContinue`, taskName))
	runPS(fmt.Sprintf(`Get-ScheduledTask -TaskName "%s" -ErrorAction SilentlyContinue | Unregister-ScheduledTask -Confirm:$false -ErrorAction SilentlyContinue`, wdTaskName))

	// 创建新的主任务（交互式用户）
	script := fmt.Sprintf(
		`$action = New-ScheduledTaskAction -Execute "%s\agent-windows.exe" -WorkingDirectory "%s"
$trigger = New-ScheduledTaskTrigger -AtLogOn -User "%s"
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -RestartCount 9999 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit (New-TimeSpan -Days 3650)
$principal = New-ScheduledTaskPrincipal -UserId "%s" -LogonType Interactive -RunLevel Highest
Register-ScheduledTask -TaskName "%s" -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Description "Server Monitor Agent" | Out-Null`,
		installDir, installDir, activeUser, activeUser, taskName)
	if err := runPS(script); err != nil {
		log.Printf("创建主任务失败: %v", err)
		return
	}

	// 创建看门狗
	wdScript := fmt.Sprintf(
		`$wdPs1 = 'if (!(Get-Process -Name "agent-windows" -ErrorAction SilentlyContinue)) { Start-Process -FilePath "%s\agent-windows.exe" -WorkingDirectory "%s" -WindowStyle Hidden }'
[System.IO.File]::WriteAllText("%s\watchdog.ps1", $wdPs1)
$wdAction = New-ScheduledTaskAction -Execute "powershell.exe" -Argument ('-NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File "%s\watchdog.ps1"') -WorkingDirectory "%s"
$wdTrigger = New-ScheduledTaskTrigger -Once -At (Get-Date) -RepetitionInterval (New-TimeSpan -Minutes 2) -RepetitionDuration (New-TimeSpan -Days 3650)
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -RestartCount 9999 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit (New-TimeSpan -Days 3650)
$principal = New-ScheduledTaskPrincipal -UserId "%s" -LogonType Interactive -RunLevel Highest
Register-ScheduledTask -TaskName "%s" -Action $wdAction -Trigger $wdTrigger -Settings $settings -Principal $principal -Description "Agent Watchdog" | Out-Null`,
		installDir, installDir, installDir, installDir, installDir, activeUser, wdTaskName)
	runPS(wdScript)

	log.Printf("计划任务已修复为用户 %s 的交互式会话，正在重启...", activeUser)

	// 启动新任务（会在用户会话中运行），然后退出当前 SYSTEM 进程
	runPS(fmt.Sprintf(`Start-ScheduledTask -TaskName "%s"`, taskName))

	// 退出当前 SYSTEM 进程，让新任务接管
	log.Println("SYSTEM 进程退出，由用户会话任务接管")
	syscall.Exit(0)
}

// getActiveConsoleUser 获取当前活动控制台的登录用户名（DOMAIN\User 格式）
func getActiveConsoleUser() string {
	out, err := exec.Command("powershell.exe", "-NoProfile", "-Command",
		`(Get-WmiObject -Class Win32_ComputerSystem).UserName`).CombinedOutput()
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(string(out))
	if name == "" || strings.Contains(name, "Exception") {
		return ""
	}
	return name
}

func runPS(script string) error {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, string(out))
	}
	return nil
}

func getSelfPath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(p)
}
