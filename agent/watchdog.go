package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// setupLogging 把日志同时输出到文件和stdout（GUI模式下stdout被丢弃但不会崩溃）
// 日志按天轮转，保留最近3天
func setupLogging() {
	selfPath, err := os.Executable()
	if err != nil {
		return
	}
	selfPath, _ = filepath.EvalSymlinks(selfPath)
	logDir := filepath.Join(filepath.Dir(selfPath), "logs")
	os.MkdirAll(logDir, 0755)

	logPath := filepath.Join(logDir, fmt.Sprintf("agent-%s.log", time.Now().Format("20060102")))
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}

	// 同时输出到stdout和文件
	log.SetOutput(io.MultiWriter(os.Stdout, f))
	log.SetFlags(log.LstdFlags)

	// 清理3天前的日志
	go cleanOldLogs(logDir)
}

func cleanOldLogs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -3)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// spawnWatchdog 启动一个看门狗子进程用于互相监控
// 如果 AGENT_ROLE=watchdog 则当前进程就是看门狗
func spawnWatchdog() {
	role := os.Getenv("AGENT_ROLE")
	if role == "watchdog" {
		// 当前就是看门狗，监控父进程
		runWatchdogLoop()
		return
	}

	// 主进程：启动看门狗子进程
	selfPath, err := os.Executable()
	if err != nil {
		return
	}

	parentPID := os.Getpid()
	cmd := exec.Command(selfPath)
	cmd.Env = append(os.Environ(),
		"AGENT_ROLE=watchdog",
		fmt.Sprintf("AGENT_PARENT_PID=%d", parentPID),
	)
	hideWindow(cmd) // 平台特定：Windows 隐藏窗口
	if err := cmd.Start(); err != nil {
		log.Printf("启动看门狗失败: %v", err)
		return
	}
	log.Printf("看门狗已启动 PID=%d", cmd.Process.Pid)
	// 不 Wait，让它独立运行
}

// runWatchdogLoop 看门狗主循环：监控主进程，挂了就重启
func runWatchdogLoop() {
	parentPIDStr := os.Getenv("AGENT_PARENT_PID")
	if parentPIDStr == "" {
		os.Exit(0)
	}
	var parentPID int
	fmt.Sscanf(parentPIDStr, "%d", &parentPID)
	if parentPID <= 0 {
		os.Exit(0)
	}

	selfPath, err := os.Executable()
	if err != nil {
		os.Exit(1)
	}

	// 每 5 秒检查一次
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if !processAlive(parentPID) {
			// 父进程死了，拉起新的主进程
			cmd := exec.Command(selfPath)
			cmd.Env = os.Environ()
			// 清除 watchdog 标记，让子进程作为主进程启动
			newEnv := make([]string, 0, len(cmd.Env))
			for _, e := range cmd.Env {
				if len(e) > 11 && e[:11] == "AGENT_ROLE=" {
					continue
				}
				if len(e) > 17 && e[:17] == "AGENT_PARENT_PID=" {
					continue
				}
				newEnv = append(newEnv, e)
			}
			cmd.Env = newEnv
			hideWindow(cmd)
			cmd.Stdout = nil
			cmd.Stderr = nil
			if err := cmd.Start(); err == nil {
				// 新主进程启动成功，当前 watchdog 退出，让新主进程重新 fork watchdog
				os.Exit(0)
			}
			// 启动失败，等会儿再试
			time.Sleep(5 * time.Second)
		}
	}
}
