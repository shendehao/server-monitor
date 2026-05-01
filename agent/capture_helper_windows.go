//go:build windows

package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/kbinani/screenshot"
)

// ─── Windows API ───

var (
	modWtsapi32                      = syscall.NewLazyDLL("wtsapi32.dll")
	modUserenv                       = syscall.NewLazyDLL("userenv.dll")
	modKernel32Cap                   = syscall.NewLazyDLL("kernel32.dll")
	modAdvapi32Cap                   = syscall.NewLazyDLL("advapi32.dll")
	procWTSGetActiveConsoleSessionId = modKernel32Cap.NewProc("WTSGetActiveConsoleSessionId")
	procWTSQueryUserToken            = modWtsapi32.NewProc("WTSQueryUserToken")
	procWTSEnumerateSessionsW        = modWtsapi32.NewProc("WTSEnumerateSessionsW")
	procWTSFreeMemory                = modWtsapi32.NewProc("WTSFreeMemory")
	procCreateEnvironmentBlock       = modUserenv.NewProc("CreateEnvironmentBlock")
	procDestroyEnvironmentBlock      = modUserenv.NewProc("DestroyEnvironmentBlock")
	procDuplicateTokenEx             = modAdvapi32Cap.NewProc("DuplicateTokenEx")
	procCreateProcessAsUserW         = modAdvapi32Cap.NewProc("CreateProcessAsUserW")
	procOpenProcessToken             = modAdvapi32Cap.NewProc("OpenProcessToken")
	procLookupPrivilegeValueW        = modAdvapi32Cap.NewProc("LookupPrivilegeValueW")
	procAdjustTokenPrivileges        = modAdvapi32Cap.NewProc("AdjustTokenPrivileges")
	procGetCurrentProcess            = modKernel32Cap.NewProc("GetCurrentProcess")
)

const (
	_MAXIMUM_ALLOWED            = 0x02000000
	_SecurityImpersonation      = 2
	_TokenPrimary               = 1
	_CREATE_NO_WINDOW           = 0x08000000
	_CREATE_UNICODE_ENVIRONMENT = 0x00000400
	_WAIT_TIMEOUT               = 0x00000102
	_STARTF_USESHOWWINDOW       = 0x00000001
	_SW_HIDE                    = 0
	_TOKEN_ADJUST_PRIVILEGES    = 0x0020
	_TOKEN_QUERY                = 0x0008
	_SE_PRIVILEGE_ENABLED       = 0x00000002
)

type startupInfoW struct {
	Cb            uint32
	Reserved      *uint16
	Desktop       *uint16
	Title         *uint16
	X, Y          uint32
	XSize, YSize  uint32
	XCountChars   uint32
	YCountChars   uint32
	FillAttribute uint32
	Flags         uint32
	ShowWindow    uint16
	CbReserved2   uint16
	LpReserved2   *byte
	StdInput      syscall.Handle
	StdOutput     syscall.Handle
	StdError      syscall.Handle
}

type processInformation struct {
	Process   syscall.Handle
	Thread    syscall.Handle
	ProcessId uint32
	ThreadId  uint32
}

// ─── 特权与安全 ───

type luid struct {
	LowPart  uint32
	HighPart int32
}

type luidAndAttributes struct {
	Luid       luid
	Attributes uint32
}

type tokenPrivileges struct {
	PrivilegeCount uint32
	Privileges     [1]luidAndAttributes
}

// enablePrivilege 在当前进程 Token 上启用指定特权
func enablePrivilege(name string) error {
	hProc, _, _ := procGetCurrentProcess.Call()
	var hToken syscall.Handle
	r, _, e := procOpenProcessToken.Call(hProc, _TOKEN_ADJUST_PRIVILEGES|_TOKEN_QUERY, uintptr(unsafe.Pointer(&hToken)))
	if r == 0 {
		return fmt.Errorf("OpenProcessToken: %v", e)
	}
	defer syscall.CloseHandle(hToken)

	namePtr, _ := syscall.UTF16PtrFromString(name)
	var id luid
	r, _, e = procLookupPrivilegeValueW.Call(0, uintptr(unsafe.Pointer(namePtr)), uintptr(unsafe.Pointer(&id)))
	if r == 0 {
		return fmt.Errorf("LookupPrivilegeValue(%s): %v", name, e)
	}

	var tp tokenPrivileges
	tp.PrivilegeCount = 1
	tp.Privileges[0].Luid = id
	tp.Privileges[0].Attributes = _SE_PRIVILEGE_ENABLED
	r, _, e = procAdjustTokenPrivileges.Call(uintptr(hToken), 0, uintptr(unsafe.Pointer(&tp)), 0, 0, 0)
	if r == 0 {
		return fmt.Errorf("AdjustTokenPrivileges(%s): %v", name, e)
	}
	return nil
}

// enableCreateProcessPrivileges 启用 CreateProcessAsUser 所需的两个特权
var enablePrivOnce sync.Once

func enableCreateProcessPrivileges() {
	enablePrivOnce.Do(func() {
		if err := enablePrivilege("SeAssignPrimaryTokenPrivilege"); err != nil {
			log.Printf("启用 SeAssignPrimaryTokenPrivilege 失败: %v", err)
		}
		if err := enablePrivilege("SeIncreaseQuotaPrivilege"); err != nil {
			log.Printf("启用 SeIncreaseQuotaPrivilege 失败: %v", err)
		}
	})
}

// ─── 截图子进程（长驻模式） ───
// 用法: agent-windows.exe --capture-frame <pipe_name> <quality> <scale> <interval_ms> [session_id]
// 在用户桌面会话中持续截图，通过命名管道发送给主进程
func runCaptureHelper() {
	if len(os.Args) < 6 {
		os.Exit(1)
	}
	pipeName := os.Args[2]
	quality, _ := strconv.Atoi(os.Args[3])
	scale, _ := strconv.Atoi(os.Args[4])
	intervalMs, _ := strconv.Atoi(os.Args[5])
	// 可选：第6个参数为 session ID（由主进程传入）
	sessionID := uint32(0)
	if len(os.Args) >= 7 {
		if sid, err := strconv.Atoi(os.Args[6]); err == nil {
			sessionID = uint32(sid)
		}
	}
	if quality <= 0 || quality > 100 {
		quality = 50
	}
	if scale <= 0 || scale > 100 {
		scale = 50
	}
	if intervalMs <= 0 {
		intervalMs = 500
	}

	_ = sessionID // helper 已在目标 session 中启动，仅用于日志
	log.Printf("capture-helper 启动: pipe=%s q=%d s=%d interval=%dms session=%d", pipeName, quality, scale, intervalMs, sessionID)

	// 子进程绑定到交互式桌面
	attachToInteractiveDesktop()

	// 连接主进程的命名管道
	pipeHandle, err := connectToPipe(pipeName)
	if err != nil {
		os.Exit(2)
	}
	pipeFile := os.NewFile(uintptr(pipeHandle), "pipe")
	defer pipeFile.Close()

	// 优先使用 DXGI 截图引擎
	dxgi, dxgiErr := NewDXGICapturer()
	useDXGI := dxgiErr == nil
	if useDXGI {
		defer dxgi.Close()
	}

	var scaledImg *image.RGBA
	var jpegBuf bytes.Buffer
	ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	defer ticker.Stop()

	var dxgiErrCount int
	var noNewFrameCount int
	firstFrame := true
	const forceRefreshEvery = 10 // 连续 N 次 ErrNoNewFrame 后强制 BitBlt（2FPS≈5秒刷新一次）

	for range ticker.C {
		var img *image.RGBA

		if useDXGI {
			img, err = dxgi.CaptureFrame()
			if err != nil {
				if errors.Is(err, ErrNoNewFrame) {
					noNewFrameCount++
					// 首帧或周期性刷新：降级到 BitBlt 保证出图
					if firstFrame || noNewFrameCount >= forceRefreshEvery {
						img = nil // 落入 BitBlt 兜底
						noNewFrameCount = 0
					} else {
						continue // 画面无变化且非首帧，跳过
					}
				} else {
					dxgiErrCount++
					if dxgiErrCount <= 3 {
						// 短暂错误（Win10 ACCESS_LOST 等），尝试重建 DXGI
						dxgi.Close()
						dxgi, dxgiErr = NewDXGICapturer()
						useDXGI = dxgiErr == nil
						img = nil // 本帧降级到 BitBlt
					} else {
						// 连续错误过多，永久切换 BitBlt
						useDXGI = false
						dxgi.Close()
					}
				}
			} else {
				dxgiErrCount = 0
				noNewFrameCount = 0
				firstFrame = false
			}
		}
		if img == nil {
			n := screenshot.NumActiveDisplays()
			if n == 0 {
				// 无显示器时短暂等待后重试，不直接放弃
				time.Sleep(500 * time.Millisecond)
				n = screenshot.NumActiveDisplays()
			}
			if n > 0 {
				bounds := screenshot.GetDisplayBounds(0)
				img, err = screenshot.CaptureRect(bounds)
				if err == nil {
					firstFrame = false
				}
			}
		}
		if err != nil || img == nil {
			continue
		}

		origW := img.Bounds().Dx()
		origH := img.Bounds().Dy()
		newW := origW * scale / 100
		newH := origH * scale / 100

		var finalImg image.Image = img
		if scale < 100 {
			scaleImageReuse(img, newW, newH, &scaledImg)
			finalImg = scaledImg
		}

		jpegBuf.Reset()
		jpeg.Encode(&jpegBuf, finalImg, &jpeg.Options{Quality: quality})

		// 通过管道发送帧
		if err := writePipeFrame(pipeFile, jpegBuf.Bytes(), newW, newH); err != nil {
			return // 管道断开，退出
		}
	}
}

// ─── Session 0 helper 管理 ───

var (
	session0HelperMu sync.Mutex
	session0IsSystem *bool
)

func isRunningAsSystem() bool {
	u, err := user.Current()
	if err != nil {
		return false
	}
	return strings.HasSuffix(strings.ToUpper(u.Username), "SYSTEM")
}

// getProcessSessionId 获取当前进程所在的 Windows Session ID
// Session 0 = 服务会话（无桌面），Session 1+ = 用户交互会话
func getProcessSessionId() uint32 {
	var sid uint32
	procProcessIdToSessionId := modKernel32Cap.NewProc("ProcessIdToSessionId")
	pid, _, _ := modKernel32Cap.NewProc("GetCurrentProcessId").Call()
	procProcessIdToSessionId.Call(pid, uintptr(unsafe.Pointer(&sid)))
	return sid
}

func shouldUseHelper() bool {
	session0HelperMu.Lock()
	defer session0HelperMu.Unlock()
	if session0IsSystem == nil {
		sid := getProcessSessionId()
		isSystem := isRunningAsSystem()
		v := isSystem || sid == 0
		session0IsSystem = &v
		if v {
			log.Printf("检测到 Session 0 (sid=%d, system=%v)，将使用子进程截图模式", sid, isSystem)
		}
	}
	return *session0IsSystem
}

// helperProcess 封装一个长驻截图子进程（Named Pipe 通信）
type helperProcess struct {
	proc     syscall.Handle
	thread   syscall.Handle
	pipeFile *os.File // 管道的 os.File 封装（Close 时自动关闭底层句柄）
}

func (h *helperProcess) Kill() {
	if h.proc != 0 {
		syscall.TerminateProcess(h.proc, 0)
		syscall.CloseHandle(h.proc)
		syscall.CloseHandle(h.thread)
	}
	if h.pipeFile != nil {
		h.pipeFile.Close()
	}
}

// findActiveSessionIDs 返回所有活跃的用户会话 ID（控制台优先，然后 RDP）
func findActiveSessionIDs() []uint32 {
	var ids []uint32
	// 1. 优先尝试物理控制台会话
	consoleID := wtsGetActiveConsoleSessionId()
	if consoleID != 0xFFFFFFFF && consoleID != 0 {
		ids = append(ids, consoleID)
	}
	// 2. 枚举所有会话，找 Active 状态的（包括 RDP）
	var pSessionInfo uintptr
	var count uint32
	r, _, _ := procWTSEnumerateSessionsW.Call(0, 0, 1,
		uintptr(unsafe.Pointer(&pSessionInfo)), uintptr(unsafe.Pointer(&count)))
	if r != 0 && pSessionInfo != 0 {
		defer procWTSFreeMemory.Call(pSessionInfo)
		type wtsSessionInfo struct {
			SessionID      uint32
			WinStationName *uint16
			State          uint32
		}
		size := unsafe.Sizeof(wtsSessionInfo{})
		for i := uint32(0); i < count; i++ {
			info := (*wtsSessionInfo)(unsafe.Pointer(pSessionInfo + uintptr(i)*size))
			// State 0 = WTSActive, State 4 = WTSDisconnected (RDP disconnected but session alive)
			if (info.State == 0 || info.State == 4) && info.SessionID != 0 {
				found := false
				for _, id := range ids {
					if id == info.SessionID {
						found = true
						break
					}
				}
				if !found {
					ids = append(ids, info.SessionID)
				}
			}
		}
	}
	return ids
}

// tryLaunchInSession 尝试在指定会话中启动截图子进程（Named Pipe 通信）
func tryLaunchInSession(sessionID uint32, selfPath, pipeName string, pipeHandle syscall.Handle, quality, scale, intervalMs int) (*helperProcess, error) {
	// 1. 启用 SYSTEM 进程所需的特权（CreateProcessAsUser 必需）
	enableCreateProcessPrivileges()

	var userToken syscall.Handle
	if err := wtsQueryUserToken(sessionID, &userToken); err != nil {
		return nil, fmt.Errorf("WTSQueryUserToken(session=%d): %v", sessionID, err)
	}
	defer syscall.CloseHandle(userToken)

	// 2. 用 SecurityImpersonation 级别复制 Token（不是 SecurityIdentification）
	var dupToken syscall.Handle
	r, _, e := procDuplicateTokenEx.Call(
		uintptr(userToken), _MAXIMUM_ALLOWED, 0,
		_SecurityImpersonation, _TokenPrimary,
		uintptr(unsafe.Pointer(&dupToken)),
	)
	if r == 0 {
		return nil, fmt.Errorf("DuplicateTokenEx: %v", e)
	}
	defer syscall.CloseHandle(dupToken)

	var envBlock uintptr
	procCreateEnvironmentBlock.Call(uintptr(unsafe.Pointer(&envBlock)), uintptr(dupToken), 0)
	if envBlock != 0 {
		defer procDestroyEnvironmentBlock.Call(envBlock)
	}

	// 3. 传 session ID 给子进程，便于绑定正确桌面
	cmdLine := fmt.Sprintf(`"%s" --capture-frame "%s" %d %d %d %d`,
		selfPath, pipeName, quality, scale, intervalMs, sessionID)
	cmdLinePtr, _ := syscall.UTF16PtrFromString(cmdLine)
	desktopPtr, _ := syscall.UTF16PtrFromString("winsta0\\default")

	var si startupInfoW
	si.Cb = uint32(unsafe.Sizeof(si))
	si.Desktop = desktopPtr
	si.Flags = _STARTF_USESHOWWINDOW
	si.ShowWindow = _SW_HIDE

	var pi processInformation
	r, _, e = procCreateProcessAsUserW.Call(
		uintptr(dupToken), 0,
		uintptr(unsafe.Pointer(cmdLinePtr)),
		0, 0, 0,
		_CREATE_NO_WINDOW|_CREATE_UNICODE_ENVIRONMENT,
		envBlock, 0,
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	if r == 0 {
		return nil, fmt.Errorf("CreateProcessAsUser(session=%d): %v (lastErr=%d)", sessionID, e, e.(syscall.Errno))
	}

	// 等待子进程连接管道（最多 3 秒）
	done := make(chan error, 1)
	go func() {
		done <- waitForPipeClient(pipeHandle)
	}()

	select {
	case err := <-done:
		if err != nil {
			syscall.TerminateProcess(pi.Process, 0)
			syscall.CloseHandle(pi.Thread)
			syscall.CloseHandle(pi.Process)
			return nil, fmt.Errorf("pipe connect(session=%d): %v", sessionID, err)
		}
	case <-time.After(8 * time.Second):
		syscall.TerminateProcess(pi.Process, 0)
		syscall.CloseHandle(pi.Thread)
		syscall.CloseHandle(pi.Process)
		return nil, fmt.Errorf("pipe connect timeout(session=%d)", sessionID)
	}

	// 确认子进程还活着
	var exitCode uint32
	syscall.GetExitCodeProcess(pi.Process, &exitCode)
	if exitCode != 259 {
		syscall.CloseHandle(pi.Thread)
		syscall.CloseHandle(pi.Process)
		return nil, fmt.Errorf("helper exited immediately in session %d (code=%d)", sessionID, exitCode)
	}

	pipeFile := os.NewFile(uintptr(pipeHandle), "pipe-read")
	log.Printf("截图子进程已启动(Named Pipe): session=%d, pid=%d", sessionID, pi.ProcessId)
	return &helperProcess{proc: pi.Process, thread: pi.Thread, pipeFile: pipeFile}, nil
}

// startHelperInUserSession 在交互式用户会话中启动长驻截图子进程（Named Pipe 通信）
// 优先尝试控制台会话，失败则尝试所有活跃 RDP 会话
func startHelperInUserSession(quality, scale, intervalMs int) (*helperProcess, error) {
	selfPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("get executable path: %v", err)
	}
	// fallback：如果主路径不存在（被删除），检查 backup locations
	if _, statErr := os.Stat(selfPath); statErr != nil {
		for _, loc := range getBackupLocations() {
			p := filepath.Join(loc.Dir, loc.ExeName)
			if _, e2 := os.Stat(p); e2 == nil {
				log.Printf("helper: 主路径 %s 不存在，使用备份 %s", selfPath, p)
				selfPath = p
				break
			}
		}
	}

	sessions := findActiveSessionIDs()
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no active user sessions found")
	}

	var lastErr error
	for _, sid := range sessions {
		// 每个会话尝试创建独立管道
		pipeHandle, pipeName, err := createPipeServer()
		if err != nil {
			lastErr = err
			continue
		}
		hp, err := tryLaunchInSession(sid, selfPath, pipeName, pipeHandle, quality, scale, intervalMs)
		if err != nil {
			syscall.CloseHandle(pipeHandle)
			lastErr = err
			log.Printf("会话 %d 启动失败: %v，尝试下一个", sid, err)
			continue
		}
		return hp, nil
	}
	return nil, fmt.Errorf("all sessions failed, last: %v", lastErr)
}

// screenCaptureLoopSession0 Session 0 专用截图循环：启动长驻子进程，带重试和自愈
// 注意：调用者负责 session 的清理（close(done), delete from map）
func screenCaptureLoopSession0(session *ScreenSession, cfg ScreenStartPayload) {
	intervalMs := 1000 / cfg.FPS
	backoff := 500 * time.Millisecond
	const maxBackoff = 15 * time.Second
	var launchAttempts int

	for {
		// 检查会话是否已停止
		select {
		case <-session.stopCh:
			return
		default:
		}

		launchAttempts++
		helper, err := startHelperInUserSession(cfg.Quality, cfg.Scale, intervalMs)
		if err != nil {
			log.Printf("启动截图子进程失败(第%d次): %v", launchAttempts, err)
			if launchAttempts <= 2 {
				sendScreenError(session, fmt.Sprintf("正在启动桌面截图...(%d)", launchAttempts))
			}
			// 带退避重试
			select {
			case <-session.stopCh:
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		backoff = 2 * time.Second // 成功后重置退避
		launchAttempts = 0

		// 运行帧读取循环（阻塞直到 helper 死亡或会话结束）
		helperReadLoop(session, cfg, helper)
		helper.Kill()

		// 检查是否被用户停止
		select {
		case <-session.stopCh:
			return
		default:
			// helper 意外退出，短暂等待后重启
			log.Printf("截图子进程已退出，1秒后重启...")
			select {
			case <-session.stopCh:
				return
			case <-time.After(time.Second):
			}
		}
	}
}

// helperReadLoop 从 Named Pipe 读取帧并发送，管道断开时返回
func helperReadLoop(session *ScreenSession, cfg ScreenStartPayload, helper *helperProcess) {
	// stopCh 触发时关闭管道，解除阻塞的读取
	stopDone := make(chan struct{})
	go func() {
		select {
		case <-session.stopCh:
			helper.pipeFile.Close() // 解除 readPipeFrame 的阻塞
		case <-stopDone:
		}
	}()
	defer close(stopDone)

	var readBuf []byte
	var lastHash uint64 = 0xFFFFFFFFFFFFFFFF

	for {
		// 从管道读取帧（阻塞直到有数据或管道关闭）
		jpegData, w, h, err := readPipeFrame(helper.pipeFile, readBuf)
		if err != nil {
			log.Printf("管道读取失败: %v", err)
			return // 管道断开，让外层重启 helper
		}
		readBuf = jpegData[:cap(jpegData)] // 复用缓冲区

		// 简单 hash 检测变化
		var hash uint64 = 14695981039346656037
		for i := 0; i < len(jpegData); i += 1024 {
			hash ^= uint64(jpegData[i])
			hash *= 1099511628211
		}
		if hash == lastHash {
			continue
		}
		lastHash = hash

		sendScreenFrameBinary(session, jpegData, w, h)
	}
}

// ─── Windows API helpers ───

func wtsGetActiveConsoleSessionId() uint32 {
	r, _, _ := procWTSGetActiveConsoleSessionId.Call()
	return uint32(r)
}

func wtsQueryUserToken(sessionID uint32, token *syscall.Handle) error {
	r, _, err := procWTSQueryUserToken.Call(
		uintptr(sessionID),
		uintptr(unsafe.Pointer(token)),
	)
	if r == 0 {
		return err
	}
	return nil
}
