//go:build windows

package main

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// getMachineID 获取 Windows 机器唯一标识（MachineGuid）
// 该值在 Windows 安装时生成，重装系统后变化，不可跨机器复制配置
func getMachineID() (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("open registry: %v", err)
	}
	defer k.Close()
	guid, _, err := k.GetStringValue("MachineGuid")
	if err != nil {
		return "", fmt.Errorf("read MachineGuid: %v", err)
	}
	return guid, nil
}
