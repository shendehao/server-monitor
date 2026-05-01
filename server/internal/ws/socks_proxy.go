package ws

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// SocksProxy 管理每个 Agent 的 SOCKS5 反向代理
type SocksProxy struct {
	mu           sync.Mutex
	listeners    map[string]*socksListener    // serverID → SOCKS5 listener
	portForwards map[string]*portForwardEntry // "serverID:port" → 端口转发
	hub          *AgentHub
}

type socksListener struct {
	listener net.Listener
	port     int
	serverID string
	done     chan struct{}
	conns    sync.WaitGroup
	authUser string // 空 = 无需认证
	authPass string
}

// SocksProxyStatus 返回给前端的状态
type SocksProxyStatus struct {
	Running  bool   `json:"running"`
	Port     int    `json:"port"`
	AuthUser string `json:"authUser,omitempty"`
}

func NewSocksProxy(hub *AgentHub) *SocksProxy {
	return &SocksProxy{
		listeners:    make(map[string]*socksListener),
		portForwards: make(map[string]*portForwardEntry),
		hub:          hub,
	}
}

// Start 为指定 Agent 启动 SOCKS5 代理（支持可选认证）
func (sp *SocksProxy) Start(serverID string, port int, authUser ...string) (int, error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	// 如果已存在，先停止
	if old, ok := sp.listeners[serverID]; ok {
		old.listener.Close()
		close(old.done)
		delete(sp.listeners, serverID)
	}

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return 0, fmt.Errorf("监听端口 %d 失败: %v", port, err)
	}

	actualPort := ln.Addr().(*net.TCPAddr).Port
	sl := &socksListener{
		listener: ln,
		port:     actualPort,
		serverID: serverID,
		done:     make(chan struct{}),
	}
	if len(authUser) >= 2 && authUser[0] != "" {
		sl.authUser = authUser[0]
		sl.authPass = authUser[1]
	}
	sp.listeners[serverID] = sl

	go sp.acceptLoop(sl)
	log.Printf("SOCKS5 代理已启动: server=%s port=%d", serverID, actualPort)
	return actualPort, nil
}

// Stop 停止指定 Agent 的 SOCKS5 代理
func (sp *SocksProxy) Stop(serverID string) {
	sp.mu.Lock()
	sl, ok := sp.listeners[serverID]
	if ok {
		delete(sp.listeners, serverID)
	}
	sp.mu.Unlock()

	if ok && sl != nil {
		sl.listener.Close()
		close(sl.done)
		sl.conns.Wait()
		log.Printf("SOCKS5 代理已停止: server=%s", serverID)
	}
}

// Status 获取代理状态
func (sp *SocksProxy) Status(serverID string) SocksProxyStatus {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	if sl, ok := sp.listeners[serverID]; ok {
		return SocksProxyStatus{Running: true, Port: sl.port, AuthUser: sl.authUser}
	}
	return SocksProxyStatus{Running: false}
}

func (sp *SocksProxy) acceptLoop(sl *socksListener) {
	for {
		conn, err := sl.listener.Accept()
		if err != nil {
			select {
			case <-sl.done:
				return
			default:
			}
			continue
		}
		sl.conns.Add(1)
		go func() {
			defer sl.conns.Done()
			sp.handleSocks5(conn, sl)
		}()
	}
}

func (sp *SocksProxy) handleSocks5(conn net.Conn, sl *socksListener) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	// 1. SOCKS5 握手（支持无认证 0x00 和用户名密码认证 0x02）
	buf := make([]byte, 258)
	n, err := conn.Read(buf)
	if err != nil || n < 3 || buf[0] != 0x05 {
		return
	}
	nMethods := int(buf[1])
	if n < 2+nMethods {
		return
	}
	methods := buf[2 : 2+nMethods]

	needAuth := sl.authUser != ""
	if needAuth {
		// 需要认证：检查客户端是否支持 0x02
		hasUserPass := false
		for _, m := range methods {
			if m == 0x02 {
				hasUserPass = true
				break
			}
		}
		if !hasUserPass {
			conn.Write([]byte{0x05, 0xFF}) // 不可接受的方法
			return
		}
		conn.Write([]byte{0x05, 0x02}) // 选择用户名密码认证

		// RFC 1929: 用户名密码子协商
		n, err = conn.Read(buf)
		if err != nil || n < 3 || buf[0] != 0x01 {
			return
		}
		uLen := int(buf[1])
		if n < 2+uLen+1 {
			return
		}
		clientUser := string(buf[2 : 2+uLen])
		pLen := int(buf[2+uLen])
		if n < 3+uLen+pLen {
			return
		}
		clientPass := string(buf[3+uLen : 3+uLen+pLen])

		if clientUser != sl.authUser || clientPass != sl.authPass {
			conn.Write([]byte{0x01, 0x01}) // 认证失败
			return
		}
		conn.Write([]byte{0x01, 0x00}) // 认证成功
	} else {
		// 无认证
		conn.Write([]byte{0x05, 0x00})
	}

	// 2. 读取连接请求
	n, err = conn.Read(buf)
	if err != nil || n < 7 || buf[0] != 0x05 || buf[1] != 0x01 {
		conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) // command not supported
		return
	}

	var targetHost string
	var targetPort int

	switch buf[3] {
	case 0x01: // IPv4
		if n < 10 {
			return
		}
		targetHost = fmt.Sprintf("%d.%d.%d.%d", buf[4], buf[5], buf[6], buf[7])
		targetPort = int(binary.BigEndian.Uint16(buf[8:10]))
	case 0x03: // Domain
		domainLen := int(buf[4])
		if n < 5+domainLen+2 {
			return
		}
		targetHost = string(buf[5 : 5+domainLen])
		targetPort = int(binary.BigEndian.Uint16(buf[5+domainLen : 7+domainLen]))
	case 0x04: // IPv6
		if n < 22 {
			return
		}
		targetHost = net.IP(buf[4:20]).String()
		targetPort = int(binary.BigEndian.Uint16(buf[20:22]))
	default:
		conn.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	// 3. 通过 Agent 建立隧道
	channelID := fmt.Sprintf("s5-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&msgSeq, 1))

	params, _ := json.Marshal(map[string]string{
		"channel": channelID,
		"host":    targetHost,
		"port":    fmt.Sprintf("%d", targetPort),
	})

	result, err := sp.hub.sendAndWait(sl.serverID, "socks_connect", params, "s5c", 15*time.Second, "SOCKS 连接超时")
	if err != nil {
		conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) // connection refused
		return
	}

	// 检查结果
	var connectResult struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(result), &connectResult) != nil || !connectResult.OK {
		conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	// 4. 回复 SOCKS5 成功
	conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	conn.SetDeadline(time.Time{}) // 移除超时

	// 5. 注册数据回调
	dataCh := make(chan []byte, 64)
	closeCh := make(chan struct{})
	var closeOnce sync.Once

	sp.hub.RegisterTunnelCallback(channelID, func(data []byte) {
		select {
		case dataCh <- data:
		case <-closeCh:
		}
	}, func() {
		closeOnce.Do(func() { close(closeCh) })
	})

	defer func() {
		closeOnce.Do(func() { close(closeCh) })
		sp.hub.UnregisterTunnelCallback(channelID)
		// 通知 agent 关闭隧道
		closePayload, _ := json.Marshal(map[string]string{"channel": channelID})
		sp.hub.sendAndWait(sl.serverID, "socks_close", closePayload, "s5x", 3*time.Second, "")
	}()

	// 6. 双向数据中转
	// Agent → SOCKS Client
	go func() {
		for {
			select {
			case data := <-dataCh:
				if _, err := conn.Write(data); err != nil {
					closeOnce.Do(func() { close(closeCh) })
					return
				}
			case <-closeCh:
				return
			}
		}
	}()

	// SOCKS Client → Agent
	buf2 := make([]byte, 32768)
	for {
		select {
		case <-closeCh:
			return
		default:
		}
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		n, err := conn.Read(buf2)
		if err != nil {
			if err != io.EOF {
				select {
				case <-closeCh:
				default:
				}
			}
			return
		}
		if n > 0 {
			// 发送数据到 Agent
			dataPayload, _ := json.Marshal(map[string]string{
				"channel": channelID,
				"data":    encodeBase64(buf2[:n]),
			})
			sp.hub.sendNoWait(sl.serverID, "socks_data", dataPayload)
		}
	}
}

// ══════════════════════════════════════
//  端口转发（通过 Agent 隧道中转）
// ══════════════════════════════════════

type portForwardEntry struct {
	listener   net.Listener
	localPort  int
	remoteHost string
	remotePort int
	done       chan struct{}
	conns      sync.WaitGroup
}

// PortForwardInfo 前端展示用
type PortForwardInfo struct {
	LocalPort  int    `json:"localPort"`
	RemoteHost string `json:"remoteHost"`
	RemotePort int    `json:"remotePort"`
}

// StartPortForward 为指定 Agent 启动端口转发
func (sp *SocksProxy) StartPortForward(serverID string, localPort int, remoteHost string, remotePort int) (int, error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	key := fmt.Sprintf("%s:%d", serverID, localPort)

	// 如果已存在相同本地端口转发，先停止
	if old, ok := sp.portForwards[key]; ok {
		old.listener.Close()
		close(old.done)
		delete(sp.portForwards, key)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", localPort))
	if err != nil {
		return 0, fmt.Errorf("监听端口 %d 失败: %v", localPort, err)
	}

	actualPort := ln.Addr().(*net.TCPAddr).Port
	pf := &portForwardEntry{
		listener:   ln,
		localPort:  actualPort,
		remoteHost: remoteHost,
		remotePort: remotePort,
		done:       make(chan struct{}),
	}
	if sp.portForwards == nil {
		sp.portForwards = make(map[string]*portForwardEntry)
	}
	sp.portForwards[key] = pf

	go sp.pfAcceptLoop(serverID, pf)
	log.Printf("端口转发已启动: server=%s local=%d → %s:%d", serverID, actualPort, remoteHost, remotePort)
	return actualPort, nil
}

// StopPortForward 停止指定端口转发
func (sp *SocksProxy) StopPortForward(serverID string, localPort int) {
	sp.mu.Lock()
	key := fmt.Sprintf("%s:%d", serverID, localPort)
	pf, ok := sp.portForwards[key]
	if ok {
		delete(sp.portForwards, key)
	}
	sp.mu.Unlock()

	if ok && pf != nil {
		pf.listener.Close()
		close(pf.done)
		pf.conns.Wait()
		log.Printf("端口转发已停止: server=%s local=%d", serverID, localPort)
	}
}

// ListPortForwards 列出指定 Agent 的所有端口转发
func (sp *SocksProxy) ListPortForwards(serverID string) []PortForwardInfo {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	var result []PortForwardInfo
	prefix := serverID + ":"
	for key, pf := range sp.portForwards {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			result = append(result, PortForwardInfo{
				LocalPort:  pf.localPort,
				RemoteHost: pf.remoteHost,
				RemotePort: pf.remotePort,
			})
		}
	}
	if result == nil {
		result = []PortForwardInfo{}
	}
	return result
}

func (sp *SocksProxy) pfAcceptLoop(serverID string, pf *portForwardEntry) {
	for {
		conn, err := pf.listener.Accept()
		if err != nil {
			select {
			case <-pf.done:
				return
			default:
			}
			continue
		}
		pf.conns.Add(1)
		go func() {
			defer pf.conns.Done()
			sp.handlePortForward(conn, serverID, pf)
		}()
	}
}

func (sp *SocksProxy) handlePortForward(conn net.Conn, serverID string, pf *portForwardEntry) {
	defer conn.Close()

	channelID := fmt.Sprintf("pf-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&msgSeq, 1))

	params, _ := json.Marshal(map[string]string{
		"channel": channelID,
		"host":    pf.remoteHost,
		"port":    fmt.Sprintf("%d", pf.remotePort),
	})

	result, err := sp.hub.sendAndWait(serverID, "socks_connect", params, "pfc", 15*time.Second, "端口转发连接超时")
	if err != nil {
		return
	}

	var connectResult struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(result), &connectResult) != nil || !connectResult.OK {
		return
	}

	// 注册数据回调
	dataCh := make(chan []byte, 64)
	closeCh := make(chan struct{})
	var closeOnce sync.Once

	sp.hub.RegisterTunnelCallback(channelID, func(data []byte) {
		select {
		case dataCh <- data:
		case <-closeCh:
		}
	}, func() {
		closeOnce.Do(func() { close(closeCh) })
	})

	defer func() {
		closeOnce.Do(func() { close(closeCh) })
		sp.hub.UnregisterTunnelCallback(channelID)
		closePayload, _ := json.Marshal(map[string]string{"channel": channelID})
		sp.hub.sendAndWait(serverID, "socks_close", closePayload, "pfx", 3*time.Second, "")
	}()

	// Agent → Local Client
	go func() {
		for {
			select {
			case data := <-dataCh:
				if _, err := conn.Write(data); err != nil {
					closeOnce.Do(func() { close(closeCh) })
					return
				}
			case <-closeCh:
				return
			}
		}
	}()

	// Local Client → Agent
	buf := make([]byte, 32768)
	for {
		select {
		case <-closeCh:
			return
		default:
		}
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		if n > 0 {
			dataPayload, _ := json.Marshal(map[string]string{
				"channel": channelID,
				"data":    encodeBase64(buf[:n]),
			})
			sp.hub.sendNoWait(serverID, "socks_data", dataPayload)
		}
	}
}

func encodeBase64(data []byte) string {
	const enc = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	result := make([]byte, ((len(data)+2)/3)*4)
	i, j := 0, 0
	for i < len(data)-2 {
		result[j] = enc[data[i]>>2]
		result[j+1] = enc[((data[i]&0x3)<<4)|(data[i+1]>>4)]
		result[j+2] = enc[((data[i+1]&0xF)<<2)|(data[i+2]>>6)]
		result[j+3] = enc[data[i+2]&0x3F]
		i += 3
		j += 4
	}
	if i < len(data) {
		result[j] = enc[data[i]>>2]
		if i+1 < len(data) {
			result[j+1] = enc[((data[i]&0x3)<<4)|(data[i+1]>>4)]
			result[j+2] = enc[(data[i+1]&0xF)<<2]
			result[j+3] = '='
		} else {
			result[j+1] = enc[(data[i]&0x3)<<4]
			result[j+2] = '='
			result[j+3] = '='
		}
	}
	return string(result)
}
