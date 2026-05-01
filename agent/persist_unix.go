//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ensurePersistence 自动检测并静默安装持久化（首次启动时）
// 注意：只创建和启用服务，不 start，因为当前进程已经在运行
func ensurePersistence() {
	if _, err := os.Stat("/etc/systemd/system/sysmon-agent.service"); err == nil {
		return // 已安装，跳过
	}
	// 创建服务文件并启用，但不 start（避免启动第二个进程）
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	exePath, _ = filepath.EvalSymlinks(exePath)
	workDir := filepath.Dir(exePath)

	unit := fmt.Sprintf(`[Unit]
Description=System Monitor Agent
After=network.target

[Service]
Type=simple
WorkingDirectory=%s
ExecStart=%s
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`, workDir, exePath)

	if err := os.WriteFile("/etc/systemd/system/sysmon-agent.service", []byte(unit), 0644); err != nil {
		return
	}
	exec.Command("systemctl", "daemon-reload").Run()
	exec.Command("systemctl", "enable", "sysmon-agent").Run()
	// 不调用 systemctl start — 当前进程已经在运行，下次开机 systemd 会自动拉起
}

// installAgent 创建 systemd 服务并启用自启动
func installAgent() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取路径失败: %v", err)
	}
	exePath, _ = filepath.EvalSymlinks(exePath)
	workDir := filepath.Dir(exePath)

	unit := fmt.Sprintf(`[Unit]
Description=System Monitor Agent
After=network.target

[Service]
Type=simple
WorkingDirectory=%s
ExecStart=%s
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`, workDir, exePath)

	if err := os.WriteFile("/etc/systemd/system/sysmon-agent.service", []byte(unit), 0644); err != nil {
		return fmt.Errorf("写入服务文件失败: %v", err)
	}

	exec.Command("systemctl", "daemon-reload").Run()
	exec.Command("systemctl", "enable", "sysmon-agent").Run()
	exec.Command("systemctl", "start", "sysmon-agent").Run()
	return nil
}

// uninstallAgent 停止并移除 systemd 服务
func uninstallAgent() error {
	exec.Command("systemctl", "stop", "sysmon-agent").Run()
	exec.Command("systemctl", "disable", "sysmon-agent").Run()
	os.Remove("/etc/systemd/system/sysmon-agent.service")
	exec.Command("systemctl", "daemon-reload").Run()
	return nil
}

// cleanAllPersistence Linux 下清除 systemd 服务（更新前调用）
func cleanAllPersistence() {
	exec.Command("systemctl", "stop", "sysmon-agent").Run()
	exec.Command("systemctl", "disable", "sysmon-agent").Run()
	os.Remove("/etc/systemd/system/sysmon-agent.service")
	exec.Command("systemctl", "daemon-reload").Run()
}

// runGuardA Linux 下不需要看门狗守护（systemd 自动重启）
func runGuardA() {}

// runGuardB Linux 下不需要看门狗守护（systemd 自动重启）
func runGuardB() {}
