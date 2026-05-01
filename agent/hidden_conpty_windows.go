//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modKernel32ConPTY                     = windows.NewLazySystemDLL("kernel32.dll")
	procCreatePseudoConsole               = modKernel32ConPTY.NewProc("CreatePseudoConsole")
	procResizePseudoConsole               = modKernel32ConPTY.NewProc("ResizePseudoConsole")
	procClosePseudoConsole                = modKernel32ConPTY.NewProc("ClosePseudoConsole")
	procInitializeProcThreadAttributeList = modKernel32ConPTY.NewProc("InitializeProcThreadAttributeList")
	procUpdateProcThreadAttribute         = modKernel32ConPTY.NewProc("UpdateProcThreadAttribute")
	procDeleteProcThreadAttributeList     = modKernel32ConPTY.NewProc("DeleteProcThreadAttributeList")

	errHiddenConPTYUnsupported = errors.New("ConPTY is not available on this version of Windows")
)

const (
	_hiddenStillActive                uint32  = 259
	_hiddenSOK                        uintptr = 0
	_procThreadAttributePseudoConsole uintptr = 0x20016
	_createNoWindow                   uint32  = 0x08000000
	_swHide                           uint16  = 0
)

type hiddenCoord struct {
	X int16
	Y int16
}

func (c hiddenCoord) pack() uintptr {
	return uintptr((int32(c.Y) << 16) | int32(uint16(c.X)))
}

type hiddenHPCON windows.Handle

type hiddenHandleIO struct {
	handle windows.Handle
}

func (h *hiddenHandleIO) Read(p []byte) (int, error) {
	var n uint32
	err := windows.ReadFile(h.handle, p, &n, nil)
	return int(n), err
}

func (h *hiddenHandleIO) Write(p []byte) (int, error) {
	var n uint32
	err := windows.WriteFile(h.handle, p, &n, nil)
	return int(n), err
}

func (h *hiddenHandleIO) Close() error {
	if h == nil || h.handle == windows.InvalidHandle {
		return nil
	}
	err := windows.CloseHandle(h.handle)
	h.handle = windows.InvalidHandle
	return err
}

type hiddenConPTY struct {
	hpc       hiddenHPCON
	pi        *windows.ProcessInformation
	ptyIn     *hiddenHandleIO
	ptyOut    *hiddenHandleIO
	cmdIn     *hiddenHandleIO
	cmdOut    *hiddenHandleIO
	closeOnce sync.Once
}

type hiddenStartupInfoEx struct {
	windows.StartupInfo
	AttributeList *byte
}

func isHiddenConPTYAvailable() bool {
	return procCreatePseudoConsole.Find() == nil &&
		procResizePseudoConsole.Find() == nil &&
		procClosePseudoConsole.Find() == nil &&
		procInitializeProcThreadAttributeList.Find() == nil &&
		procUpdateProcThreadAttribute.Find() == nil
}

func closeHiddenHandles(handles ...windows.Handle) error {
	var firstErr error
	for _, h := range handles {
		if h == 0 || h == windows.InvalidHandle {
			continue
		}
		if err := windows.CloseHandle(h); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func createEnvBlockUTF16(envv []string) *uint16 {
	if len(envv) == 0 {
		v := utf16.Encode([]rune("\x00\x00"))
		return &v[0]
	}
	length := 0
	for _, s := range envv {
		length += len(s) + 1
	}
	length++
	b := make([]byte, length)
	i := 0
	for _, s := range envv {
		l := len(s)
		copy(b[i:i+l], []byte(s))
		b[i+l] = 0
		i += l + 1
	}
	b[i] = 0
	v := utf16.Encode([]rune(string(b)))
	return &v[0]
}

func createPseudoConsoleHidden(coord hiddenCoord, hIn, hOut windows.Handle) (hiddenHPCON, error) {
	var hpc hiddenHPCON
	r, _, _ := procCreatePseudoConsole.Call(
		coord.pack(),
		uintptr(hIn),
		uintptr(hOut),
		0,
		uintptr(unsafe.Pointer(&hpc)),
	)
	if r != _hiddenSOK {
		return 0, fmt.Errorf("CreatePseudoConsole failed: 0x%x", r)
	}
	return hpc, nil
}

func resizePseudoConsoleHidden(hpc hiddenHPCON, coord hiddenCoord) error {
	r, _, _ := procResizePseudoConsole.Call(uintptr(hpc), coord.pack())
	if r != _hiddenSOK {
		return fmt.Errorf("ResizePseudoConsole failed: 0x%x", r)
	}
	return nil
}

func closePseudoConsoleHidden(hpc hiddenHPCON) {
	if hpc == 0 {
		return
	}
	procClosePseudoConsole.Call(uintptr(hpc))
}

func buildStartupInfoExHidden(hpc hiddenHPCON) (*hiddenStartupInfoEx, []byte, error) {
	var size uintptr
	procInitializeProcThreadAttributeList.Call(0, 1, 0, uintptr(unsafe.Pointer(&size)))
	if size == 0 {
		return nil, nil, fmt.Errorf("InitializeProcThreadAttributeList size=0")
	}

	attrList := make([]byte, size)
	r, _, e := procInitializeProcThreadAttributeList.Call(
		uintptr(unsafe.Pointer(&attrList[0])),
		1,
		0,
		uintptr(unsafe.Pointer(&size)),
	)
	if r != 1 {
		return nil, nil, fmt.Errorf("InitializeProcThreadAttributeList: %v", e)
	}

	siEx := &hiddenStartupInfoEx{}
	siEx.Cb = uint32(unsafe.Sizeof(*siEx))
	siEx.Flags = windows.STARTF_USESHOWWINDOW
	siEx.ShowWindow = _swHide
	siEx.AttributeList = &attrList[0]

	r, _, e = procUpdateProcThreadAttribute.Call(
		uintptr(unsafe.Pointer(siEx.AttributeList)),
		0,
		_procThreadAttributePseudoConsole,
		uintptr(hpc),
		unsafe.Sizeof(hpc),
		0,
		0,
	)
	if r != 1 {
		procDeleteProcThreadAttributeList.Call(uintptr(unsafe.Pointer(siEx.AttributeList)))
		return nil, nil, fmt.Errorf("UpdateProcThreadAttribute: %v", e)
	}

	return siEx, attrList, nil
}

func createHiddenConsoleProcessAttachedToPTY(hpc hiddenHPCON, commandLine, workDir string, env []string) (*windows.ProcessInformation, error) {
	cmdLine, err := windows.UTF16PtrFromString(commandLine)
	if err != nil {
		return nil, err
	}

	var currentDirectory *uint16
	if workDir != "" {
		currentDirectory, err = windows.UTF16PtrFromString(workDir)
		if err != nil {
			return nil, err
		}
	}

	siEx, attrList, err := buildStartupInfoExHidden(hpc)
	if err != nil {
		return nil, err
	}
	defer procDeleteProcThreadAttributeList.Call(uintptr(unsafe.Pointer(siEx.AttributeList)))
	defer func() { _ = attrList }()

	var envBlock *uint16
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | _createNoWindow)
	if len(env) > 0 {
		flags |= windows.CREATE_UNICODE_ENVIRONMENT
		envBlock = createEnvBlockUTF16(env)
	}

	var pi windows.ProcessInformation
	err = windows.CreateProcess(
		nil,
		cmdLine,
		nil,
		nil,
		false,
		flags,
		envBlock,
		currentDirectory,
		&siEx.StartupInfo,
		&pi,
	)
	if err != nil {
		return nil, err
	}
	return &pi, nil
}

func startHiddenConPTY(commandLine string, cols, rows int, workDir string, env []string) (*hiddenConPTY, error) {
	if !isHiddenConPTYAvailable() {
		return nil, errHiddenConPTYUnsupported
	}
	if cols <= 0 {
		cols = 120
	}
	if rows <= 0 {
		rows = 30
	}

	coord := hiddenCoord{X: int16(cols), Y: int16(rows)}

	var cmdIn, cmdOut, ptyIn, ptyOut windows.Handle
	if err := windows.CreatePipe(&ptyIn, &cmdIn, nil, 0); err != nil {
		return nil, fmt.Errorf("CreatePipe(stdin): %v", err)
	}
	if err := windows.CreatePipe(&cmdOut, &ptyOut, nil, 0); err != nil {
		closeHiddenHandles(ptyIn, cmdIn)
		return nil, fmt.Errorf("CreatePipe(stdout): %v", err)
	}

	hpc, err := createPseudoConsoleHidden(coord, ptyIn, ptyOut)
	if err != nil {
		closeHiddenHandles(ptyIn, ptyOut, cmdIn, cmdOut)
		return nil, err
	}

	pi, err := createHiddenConsoleProcessAttachedToPTY(hpc, commandLine, workDir, env)
	if err != nil {
		closePseudoConsoleHidden(hpc)
		closeHiddenHandles(ptyIn, ptyOut, cmdIn, cmdOut)
		return nil, fmt.Errorf("CreateProcess hidden ConPTY: %v", err)
	}

	return &hiddenConPTY{
		hpc:    hpc,
		pi:     pi,
		ptyIn:  &hiddenHandleIO{handle: ptyIn},
		ptyOut: &hiddenHandleIO{handle: ptyOut},
		cmdIn:  &hiddenHandleIO{handle: cmdIn},
		cmdOut: &hiddenHandleIO{handle: cmdOut},
	}, nil
}

func (c *hiddenConPTY) Resize(width, height int) error {
	return resizePseudoConsoleHidden(c.hpc, hiddenCoord{X: int16(width), Y: int16(height)})
}

func (c *hiddenConPTY) Read(p []byte) (int, error) {
	return c.cmdOut.Read(p)
}

func (c *hiddenConPTY) Write(p []byte) (int, error) {
	return c.cmdIn.Write(p)
}

func (c *hiddenConPTY) Wait(ctx context.Context) (uint32, error) {
	var exitCode uint32 = _hiddenStillActive
	for {
		if err := ctx.Err(); err != nil {
			return _hiddenStillActive, fmt.Errorf("wait canceled: %v", err)
		}
		ret, err := windows.WaitForSingleObject(c.pi.Process, 1000)
		if err != nil {
			return _hiddenStillActive, err
		}
		if ret != uint32(windows.WAIT_TIMEOUT) {
			if err := windows.GetExitCodeProcess(c.pi.Process, &exitCode); err != nil {
				return exitCode, err
			}
			return exitCode, nil
		}
	}
}

func (c *hiddenConPTY) Close() error {
	var firstErr error
	c.closeOnce.Do(func() {
		closePseudoConsoleHidden(c.hpc)
		if c.pi != nil {
			if err := closeHiddenHandles(c.pi.Process, c.pi.Thread); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if err := c.ptyIn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := c.ptyOut.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := c.cmdIn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := c.cmdOut.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	})
	return firstErr
}
