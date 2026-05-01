package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// GeoIP 通过 ip-api.com 解析 IP 对应国家代码，带内存缓存
type GeoIP struct {
	mu    sync.RWMutex
	cache map[string]string // ip -> country code (e.g. "CN", "US")
	cl    *http.Client
}

func NewGeoIP() *GeoIP {
	return &GeoIP{
		cache: make(map[string]string),
		cl:    &http.Client{Timeout: 5 * time.Second},
	}
}

type ipAPIResp struct {
	Status      string `json:"status"`
	CountryCode string `json:"countryCode"`
}

// Lookup 返回 IP 对应的两位国家代码，缓存命中直接返回
func (g *GeoIP) Lookup(ip string) string {
	if ip == "" {
		return ""
	}
	g.mu.RLock()
	cc, ok := g.cache[ip]
	g.mu.RUnlock()
	if ok {
		return cc
	}

	// 调用 ip-api.com（服务端无 CORS 限制）
	resp, err := g.cl.Get(fmt.Sprintf("http://ip-api.com/json/%s?fields=status,countryCode", ip))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var r ipAPIResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil || r.Status != "success" {
		return ""
	}

	g.mu.Lock()
	g.cache[ip] = r.CountryCode
	g.mu.Unlock()
	return r.CountryCode
}

// BatchLookup 批量解析，对未缓存的 IP 逐个查询（ip-api 免费限制 45/min，服务器数量一般够用）
func (g *GeoIP) BatchLookup(ips []string) map[string]string {
	result := make(map[string]string, len(ips))
	var toQuery []string

	g.mu.RLock()
	for _, ip := range ips {
		if cc, ok := g.cache[ip]; ok {
			result[ip] = cc
		} else {
			toQuery = append(toQuery, ip)
		}
	}
	g.mu.RUnlock()

	for _, ip := range toQuery {
		cc := g.Lookup(ip)
		if cc != "" {
			result[ip] = cc
		}
	}
	return result
}
