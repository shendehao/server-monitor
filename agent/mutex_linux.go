//go:build !windows

package main

// createAgentMutex Linux 下无需命名互斥体（使用 PID 文件）
func createAgentMutex() {}
