package handler

import (
	"log"
	"net/http"
	"server-monitor/internal/model"
	"server-monitor/internal/service"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MetricHandler struct {
	db        *gorm.DB
	collector *service.Collector
	geoip     *service.GeoIP
}

func NewMetricHandler(db *gorm.DB, collector *service.Collector) *MetricHandler {
	return &MetricHandler{db: db, collector: collector, geoip: service.NewGeoIP()}
}

var serverColors = map[string]string{
	"s1": "#3b82f6",
	"s2": "#10b981",
	"s3": "#8b5cf6",
	"s4": "#f59e0b",
	"s5": "#ec4899",
	"s6": "#06b6d4",
	"s7": "#f97316",
}

func (h *MetricHandler) Overview(c *gin.Context) {
	var servers []model.Server
	h.db.Where("is_active = ?", true).Order("sort_order ASC").Find(&servers)

	var totalCPU, totalMem, totalDisk float64
	onlineCount := 0
	var summaries []model.ServerSummary

	for _, s := range servers {
		isOnline := h.collector.IsOnline(s.ID)
		var cpu, mem, disk float64
		status := "offline"

		if isOnline {
			onlineCount++
			if m := h.collector.GetLatestMetric(s.ID); m != nil {
				cpu = m.CPUUsage
				mem = m.MemUsage
				disk = m.DiskUsage
				totalCPU += cpu
				totalMem += mem
				totalDisk += disk

				if cpu >= 95 || mem >= 95 {
					status = "danger"
				} else if cpu >= 80 || mem >= 80 {
					status = "warning"
				} else {
					status = "normal"
				}
			}
		}

		summaries = append(summaries, model.ServerSummary{
			ID:           s.ID,
			Name:         s.Name,
			Host:         s.Host,
			CountryCode:  h.geoip.Lookup(s.Host),
			IsOnline:     isOnline,
			CPUUsage:     cpu,
			MemUsage:     mem,
			DiskUsage:    disk,
			Status:       status,
			AgentVersion: h.collector.GetAgentVersion(s.ID),
		})
	}

	var avgCPU, avgMem, avgDisk float64
	if onlineCount > 0 {
		avgCPU = totalCPU / float64(onlineCount)
		avgMem = totalMem / float64(onlineCount)
		avgDisk = totalDisk / float64(onlineCount)
	}

	var alertCount int64
	h.db.Model(&model.Alert{}).Where("is_resolved = ?", false).Count(&alertCount)

	overview := model.Overview{
		ServerCount:  len(servers),
		OnlineCount:  onlineCount,
		OfflineCount: len(servers) - onlineCount,
		WarningCount: int(alertCount),
		AvgCPU:       round2(avgCPU),
		AvgMemory:    round2(avgMem),
		AvgDisk:      round2(avgDisk),
		ActiveAlerts: alertCount,
		Servers:      summaries,
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": overview})
}

func (h *MetricHandler) Realtime(c *gin.Context) {
	metric := c.DefaultQuery("metric", "cpu")
	minutes, _ := strconv.Atoi(c.DefaultQuery("minutes", "30"))
	if minutes <= 0 {
		minutes = 30
	}

	since := time.Now().Add(-time.Duration(minutes) * time.Minute)

	var servers []model.Server
	h.db.Where("is_active = ?", true).Order("sort_order ASC").Find(&servers)

	// 构建 server id → name/color 映射
	serverMap := make(map[string]*model.SeriesItem, len(servers))
	serverOrder := make([]string, 0, len(servers))
	for i, s := range servers {
		color := serverColors[s.ID]
		if color == "" {
			color = palette[i%len(palette)]
		}
		serverMap[s.ID] = &model.SeriesItem{
			ServerID:   s.ID,
			ServerName: s.Name,
			Color:      color,
			Data:       make([]model.DataPoint, 0),
		}
		serverOrder = append(serverOrder, s.ID)
	}

	// 按服务器逐个查询
	start := time.Now()
	totalPoints := 0
	for _, sid := range serverOrder {
		var metrics []model.Metric
		h.db.Where("server_id = ? AND collected_at >= ?", sid, since).
			Order("collected_at ASC").
			Select("cpu_usage, mem_usage, disk_usage, load1m, collected_at").
			Find(&metrics)

		totalPoints += len(metrics)
		si := serverMap[sid]
		for _, m := range metrics {
			var v float64
			switch metric {
			case "memory":
				v = m.MemUsage
			case "disk":
				v = m.DiskUsage
			case "load":
				v = m.Load1m
			default:
				v = m.CPUUsage
			}
			si.Data = append(si.Data, model.DataPoint{T: m.CollectedAt, V: v})
		}
	}

	// 采样：每个服务器最多 90 个点
	const maxPoints = 90
	series := make([]model.SeriesItem, 0, len(servers))
	for _, sid := range serverOrder {
		si := serverMap[sid]
		if len(si.Data) > maxPoints {
			step := len(si.Data) / maxPoints
			if step < 1 {
				step = 1
			}
			sampled := make([]model.DataPoint, 0, maxPoints+1)
			for j := 0; j < len(si.Data); j += step {
				sampled = append(sampled, si.Data[j])
			}
			// 确保最后一个点在内
			if last := si.Data[len(si.Data)-1]; len(sampled) == 0 || sampled[len(sampled)-1].T != last.T {
				sampled = append(sampled, last)
			}
			si.Data = sampled
		}
		series = append(series, *si)
	}

	log.Printf("[Realtime] metric=%s servers=%d totalPoints=%d elapsed=%v", metric, len(servers), totalPoints, time.Since(start))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": model.RealtimeSeries{
			Metric:  metric,
			Minutes: minutes,
			Series:  series,
		},
	})
}

var palette = []string{
	"#3b82f6", "#10b981", "#8b5cf6", "#f59e0b",
	"#ef4444", "#06b6d4", "#ec4899", "#6366f1", "#f97316",
}

func (h *MetricHandler) History(c *gin.Context) {
	serverID := c.Param("serverId")
	period := c.DefaultQuery("period", "1h")

	var duration time.Duration
	switch period {
	case "1h":
		duration = 1 * time.Hour
	case "6h":
		duration = 6 * time.Hour
	case "24h":
		duration = 24 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	default:
		duration = 1 * time.Hour
	}

	since := time.Now().Add(-duration)
	var metrics []model.Metric
	h.db.Where("server_id = ? AND collected_at >= ?", serverID, since).
		Order("collected_at ASC").Find(&metrics)

	// 采样：最多返回 360 个点
	const maxPts = 360
	if len(metrics) > maxPts {
		step := len(metrics) / maxPts
		sampled := make([]model.Metric, 0, maxPts+1)
		for i := 0; i < len(metrics); i += step {
			sampled = append(sampled, metrics[i])
		}
		if last := metrics[len(metrics)-1]; len(sampled) == 0 || sampled[len(sampled)-1].ID != last.ID {
			sampled = append(sampled, last)
		}
		metrics = sampled
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": metrics})
}

func round2(v float64) float64 {
	return float64(int(v*100)) / 100
}
