package model

import (
	"time"
)

type Alert struct {
	ID         uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID   string     `json:"serverId" gorm:"size:36;index;not null"`
	AlertType  string     `json:"alertType" gorm:"size:20;not null"` // cpu_high / mem_high / disk_full / offline / load_high
	Message    string     `json:"message" gorm:"size:500"`
	Severity   string     `json:"severity" gorm:"size:10;not null"` // info / warning / critical
	IsResolved bool       `json:"isResolved" gorm:"default:false;index"`
	CreatedAt  time.Time  `json:"createdAt" gorm:"index"`
	ResolvedAt *time.Time `json:"resolvedAt"`
}

type AlertRule struct {
	ID                uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	MetricType        string `json:"metric" gorm:"size:20;not null;uniqueIndex"`
	WarningThreshold  int    `json:"warningThreshold"`
	CriticalThreshold int    `json:"criticalThreshold"`
	ConsecutiveCount  int    `json:"consecutiveCount" gorm:"default:3"`
	Enabled           bool   `json:"enabled" gorm:"default:true"`
	Description       string `json:"description" gorm:"size:100"`
}
