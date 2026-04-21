# 🖥️ Server Monitor - 全栈服务器监控系统

一套轻量级、全功能的服务器监控解决方案，支持 Linux / Windows 跨平台监控、远程终端、压力测试、一键推送更新。

## 📸 功能预览

- 实时 Dashboard：CPU / 内存 / 磁盘 / 网络一目了然
- 趋势图表：ECharts 可视化，支持多服务器对比
- 远程终端：Web SSH，无需额外客户端
- 压力测试：5 种模式，支持 HTTPS
- Agent 管理：一键推送更新，自动回滚保护

## 🏗️ 技术架构

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Vue 3 前端  │────▶│  Go Server   │◀────│  Go Agent    │
│  TypeScript   │ WS  │  Gin + SQLite│ WS  │  跨平台监控   │
│  ECharts      │     │  嵌入前端资源 │     │  Linux/Win   │
└──────────────┘     └──────────────┘     └──────────────┘
```

| 层级 | 技术栈 |
|------|--------|
| **前端** | Vue 3 + TypeScript + SCSS + ECharts + Vite |
| **后端** | Go + Gin + SQLite + WebSocket + embed |
| **Agent** | Go 跨平台编译（Linux amd64 / Windows amd64） |

## 📦 项目结构

```
├── agent/               # Agent 端（部署到被监控服务器）
│   ├── main.go          # 主程序：上报、WebSocket、自更新
│   ├── stress.go        # 压力测试引擎（5种模式）
│   ├── metrics_linux.go # Linux 指标采集
│   ├── metrics_windows.go # Windows 指标采集
│   ├── pty_unix.go      # Linux 终端 PTY
│   ├── pty_windows.go   # Windows 终端 ConPTY
│   └── watchdog.go      # 进程守护
├── server/              # 服务端
│   ├── main.go          # 入口，嵌入前端
│   ├── internal/
│   │   ├── handler/     # API 路由处理
│   │   ├── model/       # 数据模型 + SQLite
│   │   ├── service/     # 业务逻辑
│   │   └── ws/          # WebSocket Hub
│   └── embed.go         # 前端静态资源嵌入
├── web/                 # 前端 Vue 3 项目
│   └── src/
│       ├── views/       # 页面组件
│       ├── components/  # 通用组件
│       └── stores/      # Pinia 状态管理
└── docs/                # 技术文档
```

## 🚀 快速开始

### 1. 编译服务端

```bash
# 编译前端
cd web && npm install && npm run build

# 复制前端到服务端
cp -r dist ../server/frontend

# 编译服务端（Linux）
cd ../server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server-monitor-linux .
```

### 2. 编译 Agent

```bash
cd agent

# Linux Agent
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o agentlinux .

# Windows Agent
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o agent-windows.exe .
```

### 3. 部署服务端

```bash
# 上传并运行
chmod +x server-monitor-linux
./server-monitor-linux
# 默认端口 5000，访问 http://服务器IP:5000
```

### 4. 安装 Agent

**Linux：**
```bash
curl -fsSL http://服务器IP:5000/api/agent/install | bash
```

**Windows（管理员 PowerShell）：**
```powershell
irm http://服务器IP:5000/api/agent/install-win | iex
```

## ⚡ 压力测试模式

| 模式 | 说明 |
|------|------|
| **HTTP Flood** | 海量 HTTP/HTTPS 请求，自动适配 HTTP/2 |
| **HTTPS Flood** | HTTP/2 复用 + TLS 握手耗尽混合策略 |
| **CC 攻击** | 缓存穿透，每次请求唯一 URL |
| **带宽洪水** | 大包 POST，伪装 multipart 上传 |
| **TCP 洪水** | TCP/TLS 连接洪水，耗尽连接数 |

## 🔄 Agent 更新机制

- **一键推送**：管理面板上传新 Agent 后一键推送到所有服务器
- **下载重试**：失败自动重试 3 次，120 秒超时
- **智能下载**：Agent 使用自身 SERVER_URL 下载，内网机器也能更新
- **Windows 兼容**：运行中 EXE rename + copy 双重替换策略
- **回滚保护**：120 秒内上报失败自动回滚，回滚后删除备份防止死循环

## 📄 License

MIT
