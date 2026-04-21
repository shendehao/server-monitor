package model

import (
	"time"
)

type Metric struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID     string    `json:"serverId" gorm:"size:36;not null;index:idx_metric_server_time"`
	CPUUsage     float64   `json:"cpuUsage"`
	MemTotal     int64     `json:"memTotal"`  // MB
	MemUsed      int64     `json:"memUsed"`   // MB
	MemUsage     float64   `json:"memUsage"`  // %
	DiskTotal    int64     `json:"diskTotal"` // GB
	DiskUsed     int64     `json:"diskUsed"`  // GB
	DiskUsage    float64   `json:"diskUsage"` // %
	NetIn        int64     `json:"netIn"`     // Bytes/s
	NetOut       int64     `json:"netOut"`    // Bytes/s
	Load1m       float64   `json:"load1m"`
	Load5m       float64   `json:"load5m"`
	Load15m      float64   `json:"load15m"`
	ProcessCount int       `json:"processCount"`
	Uptime       string    `json:"uptime" gorm:"size:50"`
	CollectedAt  time.Time `json:"collectedAt" gorm:"index:idx_metric_server_time"`
}
