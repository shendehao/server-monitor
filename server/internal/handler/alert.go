package handler

import (
	"net/http"
	"server-monitor/internal/model"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AlertHandler struct {
	db *gorm.DB
}

func NewAlertHandler(db *gorm.DB) *AlertHandler {
	return &AlertHandler{db: db}
}

func (h *AlertHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	severity := c.Query("severity")
	resolved := c.Query("resolved")
	serverID := c.Query("server_id")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	query := h.db.Model(&model.Alert{})
	if severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if resolved == "true" {
		query = query.Where("is_resolved = ?", true)
	} else if resolved == "false" {
		query = query.Where("is_resolved = ?", false)
	}
	if serverID != "" {
		query = query.Where("server_id = ?", serverID)
	}

	var total int64
	query.Count(&total)

	var alerts []model.Alert
	query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&alerts)

	// 填充服务器名称
	type AlertWithServer struct {
		model.Alert
		ServerName string `json:"serverName"`
	}

	var result []AlertWithServer
	for _, a := range alerts {
		var server model.Server
		h.db.Select("name").First(&server, "id = ?", a.ServerID)
		result = append(result, AlertWithServer{
			Alert:      a,
			ServerName: server.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": model.PaginatedList{
			List:     result,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}

func (h *AlertHandler) Count(c *gin.Context) {
	var count model.AlertCount
	h.db.Model(&model.Alert{}).Where("is_resolved = ?", false).Count(&count.Total)
	h.db.Model(&model.Alert{}).Where("is_resolved = ? AND severity = ?", false, "critical").Count(&count.Critical)
	h.db.Model(&model.Alert{}).Where("is_resolved = ? AND severity = ?", false, "warning").Count(&count.Warning)
	h.db.Model(&model.Alert{}).Where("is_resolved = ? AND severity = ?", false, "info").Count(&count.Info)

	c.JSON(http.StatusOK, gin.H{"success": true, "data": count})
}

func (h *AlertHandler) Resolve(c *gin.Context) {
	id := c.Param("id")
	now := time.Now()
	result := h.db.Model(&model.Alert{}).Where("id = ? AND is_resolved = ?", id, false).
		Updates(map[string]interface{}{"is_resolved": true, "resolved_at": now})

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "告警不存在或已解决"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AlertHandler) BatchResolve(c *gin.Context) {
	var body struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "参数错误"})
		return
	}
	now := time.Now()
	h.db.Model(&model.Alert{}).Where("id IN ? AND is_resolved = ?", body.IDs, false).
		Updates(map[string]interface{}{"is_resolved": true, "resolved_at": now})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AlertHandler) ListRules(c *gin.Context) {
	var rules []model.AlertRule
	h.db.Find(&rules)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rules})
}

func (h *AlertHandler) UpdateRule(c *gin.Context) {
	id := c.Param("id")
	var rule model.AlertRule
	if err := h.db.First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "规则不存在"})
		return
	}

	var body struct {
		WarningThreshold  *int  `json:"warningThreshold"`
		CriticalThreshold *int  `json:"criticalThreshold"`
		ConsecutiveCount  *int  `json:"consecutiveCount"`
		Enabled           *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "参数错误"})
		return
	}

	if body.WarningThreshold != nil {
		rule.WarningThreshold = *body.WarningThreshold
	}
	if body.CriticalThreshold != nil {
		rule.CriticalThreshold = *body.CriticalThreshold
	}
	if body.ConsecutiveCount != nil {
		rule.ConsecutiveCount = *body.ConsecutiveCount
	}
	if body.Enabled != nil {
		rule.Enabled = *body.Enabled
	}

	h.db.Save(&rule)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rule})
}
