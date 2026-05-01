//go:build !windows

package main

import (
	"sync"

	"github.com/gorilla/websocket"
)

func handleWindowList(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	sendResult(conn, writeMu, "window_list_result", msg.ID, `{"windows":[],"error":"窗口管理仅支持 Windows"}`)
}

func handleWindowControl(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	sendResult(conn, writeMu, "window_control_result", msg.ID, `{"success":false,"message":"窗口管理仅支持 Windows"}`)
}
