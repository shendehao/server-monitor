package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"server-monitor/internal/model"
	"sync"
	"time"

	"gorm.io/gorm"
)

// NotifyService 通知推送服务
type NotifyService struct {
	db       *gorm.DB
	mu       sync.RWMutex
	config   *model.NotifyConfig
	lastSent map[string]time.Time // key: serverId+alertType -> 上次发送时间
}

func NewNotifyService(db *gorm.DB) *NotifyService {
	ns := &NotifyService{
		db:       db,
		lastSent: make(map[string]time.Time),
	}
	ns.ReloadConfig()
	return ns
}

// ReloadConfig 从数据库加载通知配置
func (ns *NotifyService) ReloadConfig() {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	var config model.NotifyConfig
	if ns.db.First(&config).Error != nil {
		// 不存在则创建默认配置
		config = model.NotifyConfig{
			Enabled:       false,
			WebhookURL:    "",
			NotifyWarning: true,
			NotifyCritical: true,
			NotifyOffline: true,
			CooldownMin:   10,
		}
		ns.db.Create(&config)
	}
	ns.config = &config
}

// SendAlert 发送告警通知
func (ns *NotifyService) SendAlert(serverName, alertType, severity, message string) {
	ns.mu.RLock()
	cfg := ns.config
	ns.mu.RUnlock()

	if cfg == nil || !cfg.Enabled || cfg.WebhookURL == "" {
		return
	}

	// 检查是否需要发送该级别
	switch severity {
	case "warning":
		if !cfg.NotifyWarning {
			return
		}
	case "critical":
		if !cfg.NotifyCritical {
			return
		}
	}

	if alertType == "offline" && !cfg.NotifyOffline {
		return
	}

	// 冷却时间检查（防止频繁推送同一类告警）
	cooldownKey := serverName + ":" + alertType
	ns.mu.RLock()
	lastTime, exists := ns.lastSent[cooldownKey]
	ns.mu.RUnlock()

	if exists && time.Since(lastTime) < time.Duration(cfg.CooldownMin)*time.Minute {
		return
	}

	// 更新发送时间
	ns.mu.Lock()
	ns.lastSent[cooldownKey] = time.Now()
	ns.mu.Unlock()

	// 构建消息
	go ns.sendWeComMessage(serverName, alertType, severity, message)
}

func (ns *NotifyService) sendWeComMessage(serverName, alertType, severity, message string) {
	ns.mu.RLock()
	webhookURL := ns.config.WebhookURL
	ns.mu.RUnlock()

	// 严重等级 emoji
	severityIcon := "⚠️"
	severityText := "警告"
	if severity == "critical" {
		severityIcon = "🔴"
		severityText = "危险"
	}

	// 告警类型映射
	typeMap := map[string]string{
		"cpu_high":  "CPU 过高",
		"mem_high":  "内存过高",
		"disk_full": "磁盘空间不足",
		"offline":   "服务器离线",
		"load_high": "负载过高",
	}
	typeName := typeMap[alertType]
	if typeName == "" {
		typeName = alertType
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	content := fmt.Sprintf(`%s **服务器监控告警**
> **服务器**: %s
> **类型**: %s
> **级别**: <font color="%s">%s</font>
> **详情**: %s
> **时间**: %s`,
		severityIcon,
		serverName,
		typeName,
		map[string]string{"warning": "warning", "critical": "red"}[severity],
		severityText,
		message,
		now,
	)

	body := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": content,
		},
	}

	jsonData, _ := json.Marshal(body)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Printf("[通知] 企业微信推送失败: %v", err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.ErrCode != 0 {
		log.Printf("[通知] 企业微信返回错误: %d %s", result.ErrCode, result.ErrMsg)
	} else {
		log.Printf("[通知] 已推送告警: %s - %s", serverName, typeName)
	}
}

// SendTestMessage 发送测试消息
func (ns *NotifyService) SendTestMessage() error {
	ns.mu.RLock()
	cfg := ns.config
	ns.mu.RUnlock()

	if cfg == nil || cfg.WebhookURL == "" {
		return fmt.Errorf("未配置 Webhook URL")
	}

	body := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": fmt.Sprintf("✅ **服务器监控 - 测试消息**\n> 推送配置验证成功\n> 时间: %s",
				time.Now().Format("2006-01-02 15:04:05")),
		},
	}

	jsonData, _ := json.Marshal(body)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(cfg.WebhookURL, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.ErrCode != 0 {
		return fmt.Errorf("企业微信错误: %s", result.ErrMsg)
	}
	return nil
}
