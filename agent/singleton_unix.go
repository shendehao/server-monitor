//go:build !windows

package main

import (
	"log"
	"os"
	"strconv"
	"syscall"
)

const singletonLockPath = "/tmp/sysmon-agent.lock"

// acquireSingleton 尝试获取单实例锁，防止多个 agent 进程同时运行
// 返回 true 表示获取成功（当前是唯一实例），false 表示另一个实例已在运行
func acquireSingleton() bool {
	f, err := os.OpenFile(singletonLockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return true // 无法创建锁文件，放行
	}
	// 非阻塞排他锁
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		f.Close()
		// 读取已有锁文件中的 PID 用于日志
		if data, _ := os.ReadFile(singletonLockPath); len(data) > 0 {
			log.Printf("另一个 agent 实例已在运行 (PID=%s)，当前进程退出", string(data))
		}
		return false
	}
	// 获取锁成功，写入当前 PID
	f.Truncate(0)
	f.Seek(0, 0)
	f.WriteString(strconv.Itoa(os.Getpid()))
	// 注意：不关闭 f，保持锁直到进程退出
	return true
}
