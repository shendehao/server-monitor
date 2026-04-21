package model

import (
	cryptorand "crypto/rand"
	"encoding/hex"

	"gorm.io/gorm"
)

func seedToken() string {
	b := make([]byte, 32)
	cryptorand.Read(b)
	return hex.EncodeToString(b)
}

func SeedServers(db *gorm.DB) {
	var count int64
	db.Model(&Server{}).Count(&count)
	if count > 0 {
		return
	}

	servers := []Server{
		{ID: "s1", Name: "Web-主站", Host: "192.168.1.10", Port: 22, Username: "root", AuthType: "password", AuthValue: "***", ConnectMethod: "ssh", AgentToken: seedToken(), OSType: "linux", Group: "生产", SortOrder: 1, IsActive: true},
		{ID: "s2", Name: "Web-备站", Host: "192.168.1.11", Port: 22, Username: "root", AuthType: "password", AuthValue: "***", ConnectMethod: "ssh", AgentToken: seedToken(), OSType: "linux", Group: "生产", SortOrder: 2, IsActive: true},
		{ID: "s3", Name: "API-网关", Host: "192.168.1.20", Port: 22, Username: "root", AuthType: "password", AuthValue: "***", ConnectMethod: "ssh", AgentToken: seedToken(), OSType: "linux", Group: "生产", SortOrder: 3, IsActive: true},
		{ID: "s4", Name: "DB-主库", Host: "192.168.1.30", Port: 22, Username: "root", AuthType: "password", AuthValue: "***", ConnectMethod: "ssh", AgentToken: seedToken(), OSType: "linux", Group: "数据库", SortOrder: 4, IsActive: true},
		{ID: "s5", Name: "DB-从库", Host: "192.168.1.31", Port: 22, Username: "root", AuthType: "password", AuthValue: "***", ConnectMethod: "ssh", AgentToken: seedToken(), OSType: "linux", Group: "数据库", SortOrder: 5, IsActive: true},
		{ID: "s6", Name: "缓存服务器", Host: "192.168.1.40", Port: 22, Username: "root", AuthType: "password", AuthValue: "***", ConnectMethod: "ssh", AgentToken: seedToken(), OSType: "linux", Group: "中间件", SortOrder: 6, IsActive: true},
		{ID: "s7", Name: "文件服务器", Host: "192.168.1.50", Port: 22, Username: "root", AuthType: "password", AuthValue: "***", ConnectMethod: "ssh", AgentToken: seedToken(), OSType: "linux", Group: "存储", SortOrder: 7, IsActive: true},
	}

	for _, s := range servers {
		db.Create(&s)
	}
}

// BackfillAgentTokens 为已有服务器补充空的 AgentToken
func BackfillAgentTokens(db *gorm.DB) {
	var servers []Server
	db.Where("agent_token = '' OR agent_token IS NULL").Find(&servers)
	for _, s := range servers {
		db.Model(&s).Update("agent_token", seedToken())
	}
}
