//go:build !windows

package main

import (
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
)

// captureFrameNative Linux 不支持原生采集，返回错误后回退到 ffmpeg
func captureFrameNative() ([]byte, int, int, error) {
	return nil, 0, 0, fmt.Errorf("native webcam not supported on this platform")
}

// tryNativeWebcamStream Linux 不支持，返回 false 后回退到 ffmpeg
func tryNativeWebcamStream(conn *websocket.Conn, writeMu *sync.Mutex, stopCh chan struct{}) bool {
	return false
}
