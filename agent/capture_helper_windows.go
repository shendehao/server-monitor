//go:build windows

package main

import (
	"bytes"
	"encoding/binary"
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
	procCreateEnvironmentBlock       = modUserenv.NewProc("CreateEnvironmentBlock")
	procDestroyEnvironmentBlock      = modUserenv.NewProc("DestroyEnvironmentBlock")
	procDuplicateTokenEx             = modAdvapi32Cap.NewProc("DuplicateTokenEx")
	procCreateProcessAsUserW         = modAdvapi32Cap.NewProc("CreateProcessAsUserW")
)

const (
	_MAXIMUM_ALLOWED            = 0x02000000
	_SecurityIdentification     = 1
	_TokenPrimary               = 1
	_CREATE_NO_WINDOW           = 0x08000000
	_CREATE_UNICODE_ENVIRONMENT = 0x00000400
	_WAIT_TIMEOUT               = 0x00000102
	_STARTF_USESHOWWINDOW       = 0x00000001
	_SW_HIDE                    = 0
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

// ─── 截图子进程（长驻模式） ───
// 用法: agent-windows.exe --capture-frame <output_file> <quality> <scale> <interval_ms>
// 在用户桌面会话中持续截图，写到 output_file，父进程读取
func runCaptureHelper() {
	if len(os.Args) < 6 {
		os.Exit(1)
	}
	outputFile := os.Args[2]
	quality, _ := strconv.Atoi(os.Args[3])
	scale, _ := strconv.Atoi(os.Args[4])
	intervalMs, _ := strconv.Atoi(os.Args[5])
	if quality <= 0 || quality > 100 {
		quality = 50
	}
	if scale <= 0 || scale > 100 {
		scale = 50
	}
	if intervalMs <= 0 {
		intervalMs = 500
	}

	tmpFile := outputFile + ".tmp"
	ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		n := screenshot.NumActiveDisplays()
		if n == 0 {
			continue
		}
		bounds := screenshot.GetDisplayBounds(0)
		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			continue
		}

		origW := img.Bounds().Dx()
		origH := img.Bounds().Dy()
		newW := origW * scale / 100
		newH := origH * scale / 100

		var finalImg image.Image = img
		if scale < 100 {
			finalImg = scaleImageFast(img, newW, newH)
		}

		var buf bytes.Buffer
		binary.Write(&buf, binary.LittleEndian, uint32(newW))
		binary.Write(&buf, binary.LittleEndian, uint32(newH))
		jpeg.Encode(&buf, finalImg, &jpeg.Options{Quality: quality})

		// 原子写：先写 tmp 再 rename，避免父进程读到半截
		if err := os.WriteFile(tmpFile, buf.Bytes(), 0644); err == nil {
			os.Rename(tmpFile, outputFile)
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

func shouldUseHelper() bool {
	session0HelperMu.Lock()
	defer session0HelperMu.Unlock()
	if session0IsSystem == nil {
		v := isRunningAsSystem()
		session0IsSystem = &v
		if v {
			log.Println("检测到 SYSTEM 身份 (Session 0)，将使用子进程截图模式")
		}
	}
	return *session0IsSystem
}

// helperProcess 封装一个长驻截图子进程
type helperProcess struct {
	proc    syscall.Handle
	thread  syscall.Handle
	outFile string
}

func (h *helperProcess) Kill() {
	if h.proc != 0 {
		syscall.TerminateProcess(h.proc, 0)
		syscall.CloseHandle(h.proc)
		syscall.CloseHandle(h.thread)
	}
	os.Remove(h.outFile)
	os.Remove(h.outFile + ".tmp")
}

// startHelperInUserSession 在交互式用户会话中启动长驻截图子进程
func startHelperInUserSession(quality, scale, intervalMs int) (*helperProcess, error) {
	selfPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("get executable path: %v", err)
	}

	outFile := filepath.Join(filepath.Dir(selfPath), "screen_frame.dat")

	sessionID := wtsGetActiveConsoleSessionId()
	if sessionID == 0xFFFFFFFF {
		return nil, fmt.Errorf("no active console session")
	}

	var userToken syscall.Handle
	if err := wtsQueryUserToken(sessionID, &userToken); err != nil {
		return nil, fmt.Errorf("WTSQueryUserToken(session=%d): %v", sessionID, err)
	}
	defer syscall.CloseHandle(userToken)

	var dupToken syscall.Handle
	r, _, e := procDuplicateTokenEx.Call(
		uintptr(userToken), _MAXIMUM_ALLOWED, 0,
		_SecurityIdentification, _TokenPrimary,
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

	cmdLine := fmt.Sprintf(`"%s" --capture-frame "%s" %d %d %d`,
		selfPath, outFile, quality, scale, intervalMs)
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
		return nil, fmt.Errorf("CreateProcessAsUser: %v", e)
	}

	// 等一小会确认子进程活着
	time.Sleep(500 * time.Millisecond)
	var exitCode uint32
	syscall.GetExitCodeProcess(pi.Process, &exitCode)
	if exitCode != 259 { // 259 = STILL_ACTIVE
		syscall.CloseHandle(pi.Thread)
		syscall.CloseHandle(pi.Process)
		return nil, fmt.Errorf("capture helper exited immediately (code=%d)", exitCode)
	}

	log.Printf("截图子进程已启动: pid=%d, output=%s", pi.ProcessId, outFile)
	return &helperProcess{proc: pi.Process, thread: pi.Thread, outFile: outFile}, nil
}

// readFrameFromFile 读取子进程写的帧文件
func readFrameFromFile(path string, buf *bytes.Buffer) (int, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	if len(data) < 8 {
		return 0, 0, fmt.Errorf("frame too small: %d bytes", len(data))
	}
	w := int(binary.LittleEndian.Uint32(data[0:4]))
	h := int(binary.LittleEndian.Uint32(data[4:8]))
	buf.Write(data[8:])
	return w, h, nil
}

// ─── 截图主循环：Session 0 使用 helper，否则直接截图 ───

func captureScreenWithFallback(buf *bytes.Buffer, quality, scale int) (int, int, uint64, error) {
	// 非 SYSTEM：直接截图
	return captureScreenBinary(buf, quality, scale)
}

// screenCaptureLoopSession0 Session 0 专用截图循环：启动一个长驻子进程
func screenCaptureLoopSession0(session *ScreenSession, cfg ScreenStartPayload) {
	defer func() {
		close(session.done)
		screenManager.mu.Lock()
		delete(screenManager.sessions, session.id)
		screenManager.mu.Unlock()
		log.Printf("桌面截图会话已结束(Session0): id=%s", session.id)
	}()

	intervalMs := 1000 / cfg.FPS
	helper, err := startHelperInUserSession(cfg.Quality, cfg.Scale, intervalMs)
	if err != nil {
		log.Printf("启动截图子进程失败: %v", err)
		sendScreenError(session, fmt.Sprintf("启动截图子进程失败: %v", err))
		return
	}
	defer helper.Kill()

	// 等子进程产出第一帧
	time.Sleep(time.Duration(intervalMs+500) * time.Millisecond)

	interval := time.Second / time.Duration(cfg.FPS)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastHash uint64 = 0xFFFFFFFFFFFFFFFF
	var jpegBuf bytes.Buffer
	var errCount int

	for {
		select {
		case <-session.stopCh:
			return
		case <-ticker.C:
			jpegBuf.Reset()
			w, h, err := readFrameFromFile(helper.outFile, &jpegBuf)
			if err != nil {
				errCount++
				if errCount <= 3 {
					sendScreenError(session, fmt.Sprintf("读取截图失败: %v", err))
				}
				continue
			}
			errCount = 0

			// 简单 hash 检测变化
			var hash uint64 = 14695981039346656037
			data := jpegBuf.Bytes()
			for i := 0; i < len(data); i += 1024 {
				hash ^= uint64(data[i])
				hash *= 1099511628211
			}

			if hash == lastHash {
				continue
			}
			lastHash = hash
			sendScreenFrameBinary(session, jpegBuf.Bytes(), w, h)
		}
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
