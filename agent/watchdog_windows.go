//go:build windows

package main

import (
	"os/exec"
	"syscall"
	"unsafe"
)

var (
	modKernel32     = syscall.NewLazyDLL("kernel32.dll")
	procFreeConsole = modKernel32.NewProc("FreeConsole")
	procGetConsoleW = modKernel32.NewProc("GetConsoleWindow")
	procShowWindow  = modKernel32.NewProc("ShowWindow")
)

func init() {
	// 立即隐藏控制台窗口（如果有的话）
	if hwnd, _, _ := procGetConsoleW.Call(); hwnd != 0 {
		procShowWindow.Call(hwnd, 0) // SW_HIDE = 0
	}
	// 完全脱离控制台
	procFreeConsole.Call()
	_ = unsafe.Sizeof(0) // keep unsafe import
}

// hideWindow 让子进程不显示控制台窗口
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

// processAlive 检查进程是否存活
func processAlive(pid int) bool {
	// OpenProcess PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	h, err := syscall.OpenProcess(0x1000, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)

	var exitCode uint32
	err = syscall.GetExitCodeProcess(h, &exitCode)
	if err != nil {
		return false
	}
	// STILL_ACTIVE = 259
	return exitCode == 259
}
