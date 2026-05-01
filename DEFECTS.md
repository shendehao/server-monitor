# Agent 缺陷与改进清单

> 对比参考：gh0st (大灰狼9.8) / Hero / 机器猫  
> 审计日期：2025-04-29  
> 涉及组件：Go Agent (`agent/`) + C# DLL (`agent-cs/MiniAgent.cs`) + Server (`server/`)

---

## 一、功能缺失（按优先级排序）

### 🔴 P0 — 核心渗透能力缺失

#### 1. SOCKS5 反向代理隧道
- **现状**：完全没有
- **gh0st 参考**：`Plugins/PROXY/OpenProxy.h` — 完整 SOCKS4/5 + HTTP 代理，支持认证、TCP CONNECT、UDP ASSOCIATE
- **影响**：无法通过被控机器访问内网其他资源，无法 pivot，横向渗透严重受限
- **建议方案**：通过已有 WebSocket 连接实现反向 SOCKS5 隧道（不需目标开端口）
  - Agent 端：接收 `socks_connect` 指令 → 连接目标内网地址 → 通过 WebSocket 回传数据
  - Server 端：开启本地 SOCKS5 监听端口，接收本地工具的代理请求，转发给 Agent
  - 支持 TCP CONNECT（覆盖 90% 场景）
- **工作量估计**：Agent 端 ~300行，Server 端 ~400行

#### 2. TCP 端口转发/映射
- **现状**：完全没有
- **gh0st 参考**：`Plugins/PROXYMAP/ProxyManager.h` — TCP 端口映射，支持 10000 并发
- **影响**：无法把内网端口（RDP/数据库/Web）映射到攻击机
- **建议方案**：
  - `port_forward_start {remoteHost}:{remotePort}` → Agent 连接内网目标 → 数据通过 WebSocket 中转
  - Server 端本地开监听端口，转发流量
- **工作量估计**：~250行（可复用 SOCKS5 的数据中转逻辑）

#### 3. 远程桌面控制输入（鼠标+键盘）
- **现状**：只有屏幕监控（只能看），无法发送鼠标/键盘事件
- **gh0st 参考**：`Plugins/SCREEN1/ScreenManager.cpp` — 支持鼠标移动/点击/双击/右键 + 键盘输入
- **影响**：只能被动观察，不能远程操作目标桌面
- **建议方案**：
  - 新增 `screen_input` 命令，接收 `{type:"mouse/key", x, y, button, keyCode}` 参数
  - Agent 端使用 `SendInput()` / `mouse_event()` / `keybd_event()` Win32 API 注入输入
  - 前端在屏幕画面上捕获鼠标/键盘事件，通过 WebSocket 发送
- **工作量估计**：Agent ~150行，前端 ~200行

#### 4. 文件上传到目标（反向传文件）
- **现状**：只能从目标下载文件到服务器，不能上传文件到目标
- **gh0st 参考**：`Plugins/FILE/FileManager.h` — 双向文件传输，支持进度、断点续传
- **影响**：无法上传渗透工具/payload/配置文件到目标机器
- **建议方案**：
  - 新增 `file_upload` 命令，服务端发送文件数据（base64 或分块二进制）
  - Agent 端接收并写入指定路径
  - 支持分块传输（大文件）
- **工作量估计**：~150行

---

### 🟡 P1 — 实用功能缺失

#### 5. 注册表编辑器
- **现状**：没有专门的注册表管理功能
- **gh0st 参考**：`Plugins/REGEDIT/RegeditManager.h` — 完整注册表浏览/创建/修改/删除
- **影响**：无法远程查看/修改注册表（安全策略、启动项、服务配置等）
- **建议方案**：
  - `reg_browse` — 列出子键和值
  - `reg_read` — 读取指定值
  - `reg_write` — 写入值（STRING/DWORD/BINARY）
  - `reg_delete` — 删除键或值
- **工作量估计**：~200行

#### 6. 用户账户管理
- **现状**：没有
- **gh0st 参考**：`Plugins/SYSTEM/SystemManager.h` — 创建/删除/启用/禁用用户，改密码
- **影响**：无法创建后门账户、管理 RDP 访问权限
- **建议方案**：
  - `user_list` — 列出本地用户
  - `user_add` — 创建用户并加入管理员/远程桌面组
  - `user_delete` — 删除用户
  - `user_passwd` — 修改密码
  - 使用 `net user` / `net localgroup` 命令或 NetUserAdd API
- **工作量估计**：~120行

#### 7. RDP 远程桌面管理
- **现状**：没有
- **gh0st 参考**：`SystemManager.h` — Open3389 / Change3389Port / GetTermsrvFile
- **影响**：无法远程开启 RDP、修改端口
- **建议方案**：
  - `rdp_enable` — 开启 RDP（修改注册表 + 防火墙规则）
  - `rdp_port` — 修改 RDP 端口
  - 配合端口转发使用，从外网直接 RDP 到内网机器
- **工作量估计**：~80行

#### 8. 网络连接状态（增强版 Netstat）
- **现状**：需要手动 `exec netstat`，输出不结构化
- **gh0st 参考**：`Plugins/SYSTEM/GetNetState.h` — TCP/UDP 连接 + 进程PID/名称映射
- **影响**：无法快速了解目标机器的网络连接情况
- **建议方案**：
  - `netstat` 命令 → 返回结构化 JSON（协议/本地地址/远程地址/状态/PID/进程名）
  - 使用 `GetExtendedTcpTable` / `GetExtendedUdpTable` API
- **工作量估计**：~100行

#### 9. 已安装软件列表
- **现状**：没有
- **gh0st 参考**：`SystemManager.h` — getSoftWareList()
- **影响**：无法了解目标安装了哪些安全软件/远程工具/开发环境
- **建议方案**：
  - 枚举 `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall` 注册表
  - 返回 JSON 列表（名称/版本/发布者/安装日期）
- **工作量估计**：~60行

---

### 🟢 P2 — 性能/体验优化

#### 10. 音频压缩
- **现状**：原始 PCM16LE 16kHz 单声道 → base64 → 约 256kbps + 33% base64 膨胀
- **gh0st 参考**：G729a 编解码，约 8kbps
- **影响**：麦克风监听带宽消耗大，网络慢时容易卡顿/断开
- **建议方案**：
  - C# DLL：不方便引入 native codec，可用简单的 ADPCM 压缩（4:1 压缩比）
  - Go Agent：可引入 Opus 编码
  - 或改为二进制帧传输（去掉 base64 膨胀）
- **工作量估计**：~200行

#### 11. 屏幕差分帧算法
- **现状**：DXGI 截图 → JPEG → 帧哈希去重（相同帧不发），但没有像素级差分
- **gh0st 参考**：逐行像素比较 → 只发送变化区域 → Xvid 视频编码
- **影响**：静态桌面时效率可以，但鼠标移动/窗口拖动时带宽消耗大
- **建议方案**：
  - 将屏幕分成 N×M 块，只发送有变化的块
  - 或引入 WebP 有损压缩（比 JPEG 小 25-35%）
- **工作量估计**：~300行

#### 12. LSA Secret / DPAPI 深度提取
- **现状**：有 WiFi/浏览器/CredMan 提取，但没有 LSA secret dumping
- **gh0st 参考**：`Dialupass.h` — GetLsaPasswords / ParseLsaBuffer
- **影响**：遗漏部分系统级凭证（服务账户密码、自动登录密码等）
- **建议方案**：
  - 使用 `LsaOpenPolicy` + `LsaRetrievePrivateData` API
  - 或通过 reg save SAM/SYSTEM + 离线解析
- **工作量估计**：~150行

---

## 二、已有功能的已知缺陷

### Bug / 可靠性问题

| # | 模块 | 问题 | 严重程度 | 状态 |
|---|------|------|---------|------|
| B1 | 麦克风 (C# DLL) | 之前 8kHz 音质差，已改 16kHz/60ms/6缓冲区 | 中 | ✅ 已修复 |
| B2 | COM 劫持 (C# DLL) | 旧 CLSID 目标不常被加载，已更换为 MMDeviceEnumerator/WbemLocator | 中 | ✅ 已修复 |
| B3 | 持久化 (C# DLL) | 已添加 UserInitMprLogonScript 作为第6层持久化 | 低 | ✅ 已修复 |
| B4 | 屏幕截图 (Go) | Session 0 下 DXGI 失败后回退 BitBlt，但部分场景仍黑屏 | 中 | 需跟踪 |
| B5 | 摄像头 (Go/C#) | DirectShow 在部分机器上找不到设备，需 Media Foundation 回退 | 低 | Go 已有回退，C# 需验证 |
| B6 | 键盘记录 (C#) | Session 0 下 GetAsyncKeyState 可能无效 | 低 | 需测试 |

### 安全/隐蔽性问题

| # | 问题 | 说明 |
|---|------|------|
| S1 | PowerShell cradle 明文 | 持久化的 cradle 命令包含明文服务器 URL，EDR 可以提取 |
| S2 | WebSocket 无域前置 | 直连 C2 IP，流量分析可识别 |
| S3 | 进程特征 | powershell.exe 进程常驻，行为检测可能报警 |
| S4 | ETW 致盲不完整 | 只 patch 了 NtTraceEvent，部分 ETW provider 仍可记录 |
| S5 | 磁盘残留 | COM 劫持 DLL、VBS 启动器落地磁盘，有 IoC |
| S6 | 无通信混淆 | WebSocket 帧内容虽 TLS 加密，但流量模式（心跳间隔/帧大小）可被 ML 检测 |

---

## 三、架构层面建议

| # | 建议 | 说明 |
|---|------|------|
| A1 | **插件化架构** | gh0st 每个功能是独立 DLL 插件，按需加载。我们所有功能编译在一个 DLL 里（122KB），功能越多体积越大。建议对不常用功能改为内存加载的独立模块 |
| A2 | **二进制协议** | 当前 JSON + base64 编码效率低。屏幕/音频/文件传输建议改为二进制帧头 + 原始数据 |
| A3 | **通信压缩** | gh0st 用 zlib 压缩所有通信。WebSocket 支持 permessage-deflate 扩展，建议启用 |
| A4 | **心跳伪装** | 心跳间隔/包大小加入随机抖动，模拟正常 HTTPS 流量模式 |
| A5 | **多协议回退** | WebSocket 被封时回退到 HTTP 长轮询 / DNS 隧道 / ICMP 隧道 |
| A6 | **Go Agent 与 C# DLL 功能同步** | 部分功能只在 Go Agent 实现（如 DXGI 截屏优化），C# DLL 缺失相应优化 |

---

## 四、实施优先级总览

```
优先级   功能                    预估工时    价值
──────────────────────────────────────────────────
P0-1    SOCKS5 反向代理          3天       ★★★★★  内网渗透核心
P0-2    TCP 端口转发             2天       ★★★★★  配合 SOCKS5
P0-3    远程桌面输入             2天       ★★★★   看→控
P0-4    文件上传到目标           1天       ★★★★   上传工具

P1-5    注册表编辑器             1天       ★★★    远程配置
P1-6    用户账户管理             0.5天     ★★★    后门账户
P1-7    RDP 管理                0.5天     ★★★    开远程桌面
P1-8    Netstat 增强             0.5天     ★★     网络侦察
P1-9    软件列表                 0.5天     ★★     信息收集

P2-10   音频压缩                 2天       ★★     降带宽
P2-11   屏幕差分帧               3天       ★★     降带宽
P2-12   LSA Secret               1天       ★★     深度凭证
──────────────────────────────────────────────────
合计约 17 天
```
