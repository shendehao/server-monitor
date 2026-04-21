package model

import "time"

// 统一响应结构
type Response struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data"`
	Error     string      `json:"error,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

// 分页响应
type PaginatedList struct {
	List     interface{} `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// 概览数据
type Overview struct {
	ServerCount  int             `json:"serverCount"`
	OnlineCount  int             `json:"onlineCount"`
	OfflineCount int             `json:"offlineCount"`
	WarningCount int             `json:"warningCount"`
	AvgCPU       float64         `json:"avgCpu"`
	AvgMemory    float64         `json:"avgMemory"`
	AvgDisk      float64         `json:"avgDisk"`
	ActiveAlerts int64           `json:"activeAlerts"`
	Servers      []ServerSummary `json:"servers"`
}

type ServerSummary struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	IsOnline     bool    `json:"isOnline"`
	CPUUsage     float64 `json:"cpuUsage"`
	MemUsage     float64 `json:"memUsage"`
	DiskUsage    float64 `json:"diskUsage"`
	Status       string  `json:"status"` // normal / warning / danger / offline
	AgentVersion string  `json:"agentVersion,omitempty"`
}

// 服务器详情（含最新指标）
type ServerDetail struct {
	Server
	IsOnline      bool    `json:"isOnline"`
	Uptime        string  `json:"uptime"`
	LatestMetrics *Metric `json:"latestMetrics"`
}

// 实时趋势数据
type RealtimeSeries struct {
	Metric  string       `json:"metric"`
	Minutes int          `json:"minutes"`
	Series  []SeriesItem `json:"series"`
}

type SeriesItem struct {
	ServerID   string      `json:"serverId"`
	ServerName string      `json:"serverName"`
	Color      string      `json:"color"`
	Data       []DataPoint `json:"data"`
}

type DataPoint struct {
	T time.Time `json:"t"`
	V float64   `json:"v"`
}

// 告警计数
type AlertCount struct {
	Total    int64 `json:"total"`
	Critical int64 `json:"critical"`
	Warning  int64 `json:"warning"`
	Info     int64 `json:"info"`
}

// 连接测试
type TestResult struct {
	Connected  bool                   `json:"connected"`
	Latency    int64                  `json:"latency"`
	ServerInfo map[string]interface{} `json:"serverInfo"`
	Message    string                 `json:"message"`
}
