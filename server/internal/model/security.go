package model

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AdminUser 管理员账号
type AdminUser struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Username  string    `json:"username" gorm:"size:50;uniqueIndex;not null"`
	Password  string    `json:"-" gorm:"size:200;not null"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// IPBlacklist IP 黑名单
type IPBlacklist struct {
	ID        uint       `json:"id" gorm:"primaryKey"`
	IP        string     `json:"ip" gorm:"size:45;uniqueIndex;not null"`
	Reason    string     `json:"reason" gorm:"size:200"`
	AutoBan   bool       `json:"autoBan" gorm:"default:false"`
	ExpiresAt *time.Time `json:"expiresAt"`
	CreatedAt time.Time  `json:"createdAt"`
}

// LoginAttempt 登录尝试记录
type LoginAttempt struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	IP        string    `json:"ip" gorm:"size:45;index;not null"`
	Username  string    `json:"username" gorm:"size:50"`
	Success   bool      `json:"success" gorm:"default:false"`
	UserAgent string    `json:"userAgent" gorm:"size:500"`
	CreatedAt time.Time `json:"createdAt" gorm:"index"`
}

// SecurityLog 安全审计日志
type SecurityLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Action    string    `json:"action" gorm:"size:50;index;not null"`
	IP        string    `json:"ip" gorm:"size:45"`
	Username  string    `json:"username" gorm:"size:50"`
	Detail    string    `json:"detail" gorm:"size:500"`
	CreatedAt time.Time `json:"createdAt" gorm:"index"`
}

// SystemSetting 系统配置（存储 JWT Secret 等）
type SystemSetting struct {
	Key   string `json:"key" gorm:"primaryKey;size:50"`
	Value string `json:"value" gorm:"size:500"`
}

// NotifyConfig 通知推送配置
type NotifyConfig struct {
	ID             uint   `json:"id" gorm:"primaryKey"`
	Enabled        bool   `json:"enabled" gorm:"default:false"`
	WebhookURL     string `json:"webhookUrl" gorm:"size:500"`
	NotifyWarning  bool   `json:"notifyWarning" gorm:"default:true"`
	NotifyCritical bool   `json:"notifyCritical" gorm:"default:true"`
	NotifyOffline  bool   `json:"notifyOffline" gorm:"default:true"`
	CooldownMin    int    `json:"cooldownMin" gorm:"default:10"` // 同类告警冷却时间（分钟）
}

// HashPassword 使用 bcrypt 哈希密码
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

// CheckPassword 验证密码
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// InitSecurity 初始化安全相关数据
func InitSecurity(db *gorm.DB) {
	// 自动迁移安全表
	if err := db.AutoMigrate(&AdminUser{}, &IPBlacklist{}, &LoginAttempt{}, &SecurityLog{}, &SystemSetting{}, &NotifyConfig{}); err != nil {
		log.Fatalf("安全表迁移失败: %v", err)
	}

	// 初始化默认管理员
	var userCount int64
	db.Model(&AdminUser{}).Count(&userCount)
	if userCount == 0 {
		hash, _ := HashPassword("admin123")
		db.Create(&AdminUser{
			Username: "admin",
			Password: hash,
		})
		log.Println("已创建默认管理员: admin / admin123（请尽快修改密码）")
	}

	// 初始化 JWT Secret（随机生成，只生成一次）
	var setting SystemSetting
	if db.Where("key = ?", "jwt_secret").First(&setting).Error != nil {
		secret := make([]byte, 32)
		rand.Read(secret)
		db.Create(&SystemSetting{
			Key:   "jwt_secret",
			Value: hex.EncodeToString(secret),
		})
		log.Println("已生成随机 JWT Secret")
	}
}

// GetJWTSecret 获取 JWT Secret
func GetJWTSecret(db *gorm.DB) string {
	var setting SystemSetting
	if db.Where("key = ?", "jwt_secret").First(&setting).Error == nil {
		return setting.Value
	}
	return "fallback-secret-key"
}

// GetSignKey 获取 Agent 通信签名密钥（自动生成）
func GetSignKey(db *gorm.DB) string {
	var setting SystemSetting
	if db.Where("key = ?", "sign_key").First(&setting).Error == nil {
		return setting.Value
	}
	// 自动生成
	secret := make([]byte, 32)
	rand.Read(secret)
	val := hex.EncodeToString(secret)
	db.Create(&SystemSetting{Key: "sign_key", Value: val})
	log.Println("已生成 Agent 通信签名密钥 (SIGN_KEY)")
	return val
}

// GetRegisterSecret 获取 Agent 注册密钥（自动生成）
func GetRegisterSecret(db *gorm.DB) string {
	var setting SystemSetting
	if db.Where("key = ?", "register_secret").First(&setting).Error == nil {
		return setting.Value
	}
	secret := make([]byte, 16)
	rand.Read(secret)
	val := hex.EncodeToString(secret)
	db.Create(&SystemSetting{Key: "register_secret", Value: val})
	log.Println("已生成 Agent 注册密钥 (REGISTER_SECRET)")
	return val
}
