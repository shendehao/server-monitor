package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// ═══ 服务管理模块 ═══

type ServiceInfo struct {
	Name    string `json:"name"`
	Display string `json:"display"`
	State   string `json:"state"`
	Start   string `json:"start"`
	PID     int    `json:"pid"`
}

func handleServiceList(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	services := gatherServiceList()
	result, _ := json.Marshal(map[string]interface{}{
		"services": services,
	})
	sendResult(conn, writeMu, "service_list_result", msg.ID, string(result))
}

func handleServiceControl(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	var req struct {
		Name   string `json:"name"`
		Action string `json:"action"`
	}
	json.Unmarshal(msg.Payload, &req)

	if req.Name == "" || req.Action == "" {
		sendResult(conn, writeMu, "service_control_result", msg.ID,
			`{"success":false,"message":"缺少服务名或操作"}`)
		return
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		switch req.Action {
		case "start":
			cmd = exec.Command("sc", "start", req.Name)
		case "stop":
			cmd = exec.Command("sc", "stop", req.Name)
		case "restart":
			// Windows: stop then start
			stopCmd := exec.Command("sc", "stop", req.Name)
			hideWindow(stopCmd)
			stopCmd.CombinedOutput()
			cmd = exec.Command("sc", "start", req.Name)
		default:
			sendResult(conn, writeMu, "service_control_result", msg.ID,
				fmt.Sprintf(`{"success":false,"message":"未知操作: %s"}`, jsonEsc(req.Action)))
			return
		}
		hideWindow(cmd)
	} else {
		switch req.Action {
		case "start", "stop", "restart":
			cmd = exec.Command("systemctl", req.Action, req.Name)
		default:
			sendResult(conn, writeMu, "service_control_result", msg.ID,
				fmt.Sprintf(`{"success":false,"message":"未知操作: %s"}`, jsonEsc(req.Action)))
			return
		}
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		sendResult(conn, writeMu, "service_control_result", msg.ID,
			fmt.Sprintf(`{"success":false,"message":"%s"}`, jsonEsc(strings.TrimSpace(string(out))+" "+err.Error())))
		return
	}
	sendResult(conn, writeMu, "service_control_result", msg.ID,
		`{"success":true,"message":"操作成功"}`)
}

func gatherServiceList() []ServiceInfo {
	if runtime.GOOS == "windows" {
		return gatherServiceListWindows()
	}
	return gatherServiceListLinux()
}

func gatherServiceListLinux() []ServiceInfo {
	var services []ServiceInfo
	cmd := exec.Command("systemctl", "list-units", "--type=service", "--all", "--no-pager", "--no-legend")
	out, err := cmd.Output()
	if err != nil {
		return services
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		name := strings.TrimSuffix(fields[0], ".service")
		subState := fields[3] // running/dead/exited
		state := "Stopped"
		if subState == "running" {
			state = "Running"
		}
		display := ""
		if len(fields) > 4 {
			display = strings.Join(fields[4:], " ")
		}
		services = append(services, ServiceInfo{
			Name:    name,
			Display: display,
			State:   state,
		})
	}
	return services
}

func gatherServiceListWindows() []ServiceInfo {
	var services []ServiceInfo
	// 使用 wmic 获取服务信息（与 C# DLL 使用的 WMI 一致）
	cmd := exec.Command("wmic", "service", "get", "Name,DisplayName,State,StartMode,ProcessId", "/format:csv")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return gatherServiceListWindowsFallback()
	}

	utf8Out := oemToUTF8(out)
	lines := strings.Split(utf8Out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Node,") {
			continue
		}
		// CSV 字段按字母排序: Node,DisplayName,Name,ProcessId,StartMode,State
		parts := strings.SplitN(line, ",", 7)
		if len(parts) < 6 {
			continue
		}
		display := strings.TrimSpace(parts[1])
		name := strings.TrimSpace(parts[2])
		pidVal := 0
		fmt.Sscanf(strings.TrimSpace(parts[3]), "%d", &pidVal)
		startMode := strings.TrimSpace(parts[4])
		state := strings.TrimSpace(parts[5])

		services = append(services, ServiceInfo{
			Name:    name,
			Display: display,
			State:   state,
			Start:   startMode,
			PID:     pidVal,
		})
	}
	if len(services) == 0 {
		return gatherServiceListWindowsFallback()
	}
	return services
}

func gatherServiceListWindowsFallback() []ServiceInfo {
	var services []ServiceInfo
	cmd := exec.Command("sc", "query", "type=", "service", "state=", "all")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return services
	}

	utf8Out := oemToUTF8(out)
	var current ServiceInfo
	for _, line := range strings.Split(utf8Out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SERVICE_NAME:") {
			if current.Name != "" {
				services = append(services, current)
			}
			current = ServiceInfo{Name: strings.TrimSpace(strings.TrimPrefix(line, "SERVICE_NAME:"))}
		} else if strings.HasPrefix(line, "DISPLAY_NAME:") {
			current.Display = strings.TrimSpace(strings.TrimPrefix(line, "DISPLAY_NAME:"))
		} else if strings.Contains(line, "STATE") && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				state := strings.TrimSpace(parts[1])
				fields := strings.Fields(state)
				if len(fields) >= 2 {
					// RUNNING → Running, STOPPED → Stopped
					raw := fields[1]
					if len(raw) > 1 {
						current.State = strings.ToUpper(raw[:1]) + strings.ToLower(raw[1:])
					}
				}
			}
		}
	}
	if current.Name != "" {
		services = append(services, current)
	}
	return services
}
