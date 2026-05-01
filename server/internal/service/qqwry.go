package service

import (
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// QQwry 纯真 IP 数据库解析器
// 数据库格式：https://github.com/out0fmemory/qqwry.dat
// 服务端查询，Agent 不携带数据库文件（隐蔽性）

type QQwry struct {
	data    []byte
	mu      sync.RWMutex
	loaded  bool
	idxStart uint32
	idxEnd   uint32
}

var (
	defaultQQwry     *QQwry
	defaultQQwryOnce sync.Once
)

// GetQQwry 获取全局 QQwry 实例
func GetQQwry(dataDir string) *QQwry {
	defaultQQwryOnce.Do(func() {
		defaultQQwry = &QQwry{}
		dbPath := filepath.Join(dataDir, "qqwry.dat")
		if err := defaultQQwry.LoadFile(dbPath); err != nil {
			fmt.Printf("QQwry 加载失败 (%s): %v，IP 归属地查询不可用\n", dbPath, err)
		}
	})
	return defaultQQwry
}

// LoadFile 从文件加载数据库
func (q *QQwry) LoadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return q.load(data)
}

// LoadFromURL 从 URL 下载数据库（备用）
func (q *QQwry) LoadFromURL(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return q.load(data)
}

func (q *QQwry) load(data []byte) error {
	if len(data) < 8 {
		return fmt.Errorf("数据库文件过小")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.data = data
	q.idxStart = binary.LittleEndian.Uint32(data[0:4])
	q.idxEnd = binary.LittleEndian.Uint32(data[4:8])
	q.loaded = true
	fmt.Printf("QQwry 加载成功，记录数: %d\n", (q.idxEnd-q.idxStart)/7+1)
	return nil
}

// IsLoaded 是否已加载
func (q *QQwry) IsLoaded() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.loaded
}

// Lookup 查询 IP 归属地
func (q *QQwry) Lookup(ipStr string) string {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if !q.loaded || len(q.data) < 8 {
		return ""
	}

	ip := parseIPv4(ipStr)
	if ip == 0 && ipStr != "0.0.0.0" {
		return ""
	}

	// 跳过私有/保留地址
	b0 := (ip >> 24) & 0xFF
	if b0 == 10 || b0 == 127 || b0 == 0 {
		return "内网IP"
	}
	if b0 == 172 && ((ip>>16)&0xFF) >= 16 && ((ip>>16)&0xFF) <= 31 {
		return "内网IP"
	}
	if b0 == 192 && ((ip>>16)&0xFF) == 168 {
		return "内网IP"
	}

	offset := q.searchIndex(ip)
	if offset == 0 {
		return ""
	}

	country, area := q.readRecord(offset)
	result := strings.TrimSpace(country)
	area = strings.TrimSpace(area)
	if area != "" && area != "CZ88.NET" {
		result += " " + area
	}
	return result
}

// LookupBatch 批量查询
func (q *QQwry) LookupBatch(ips []string) map[string]string {
	result := make(map[string]string, len(ips))
	for _, ip := range ips {
		if _, ok := result[ip]; !ok {
			result[ip] = q.Lookup(ip)
		}
	}
	return result
}

func parseIPv4(s string) uint32 {
	var a, b, c, d uint32
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return 0
	}
	for i, p := range parts {
		var v uint32
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				return 0
			}
			v = v*10 + uint32(ch-'0')
		}
		if v > 255 {
			return 0
		}
		switch i {
		case 0:
			a = v
		case 1:
			b = v
		case 2:
			c = v
		case 3:
			d = v
		}
	}
	return (a << 24) | (b << 16) | (c << 8) | d
}

// 二分查找索引
func (q *QQwry) searchIndex(ip uint32) uint32 {
	lo := q.idxStart
	hi := q.idxEnd

	for lo <= hi {
		mid := lo + ((hi-lo)/7/2)*7
		midIP := binary.LittleEndian.Uint32(q.data[mid : mid+4])

		if ip < midIP {
			if mid < lo+7 {
				break
			}
			hi = mid - 7
		} else if ip > midIP {
			lo = mid + 7
		} else {
			// 精确命中
			return readUint24(q.data[mid+4 : mid+7])
		}
	}

	// lo 可能超过 hi，取 lo-7（前一条索引）
	if lo > q.idxStart {
		idx := lo - 7
		startIP := binary.LittleEndian.Uint32(q.data[idx : idx+4])
		recordOff := readUint24(q.data[idx+4 : idx+7])
		endIP := binary.LittleEndian.Uint32(q.data[recordOff : recordOff+4])
		if ip >= startIP && ip <= endIP {
			return recordOff + 4
		}
	}
	return 0
}

func readUint24(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
}

// 读取记录（国家 + 地区）
func (q *QQwry) readRecord(offset uint32) (string, string) {
	if int(offset) >= len(q.data) {
		return "", ""
	}

	mode := q.data[offset]

	switch mode {
	case 0x01:
		// 模式1：国家和地区都重定向
		newOff := readUint24(q.data[offset+1 : offset+4])
		return q.readRecord(newOff)
	case 0x02:
		// 模式2：国家重定向，地区在后面
		countryOff := readUint24(q.data[offset+1 : offset+4])
		country := q.readString(countryOff)
		area := q.readArea(offset + 4)
		return country, area
	default:
		// 无重定向
		country := q.readString(offset)
		area := q.readArea(offset + uint32(len(q.gbkBytes(offset))) + 1)
		return country, area
	}
}

func (q *QQwry) readArea(offset uint32) string {
	if int(offset) >= len(q.data) {
		return ""
	}
	mode := q.data[offset]
	if mode == 0x01 || mode == 0x02 {
		areaOff := readUint24(q.data[offset+1 : offset+4])
		return q.readString(areaOff)
	}
	return q.readString(offset)
}

func (q *QQwry) readString(offset uint32) string {
	raw := q.gbkBytes(offset)
	if len(raw) == 0 {
		return ""
	}
	reader := transform.NewReader(strings.NewReader(string(raw)), simplifiedchinese.GBK.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return string(raw)
	}
	return string(decoded)
}

func (q *QQwry) gbkBytes(offset uint32) []byte {
	var buf []byte
	for i := offset; int(i) < len(q.data); i++ {
		if q.data[i] == 0 {
			break
		}
		buf = append(buf, q.data[i])
	}
	return buf
}
