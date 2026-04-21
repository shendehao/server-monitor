package handler

import (
	"net/http"
	"server-monitor/internal/model"
	"server-monitor/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type NotifyHandler struct {
	db        *gorm.DB
	collector *service.Collector
}

func NewNotifyHandler(db *gorm.DB, collector *service.Collector) *NotifyHandler {
	return &NotifyHandler{db: db, collector: collector}
}

func (h *NotifyHandler) GetConfig(c *gin.Context) {
	var config model.NotifyConfig
	if h.db.First(&config).Error != nil {
		config = model.NotifyConfig{
			Enabled:        false,
			NotifyWarning:  true,
			NotifyCritical: true,
			NotifyOffline:  true,
			CooldownMin:    10,
		}
		h.db.Create(&config)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": config})
}

func (h *NotifyHandler) UpdateConfig(c *gin.Context) {
	var body struct {
		Enabled        *bool   `json:"enabled"`
		WebhookURL     *string `json:"webhookUrl"`
		NotifyWarning  *bool   `json:"notifyWarning"`
		NotifyCritical *bool   `json:"notifyCritical"`
		NotifyOffline  *bool   `json:"notifyOffline"`
		CooldownMin    *int    `json:"cooldownMin"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "参数错误"})
		return
	}

	var config model.NotifyConfig
	if h.db.First(&config).Error != nil {
		config = model.NotifyConfig{}
		h.db.Create(&config)
	}

	if body.Enabled != nil {
		config.Enabled = *body.Enabled
	}
	if body.WebhookURL != nil {
		config.WebhookURL = *body.WebhookURL
	}
	if body.NotifyWarning != nil {
		config.NotifyWarning = *body.NotifyWarning
	}
	if body.NotifyCritical != nil {
		config.NotifyCritical = *body.NotifyCritical
	}
	if body.NotifyOffline != nil {
		config.NotifyOffline = *body.NotifyOffline
	}
	if body.CooldownMin != nil {
		config.CooldownMin = *body.CooldownMin
	}

	h.db.Save(&config)

	// 重新加载通知配置
	if ns := h.collector.GetNotifyService(); ns != nil {
		ns.ReloadConfig()
	}

	username, _ := c.Get("username")
	logSecurity("notify_config_update", getClientIP(c), username.(string), "更新通知推送配置")

	c.JSON(http.StatusOK, gin.H{"success": true, "data": config})
}

func (h *NotifyHandler) TestNotify(c *gin.Context) {
	ns := h.collector.GetNotifyService()
	if ns == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "通知服务未初始化"})
		return
	}

	if err := ns.SendTestMessage(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "测试消息已发送"})
}
