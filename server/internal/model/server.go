package model

import (
	"time"
)

type Server struct {
	ID            string    `json:"id" gorm:"primaryKey;size:36"`
	Name          string    `json:"name" gorm:"size:50;not null"`
	Host          string    `json:"host" gorm:"size:255;not null"`
	Port          int       `json:"port" gorm:"default:22"`
	Username      string    `json:"username" gorm:"size:50;not null"`
	AuthType      string    `json:"authType" gorm:"size:10;default:password"` // password / key
	AuthValue     string    `json:"-" gorm:"size:2000"`
	ConnectMethod string    `json:"connectMethod" gorm:"size:10;default:ssh"` // ssh / agent / plugin / api
	AgentToken    string    `json:"agentToken" gorm:"size:64;uniqueIndex"`
	OSType        string    `json:"osType" gorm:"size:10;default:linux"` // linux / windows
	Group         string    `json:"group" gorm:"size:50"`
	SortOrder     int       `json:"sortOrder" gorm:"default:0"`
	IsActive      bool      `json:"isActive" gorm:"default:true"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type ServerForm struct {
	Name          string `json:"name" binding:"required,max=50"`
	Host          string `json:"host" binding:"required"`
	Port          int    `json:"port"`
	Username      string `json:"username" binding:"required"`
	AuthType      string `json:"authType"`
	AuthValue     string `json:"authValue"`
	ConnectMethod string `json:"connectMethod"`
	OSType        string `json:"osType"`
	Group         string `json:"group"`
	SortOrder     int    `json:"sortOrder"`
}
