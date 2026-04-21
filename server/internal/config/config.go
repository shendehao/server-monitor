package config

import (
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Port            int    `json:"port"`
	Mode            string `json:"mode"`
	DBPath          string `json:"dbPath"`
	CollectInterval int    `json:"collectInterval"` // 秒
	SSHTimeout      int    `json:"sshTimeout"`      // 秒
	RetryCount      int    `json:"retryCount"`
}

func Load() *Config {
	cfg := &Config{
		Port:            5000,
		Mode:            "debug",
		DBPath:          "data/data.db",
		CollectInterval: 10,
		SSHTimeout:      5,
		RetryCount:      2,
	}

	if p := os.Getenv("PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			cfg.Port = v
		}
	}
	if m := os.Getenv("GIN_MODE"); m != "" {
		cfg.Mode = m
	}
	if d := os.Getenv("DB_PATH"); d != "" {
		cfg.DBPath = d
	}

	// 确保数据目录存在
	os.MkdirAll(filepath.Dir(cfg.DBPath), 0755)

	return cfg
}
