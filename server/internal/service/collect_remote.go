package service

import (
	"fmt"
	"server-monitor/internal/model"
	"strconv"
	"strings"
	"time"
)

// Linux 采集命令：CPU(1秒采样)、内存、磁盘、负载、进程数、运行时间、网络流量
const linuxCollectCmd = `
N1=$(awk 'NR>2&&!/lo/{gsub(":"," ");rx+=$2;tx+=$10}END{printf "%d %d",rx,tx}' /proc/net/dev 2>/dev/null)
C1=$(awk 'NR==1{printf "%d %d",$2+$4,$2+$4+$5}' /proc/stat)
sleep 1
N2=$(awk 'NR>2&&!/lo/{gsub(":"," ");rx+=$2;tx+=$10}END{printf "%d %d",rx,tx}' /proc/net/dev 2>/dev/null)
C2=$(awk 'NR==1{printf "%d %d",$2+$4,$2+$4+$5}' /proc/stat)
echo "CPU:$(echo $C1 $C2 | awk '{if($4-$2>0)printf "%.1f",($3-$1)*100/($4-$2);else print "0.0"}')"
free -m 2>/dev/null | awk '/Mem:/{printf "MEM:%d:%d:%.1f\n",$2,$3,($2>0?$3/$2*100:0)}'
df -BG --total 2>/dev/null | awk '/^total/{gsub("G","",$2);gsub("G","",$3);printf "DISK:%d:%d:%.1f\n",$2,$3,($2>0?$3/$2*100:0)}'
awk '{printf "LOAD:%.2f:%.2f:%.2f\n",$1,$2,$3}' /proc/loadavg 2>/dev/null
echo "PROC:$(ps -e --no-headers 2>/dev/null | wc -l | tr -d ' ')"
awk '{d=int($1/86400);h=int(($1%86400)/3600);printf "UPTIME:%d天%d小时\n",d,h}' /proc/uptime 2>/dev/null
echo $N1 $N2 | awk '{printf "NET:%d:%d\n",($3>$1?$3-$1:0),($4>$2?$4-$2:0)}'
`

func parseLinuxMetrics(serverID, output string) (*model.Metric, error) {
	m := &model.Metric{
		ServerID:    serverID,
		CollectedAt: time.Now(),
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	parsed := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "CPU:") {
			val := strings.TrimPrefix(line, "CPU:")
			if v, err := strconv.ParseFloat(val, 64); err == nil {
				m.CPUUsage = round2(v)
				parsed++
			}
		} else if strings.HasPrefix(line, "MEM:") {
			parts := strings.Split(strings.TrimPrefix(line, "MEM:"), ":")
			if len(parts) >= 3 {
				m.MemTotal, _ = strconv.ParseInt(parts[0], 10, 64)
				m.MemUsed, _ = strconv.ParseInt(parts[1], 10, 64)
				m.MemUsage, _ = strconv.ParseFloat(parts[2], 64)
				m.MemUsage = round2(m.MemUsage)
				parsed++
			}
		} else if strings.HasPrefix(line, "DISK:") {
			parts := strings.Split(strings.TrimPrefix(line, "DISK:"), ":")
			if len(parts) >= 3 {
				m.DiskTotal, _ = strconv.ParseInt(parts[0], 10, 64)
				m.DiskUsed, _ = strconv.ParseInt(parts[1], 10, 64)
				m.DiskUsage, _ = strconv.ParseFloat(parts[2], 64)
				m.DiskUsage = round2(m.DiskUsage)
				parsed++
			}
		} else if strings.HasPrefix(line, "LOAD:") {
			parts := strings.Split(strings.TrimPrefix(line, "LOAD:"), ":")
			if len(parts) >= 3 {
				m.Load1m, _ = strconv.ParseFloat(parts[0], 64)
				m.Load5m, _ = strconv.ParseFloat(parts[1], 64)
				m.Load15m, _ = strconv.ParseFloat(parts[2], 64)
				parsed++
			}
		} else if strings.HasPrefix(line, "PROC:") {
			val := strings.TrimPrefix(line, "PROC:")
			if v, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
				m.ProcessCount = v
				parsed++
			}
		} else if strings.HasPrefix(line, "UPTIME:") {
			m.Uptime = strings.TrimPrefix(line, "UPTIME:")
			parsed++
		} else if strings.HasPrefix(line, "NET:") {
			parts := strings.Split(strings.TrimPrefix(line, "NET:"), ":")
			if len(parts) >= 2 {
				m.NetIn, _ = strconv.ParseInt(parts[0], 10, 64)
				m.NetOut, _ = strconv.ParseInt(parts[1], 10, 64)
				parsed++
			}
		}
	}

	if parsed < 3 {
		return m, fmt.Errorf("解析指标不完整, 仅解析到 %d 项, 原始输出: %s", parsed, output)
	}

	return m, nil
}
