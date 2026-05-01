//go:build !windows

package main

import "fmt"

// executeQuickCmd 快捷指令（Linux 暂不支持桌面操作）
func executeQuickCmd(cmd string) (string, error) {
	return "", fmt.Errorf("快捷指令仅支持 Windows")
}
