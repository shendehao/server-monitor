//go:build !windows

package main

import (
	"fmt"
	"os"
	"strings"
)

// getMachineID 获取 Linux 机器唯一标识（/etc/machine-id）
// systemd 系统在安装时自动生成，不可跨机器复制配置
func getMachineID() (string, error) {
	paths := []string{"/etc/machine-id", "/var/lib/dbus/machine-id"}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			id := strings.TrimSpace(string(data))
			if id != "" {
				return id, nil
			}
		}
	}
	// 降级：用 hostname 作为标识
	name, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("no machine-id and hostname failed: %v", err)
	}
	return "host-" + name, nil
}
