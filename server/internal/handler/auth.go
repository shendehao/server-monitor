package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"server-monitor/internal/model"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ──────────────────── 全局安全状态 ────────────────────

var (
	authDB    *gorm.DB
	jwtSecret []byte

	// 登录频率限制: IP -> 最近失败记录
	loginAttempts   = make(map[string]*attemptInfo)
	loginAttemptsMu sync.RWMutex

	// IP 黑名单缓存
	blacklistCache   = make(map[string]time.Time) // IP -> 过期时间
	blacklistCacheMu sync.RWMutex

	// 请求频率限制
	rateLimitMap   = make(map[string]*rateInfo)
	rateLimitMapMu sync.RWMutex
)

type attemptInfo struct {
	Count    int
	LastFail time.Time
	Locked   bool
	LockUtil time.Time
}

type rateInfo struct {
	Count  int
	Window time.Time
}

const (
	maxLoginAttempts  = 5                // 最大连续登录失败次数
	loginLockDuration = 15 * time.Minute // 锁定时长
	autoBanThreshold  = 15               // 自动封禁阈值 (24h 内)
	autoBanDuration   = 24 * time.Hour   // 自动封禁时长
	rateLimit         = 300              // 每分钟最大请求数
	rateLimitWindow   = time.Minute
)

// InitAuth 初始化认证模块
func InitAuth(db *gorm.DB) {
	authDB = db
	secret := model.GetJWTSecret(db)
	jwtSecret = []byte(secret)

	// 加载黑名单到缓存
	refreshBlacklistCache()

	// 后台清理过期数据
	go securityCleanupLoop()
}

func refreshBlacklistCache() {
	if authDB == nil {
		return
	}
	var items []model.IPBlacklist
	authDB.Find(&items)

	blacklistCacheMu.Lock()
	blacklistCache = make(map[string]time.Time)
	for _, item := range items {
		if item.ExpiresAt == nil {
			blacklistCache[item.IP] = time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC) // 永久
		} else {
			blacklistCache[item.IP] = *item.ExpiresAt
		}
	}
	blacklistCacheMu.Unlock()
}

func securityCleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		// 清理过期黑名单
		if authDB != nil {
			authDB.Where("expires_at IS NOT NULL AND expires_at < ?", time.Now()).Delete(&model.IPBlacklist{})
			refreshBlacklistCache()
		}

		// 清理过期登录锁定
		loginAttemptsMu.Lock()
		now := time.Now()
		for ip, info := range loginAttempts {
			if info.Locked && now.After(info.LockUtil) {
				delete(loginAttempts, ip)
			} else if now.Sub(info.LastFail) > loginLockDuration {
				delete(loginAttempts, ip)
			}
		}
		loginAttemptsMu.Unlock()

		// 清理过期速率限制
		rateLimitMapMu.Lock()
		for ip, info := range rateLimitMap {
			if now.Sub(info.Window) > rateLimitWindow {
				delete(rateLimitMap, ip)
			}
		}
		rateLimitMapMu.Unlock()

		// 清理 30 天前的登录记录
		if authDB != nil {
			authDB.Where("created_at < ?", now.Add(-30*24*time.Hour)).Delete(&model.LoginAttempt{})
		}
	}
}

// ──────────────────── 中间件 ────────────────────

// IPBlacklistMiddleware IP 黑名单检查
func IPBlacklistMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := getClientIP(c)

		blacklistCacheMu.RLock()
		expiresAt, banned := blacklistCache[ip]
		blacklistCacheMu.RUnlock()

		if banned {
			if time.Now().Before(expiresAt) {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "您的 IP 已被封禁"})
				c.Abort()
				return
			}
			// 已过期，从缓存移除
			blacklistCacheMu.Lock()
			delete(blacklistCache, ip)
			blacklistCacheMu.Unlock()
		}

		c.Next()
	}
}

// RateLimitMiddleware 全局请求频率限制
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 排除静态页面/资源和 Agent 高频接口（report / ws / stager / payload 需要稳定访问）
		path := c.Request.URL.Path
		if path == "/" || path == "/favicon.ico" || strings.HasPrefix(path, "/assets/") ||
			path == "/api/agent/report" || path == "/ws/agent" ||
			path == "/api/agent/download" || path == "/api/agent/download-win" ||
			path == "/api/agent/install.sh" || path == "/api/agent/install.ps1" || path == "/api/agent/cleanup.ps1" ||
			path == "/api/agent/payload" || path == "/api/agent/stager" || path == "/api/agent/cradle" || path == "/api/agent/cradle-b64" {
			c.Next()
			return
		}

		ip := getClientIP(c)

		rateLimitMapMu.Lock()
		info, exists := rateLimitMap[ip]
		now := time.Now()

		if !exists || now.Sub(info.Window) > rateLimitWindow {
			rateLimitMap[ip] = &rateInfo{Count: 1, Window: now}
			rateLimitMapMu.Unlock()
			c.Next()
			return
		}

		info.Count++
		if info.Count > rateLimit {
			rateLimitMapMu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{"success": false, "error": "请求过于频繁，请稍后重试"})
			c.Abort()
			return
		}
		rateLimitMapMu.Unlock()
		c.Next()
	}
}

// AuthMiddleware JWT 认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "未登录"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		claims, err := parseToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "令牌无效"})
			c.Abort()
			return
		}

		if claims.Exp < time.Now().Unix() {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "令牌已过期"})
			c.Abort()
			return
		}

		c.Set("username", claims.Username)
		c.Next()
	}
}

// ──────────────────── 登录/认证 ────────────────────

type LoginForm struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type Claims struct {
	Username string `json:"username"`
	Exp      int64  `json:"exp"`
}

func Login(c *gin.Context) {
	var form LoginForm
	if err := c.ShouldBindJSON(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请输入用户名和密码"})
		return
	}

	ip := getClientIP(c)

	// 检查是否被锁定
	loginAttemptsMu.RLock()
	info, exists := loginAttempts[ip]
	loginAttemptsMu.RUnlock()

	if exists && info.Locked && time.Now().Before(info.LockUtil) {
		remaining := time.Until(info.LockUtil).Minutes()
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success": false,
			"error":   fmt.Sprintf("登录失败次数过多，请 %.0f 分钟后重试", remaining),
		})
		logSecurity("login_locked", ip, form.Username, "IP 被锁定")
		return
	}

	// 从数据库验证
	var user model.AdminUser
	if authDB.Where("username = ?", form.Username).First(&user).Error != nil {
		recordLoginFail(ip, form.Username, c.GetHeader("User-Agent"))
		time.Sleep(500 * time.Millisecond)
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "用户名或密码错误"})
		return
	}

	if !model.CheckPassword(form.Password, user.Password) {
		recordLoginFail(ip, form.Username, c.GetHeader("User-Agent"))
		time.Sleep(500 * time.Millisecond)
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "用户名或密码错误"})
		return
	}

	// 登录成功，清除失败记录
	loginAttemptsMu.Lock()
	delete(loginAttempts, ip)
	loginAttemptsMu.Unlock()

	// 记录成功登录
	if authDB != nil {
		authDB.Create(&model.LoginAttempt{
			IP:        ip,
			Username:  form.Username,
			Success:   true,
			UserAgent: truncate(c.GetHeader("User-Agent"), 500),
		})
	}
	logSecurity("login_success", ip, form.Username, "登录成功")

	token, err := generateToken(form.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "生成令牌失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"token":     token,
			"username":  form.Username,
			"expiresIn": 86400,
		},
	})
}

func recordLoginFail(ip, username, ua string) {
	// 记录到数据库
	if authDB != nil {
		authDB.Create(&model.LoginAttempt{
			IP:        ip,
			Username:  username,
			Success:   false,
			UserAgent: truncate(ua, 500),
		})
	}

	// 更新内存计数
	loginAttemptsMu.Lock()
	info, exists := loginAttempts[ip]
	if !exists {
		info = &attemptInfo{}
		loginAttempts[ip] = info
	}
	info.Count++
	info.LastFail = time.Now()

	if info.Count >= maxLoginAttempts {
		info.Locked = true
		info.LockUtil = time.Now().Add(loginLockDuration)
		log.Printf("[安全] IP %s 登录失败 %d 次，锁定 %v", ip, info.Count, loginLockDuration)
	}
	loginAttemptsMu.Unlock()

	logSecurity("login_fail", ip, username, fmt.Sprintf("登录失败 (第 %d 次)", info.Count))

	// 检查是否需要自动封禁
	if authDB != nil {
		var failCount int64
		authDB.Model(&model.LoginAttempt{}).
			Where("ip = ? AND success = false AND created_at > ?", ip, time.Now().Add(-24*time.Hour)).
			Count(&failCount)

		if failCount >= autoBanThreshold {
			autoBanIP(ip)
		}
	}
}

func autoBanIP(ip string) {
	if authDB == nil {
		return
	}

	// 检查是否已在黑名单
	var count int64
	authDB.Model(&model.IPBlacklist{}).Where("ip = ?", ip).Count(&count)
	if count > 0 {
		return
	}

	expires := time.Now().Add(autoBanDuration)
	authDB.Create(&model.IPBlacklist{
		IP:        ip,
		Reason:    fmt.Sprintf("登录失败超过 %d 次，自动封禁", autoBanThreshold),
		AutoBan:   true,
		ExpiresAt: &expires,
	})

	blacklistCacheMu.Lock()
	blacklistCache[ip] = expires
	blacklistCacheMu.Unlock()

	log.Printf("[安全] IP %s 自动封禁至 %s", ip, expires.Format("2006-01-02 15:04:05"))
	logSecurity("auto_ban", ip, "", fmt.Sprintf("自动封禁至 %s", expires.Format("2006-01-02 15:04:05")))
}

// ──────────────────── 修改密码 ────────────────────

type ChangePasswordForm struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}

func ChangePassword(c *gin.Context) {
	username, _ := c.Get("username")
	var form ChangePasswordForm
	if err := c.ShouldBindJSON(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "参数错误，新密码至少 6 位"})
		return
	}

	var user model.AdminUser
	if authDB.Where("username = ?", username).First(&user).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "用户不存在"})
		return
	}

	if !model.CheckPassword(form.OldPassword, user.Password) {
		logSecurity("password_change_fail", getClientIP(c), username.(string), "旧密码错误")
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "原密码错误"})
		return
	}

	hash, err := model.HashPassword(form.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "密码加密失败"})
		return
	}

	authDB.Model(&user).Update("password", hash)
	logSecurity("password_changed", getClientIP(c), username.(string), "密码修改成功")

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "密码修改成功"})
}

func GetUserInfo(c *gin.Context) {
	username, _ := c.Get("username")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"username": username,
			"role":     "admin",
		},
	})
}

// ──────────────────── 黑名单管理 ────────────────────

func ListBlacklist(c *gin.Context) {
	var items []model.IPBlacklist
	authDB.Order("created_at DESC").Find(&items)
	c.JSON(200, gin.H{"success": true, "data": items})
}

type AddBlacklistForm struct {
	IP       string `json:"ip" binding:"required"`
	Reason   string `json:"reason"`
	Duration int    `json:"duration"` // 小时, 0=永久
}

func AddBlacklist(c *gin.Context) {
	var form AddBlacklistForm
	if err := c.ShouldBindJSON(&form); err != nil {
		c.JSON(400, gin.H{"success": false, "error": "参数错误"})
		return
	}

	// 验证 IP 格式
	if net.ParseIP(form.IP) == nil {
		// 允许 CIDR 格式
		if _, _, err := net.ParseCIDR(form.IP); err != nil {
			c.JSON(400, gin.H{"success": false, "error": "IP 格式无效"})
			return
		}
	}

	// 检查是否已存在
	var count int64
	authDB.Model(&model.IPBlacklist{}).Where("ip = ?", form.IP).Count(&count)
	if count > 0 {
		c.JSON(400, gin.H{"success": false, "error": "该 IP 已在黑名单中"})
		return
	}

	item := model.IPBlacklist{
		IP:      form.IP,
		Reason:  form.Reason,
		AutoBan: false,
	}

	if form.Duration > 0 {
		expires := time.Now().Add(time.Duration(form.Duration) * time.Hour)
		item.ExpiresAt = &expires
	}

	authDB.Create(&item)

	// 更新缓存
	blacklistCacheMu.Lock()
	if item.ExpiresAt == nil {
		blacklistCache[form.IP] = time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	} else {
		blacklistCache[form.IP] = *item.ExpiresAt
	}
	blacklistCacheMu.Unlock()

	username, _ := c.Get("username")
	logSecurity("blacklist_add", getClientIP(c), username.(string), fmt.Sprintf("添加黑名单: %s", form.IP))

	c.JSON(200, gin.H{"success": true, "message": "已添加"})
}

func RemoveBlacklist(c *gin.Context) {
	id := c.Param("id")
	var item model.IPBlacklist
	if authDB.First(&item, id).Error != nil {
		c.JSON(404, gin.H{"success": false, "error": "记录不存在"})
		return
	}

	authDB.Delete(&item)

	blacklistCacheMu.Lock()
	delete(blacklistCache, item.IP)
	blacklistCacheMu.Unlock()

	username, _ := c.Get("username")
	logSecurity("blacklist_remove", getClientIP(c), username.(string), fmt.Sprintf("移除黑名单: %s", item.IP))

	c.JSON(200, gin.H{"success": true, "message": "已移除"})
}

// ──────────────────── 安全日志 ────────────────────

func ListSecurityLogs(c *gin.Context) {
	var logs []model.SecurityLog
	authDB.Order("created_at DESC").Limit(200).Find(&logs)
	c.JSON(200, gin.H{"success": true, "data": logs})
}

func ListLoginAttempts(c *gin.Context) {
	var attempts []model.LoginAttempt
	authDB.Order("created_at DESC").Limit(200).Find(&attempts)
	c.JSON(200, gin.H{"success": true, "data": attempts})
}

// ──────────────────── 工具函数 ────────────────────

func getClientIP(c *gin.Context) string {
	// 支持反向代理
	if ip := c.GetHeader("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := c.GetHeader("X-Forwarded-For"); ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}
	ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
	return ip
}

func logSecurity(action, ip, username, detail string) {
	if authDB != nil {
		authDB.Create(&model.SecurityLog{
			Action:   action,
			IP:       ip,
			Username: username,
			Detail:   detail,
		})
	}
	log.Printf("[安全] %s | IP=%s | 用户=%s | %s", action, ip, username, detail)
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

// ──────────────────── JWT ────────────────────

func generateToken(username string) (string, error) {
	header := base64url([]byte(`{"alg":"HS256","typ":"JWT"}`))

	claims := Claims{
		Username: username,
		Exp:      time.Now().Add(24 * time.Hour).Unix(),
	}
	payload, _ := json.Marshal(claims)
	payloadEnc := base64url(payload)

	sig := sign(header + "." + payloadEnc)
	return header + "." + payloadEnc + "." + sig, nil
}

func parseToken(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	expectedSig := sign(parts[0] + "." + parts[1])
	if parts[2] != expectedSig {
		return nil, fmt.Errorf("invalid signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	return &claims, nil
}

func base64url(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func sign(data string) string {
	h := hmac.New(sha256.New, jwtSecret)
	h.Write([]byte(data))
	return base64url(h.Sum(nil))
}
