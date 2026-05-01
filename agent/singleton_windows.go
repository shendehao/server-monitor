//go:build windows

package main

import (
	"log"
	"syscall"
	"unsafe"
)

var (
	modKernel32Singleton = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutexW     = modKernel32Singleton.NewProc("CreateMutexW")
)

const _ERROR_ALREADY_EXISTS = 183

// acquireSingleton 尝试获取单实例锁（Windows 命名互斥量），防止多个 agent 进程同时运行
// 返回 true 表示获取成功（当前是唯一实例），false 表示另一个实例已在运行
func acquireSingleton() bool {
	// Global\ 前缀确保跨所有 session 生效（包括 Session 0 和用户 session）
	name, _ := syscall.UTF16PtrFromString("Global\\SysmonAgentSingleton")
	handle, _, err := procCreateMutexW.Call(
		0, // lpMutexAttributes
		0, // bInitialOwner = FALSE
		uintptr(unsafe.Pointer(name)),
	)
	if handle == 0 {
		// ERROR_ACCESS_DENIED (5): SYSTEM 创建的 mutex 普通用户无权打开 → 说明已有实例
		if err == syscall.Errno(5) {
			log.Printf("另一个 agent 实例已在运行（mutex 权限拒绝），当前进程退出")
			return false
		}
		return true // 其他错误，放行
	}
	if err == syscall.Errno(_ERROR_ALREADY_EXISTS) {
		syscall.CloseHandle(syscall.Handle(handle))
		log.Printf("另一个 agent 实例已在运行，当前进程退出")
		return false
	}
	// 获取成功，不关闭 handle，保持互斥量直到进程退出
	return true
}
