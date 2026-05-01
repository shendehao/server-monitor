package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// ═══ 文件管理模块 ═══

func handleFileBrowse(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	var req struct {
		Path string `json:"path"`
	}
	json.Unmarshal(msg.Payload, &req)

	dir := req.Path
	if dir == "" {
		if u, err := user.Current(); err == nil {
			dir = u.HomeDir
		} else if runtime.GOOS == "windows" {
			dir = os.Getenv("USERPROFILE")
		} else {
			dir = "/"
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		sendResult(conn, writeMu, "file_browse_result", msg.ID,
			fmt.Sprintf(`{"error":"%s"}`, jsonEsc(err.Error())))
		return
	}

	type fileEntry struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Size     int64  `json:"size"`
		Modified string `json:"modified"`
	}

	var items []fileEntry
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		ft := "file"
		if e.IsDir() {
			ft = "dir"
		}
		mod := ""
		if !e.IsDir() {
			mod = info.ModTime().Format("2006-01-02 15:04:05")
		}
		items = append(items, fileEntry{
			Name:     e.Name(),
			Type:     ft,
			Size:     info.Size(),
			Modified: mod,
		})
	}

	// Sort: dirs first, then by name
	sort.Slice(items, func(i, j int) bool {
		if items[i].Type != items[j].Type {
			return items[i].Type == "dir"
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	result, _ := json.Marshal(map[string]interface{}{
		"path":  dir,
		"items": items,
	})
	sendResult(conn, writeMu, "file_browse_result", msg.ID, string(result))
}

func handleFileDownload(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	var req struct {
		Path string `json:"path"`
	}
	json.Unmarshal(msg.Payload, &req)

	if req.Path == "" {
		sendResult(conn, writeMu, "file_download_result", msg.ID, `{"error":"路径不能为空"}`)
		return
	}

	info, err := os.Stat(req.Path)
	if err != nil {
		sendResult(conn, writeMu, "file_download_result", msg.ID,
			fmt.Sprintf(`{"error":"%s"}`, jsonEsc(err.Error())))
		return
	}

	const maxSize = 100 * 1024 * 1024 // 100MB
	if info.Size() > maxSize {
		// Large file: use chunk upload
		go uploadFileChunked(conn, writeMu, msg.ID, req.Path, info.Size())
		return
	}

	data, err := os.ReadFile(req.Path)
	if err != nil {
		sendResult(conn, writeMu, "file_download_result", msg.ID,
			fmt.Sprintf(`{"error":"%s"}`, jsonEsc(err.Error())))
		return
	}

	// Upload via HTTP chunk API
	downloadID := msg.ID
	uploadURL := agentServerURL + "/api/agent/file-chunk"
	fileName := filepath.Base(req.Path)

	req2, _ := http.NewRequest("POST", uploadURL, strings.NewReader(string(data)))
	req2.Header.Set("X-Download-ID", downloadID)
	req2.Header.Set("X-Chunk-Index", "0")
	req2.Header.Set("X-Total-Chunks", "1")
	req2.Header.Set("X-File-Name", fileName)
	req2.Header.Set("Content-Type", "application/octet-stream")

	resp, err := secureHTTPClient.Do(req2)
	if err != nil {
		sendResult(conn, writeMu, "file_download_result", msg.ID,
			fmt.Sprintf(`{"error":"上传失败: %s"}`, jsonEsc(err.Error())))
		return
	}
	resp.Body.Close()

	sendResult(conn, writeMu, "file_download_result", msg.ID,
		fmt.Sprintf(`{"downloadId":"%s","fileName":"%s","size":%d}`,
			downloadID, jsonEsc(fileName), info.Size()))
}

func uploadFileChunked(conn *websocket.Conn, writeMu *sync.Mutex, msgID, filePath string, totalSize int64) {
	const chunkSize = 4 * 1024 * 1024 // 4MB per chunk
	totalChunks := int((totalSize + chunkSize - 1) / chunkSize)
	fileName := filepath.Base(filePath)

	f, err := os.Open(filePath)
	if err != nil {
		sendResult(conn, writeMu, "file_download_result", msgID,
			fmt.Sprintf(`{"error":"%s"}`, jsonEsc(err.Error())))
		return
	}
	defer f.Close()

	uploadURL := agentServerURL + "/api/agent/file-chunk"
	buf := make([]byte, chunkSize)

	for i := 0; i < totalChunks; i++ {
		n, err := f.Read(buf)
		if err != nil && err != io.EOF {
			log.Printf("文件读取失败: %v", err)
			return
		}
		if n == 0 {
			break
		}

		req, _ := http.NewRequest("POST", uploadURL, strings.NewReader(string(buf[:n])))
		req.Header.Set("X-Download-ID", msgID)
		req.Header.Set("X-Chunk-Index", fmt.Sprintf("%d", i))
		req.Header.Set("X-Total-Chunks", fmt.Sprintf("%d", totalChunks))
		req.Header.Set("X-File-Name", fileName)
		req.Header.Set("Content-Type", "application/octet-stream")

		resp, err := secureHTTPClient.Do(req)
		if err != nil {
			log.Printf("分片上传失败: %v", err)
			return
		}
		resp.Body.Close()
	}

	sendResult(conn, writeMu, "file_download_result", msgID,
		fmt.Sprintf(`{"downloadId":"%s","fileName":"%s","size":%d}`,
			msgID, jsonEsc(fileName), totalSize))
}

// sendResult helper to send typed result back via WS
func sendResult(conn *websocket.Conn, writeMu *sync.Mutex, msgType, msgID, payload string) {
	resp, _ := json.Marshal(AgentMessage{
		Type:    c2e(msgType),
		ID:      msgID,
		Payload: json.RawMessage(payload),
	})
	writeMu.Lock()
	conn.WriteMessage(websocket.TextMessage, resp)
	writeMu.Unlock()
}

// jsonEsc escapes a string for JSON embedding
func jsonEsc(s string) string {
	b, _ := json.Marshal(s)
	// Remove surrounding quotes
	return string(b[1 : len(b)-1])
}
