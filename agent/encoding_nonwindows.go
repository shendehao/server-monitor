//go:build !windows

package main

// oemToUTF8 Linux 下命令输出已是 UTF-8，直接返回
func oemToUTF8(data []byte) string {
	return string(data)
}

// acpToUTF8 Linux 下直接返回
func acpToUTF8(data []byte) string {
	return string(data)
}
