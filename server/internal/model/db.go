package model

import (
	"log"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitDB(dbPath string) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}

	// 自动迁移
	if err := db.AutoMigrate(&Server{}, &Metric{}, &Alert{}, &AlertRule{}); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	// 初始化默认告警规则
	initDefaultAlertRules(db)

	return db
}

func initDefaultAlertRules(db *gorm.DB) {
	var count int64
	db.Model(&AlertRule{}).Count(&count)
	if count > 0 {
		return
	}

	rules := []AlertRule{
		{MetricType: "cpu", WarningThreshold: 80, CriticalThreshold: 95, ConsecutiveCount: 3, Enabled: true, Description: "CPU 使用率告警"},
		{MetricType: "memory", WarningThreshold: 80, CriticalThreshold: 95, ConsecutiveCount: 3, Enabled: true, Description: "内存使用率告警"},
		{MetricType: "disk", WarningThreshold: 80, CriticalThreshold: 95, ConsecutiveCount: 1, Enabled: true, Description: "磁盘使用率告警"},
		{MetricType: "offline", WarningThreshold: 0, CriticalThreshold: 0, ConsecutiveCount: 2, Enabled: true, Description: "服务器离线告警"},
	}

	for _, rule := range rules {
		db.Create(&rule)
	}
}
