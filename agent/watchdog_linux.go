//go:build linux

package main

import (
	"os/exec"
	"syscall"
)

// hideWindow Linux 下无需隐藏窗口
func hideWindow(cmd *exec.Cmd) {
	// 让子进程独立于父进程的会话，避免被一起杀
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}

// processAlive 检查进程是否存活
func processAlive(pid int) bool {
	// 发送信号 0 仅检查进程是否存在
	p, err := syscall.Getpgid(pid)
	_ = p
	return err == nil
}
