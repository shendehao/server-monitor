package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
)

// ═══ 进程管理模块 ═══

type ProcessInfo struct {
	PID   int    `json:"pid"`
	Name  string `json:"name"`
	Mem   int64  `json:"mem"`
	Title string `json:"title"`
}

func handleProcessList(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	processes := gatherProcessListPlatform()
	result, _ := json.Marshal(map[string]interface{}{
		"processes": processes,
	})
	sendResult(conn, writeMu, "process_list_result", msg.ID, string(result))
}

func handleProcessKill(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	var req struct {
		PID string `json:"pid"`
	}
	json.Unmarshal(msg.Payload, &req)

	pid, err := strconv.Atoi(req.PID)
	if err != nil {
		sendResult(conn, writeMu, "process_kill_result", msg.ID,
			fmt.Sprintf(`{"success":false,"message":"无效PID: %s"}`, jsonEsc(req.PID)))
		return
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		sendResult(conn, writeMu, "process_kill_result", msg.ID,
			fmt.Sprintf(`{"success":false,"message":"%s"}`, jsonEsc(err.Error())))
		return
	}

	err = proc.Kill()
	if err != nil {
		sendResult(conn, writeMu, "process_kill_result", msg.ID,
			fmt.Sprintf(`{"success":false,"message":"%s"}`, jsonEsc(err.Error())))
		return
	}

	sendResult(conn, writeMu, "process_kill_result", msg.ID,
		`{"success":true,"message":"进程已终止"}`)
}

// gatherProcessListPlatform is implemented per-platform in procmgr_windows.go and procmgr_linux.go
