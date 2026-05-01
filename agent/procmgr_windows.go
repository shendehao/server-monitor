//go:build windows

package main

import (
	"os/exec"
	"strconv"
	"strings"
)

// gatherProcessListPlatform 使用 tasklist /v /fo csv 获取进程列表
// 输出格式与 C# DLL Agent 一致: pid(int), name(string), mem(int64 KB), title(string)
func gatherProcessListPlatform() []ProcessInfo {
	var procs []ProcessInfo

	// tasklist /v /fo csv /nh 输出:
	// "name.exe","PID","Session","Session#","Mem Usage","Status","Username","CPU Time","Window Title"
	cmd := exec.Command("tasklist", "/v", "/fo", "csv", "/nh")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return procs
	}

	// Windows 命令输出为 OEM 代码页(GBK)，转为 UTF-8
	utf8Out := oemToUTF8(out)

	for _, line := range strings.Split(utf8Out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := parseCSVLine(line)
		if len(fields) < 9 {
			continue
		}
		pid, _ := strconv.Atoi(fields[1])
		name := fields[0]

		// mem 格式: "123,456 K" → 解析为 KB 数字
		memStr := strings.ReplaceAll(fields[4], ",", "")
		memStr = strings.ReplaceAll(memStr, ".", "")
		memStr = strings.TrimSuffix(strings.TrimSpace(memStr), " K")
		memStr = strings.TrimSpace(memStr)
		mem, _ := strconv.ParseInt(memStr, 10, 64)

		title := fields[8]
		if title == "N/A" || title == "暂缺" || title == "\xe6\x9a\x82\xe7\xbc\xba" {
			title = ""
		}

		procs = append(procs, ProcessInfo{
			PID:   pid,
			Name:  name,
			Mem:   mem,
			Title: title,
		})
	}
	return procs
}

func parseCSVLine(line string) []string {
	var fields []string
	var current strings.Builder
	inQuote := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '"' {
			inQuote = !inQuote
		} else if c == ',' && !inQuote {
			fields = append(fields, current.String())
			current.Reset()
		} else {
			current.WriteByte(c)
		}
	}
	fields = append(fields, current.String())
	return fields
}
