//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

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
