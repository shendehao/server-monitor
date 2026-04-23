//go:build !windows

package main

import (
	"sync"

	"github.com/gorilla/websocket"
)

// Linux/Unix: 桌面截图不可用（无图形界面）

func handleScreenStart(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	// 非 Windows 平台不支持截图
}

func handleScreenStop(msg AgentMessage) {}

func cleanupAllScreenSessions() {}
