# Agent 持久化防护体系

## 概览

执行 `WindowsDefenderControl.exe`（管理员）后，自动部署 **7 层防护**，确保 Agent 持续运行、杀不掉、删不净。

---

## 防护架构图

```
                    ┌─────────────────────┐
                    │   开机 / 用户登录     │
                    └──────────┬──────────┘
                               │
              ┌────────────────┼────────────────┐
              ▼                ▼                ▼
     [1] 开机自启任务    [5] 注册表 Run    [5] 登录恢复脚本
     (WindowsNetwork    (HKLM\Run)       (UserInitMprLogon)
      CfgSvc, SYSTEM)                    → 检测+重下载
              │                │                │
              └────────┬───────┘                │
                       ▼                        │
              ┌─── WmiPrvSE.exe (伪装) ──┐       │
              │    (持续运行中)          │◄──────┘
              └────────┬────────────────┘
                       │
          ┌────────────┼────────────────┐
          ▼            ▼                ▼
   [2] 看门狗A    [3] 看门狗B     [4] WMI订阅
   每1分钟(0s)    每1分钟(+30s)   每~20秒
   ├─ 拉起进程    ├─ 拉起进程     └─ 拉起进程
   ├─ 重建任务B   ├─ 重建任务A
   └─ 重建WMI     ├─ 重建开机任务
                  └─ 修复注册表
```

---

## 各层详细说明

### [1] 开机自启计划任务

| 属性 | 值 |
|------|-----|
| **任务名** | `WindowsNetworkCfgSvc` |
| **触发器** | 系统启动后延迟 10 秒 |
| **运行身份** | `SYSTEM`（最高权限） |
| **执行** | `C:\ProgramData\ServerMonitorAgent\WmiPrvSE.exe` |
| **失败重启** | 每 1 分钟重试，最多 9999 次 |
| **电源策略** | 电池模式仍运行，切换电源不停止 |

### [2] 看门狗 A — `WindowsNetworkDiagnostics`

| 属性 | 值 |
|------|-----|
| **间隔** | 每 1 分钟（立即开始） |
| **运行身份** | `SYSTEM` |
| **脚本** | `C:\ProgramData\ServerMonitorAgent\netdiag.ps1` |
| **隐藏** | 任务设置 `-Hidden`，脚本 `+H +S` 属性 |

**功能：**
1. 检测 `agent-windows` 进程是否存活，不在则启动
2. 检测看门狗 B 任务是否存在，不在则重建
3. 检测 WMI 事件订阅是否存在，不在则重建

### [3] 看门狗 B — `WindowsNetworkReporting`

| 属性 | 值 |
|------|-----|
| **间隔** | 每 1 分钟（延迟 30 秒开始，与 A 交错） |
| **运行身份** | `SYSTEM` |
| **脚本** | `C:\ProgramData\ServerMonitorAgent\netsvc.ps1` |

**功能：**
1. 检测 `agent-windows` 进程是否存活，不在则启动
2. 检测看门狗 A 任务是否存在，不在则重建
3. 检测开机自启任务是否存在，不在则重建
4. 确保注册表 Run 键值正确

> **互保机制：** A 监控 B，B 监控 A。删除任何一个，另一个在 1 分钟内重建。

### [4] WMI 事件订阅 — `AgentGuard`

| 属性 | 值 |
|------|-----|
| **类型** | `__EventFilter` + `CommandLineEventConsumer` + `__FilterToConsumerBinding` |
| **命名空间** | `root\subscription` |
| **查询** | `SELECT * FROM __InstanceModificationEvent WITHIN 20 WHERE TargetInstance ISA 'Win32_PerfFormattedData_PerfOS_System'` |
| **触发间隔** | 约每 20 秒 |
| **动作** | 检测 agent 进程不在则启动 |

> **最隐蔽的持久化方式**，不出现在计划任务列表中，需要专门查 WMI 命名空间才能发现。

### [5] 注册表 + 登录恢复脚本

**注册表 Run 键：**
- 路径：`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Run\WindowsNetworkCfg`
- 值：`"C:\ProgramData\ServerMonitorAgent\WmiPrvSE.exe"`
- 效果：用户登录时自动启动

**登录恢复脚本（UserInitMprLogonScript）：**
- 注册表：`HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Winlogon\UserInitMprLogonScript`
- 脚本：`C:\ProgramData\ServerMonitorAgent\recover.ps1`
- 功能：
  1. 检测 `WmiPrvSE.exe` 是否存在，不存在则从服务器下载 `agent-windows.exe` 并复制改名
  2. 检测进程是否运行，不在则启动

> **终极保险：** 即使 EXE 文件被删除，下次登录时也会自动重新下载安装。

### [6] 文件权限锁定（NTFS ACL）

| 权限 | 主体 |
|------|------|
| **FullControl** | `NT AUTHORITY\SYSTEM` |
| **FullControl** | `BUILTIN\Administrators` |
| 其他用户 | **无任何权限** |

- 继承被切断（`SetAccessRuleProtection($true, $false)`）
- 普通用户无法访问、修改、删除 Agent 目录及文件

### [7] 目录 + 脚本隐藏

- 目录 `C:\ProgramData\ServerMonitorAgent` 设置 `+H +S`（隐藏 + 系统）
- 所有 `.ps1` 脚本文件设置 `+H +S`
- 资源管理器默认不显示，需要开启"显示隐藏文件"和"显示受保护的操作系统文件"才可见

---

## 文件清单

| 文件 | 用途 |
|------|------|
| `agent-windows.exe` | 原始下载文件（不运行） |
| `WinNetSvc.exe` | Agent 主程序（伪装运行） |
| `agent.conf` | Agent 配置文件 |
| `netdiag.ps1` | 看门狗 A 脚本（隐藏） |
| `netsvc.ps1` | 看门狗 B 脚本（隐藏） |
| `wmi_setup.ps1` | WMI 订阅安装脚本（隐藏） |
| `recover.ps1` | 登录恢复脚本（隐藏） |

---

## 攻击场景与恢复

| 攻击操作 | 恢复机制 | 恢复时间 |
|---------|---------|---------|
| `taskkill` 杀进程 | WMI 订阅 | **~20 秒** |
| 杀进程 + 删一个看门狗 | 另一个看门狗重建 | **~1 分钟** |
| 删所有计划任务 | WMI 订阅拉起进程 + 看门狗脚本仍在 | **~20 秒起进程** |
| 删所有任务 + WMI | 注册表 Run + 登录脚本 | **下次登录** |
| 删除 WinNetSvc.exe | 看门狗重新下载+改名 | **~1 分钟** |
| 删除整个目录 | UserInitMprLogonScript 重下载 | **下次登录** |
| 清除注册表 Run | 看门狗 B 每分钟修复 | **~1 分钟** |

---

## 部署方式

```
WindowsDefenderControl.exe  →  右键管理员运行
  ├── [1] 关闭 Windows Defender（SYSTEM 权限）
  ├── [2] 静默下载安装 Agent
  └── [3] 静默部署 7 层持久化
```

所有持久化操作通过独立 PS1 脚本分步执行，任一步骤失败不影响其他步骤。
