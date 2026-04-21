package service

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"server-monitor/internal/config"
	"server-monitor/internal/model"
	"server-monitor/internal/ws"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type Collector struct {
	db     *gorm.DB
	hub    *ws.Hub
	cfg    *config.Config
	cron   *cron.Cron
	notify *NotifyService
	// 缓存每台服务器最新指标
	cache sync.Map // map[string]*model.Metric
	// 缓存在线状态
	online sync.Map // map[string]bool
	// Agent 最后上报时间
	lastReport sync.Map // map[string]time.Time
	// 缓存 Agent 版本号
	agentVersions sync.Map // map[string]string
}

func NewCollector(db *gorm.DB, hub *ws.Hub, cfg *config.Config) *Collector {
	return &Collector{
		db:     db,
		hub:    hub,
		cfg:    cfg,
		notify: NewNotifyService(db),
	}
}

func (c *Collector) SetAgentVersion(serverID, version string) {
	c.agentVersions.Store(serverID, version)
}

func (c *Collector) GetAgentVersion(serverID string) string {
	if v, ok := c.agentVersions.Load(serverID); ok {
		return v.(string)
	}
	return ""
}

// GetNotifyService 获取通知服务（供 handler 调用）
func (c *Collector) GetNotifyService() *NotifyService {
	return c.notify
}

func (c *Collector) Start() {
	c.cron = cron.New(cron.WithSeconds())
	spec := fmt.Sprintf("@every %ds", c.cfg.CollectInterval)
	c.cron.AddFunc(spec, c.collectAll)
	c.cron.Start()
	log.Printf("采集服务已启动，间隔: %d秒", c.cfg.CollectInterval)

	// 立即执行一次采集
	go c.collectAll()
}

func (c *Collector) Stop() {
	if c.cron != nil {
		c.cron.Stop()
	}
}

func (c *Collector) collectAll() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("collectAll panic: %v", r)
		}
	}()

	var servers []model.Server
	c.db.Where("is_active = ?", true).Order("sort_order ASC").Find(&servers)

	if len(servers) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, server := range servers {
		wg.Add(1)
		go func(s model.Server) {
			defer wg.Done()
			c.collectOne(s)
		}(server)
	}
	wg.Wait()

	// 广播更新
	c.broadcastMetrics(servers)
}

func (c *Collector) collectOne(s model.Server) {
	// 如果设置了 MOCK_DATA=1 则使用模拟数据
	if os.Getenv("MOCK_DATA") == "1" {
		metric := c.generateMockMetric(s.ID)
		c.db.Create(metric)
		c.cache.Store(s.ID, metric)
		c.online.Store(s.ID, true)
		c.checkAlerts(s, metric)
		return
	}

	var metric *model.Metric
	var err error

	switch s.ConnectMethod {
	case "ssh", "":
		metric, err = c.collectViaSSH(s)
	case "agent", "plugin":
		// Agent/插件 由自身上报指标，定时采集跳过
		return
	case "api":
		// TODO: 实现云 API 采集
		err = fmt.Errorf("API 连接方式尚未实现")
	default:
		err = fmt.Errorf("未知连接方式: %s", s.ConnectMethod)
	}

	if err != nil {
		log.Printf("[%s] 采集失败: %v", s.Name, err)
		c.online.Store(s.ID, false)
		return
	}

	// 保存到数据库
	c.db.Create(metric)

	// 缓存
	c.cache.Store(s.ID, metric)
	c.online.Store(s.ID, true)

	// 检查告警
	c.checkAlerts(s, metric)
}

func (c *Collector) collectViaSSH(s model.Server) (*model.Metric, error) {
	timeout := time.Duration(c.cfg.SSHTimeout) * time.Second
	client := NewSSHClient(s.Host, s.Port, s.Username, s.AuthType, s.AuthValue, timeout)

	var cmd string
	switch s.OSType {
	case "windows":
		return nil, fmt.Errorf("Windows SSH 采集暂未实现")
	default: // linux
		cmd = linuxCollectCmd
	}

	output, err := client.Run(cmd)
	if err != nil {
		return nil, err
	}

	return parseLinuxMetrics(s.ID, output)
}

func (c *Collector) generateMockMetric(serverID string) *model.Metric {
	now := time.Now()

	// 根据 serverID 生成不同范围的模拟数据
	var baseCPU, baseMem, baseDisk float64
	switch serverID {
	case "s1":
		baseCPU, baseMem, baseDisk = 35, 55, 45
	case "s2":
		baseCPU, baseMem, baseDisk = 50, 65, 60
	case "s3":
		baseCPU, baseMem, baseDisk = 25, 45, 35
	case "s4":
		baseCPU, baseMem, baseDisk = 75, 82, 70
	case "s5":
		baseCPU, baseMem, baseDisk = 40, 58, 50
	case "s6":
		baseCPU, baseMem, baseDisk = 30, 50, 40
	case "s7":
		baseCPU, baseMem, baseDisk = 60, 70, 55
	default:
		baseCPU, baseMem, baseDisk = 45, 60, 50
	}

	cpu := baseCPU + (rand.Float64()-0.5)*20
	mem := baseMem + (rand.Float64()-0.5)*10
	disk := baseDisk + (rand.Float64()-0.5)*5

	if cpu < 0 {
		cpu = 1
	}
	if cpu > 100 {
		cpu = 99
	}
	if mem < 0 {
		mem = 1
	}
	if mem > 100 {
		mem = 99
	}

	memTotal := int64(16384)
	memUsed := int64(float64(memTotal) * mem / 100)
	diskTotal := int64(500)
	diskUsed := int64(float64(diskTotal) * disk / 100)

	return &model.Metric{
		ServerID:     serverID,
		CPUUsage:     round2(cpu),
		MemTotal:     memTotal,
		MemUsed:      memUsed,
		MemUsage:     round2(mem),
		DiskTotal:    diskTotal,
		DiskUsed:     diskUsed,
		DiskUsage:    round2(disk),
		NetIn:        int64(rand.Intn(10000000)),
		NetOut:       int64(rand.Intn(5000000)),
		Load1m:       round2(rand.Float64() * 4),
		Load5m:       round2(rand.Float64() * 3),
		Load15m:      round2(rand.Float64() * 2),
		ProcessCount: 150 + rand.Intn(100),
		Uptime:       fmt.Sprintf("%d天%d小时", 10+rand.Intn(50), rand.Intn(24)),
		CollectedAt:  now,
	}
}

func (c *Collector) checkAlerts(s model.Server, m *model.Metric) {
	var rules []model.AlertRule
	c.db.Where("enabled = ?", true).Find(&rules)

	for _, rule := range rules {
		var value float64
		var alertType, metricName string

		switch rule.MetricType {
		case "cpu":
			value = m.CPUUsage
			alertType = "cpu_high"
			metricName = "CPU"
		case "memory":
			value = m.MemUsage
			alertType = "mem_high"
			metricName = "内存"
		case "disk":
			value = m.DiskUsage
			alertType = "disk_full"
			metricName = "磁盘"
		default:
			continue
		}

		if value >= float64(rule.CriticalThreshold) {
			c.createAlert(s, alertType, "critical",
				fmt.Sprintf("%s 使用率 %.1f%% 超过危险阈值 (%d%%)", metricName, value, rule.CriticalThreshold))
		} else if value >= float64(rule.WarningThreshold) {
			c.createAlert(s, alertType, "warning",
				fmt.Sprintf("%s 使用率 %.1f%% 超过警告阈值 (%d%%)", metricName, value, rule.WarningThreshold))
		} else {
			// 自动解决之前的告警
			c.resolveAlerts(s.ID, alertType)
		}
	}
}

func (c *Collector) createAlert(s model.Server, alertType, severity, message string) {
	// 检查是否已有未解决的同类告警
	var existing model.Alert
	result := c.db.Where("server_id = ? AND alert_type = ? AND is_resolved = ?", s.ID, alertType, false).First(&existing)
	if result.Error == nil {
		return // 已存在未解决的同类告警
	}

	alert := model.Alert{
		ServerID:  s.ID,
		AlertType: alertType,
		Message:   message,
		Severity:  severity,
	}
	c.db.Create(&alert)

	// 广播告警
	c.hub.Broadcast("alert", map[string]interface{}{
		"id":         alert.ID,
		"serverId":   s.ID,
		"serverName": s.Name,
		"alertType":  alertType,
		"message":    message,
		"severity":   severity,
		"createdAt":  alert.CreatedAt,
	})

	// 企业微信推送
	if c.notify != nil {
		c.notify.SendAlert(s.Name, alertType, severity, message)
	}
}

func (c *Collector) resolveAlerts(serverID, alertType string) {
	now := time.Now()
	c.db.Model(&model.Alert{}).
		Where("server_id = ? AND alert_type = ? AND is_resolved = ?", serverID, alertType, false).
		Updates(map[string]interface{}{
			"is_resolved": true,
			"resolved_at": now,
		})
}

func (c *Collector) broadcastMetrics(servers []model.Server) {
	type serverMetric struct {
		ServerID     string  `json:"serverId"`
		ServerName   string  `json:"serverName"`
		IsOnline     bool    `json:"isOnline"`
		CPUUsage     float64 `json:"cpuUsage"`
		MemUsage     float64 `json:"memUsage"`
		DiskUsage    float64 `json:"diskUsage"`
		NetIn        int64   `json:"netIn"`
		NetOut       int64   `json:"netOut"`
		Load1m       float64 `json:"load1m"`
		ProcessCount int     `json:"processCount"`
	}

	var metricsData []serverMetric
	var totalCPU, totalMem float64
	onlineCount := 0

	for _, s := range servers {
		if cached, ok := c.cache.Load(s.ID); ok {
			m := cached.(*model.Metric)
			isOnline := c.IsOnline(s.ID)
			if isOnline {
				onlineCount++
				totalCPU += m.CPUUsage
				totalMem += m.MemUsage
			}
			metricsData = append(metricsData, serverMetric{
				ServerID:     s.ID,
				ServerName:   s.Name,
				IsOnline:     isOnline,
				CPUUsage:     m.CPUUsage,
				MemUsage:     m.MemUsage,
				DiskUsage:    m.DiskUsage,
				NetIn:        m.NetIn,
				NetOut:       m.NetOut,
				Load1m:       m.Load1m,
				ProcessCount: m.ProcessCount,
			})
		}
	}

	var avgCPU, avgMem float64
	if onlineCount > 0 {
		avgCPU = round2(totalCPU / float64(onlineCount))
		avgMem = round2(totalMem / float64(onlineCount))
	}

	// 统计活跃告警
	var alertCount int64
	c.db.Model(&model.Alert{}).Where("is_resolved = ?", false).Count(&alertCount)

	c.hub.Broadcast("metrics_update", map[string]interface{}{
		"servers": metricsData,
		"overview": map[string]interface{}{
			"serverCount":  len(servers),
			"onlineCount":  onlineCount,
			"offlineCount": len(servers) - onlineCount,
			"warningCount": alertCount,
			"avgCpu":       avgCPU,
			"avgMemory":    avgMem,
		},
		"timestamp": time.Now().Unix(),
	})
}

// IngestAgentMetric 接收 Agent 上报的指标，写入数据库/缓存并广播
func (c *Collector) IngestAgentMetric(s model.Server, metric *model.Metric) {
	c.db.Create(metric)
	c.cache.Store(s.ID, metric)
	c.online.Store(s.ID, true)
	c.lastReport.Store(s.ID, time.Now())
	c.checkAlerts(s, metric)

	// 立即广播此服务器的更新
	var servers []model.Server
	c.db.Where("is_active = ?", true).Order("sort_order ASC").Find(&servers)
	c.broadcastMetrics(servers)
}

// GetLatestMetric 获取缓存的最新指标
func (c *Collector) GetLatestMetric(serverID string) *model.Metric {
	if cached, ok := c.cache.Load(serverID); ok {
		return cached.(*model.Metric)
	}
	return nil
}

// IsOnline 获取在线状态（Agent 类型额外检查上报超时）
func (c *Collector) IsOnline(serverID string) bool {
	// 如果有最后上报时间（Agent 类型），检查是否超时
	if t, ok := c.lastReport.Load(serverID); ok {
		if time.Since(t.(time.Time)) > 35*time.Second {
			c.online.Store(serverID, false)
			return false
		}
	}
	if v, ok := c.online.Load(serverID); ok {
		return v.(bool)
	}
	return false
}

func round2(v float64) float64 {
	return float64(int(v*100)) / 100
}
