//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

var (
	procKeyBdEvent      = modUser32.NewProc("keybd_event")
	procLockWorkStation = modUser32.NewProc("LockWorkStation")
)

const (
	vkLWin        = 0x5B
	vkD           = 0x44
	keyEventFKeyUp = 0x0002
)

// executeQuickCmd 执行快捷指令
func executeQuickCmd(cmd string) (string, error) {
	switch cmd {
	case "show_desktop":
		return quickShowDesktop()
	case "lock_screen":
		return quickLockScreen()
	case "task_manager":
		return quickOpenApp("taskmgr.exe", "任务管理器")
	case "file_explorer":
		return quickOpenApp("explorer.exe", "文件管理器")
	case "cmd":
		return quickOpenApp("cmd.exe", "命令提示符")
	default:
		return "", fmt.Errorf("未知指令: %s", cmd)
	}
}

// quickShowDesktop 模拟 Win+D 显示桌面
func quickShowDesktop() (string, error) {
	procKeyBdEvent.Call(uintptr(vkLWin), 0, 0, 0)
	time.Sleep(30 * time.Millisecond)
	procKeyBdEvent.Call(uintptr(vkD), 0, 0, 0)
	time.Sleep(30 * time.Millisecond)
	procKeyBdEvent.Call(uintptr(vkD), 0, uintptr(keyEventFKeyUp), 0)
	time.Sleep(30 * time.Millisecond)
	procKeyBdEvent.Call(uintptr(vkLWin), 0, uintptr(keyEventFKeyUp), 0)
	return "已切换到桌面", nil
}

// quickLockScreen 锁定屏幕
func quickLockScreen() (string, error) {
	r, _, err := procLockWorkStation.Call()
	if r == 0 {
		return "", fmt.Errorf("锁屏失败: %v", err)
	}
	return "已锁定屏幕", nil
}

// quickOpenApp 启动应用程序（隐藏启动窗口闪烁）
func quickOpenApp(app, name string) (string, error) {
	cmd := exec.Command(app)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("启动 %s 失败: %v", name, err)
	}
	return fmt.Sprintf("已启动 %s", name), nil
}
