package main

import (
	"crypto/tls"
	"net/http"
	"time"
)

// tlsConfig 全局 TLS 配置：跳过自签名证书验证
var tlsConfig = &tls.Config{
	InsecureSkipVerify: true,
	MinVersion:         tls.VersionTLS12,
}

// secureTransport 支持 TLS 的 HTTP 传输层
var secureTransport = &http.Transport{
	TLSClientConfig:     tlsConfig,
	MaxIdleConns:        10,
	IdleConnTimeout:     30 * time.Second,
	DisableCompression:  true,
	TLSHandshakeTimeout: 10 * time.Second,
}

// secureHTTPClient 支持 TLS 的 HTTP 客户端（用于所有 API 调用）
var secureHTTPClient = &http.Client{
	Transport: secureTransport,
	Timeout:   30 * time.Second,
}
