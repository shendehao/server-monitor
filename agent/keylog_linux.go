//go:build !windows

package main

import (
	"sync"

	"github.com/gorilla/websocket"
)

// ═══ 键盘记录模块 (Linux stub — 需要 root 权限读取 /dev/input) ═══
// Linux 键盘记录暂不实现，返回不支持提示

func handleKeylogStart(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	sendResult(conn, writeMu, "keylog_result", msg.ID, `{"status":"unsupported","message":"Linux 键盘记录暂不支持"}`)
}

func handleKeylogStop(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	sendResult(conn, writeMu, "keylog_result", msg.ID, `{"status":"not_running"}`)
}

func handleKeylogDump(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	sendResult(conn, writeMu, "keylog_dump_result", msg.ID, `{"log":"Linux 键盘记录暂不支持"}`)
}
