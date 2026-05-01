//go:build !windows

package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

func gatherProcessListPlatform() []ProcessInfo {
	var procs []ProcessInfo
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return procs
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}

		name := ""
		var memKB int64

		if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid)); err == nil {
			name = strings.TrimSpace(string(data))
		}
		// Read VmRSS from status for memory in KB
		if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid)); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "VmRSS:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						memKB, _ = strconv.ParseInt(fields[1], 10, 64)
					}
					break
				}
			}
		}

		procs = append(procs, ProcessInfo{
			PID:  pid,
			Name: name,
			Mem:  memKB,
		})
	}
	sort.Slice(procs, func(i, j int) bool { return procs[i].PID < procs[j].PID })
	return procs
}
