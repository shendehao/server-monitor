//go:build windows

package main

import (
	"syscall"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

// ═══ OEM/GBK → UTF-8 编码转换 (Windows) ═══
// 使用 MultiByteToWideChar API，自动适配系统代码页

var (
	kernel32Enc              = syscall.NewLazyDLL("kernel32.dll")
	procMultiByteToWideChar  = kernel32Enc.NewProc("MultiByteToWideChar")
	procGetOEMCP             = kernel32Enc.NewProc("GetOEMCP")
)

// oemToUTF8 将 Windows 命令输出（OEM 代码页）转为 UTF-8
// 如果已是有效 UTF-8 则直接返回
func oemToUTF8(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	if utf8.Valid(data) {
		// 检查是否包含替换字符（说明可能是被错误解析的 GBK）
		if !containsReplacementChar(data) {
			return string(data)
		}
	}

	// 获取 OEM 代码页（中文 Windows 通常是 936/GBK）
	cp, _, _ := procGetOEMCP.Call()
	if cp == 0 {
		cp = 936 // 默认 GBK
	}

	return mbToUTF8(uint32(cp), data)
}

// acpToUTF8 将 ACP (ANSI Code Page) 编码转为 UTF-8
func acpToUTF8(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	if utf8.Valid(data) && !containsReplacementChar(data) {
		return string(data)
	}
	return mbToUTF8(0, data) // CP_ACP = 0
}

func mbToUTF8(codePage uint32, data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// 第一次调用获取所需缓冲区大小
	n, _, _ := procMultiByteToWideChar.Call(
		uintptr(codePage), 0,
		uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)),
		0, 0,
	)
	if n == 0 {
		return string(data)
	}

	buf := make([]uint16, n)
	procMultiByteToWideChar.Call(
		uintptr(codePage), 0,
		uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)),
		uintptr(unsafe.Pointer(&buf[0])), n,
	)

	// UTF-16 → UTF-8
	runes := utf16.Decode(buf)
	result := make([]byte, 0, len(runes)*3)
	tmp := make([]byte, 4)
	for _, r := range runes {
		if r == 0 {
			continue
		}
		n := utf8.EncodeRune(tmp, r)
		result = append(result, tmp[:n]...)
	}
	return string(result)
}

func containsReplacementChar(data []byte) bool {
	for i := 0; i < len(data)-2; i++ {
		// UTF-8 replacement character: EF BF BD (U+FFFD)
		if data[i] == 0xEF && data[i+1] == 0xBF && data[i+2] == 0xBD {
			return true
		}
	}
	return false
}
