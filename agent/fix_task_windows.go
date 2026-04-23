//go:build windows

package main

// fixScheduledTaskIfSystem 已废弃，改用 CreateProcessAsUser 子进程截图方案
func fixScheduledTaskIfSystem() {}
