//go:build !windows

package main

import (
	"sync"

	"github.com/gorilla/websocket"
)

func handleMicStart(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	sendResult(conn, writeMu, "mic_start_result", msg.ID, `{"status":"unsupported","message":"麦克风监听仅支持 Windows"}`)
}

func handleMicStop(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	sendResult(conn, writeMu, "mic_stop_result", msg.ID, `{"status":"not_running"}`)
}
