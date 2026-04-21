# 服务器管理大屏系统 — API 接口文档

> **版本**: v1.0  
> **日期**: 2026-04-20  
> **基础路径**: `http://{host}:8080`  
> **协议**: HTTP REST + WebSocket  
> **数据格式**: JSON

---

## 1. 通用规范

### 1.1 统一响应结构

所有 REST API 均返回以下 JSON 结构：

```json
{
    "success": true,
    "data": {},
    "error": "",
    "timestamp": 1713614400
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `success` | boolean | 请求是否成功 |
| `data` | any | 成功时返回的数据，失败时为空 |
| `error` | string | 失败时的错误信息，成功时为空 |
| `timestamp` | number | 服务器响应时间戳（Unix 秒） |

### 1.2 HTTP 状态码

| 状态码 | 含义 | 使用场景 |
|--------|------|---------|
| `200` | 成功 | GET / PUT 请求成功 |
| `201` | 已创建 | POST 创建资源成功 |
| `204` | 无内容 | DELETE 删除成功 |
| `400` | 请求错误 | 参数校验失败 |
| `404` | 未找到 | 资源不存在 |
| `500` | 服务器错误 | 内部错误 |

### 1.3 分页参数

支持分页的接口使用以下查询参数：

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `page` | int | 1 | 页码，从 1 开始 |
| `page_size` | int | 20 | 每页条数，最大 100 |

分页响应格式：

```json
{
    "success": true,
    "data": {
        "list": [],
        "total": 100,
        "page": 1,
        "page_size": 20
    }
}
```

### 1.4 时间格式

- 所有时间字段使用 **ISO 8601** 格式：`2026-04-20T18:30:00+08:00`
- 时间戳使用 **Unix 秒**

---

## 2. 服务器管理 API

### 2.1 获取服务器列表

获取所有服务器及其最新指标数据。

```
GET /api/servers
```

**查询参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `group` | string | 否 | 按分组筛选 |
| `status` | string | 否 | 按状态筛选：`online` / `offline` / `warning` |

**响应示例：**

```json
{
    "success": true,
    "data": [
        {
            "id": "s1",
            "name": "Web服务器",
            "host": "192.168.1.101",
            "port": 22,
            "osType": "linux",
            "group": "生产环境",
            "sortOrder": 1,
            "isActive": true,
            "isOnline": true,
            "uptime": "32天14小时",
            "latestMetrics": {
                "cpuUsage": 45.2,
                "memTotal": 16384,
                "memUsed": 10240,
                "memUsage": 62.5,
                "diskTotal": 500,
                "diskUsed": 320,
                "diskUsage": 64.0,
                "netIn": 1048576,
                "netOut": 524288,
                "load1m": 1.2,
                "load5m": 0.8,
                "load15m": 0.6,
                "processCount": 218,
                "collectedAt": "2026-04-20T18:30:00+08:00"
            },
            "createdAt": "2026-03-15T10:00:00+08:00",
            "updatedAt": "2026-04-20T18:00:00+08:00"
        }
    ],
    "timestamp": 1713614400
}
```

---

### 2.2 获取单台服务器详情

```
GET /api/servers/:id
```

**路径参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | string | 服务器 ID |

**响应示例：**

```json
{
    "success": true,
    "data": {
        "id": "s1",
        "name": "Web服务器",
        "host": "192.168.1.101",
        "port": 22,
        "username": "root",
        "osType": "linux",
        "group": "生产环境",
        "sortOrder": 1,
        "isActive": true,
        "isOnline": true,
        "uptime": "32天14小时",
        "systemInfo": {
            "os": "Ubuntu 22.04.3 LTS",
            "kernel": "5.15.0-91-generic",
            "cpuModel": "Intel Xeon E5-2680 v4",
            "cpuCores": 8,
            "totalMemory": 16384,
            "totalDisk": 500,
            "hostname": "web-server-01"
        },
        "latestMetrics": {
            "cpuUsage": 45.2,
            "memTotal": 16384,
            "memUsed": 10240,
            "memUsage": 62.5,
            "diskTotal": 500,
            "diskUsed": 320,
            "diskUsage": 64.0,
            "netIn": 1048576,
            "netOut": 524288,
            "load1m": 1.2,
            "load5m": 0.8,
            "load15m": 0.6,
            "processCount": 218,
            "collectedAt": "2026-04-20T18:30:00+08:00"
        },
        "createdAt": "2026-03-15T10:00:00+08:00",
        "updatedAt": "2026-04-20T18:00:00+08:00"
    },
    "timestamp": 1713614400
}
```

**错误响应：**

```json
{
    "success": false,
    "error": "服务器不存在",
    "timestamp": 1713614400
}
```

---

### 2.3 新增服务器

```
POST /api/servers
```

**请求体：**

```json
{
    "name": "Web服务器",
    "host": "192.168.1.101",
    "port": 22,
    "username": "root",
    "authType": "password",
    "authValue": "your-password-here",
    "osType": "linux",
    "group": "生产环境",
    "sortOrder": 1
}
```

**字段说明：**

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `name` | string | ✅ | - | 服务器显示名称 |
| `host` | string | ✅ | - | IP 地址或域名 |
| `port` | int | 否 | 22 | SSH 端口号 |
| `username` | string | ✅ | - | SSH 用户名 |
| `authType` | string | 否 | `password` | 认证方式：`password` / `key` |
| `authValue` | string | ✅ | - | 密码或密钥文件路径 |
| `osType` | string | 否 | `linux` | 操作系统：`linux` / `windows` |
| `group` | string | 否 | `""` | 服务器分组 |
| `sortOrder` | int | 否 | 0 | 排序序号，越小越靠前 |

**响应：** `201 Created`

```json
{
    "success": true,
    "data": {
        "id": "s8",
        "name": "Web服务器",
        "host": "192.168.1.101",
        "port": 22,
        "osType": "linux",
        "group": "生产环境",
        "sortOrder": 1,
        "isActive": true,
        "createdAt": "2026-04-20T18:30:00+08:00",
        "updatedAt": "2026-04-20T18:30:00+08:00"
    },
    "timestamp": 1713614400
}
```

**校验规则：**

- `name`: 不能为空，最长 50 字符
- `host`: 合法 IP 或域名
- `port`: 1 ~ 65535
- `username`: 不能为空
- `authValue`: 不能为空
- `host` + `port` 组合不能重复

---

### 2.4 更新服务器配置

```
PUT /api/servers/:id
```

**请求体：** 同新增，所有字段可选（只传需要修改的字段）

```json
{
    "name": "Web服务器-改名",
    "group": "测试环境"
}
```

**响应：** `200 OK`

```json
{
    "success": true,
    "data": {
        "id": "s1",
        "name": "Web服务器-改名",
        "host": "192.168.1.101",
        "port": 22,
        "osType": "linux",
        "group": "测试环境",
        "sortOrder": 1,
        "isActive": true,
        "createdAt": "2026-03-15T10:00:00+08:00",
        "updatedAt": "2026-04-20T18:35:00+08:00"
    },
    "timestamp": 1713614400
}
```

---

### 2.5 删除服务器

```
DELETE /api/servers/:id
```

**响应：** `204 No Content`

> ⚠️ 删除服务器会同时删除该服务器的所有历史指标和告警记录。

---

### 2.6 测试服务器连接

测试 SSH 连接是否可用，不保存数据。

```
POST /api/servers/:id/test
```

**响应示例（成功）：**

```json
{
    "success": true,
    "data": {
        "connected": true,
        "latency": 23,
        "serverInfo": {
            "os": "Ubuntu 22.04.3 LTS",
            "kernel": "5.15.0-91-generic",
            "hostname": "web-server-01"
        },
        "message": "连接成功"
    },
    "timestamp": 1713614400
}
```

**响应示例（失败）：**

```json
{
    "success": true,
    "data": {
        "connected": false,
        "latency": 0,
        "serverInfo": null,
        "message": "连接超时：dial tcp 192.168.1.101:22: i/o timeout"
    },
    "timestamp": 1713614400
}
```

也可以不指定 ID，直接传连接信息测试新服务器：

```
POST /api/servers/test
```

**请求体：**

```json
{
    "host": "192.168.1.200",
    "port": 22,
    "username": "root",
    "authType": "password",
    "authValue": "test-password"
}
```

---

## 3. 指标查询 API

### 3.1 获取全局概览

获取所有服务器的汇总统计数据。

```
GET /api/metrics/overview
```

**响应示例：**

```json
{
    "success": true,
    "data": {
        "serverCount": 7,
        "onlineCount": 6,
        "offlineCount": 1,
        "warningCount": 2,
        "avgCpu": 45.3,
        "avgMemory": 62.1,
        "avgDisk": 54.8,
        "totalNetIn": 10485760,
        "totalNetOut": 5242880,
        "activeAlerts": 3,
        "servers": [
            {
                "id": "s1",
                "name": "Web服务器",
                "isOnline": true,
                "cpuUsage": 45.2,
                "memUsage": 62.5,
                "diskUsage": 64.0,
                "status": "normal"
            },
            {
                "id": "s4",
                "name": "数据库服务器",
                "isOnline": true,
                "cpuUsage": 92.1,
                "memUsage": 88.3,
                "diskUsage": 75.0,
                "status": "danger"
            },
            {
                "id": "s7",
                "name": "备份服务器",
                "isOnline": false,
                "cpuUsage": 0,
                "memUsage": 0,
                "diskUsage": 0,
                "status": "offline"
            }
        ]
    },
    "timestamp": 1713614400
}
```

`status` 字段取值：
- `normal` — 所有指标正常
- `warning` — 存在指标超过警告阈值
- `danger` — 存在指标超过危险阈值
- `offline` — 服务器离线

---

### 3.2 获取服务器历史指标

```
GET /api/metrics/:serverId
```

**路径参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `serverId` | string | 服务器 ID |

**查询参数：**

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `range` | string | 否 | `1h` | 时间范围：`1h` / `6h` / `24h` / `7d` / `30d` |
| `start` | string | 否 | - | 自定义起始时间 (ISO 8601)，与 `end` 配合使用 |
| `end` | string | 否 | - | 自定义结束时间 (ISO 8601) |
| `metric` | string | 否 | `all` | 指定指标：`cpu` / `memory` / `disk` / `network` / `load` / `all` |

**数据点说明：**

| 时间范围 | 数据粒度 | 预计数据点数 |
|----------|---------|-------------|
| 1h | 原始（10s） | ~360 |
| 6h | 1 分钟聚合 | ~360 |
| 24h | 5 分钟聚合 | ~288 |
| 7d | 30 分钟聚合 | ~336 |
| 30d | 1 小时聚合 | ~720 |

**响应示例：**

```json
{
    "success": true,
    "data": {
        "serverId": "s1",
        "serverName": "Web服务器",
        "range": "1h",
        "points": [
            {
                "timestamp": "2026-04-20T17:30:00+08:00",
                "cpuUsage": 42.1,
                "memUsage": 61.3,
                "diskUsage": 64.0,
                "netIn": 98304,
                "netOut": 49152,
                "load1m": 1.1,
                "load5m": 0.9,
                "load15m": 0.7
            },
            {
                "timestamp": "2026-04-20T17:30:10+08:00",
                "cpuUsage": 44.5,
                "memUsage": 62.0,
                "diskUsage": 64.0,
                "netIn": 102400,
                "netOut": 51200,
                "load1m": 1.2,
                "load5m": 0.8,
                "load15m": 0.6
            }
        ],
        "summary": {
            "cpu": { "min": 12.3, "max": 78.5, "avg": 45.2 },
            "memory": { "min": 58.0, "max": 68.2, "avg": 62.5 },
            "disk": { "min": 63.8, "max": 64.2, "avg": 64.0 }
        }
    },
    "timestamp": 1713614400
}
```

---

### 3.3 获取所有服务器实时指标（趋势图用）

专为大屏趋势图设计，返回所有服务器最近 N 分钟的指标。

```
GET /api/metrics/realtime
```

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `minutes` | int | 30 | 最近 N 分钟的数据 |
| `metric` | string | `cpu` | 指定指标：`cpu` / `memory` |

**响应示例：**

```json
{
    "success": true,
    "data": {
        "metric": "cpu",
        "minutes": 30,
        "series": [
            {
                "serverId": "s1",
                "serverName": "Web服务器",
                "color": "#3b82f6",
                "data": [
                    { "t": "2026-04-20T18:00:00+08:00", "v": 42.1 },
                    { "t": "2026-04-20T18:00:10+08:00", "v": 44.5 },
                    { "t": "2026-04-20T18:00:20+08:00", "v": 43.8 }
                ]
            },
            {
                "serverId": "s2",
                "serverName": "API服务器",
                "color": "#10b981",
                "data": [
                    { "t": "2026-04-20T18:00:00+08:00", "v": 67.2 },
                    { "t": "2026-04-20T18:00:10+08:00", "v": 65.8 }
                ]
            }
        ]
    },
    "timestamp": 1713614400
}
```

---

## 4. 告警 API

### 4.1 获取告警列表

```
GET /api/alerts
```

**查询参数：**

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `page` | int | 否 | 1 | 页码 |
| `page_size` | int | 否 | 20 | 每页条数 |
| `server_id` | string | 否 | - | 按服务器筛选 |
| `severity` | string | 否 | - | 按严重程度：`info` / `warning` / `critical` |
| `status` | string | 否 | - | 按状态：`active` / `resolved` |
| `alert_type` | string | 否 | - | 按类型：`cpu_high` / `mem_high` / `disk_full` / `offline` |
| `start` | string | 否 | - | 起始时间 |
| `end` | string | 否 | - | 结束时间 |

**响应示例：**

```json
{
    "success": true,
    "data": {
        "list": [
            {
                "id": 1,
                "serverId": "s4",
                "serverName": "数据库服务器",
                "alertType": "cpu_high",
                "message": "CPU 使用率 95.2% 超过危险阈值 (95%)",
                "severity": "critical",
                "isResolved": false,
                "createdAt": "2026-04-20T18:25:00+08:00",
                "resolvedAt": null
            },
            {
                "id": 2,
                "serverId": "s7",
                "serverName": "备份服务器",
                "alertType": "offline",
                "message": "服务器连接超时，疑似离线",
                "severity": "critical",
                "isResolved": false,
                "createdAt": "2026-04-20T18:20:00+08:00",
                "resolvedAt": null
            },
            {
                "id": 3,
                "serverId": "s2",
                "serverName": "API服务器",
                "alertType": "mem_high",
                "message": "内存使用率 82.3% 超过警告阈值 (80%)",
                "severity": "warning",
                "isResolved": false,
                "createdAt": "2026-04-20T18:15:00+08:00",
                "resolvedAt": null
            }
        ],
        "total": 56,
        "page": 1,
        "page_size": 20
    },
    "timestamp": 1713614400
}
```

---

### 4.2 获取活跃告警数量

轻量级接口，用于顶部概览栏轮询。

```
GET /api/alerts/count
```

**响应示例：**

```json
{
    "success": true,
    "data": {
        "total": 3,
        "critical": 2,
        "warning": 1,
        "info": 0
    },
    "timestamp": 1713614400
}
```

---

### 4.3 标记告警已解决

```
PUT /api/alerts/:id/resolve
```

**路径参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | int | 告警 ID |

**响应示例：**

```json
{
    "success": true,
    "data": {
        "id": 1,
        "serverId": "s4",
        "alertType": "cpu_high",
        "message": "CPU 使用率 95.2% 超过危险阈值 (95%)",
        "severity": "critical",
        "isResolved": true,
        "createdAt": "2026-04-20T18:25:00+08:00",
        "resolvedAt": "2026-04-20T18:40:00+08:00"
    },
    "timestamp": 1713614400
}
```

---

### 4.4 批量解决告警

```
PUT /api/alerts/batch-resolve
```

**请求体：**

```json
{
    "ids": [1, 2, 3]
}
```

**响应：**

```json
{
    "success": true,
    "data": {
        "resolved": 3
    },
    "timestamp": 1713614400
}
```

---

## 5. 告警规则 API

### 5.1 获取告警规则

```
GET /api/alert-rules
```

**响应示例：**

```json
{
    "success": true,
    "data": [
        {
            "id": 1,
            "metric": "cpu",
            "warningThreshold": 80,
            "criticalThreshold": 95,
            "consecutiveCount": 3,
            "enabled": true,
            "description": "CPU 使用率告警"
        },
        {
            "id": 2,
            "metric": "memory",
            "warningThreshold": 80,
            "criticalThreshold": 95,
            "consecutiveCount": 3,
            "enabled": true,
            "description": "内存使用率告警"
        },
        {
            "id": 3,
            "metric": "disk",
            "warningThreshold": 80,
            "criticalThreshold": 95,
            "consecutiveCount": 1,
            "enabled": true,
            "description": "磁盘使用率告警"
        },
        {
            "id": 4,
            "metric": "offline",
            "warningThreshold": 0,
            "criticalThreshold": 0,
            "consecutiveCount": 2,
            "enabled": true,
            "description": "服务器离线告警"
        }
    ],
    "timestamp": 1713614400
}
```

---

### 5.2 更新告警规则

```
PUT /api/alert-rules/:id
```

**请求体：**

```json
{
    "warningThreshold": 85,
    "criticalThreshold": 98,
    "consecutiveCount": 5,
    "enabled": true
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `warningThreshold` | int | 警告阈值 (0-100) |
| `criticalThreshold` | int | 危险阈值 (0-100) |
| `consecutiveCount` | int | 连续超标次数才触发 |
| `enabled` | bool | 是否启用 |

**响应：** `200 OK`，返回更新后的规则对象。

---

## 6. 系统配置 API

### 6.1 获取系统配置

```
GET /api/config
```

**响应示例：**

```json
{
    "success": true,
    "data": {
        "collectInterval": 10,
        "sshTimeout": 5,
        "retryCount": 2,
        "dataRetentionHours": 24,
        "aggregation5minDays": 7,
        "aggregation1hourDays": 90,
        "version": "1.0.0",
        "startedAt": "2026-04-20T10:00:00+08:00",
        "uptime": "8小时30分钟"
    },
    "timestamp": 1713614400
}
```

---

### 6.2 更新系统配置

```
PUT /api/config
```

**请求体：**

```json
{
    "collectInterval": 15,
    "sshTimeout": 10,
    "retryCount": 3
}
```

| 字段 | 类型 | 范围 | 说明 |
|------|------|------|------|
| `collectInterval` | int | 5~60 | 采集间隔（秒） |
| `sshTimeout` | int | 3~30 | SSH 连接超时（秒） |
| `retryCount` | int | 0~5 | 失败重试次数 |

**响应：** `200 OK`，返回更新后的配置。

---

## 7. WebSocket 接口

### 7.1 连接

```
ws://{host}:8080/ws
```

连接建立后，服务端会自动推送实时数据。客户端无需认证。

### 7.2 服务端 → 客户端 消息

#### 7.2.1 指标更新 `metrics_update`

每次采集完成后推送一次，包含所有服务器的最新指标。

```json
{
    "type": "metrics_update",
    "data": {
        "servers": [
            {
                "serverId": "s1",
                "serverName": "Web服务器",
                "isOnline": true,
                "cpuUsage": 45.2,
                "memTotal": 16384,
                "memUsed": 10240,
                "memUsage": 62.5,
                "diskTotal": 500,
                "diskUsed": 320,
                "diskUsage": 64.0,
                "netIn": 1048576,
                "netOut": 524288,
                "load1m": 1.2,
                "load5m": 0.8,
                "load15m": 0.6,
                "processCount": 218
            }
        ],
        "overview": {
            "serverCount": 7,
            "onlineCount": 6,
            "offlineCount": 1,
            "warningCount": 2,
            "avgCpu": 45.3,
            "avgMemory": 62.1
        },
        "timestamp": 1713614400
    }
}
```

推送频率：每 `collectInterval` 秒一次（默认 10 秒）。

---

#### 7.2.2 告警通知 `alert`

当触发新告警时立即推送。

```json
{
    "type": "alert",
    "data": {
        "id": 15,
        "serverId": "s4",
        "serverName": "数据库服务器",
        "alertType": "cpu_high",
        "message": "CPU 使用率 95.2% 超过危险阈值 (95%)",
        "severity": "critical",
        "createdAt": "2026-04-20T18:25:00+08:00"
    }
}
```

`alertType` 取值：

| 值 | 说明 |
|---|------|
| `cpu_high` | CPU 使用率超标 |
| `mem_high` | 内存使用率超标 |
| `disk_full` | 磁盘使用率超标 |
| `offline` | 服务器离线 |
| `load_high` | 系统负载过高 |

`severity` 取值：

| 值 | 说明 |
|---|------|
| `info` | 信息提示 |
| `warning` | 警告 |
| `critical` | 危险 |

---

#### 7.2.3 状态变更 `status_change`

服务器上线/下线时推送。

```json
{
    "type": "status_change",
    "data": {
        "serverId": "s7",
        "serverName": "备份服务器",
        "oldStatus": "online",
        "newStatus": "offline",
        "timestamp": 1713614400
    }
}
```

---

#### 7.2.4 告警解除 `alert_resolved`

告警自动恢复或手动解除时推送。

```json
{
    "type": "alert_resolved",
    "data": {
        "id": 15,
        "serverId": "s4",
        "serverName": "数据库服务器",
        "alertType": "cpu_high",
        "message": "CPU 使用率已恢复正常 (当前 42.1%)",
        "resolvedAt": "2026-04-20T18:35:00+08:00"
    }
}
```

---

#### 7.2.5 心跳 `ping`

服务端每 30 秒发送一次心跳，客户端应回复 `pong`。

```json
{ "type": "ping", "timestamp": 1713614400 }
```

---

### 7.3 客户端 → 服务端 消息

#### 7.3.1 刷新请求 `refresh`

请求服务端立即执行一次采集（不等定时任务）。

```json
{
    "type": "refresh",
    "serverId": ""
}
```

| 字段 | 说明 |
|------|------|
| `serverId` | 指定服务器 ID，为空则刷新全部 |

---

#### 7.3.2 心跳回复 `pong`

```json
{ "type": "pong" }
```

---

## 8. 错误码参考

| HTTP 状态码 | error 内容 | 说明 |
|------------|-----------|------|
| 400 | `参数错误：name 不能为空` | 请求参数校验失败 |
| 400 | `参数错误：host 格式不正确` | IP/域名格式错误 |
| 400 | `参数错误：该 host:port 已存在` | 重复添加 |
| 400 | `参数错误：page_size 不能超过 100` | 分页参数越界 |
| 404 | `服务器不存在` | 指定 ID 无对应资源 |
| 404 | `告警不存在` | 指定 ID 无对应告警 |
| 500 | `SSH 连接失败：connection refused` | SSH 无法连接 |
| 500 | `数据库错误` | SQLite 操作异常 |
| 500 | `内部错误` | 未预期的服务端错误 |

---

## 9. 前端 API 调用封装

### 9.1 Axios 实例 (`src/api/request.ts`)

```typescript
import axios from 'axios'
import { ElMessage } from 'element-plus'

const request = axios.create({
    baseURL: '/api',
    timeout: 10000
})

// 响应拦截器
request.interceptors.response.use(
    (response) => {
        const res = response.data
        if (!res.success) {
            ElMessage.error(res.error || '请求失败')
            return Promise.reject(new Error(res.error))
        }
        return res.data
    },
    (error) => {
        ElMessage.error('网络错误：' + error.message)
        return Promise.reject(error)
    }
)

export default request
```

### 9.2 服务器 API (`src/api/server.ts`)

```typescript
import request from './request'
import type { Server, ServerForm, TestResult } from '@/types'

export const serverApi = {
    getList: (params?: { group?: string; status?: string }) =>
        request.get<Server[]>('/servers', { params }),

    getById: (id: string) =>
        request.get<Server>(`/servers/${id}`),

    create: (data: ServerForm) =>
        request.post<Server>('/servers', data),

    update: (id: string, data: Partial<ServerForm>) =>
        request.put<Server>(`/servers/${id}`, data),

    delete: (id: string) =>
        request.delete(`/servers/${id}`),

    testConnection: (id: string) =>
        request.post<TestResult>(`/servers/${id}/test`),

    testNewConnection: (data: Pick<ServerForm, 'host' | 'port' | 'username' | 'authType' | 'authValue'>) =>
        request.post<TestResult>('/servers/test', data)
}
```

### 9.3 指标 API (`src/api/metric.ts`)

```typescript
import request from './request'
import type { Overview, MetricsHistory, RealtimeSeries } from '@/types'

export const metricApi = {
    getOverview: () =>
        request.get<Overview>('/metrics/overview'),

    getHistory: (serverId: string, params?: { range?: string; metric?: string }) =>
        request.get<MetricsHistory>(`/metrics/${serverId}`, { params }),

    getRealtime: (params?: { minutes?: number; metric?: string }) =>
        request.get<RealtimeSeries>('/metrics/realtime', { params })
}
```

### 9.4 告警 API (`src/api/alert.ts`)

```typescript
import request from './request'
import type { Alert, AlertCount, PaginatedList } from '@/types'

export const alertApi = {
    getList: (params?: {
        page?: number
        page_size?: number
        server_id?: string
        severity?: string
        status?: string
    }) => request.get<PaginatedList<Alert>>('/alerts', { params }),

    getCount: () =>
        request.get<AlertCount>('/alerts/count'),

    resolve: (id: number) =>
        request.put<Alert>(`/alerts/${id}/resolve`),

    batchResolve: (ids: number[]) =>
        request.put<{ resolved: number }>('/alerts/batch-resolve', { ids })
}
```

---

## 10. TypeScript 类型定义

```typescript
// src/types/index.ts

// ========== 服务器 ==========
export interface Server {
    id: string
    name: string
    host: string
    port: number
    username: string
    osType: 'linux' | 'windows'
    group: string
    sortOrder: number
    isActive: boolean
    isOnline: boolean
    uptime: string
    systemInfo?: SystemInfo
    latestMetrics: Metrics | null
    createdAt: string
    updatedAt: string
}

export interface ServerForm {
    name: string
    host: string
    port: number
    username: string
    authType: 'password' | 'key'
    authValue: string
    osType: 'linux' | 'windows'
    group: string
    sortOrder: number
}

export interface SystemInfo {
    os: string
    kernel: string
    cpuModel: string
    cpuCores: number
    totalMemory: number
    totalDisk: number
    hostname: string
}

export interface TestResult {
    connected: boolean
    latency: number
    serverInfo: { os: string; kernel: string; hostname: string } | null
    message: string
}

// ========== 指标 ==========
export interface Metrics {
    cpuUsage: number
    memTotal: number
    memUsed: number
    memUsage: number
    diskTotal: number
    diskUsed: number
    diskUsage: number
    netIn: number
    netOut: number
    load1m: number
    load5m: number
    load15m: number
    processCount: number
    collectedAt: string
}

export interface MetricsPoint {
    timestamp: string
    cpuUsage: number
    memUsage: number
    diskUsage: number
    netIn: number
    netOut: number
    load1m: number
    load5m: number
    load15m: number
}

export interface MetricsHistory {
    serverId: string
    serverName: string
    range: string
    points: MetricsPoint[]
    summary: {
        cpu: { min: number; max: number; avg: number }
        memory: { min: number; max: number; avg: number }
        disk: { min: number; max: number; avg: number }
    }
}

export interface RealtimeSeries {
    metric: string
    minutes: number
    series: {
        serverId: string
        serverName: string
        color: string
        data: { t: string; v: number }[]
    }[]
}

// ========== 概览 ==========
export interface Overview {
    serverCount: number
    onlineCount: number
    offlineCount: number
    warningCount: number
    avgCpu: number
    avgMemory: number
    avgDisk: number
    totalNetIn: number
    totalNetOut: number
    activeAlerts: number
    servers: ServerSummary[]
}

export interface ServerSummary {
    id: string
    name: string
    isOnline: boolean
    cpuUsage: number
    memUsage: number
    diskUsage: number
    status: 'normal' | 'warning' | 'danger' | 'offline'
}

// ========== 告警 ==========
export interface Alert {
    id: number
    serverId: string
    serverName: string
    alertType: 'cpu_high' | 'mem_high' | 'disk_full' | 'offline' | 'load_high'
    message: string
    severity: 'info' | 'warning' | 'critical'
    isResolved: boolean
    createdAt: string
    resolvedAt: string | null
}

export interface AlertCount {
    total: number
    critical: number
    warning: number
    info: number
}

export interface AlertRule {
    id: number
    metric: string
    warningThreshold: number
    criticalThreshold: number
    consecutiveCount: number
    enabled: boolean
    description: string
}

// ========== 通用 ==========
export interface PaginatedList<T> {
    list: T[]
    total: number
    page: number
    page_size: number
}

// ========== WebSocket 消息 ==========
export type WSMessage =
    | { type: 'metrics_update'; data: WSMetricsUpdate }
    | { type: 'alert'; data: Alert }
    | { type: 'status_change'; data: WSStatusChange }
    | { type: 'alert_resolved'; data: WSAlertResolved }
    | { type: 'ping'; timestamp: number }

export interface WSMetricsUpdate {
    servers: (Metrics & { serverId: string; serverName: string; isOnline: boolean })[]
    overview: {
        serverCount: number
        onlineCount: number
        offlineCount: number
        warningCount: number
        avgCpu: number
        avgMemory: number
    }
    timestamp: number
}

export interface WSStatusChange {
    serverId: string
    serverName: string
    oldStatus: 'online' | 'offline'
    newStatus: 'online' | 'offline'
    timestamp: number
}

export interface WSAlertResolved {
    id: number
    serverId: string
    serverName: string
    alertType: string
    message: string
    resolvedAt: string
}
```

---

## 11. API 接口汇总表

| 方法 | 路径 | 功能 | 认证 |
|------|------|------|------|
| `GET` | `/api/servers` | 获取服务器列表 | 否 |
| `GET` | `/api/servers/:id` | 获取服务器详情 | 否 |
| `POST` | `/api/servers` | 新增服务器 | 否 |
| `PUT` | `/api/servers/:id` | 更新服务器 | 否 |
| `DELETE` | `/api/servers/:id` | 删除服务器 | 否 |
| `POST` | `/api/servers/:id/test` | 测试已有服务器连接 | 否 |
| `POST` | `/api/servers/test` | 测试新服务器连接 | 否 |
| `GET` | `/api/metrics/overview` | 全局概览 | 否 |
| `GET` | `/api/metrics/realtime` | 实时趋势数据 | 否 |
| `GET` | `/api/metrics/:serverId` | 历史指标查询 | 否 |
| `GET` | `/api/alerts` | 告警列表 | 否 |
| `GET` | `/api/alerts/count` | 告警计数 | 否 |
| `PUT` | `/api/alerts/:id/resolve` | 解决告警 | 否 |
| `PUT` | `/api/alerts/batch-resolve` | 批量解决告警 | 否 |
| `GET` | `/api/alert-rules` | 获取告警规则 | 否 |
| `PUT` | `/api/alert-rules/:id` | 更新告警规则 | 否 |
| `GET` | `/api/config` | 获取系统配置 | 否 |
| `PUT` | `/api/config` | 更新系统配置 | 否 |
| `WS` | `/ws` | WebSocket 实时连接 | 否 |

> 当前版本未加认证，后续可通过 Gin 中间件加入 JWT 认证。

---

> **文档维护**: 随项目迭代持续更新本文档
