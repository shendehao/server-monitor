//go:build !windows

package main

// fixScheduledTaskIfSystem is a no-op on non-Windows platforms
func fixScheduledTaskIfSystem() {}
