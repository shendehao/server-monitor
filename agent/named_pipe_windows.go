//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"syscall"
	"unsafe"
)

// ─── Named Pipe IPC（替代文件中转，无竞态、无磁盘IO） ───

var (
	modKernel32Pipe         = syscall.NewLazyDLL("kernel32.dll")
	procCreateNamedPipeW    = modKernel32Pipe.NewProc("CreateNamedPipeW")
	procConnectNamedPipe    = modKernel32Pipe.NewProc("ConnectNamedPipe")
	modAdvapi32Pipe         = syscall.NewLazyDLL("advapi32.dll")
	procInitSecurityDesc    = modAdvapi32Pipe.NewProc("InitializeSecurityDescriptor")
	procSetSecurityDescDacl = modAdvapi32Pipe.NewProc("SetSecurityDescriptorDacl")
)

const (
	_PIPE_ACCESS_INBOUND          = 0x00000001
	_PIPE_TYPE_BYTE               = 0x00000000
	_PIPE_READMODE_BYTE           = 0x00000000
	_PIPE_WAIT                    = 0x00000000
	_PIPE_BUFFER_SIZE             = 1024 * 1024 // 1MB
	_SECURITY_DESCRIPTOR_REVISION = 1
)

// securityAttributes 创建允许所有用户连接的安全描述符
type securityDescriptor struct {
	Revision byte
	Sbz1     byte
	Control  uint16
	Owner    uintptr
	Group    uintptr
	Sacl     uintptr
	Dacl     uintptr
}

// generatePipeName 生成随机管道名
func generatePipeName() string {
	return fmt.Sprintf(`\\.\pipe\SMA_%d`, rand.Int63())
}

// createPipeServer 创建命名管道服务端（主进程调用）
// 返回管道句柄和管道名
func createPipeServer() (syscall.Handle, string, error) {
	pipeName := generatePipeName()
	pipeNamePtr, _ := syscall.UTF16PtrFromString(pipeName)

	// 创建允许所有用户连接的安全描述符（NULL DACL）
	var sd securityDescriptor
	procInitSecurityDesc.Call(
		uintptr(unsafe.Pointer(&sd)),
		_SECURITY_DESCRIPTOR_REVISION,
	)
	procSetSecurityDescDacl.Call(
		uintptr(unsafe.Pointer(&sd)),
		1, // bDaclPresent = TRUE
		0, // pDacl = NULL (允许所有人)
		0, // bDaclDefaulted = FALSE
	)

	sa := syscall.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(syscall.SecurityAttributes{})),
		SecurityDescriptor: uintptr(unsafe.Pointer(&sd)),
		InheritHandle:      0,
	}

	h, _, err := procCreateNamedPipeW.Call(
		uintptr(unsafe.Pointer(pipeNamePtr)),
		_PIPE_ACCESS_INBOUND, // 只读（从 helper 读帧数据）
		_PIPE_TYPE_BYTE|_PIPE_READMODE_BYTE|_PIPE_WAIT,
		1,                 // 最多1个实例
		0,                 // 输出缓冲区
		_PIPE_BUFFER_SIZE, // 输入缓冲区
		0,                 // 默认超时
		uintptr(unsafe.Pointer(&sa)),
	)
	if h == uintptr(syscall.InvalidHandle) {
		return syscall.InvalidHandle, "", fmt.Errorf("CreateNamedPipe: %v", err)
	}

	return syscall.Handle(h), pipeName, nil
}

// waitForPipeClient 等待 helper 连接管道
func waitForPipeClient(pipe syscall.Handle) error {
	r, _, err := procConnectNamedPipe.Call(uintptr(pipe), 0)
	if r == 0 {
		// ERROR_PIPE_CONNECTED (535) 表示客户端已连接
		if errno, ok := err.(syscall.Errno); ok && errno == 535 {
			return nil
		}
		return fmt.Errorf("ConnectNamedPipe: %v", err)
	}
	return nil
}

// connectToPipe helper 子进程连接到主进程创建的管道（写入端）
func connectToPipe(pipeName string) (syscall.Handle, error) {
	namePtr, _ := syscall.UTF16PtrFromString(pipeName)
	h, err := syscall.CreateFile(namePtr, syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		return syscall.InvalidHandle, fmt.Errorf("connect pipe %s: %v", pipeName, err)
	}
	return h, nil
}

// ─── 帧协议 ───
// 每帧格式: [4B width][4B height][4B jpeg_size][jpeg_data...]

// pipeFrameHeader 帧头
type pipeFrameHeader struct {
	Width    uint32
	Height   uint32
	JPEGSize uint32
}

// writePipeFrame 写入一帧到管道（helper 端调用）
func writePipeFrame(w io.Writer, jpegData []byte, width, height int) error {
	hdr := pipeFrameHeader{
		Width:    uint32(width),
		Height:   uint32(height),
		JPEGSize: uint32(len(jpegData)),
	}
	if err := binary.Write(w, binary.LittleEndian, &hdr); err != nil {
		return err
	}
	_, err := w.Write(jpegData)
	return err
}

// readPipeFrame 从管道读取一帧（主进程调用）
func readPipeFrame(r io.Reader, buf []byte) (jpegData []byte, width, height int, err error) {
	var hdr pipeFrameHeader
	if err = binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		return nil, 0, 0, fmt.Errorf("read header: %v", err)
	}
	if hdr.JPEGSize > 10*1024*1024 { // 安全检查：单帧不超过 10MB
		return nil, 0, 0, fmt.Errorf("frame too large: %d", hdr.JPEGSize)
	}
	needed := int(hdr.JPEGSize)
	if cap(buf) >= needed {
		buf = buf[:needed]
	} else {
		buf = make([]byte, needed)
	}
	if _, err = io.ReadFull(r, buf); err != nil {
		return nil, 0, 0, fmt.Errorf("read jpeg: %v", err)
	}
	return buf, int(hdr.Width), int(hdr.Height), nil
}
