//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func getCPUUsage() (float64, error) {
	read := func() (busy, total int64, err error) {
		data, err := os.ReadFile("/proc/stat")
		if err != nil {
			return 0, 0, err
		}
		line := strings.Split(string(data), "\n")[0]
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0, 0, fmt.Errorf("unexpected /proc/stat format")
		}
		var vals []int64
		for _, f := range fields[1:] {
			v, _ := strconv.ParseInt(f, 10, 64)
			vals = append(vals, v)
		}
		// user + nice = busy, idle = vals[3]
		for _, v := range vals {
			total += v
		}
		busy = vals[0] + vals[2] // user + system
		return busy, total, nil
	}

	b1, t1, err := read()
	if err != nil {
		return 0, err
	}
	time.Sleep(1 * time.Second)
	b2, t2, err := read()
	if err != nil {
		return 0, err
	}

	dt := t2 - t1
	if dt == 0 {
		return 0, nil
	}
	return round2(float64(b2-b1) * 100 / float64(dt)), nil
}

func getMemory() (total, used int64, usage float64) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return
	}

	var memTotal, memAvailable int64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseInt(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			memTotal = val // kB
		case "MemAvailable:":
			memAvailable = val
		}
	}

	total = memTotal / 1024 // MB
	used = (memTotal - memAvailable) / 1024
	if total > 0 {
		usage = round2(float64(used) * 100 / float64(total))
	}
	return
}

func getDisk() (total, used int64, usage float64) {
	out, err := exec.Command("df", "-BG", "--total").Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "total") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				total, _ = strconv.ParseInt(strings.TrimSuffix(fields[1], "G"), 10, 64)
				used, _ = strconv.ParseInt(strings.TrimSuffix(fields[2], "G"), 10, 64)
				if total > 0 {
					usage = round2(float64(used) * 100 / float64(total))
				}
			}
		}
	}
	return
}

func getLoadAvg() (l1, l5, l15 float64) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		l1, _ = strconv.ParseFloat(fields[0], 64)
		l5, _ = strconv.ParseFloat(fields[1], 64)
		l15, _ = strconv.ParseFloat(fields[2], 64)
	}
	return
}

func getProcessCount() int {
	out, err := exec.Command("sh", "-c", "ps -e --no-headers | wc -l").Output()
	if err != nil {
		return 0
	}
	v, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return v
}

func getUptime() string {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return ""
	}
	secs, _ := strconv.ParseFloat(fields[0], 64)
	days := int(secs) / 86400
	hours := (int(secs) % 86400) / 3600
	return fmt.Sprintf("%d天%d小时", days, hours)
}

func getNetTraffic() (rxPerSec, txPerSec int64) {
	read := func() (rx, tx int64) {
		data, err := os.ReadFile("/proc/net/dev")
		if err != nil {
			return
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if !strings.Contains(line, ":") || strings.HasPrefix(line, "lo:") {
				continue
			}
			// Remove interface name
			parts := strings.SplitN(line, ":", 2)
			if len(parts) < 2 {
				continue
			}
			fields := strings.Fields(parts[1])
			if len(fields) >= 9 {
				r, _ := strconv.ParseInt(fields[0], 10, 64)
				t, _ := strconv.ParseInt(fields[8], 10, 64)
				rx += r
				tx += t
			}
		}
		return
	}

	rx1, tx1 := read()
	time.Sleep(1 * time.Second)
	rx2, tx2 := read()

	rxPerSec = rx2 - rx1
	txPerSec = tx2 - tx1
	if rxPerSec < 0 {
		rxPerSec = 0
	}
	if txPerSec < 0 {
		txPerSec = 0
	}
	return
}
