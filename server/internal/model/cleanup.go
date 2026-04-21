package model

import (
	"log"
	"time"

	"gorm.io/gorm"
)

// StartMetricCleanup 定期清理超过 7 天的旧指标数据，保持数据库精简
func StartMetricCleanup(db *gorm.DB) {
	// 启动时立即清理一次
	cleanOldMetrics(db)

	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		cleanOldMetrics(db)
	}
}

func cleanOldMetrics(db *gorm.DB) {
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	result := db.Where("collected_at < ?", cutoff).Delete(&Metric{})
	if result.RowsAffected > 0 {
		log.Printf("已清理 %d 条过期指标数据（>7天）", result.RowsAffected)
		// 清理后执行 VACUUM 释放磁盘空间
		db.Exec("VACUUM")
	}
}
