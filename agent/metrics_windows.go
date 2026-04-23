//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// hiddenExec 静默执行 PowerShell 命令，不弹出任何窗口
func hiddenExec(args ...string) ([]byte, error) {
	cmd := exec.Command("powershell", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return cmd.Output()
}

func getCPUUsage() (float64, error) {
	// 使用 PowerShell 获取 CPU 使用率
	out, err := hiddenExec("-NoProfile", "-Command",
		"(Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average")
	if err != nil {
		return 0, err
	}
	val, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, err
	}
	return round2(val), nil
}

func getMemory() (total, used int64, usage float64) {
	// 获取总内存 (MB)
	outTotal, err := hiddenExec("-NoProfile", "-Command",
		"[math]::Round((Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory / 1MB)")
	if err != nil {
		return
	}
	total, _ = strconv.ParseInt(strings.TrimSpace(string(outTotal)), 10, 64)

	// 获取可用内存 (MB)
	outFree, err := hiddenExec("-NoProfile", "-Command",
		"[math]::Round((Get-CimInstance Win32_OperatingSystem).FreePhysicalMemory / 1KB)")
	if err != nil {
		return
	}
	free, _ := strconv.ParseInt(strings.TrimSpace(string(outFree)), 10, 64)

	used = total - free
	if total > 0 {
		usage = round2(float64(used) * 100 / float64(total))
	}
	return
}

func getDisk() (total, used int64, usage float64) {
	// 获取所有固定磁盘的容量和可用空间
	out, err := hiddenExec("-NoProfile", "-Command",
		"Get-CimInstance Win32_LogicalDisk -Filter 'DriveType=3' | ForEach-Object { $_.Size,$_.FreeSpace } | ForEach-Object { [math]::Round($_ / 1GB) }")
	if err != nil {
		return
	}
	lines := strings.Fields(strings.TrimSpace(string(out)))
	// 输出成对: size, free, size, free, ...
	for i := 0; i+1 < len(lines); i += 2 {
		s, _ := strconv.ParseInt(lines[i], 10, 64)
		f, _ := strconv.ParseInt(lines[i+1], 10, 64)
		total += s
		used += s - f
	}
	if total > 0 {
		usage = round2(float64(used) * 100 / float64(total))
	}
	return
}

func getLoadAvg() (l1, l5, l15 float64) {
	// Windows 没有 load average，用 CPU 队列长度近似
	out, err := hiddenExec("-NoProfile", "-Command",
		"(Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average")
	if err != nil {
		return
	}
	val, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	// 归一化到 load average 风格 (0-1 per core)
	l1 = round2(val / 100)
	l5 = l1
	l15 = l1
	return
}

func getProcessCount() int {
	out, err := hiddenExec("-NoProfile", "-Command",
		"(Get-Process).Count")
	if err != nil {
		return 0
	}
	v, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return v
}

func getUptime() string {
	out, err := hiddenExec("-NoProfile", "-Command",
		"((Get-Date) - (Get-CimInstance Win32_OperatingSystem).LastBootUpTime).TotalSeconds")
	if err != nil {
		return ""
	}
	secs, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	days := int(secs) / 86400
	hours := (int(secs) % 86400) / 3600
	return fmt.Sprintf("%d天%d小时", days, hours)
}

func getNetTraffic() (rxPerSec, txPerSec int64) {
	cmd := `$a=Get-CimInstance Win32_PerfRawData_Tcpip_NetworkInterface | Where-Object {$_.Name -notlike '*Loopback*'};` +
		`$rx=($a | Measure-Object -Property BytesReceivedPersec -Sum).Sum;` +
		`$tx=($a | Measure-Object -Property BytesSentPersec -Sum).Sum;` +
		`Start-Sleep -Seconds 1;` +
		`$b=Get-CimInstance Win32_PerfRawData_Tcpip_NetworkInterface | Where-Object {$_.Name -notlike '*Loopback*'};` +
		`$rx2=($b | Measure-Object -Property BytesReceivedPersec -Sum).Sum;` +
		`$tx2=($b | Measure-Object -Property BytesSentPersec -Sum).Sum;` +
		`"$($rx2-$rx),$($tx2-$tx)"`

	out, err := hiddenExec("-NoProfile", "-Command", cmd)
	if err != nil {
		return
	}
	parts := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(parts) >= 2 {
		rxPerSec, _ = strconv.ParseInt(parts[0], 10, 64)
		txPerSec, _ = strconv.ParseInt(parts[1], 10, 64)
	}
	if rxPerSec < 0 {
		rxPerSec = 0
	}
	if txPerSec < 0 {
		txPerSec = 0
	}

	// Windows PerfRawData 已经是每秒值，但两次采样差值更准确
	// 如果差值为0但有流量，取绝对值
	return
}

// Windows 没有信号处理的特殊需求，getCPUUsage 内部已有 sleep
// 网络采集 PowerShell 内部已有 1 秒 sleep，不需要额外等待
func init() {
	// 确保 PowerShell 可用
	_, err := exec.LookPath("powershell")
	if err != nil {
		fmt.Println("警告: 未找到 PowerShell，部分指标可能无法采集")
	}

	// 等待一小段时间让 WMI 初始化
	time.Sleep(100 * time.Millisecond)
}
