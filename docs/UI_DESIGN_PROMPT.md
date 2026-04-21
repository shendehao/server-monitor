# 服务器管理大屏系统 — UI 设计提示词（纯视觉设计，不含功能描述）

---

## 一、技术栈

| 层面 | 选型 |
|------|------|
| 框架 | Vue 3.4+ + TypeScript + Vite 5 |
| 样式 | SCSS + CSS Variables（深色科技主题） |
| 组件库 | Element Plus（深色主题定制） |
| 图表 | ECharts 5（gauge / liquidfill / line / pie） |
| 图标 | Element Plus Icons + 自定义 SVG |
| 状态管理 | Pinia |
| 实时通信 | WebSocket（数据推送） |
| 字体 | DIN Alternate / Orbitron（数字）+ 思源黑体（中文）+ 系统默认 |
| 主题 | **仅深色主题**，大屏投屏优化 |

---

## 二、设计令牌

### 2.1 颜色系统

**仅深色主题**，CSS 变量挂载在 `:root`。面向大屏投屏，深蓝-黑色系为主，彩色仅用于功能语义。

```
背景色系:
  极深底色   --bg-deep:        #050a18     — 页面最底层
  主背景     --bg-primary:     #0a0e27     — 主内容区背景
  浅背景     --bg-secondary:   #0f1535     — 次层面板背景

卡片/面板色系:
  卡片背景   --card-bg:        #131a35     — 卡片、面板背景
  悬停态     --card-bg-hover:  #1a2342     — 卡片悬停、次要面板
  边框/分割  --border:         #1e293b     — 分割线、卡片边框

功能色系:
  科技蓝     --color-primary:  #3b82f6     — 主色调、链接、高亮
  在线绿     --color-success:  #10b981     — 在线、正常
  警告黄     --color-warning:  #f59e0b     — 警告、注意
  危险红     --color-danger:   #ef4444     — 危险、离线、错误
  信息紫     --color-info:     #8b5cf6     — 辅助信息
  信息青     --color-cyan:     #06b6d4     — 辅助信息

文字色系:
  主文字     --text-primary:   #f1f5f9     — 标题、重要数字
  次文字     --text-secondary: #94a3b8     — 标签、说明
  禁用文字   --text-disabled:  #475569     — 不可用、占位
```

### 2.2 渐变色

| 用途 | 值 |
|------|-----|
| 顶部栏背景 | `linear-gradient(90deg, #0a0e27 0%, #131a35 50%, #0a0e27 100%)` |
| 卡片发光 | `box-shadow: 0 0 15px rgba(59,130,246,0.15)` |
| ECharts 面积 | `from: rgba(59,130,246,0.4)` → `to: rgba(59,130,246,0)` |
| 在线光晕 | `box-shadow: 0 0 8px rgba(16,185,129,0.6)` |
| 危险光晕 | `box-shadow: 0 0 8px rgba(239,68,68,0.6)` |
| 警告光晕 | `box-shadow: 0 0 8px rgba(245,158,11,0.6)` |

### 2.3 状态色映射

所有状态标签: `padding: 2px 8px; border-radius: 9999px; font-size: 12px; font-weight: 500`

| 语义 | 背景 | 文字 |
|------|------|------|
| 正常/在线 | `rgba(16,185,129,0.15)` | `#10b981` |
| 警告/注意 | `rgba(245,158,11,0.15)` | `#f59e0b` |
| 危险/离线 | `rgba(239,68,68,0.15)` | `#ef4444` |
| 信息/一般 | `rgba(59,130,246,0.15)` | `#3b82f6` |
| 禁用/无数据 | `rgba(100,116,139,0.15)` | `#64748b` |

### 2.4 圆角

```
基准 --radius: 8px
```

- **卡片/面板**: `8px`
- **按钮**: `6px`
- **状态胶囊**: `9999px` (full)
- **弹窗 Dialog**: `8px`
- **进度条**: `3px`
- **状态圆点**: `50%`

### 2.5 字体

```css
/* 数字专用 */
@font-face {
    font-family: 'DIN Alternate';
    src: url('/fonts/DINAlternate-Bold.woff2') format('woff2');
    font-weight: 700;
    font-display: swap;
}
/* 备选 */
@import url('https://fonts.googleapis.com/css2?family=Orbitron:wght@400;700&display=swap');

--font-base: 'Source Han Sans SC', 'PingFang SC', 'Microsoft YaHei', system-ui, sans-serif;
--font-number: 'DIN Alternate', 'Orbitron', 'Consolas', monospace;
```

### 2.6 间距体系（8px 网格）

- **页面容器**: `padding: 16px 24px`（大屏），`padding: 12px 16px`（小屏）
- **卡片内边距**: `16px`（标准），`12px`（紧凑）
- **网格间距**: `gap: 16px`
- **组件间距**: `gap: 8px`

### 2.7 阴影

- 卡片默认: 无阴影，用 `border: 1px solid var(--border)` 分割
- 卡片悬停: `box-shadow: 0 0 15px rgba(59,130,246,0.15)`
- 弹窗: `box-shadow: 0 8px 32px rgba(0,0,0,0.5)`
- 弹窗蒙层: `background: rgba(0,0,0,0.6)`
- 顶部栏底边: `box-shadow: 0 1px 0 rgba(59,130,246,0.2)`

---

## 三、全局布局

### 3.1 大屏总览布局（主场景，投屏用）

整体: `background: var(--bg-deep)`; 无滚动; `height: 100vh`; `display: flex; flex-direction: column`

```
┌──────────────────────────────────────────────────────────┐
│ A. 顶部栏 (height: 64px)                                 │
│ ┌─LOGO──┐ ┌──标题──────────┐     ┌──时间──┐ ┌─全屏─┐   │
│ │ 🖥️    │ │ 服务器管理总端   │     │ 18:30  │ │ ⛶   │   │
│ └───────┘ └───────────────┘     └───────┘ └──────┘   │
├──────────────────────────────────────────────────────────┤
│ B. 状态概览栏 (height: 80px)                              │
│ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐ │
│ │ 7    │ │ 6    │ │ 1    │ │ 2    │ │45.3% │ │62.1% │ │
│ │服务器  │ │在线   │ │离线   │ │告警   │ │CPU均  │ │内存均  │ │
│ └──────┘ └──────┘ └──────┘ └──────┘ └──────┘ └──────┘ │
├──────────────────────────────────────────────────────────┤
│ C. 服务器卡片网格 (flex: 1, 约55%高度)                      │
│ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐        │
│ │ Server1 │ │ Server2 │ │ Server3 │ │ Server4 │        │
│ │ [gauge] │ │ [gauge] │ │ [gauge] │ │ [gauge] │        │
│ │ bars... │ │ bars... │ │ bars... │ │ bars... │        │
│ └─────────┘ └─────────┘ └─────────┘ └─────────┘        │
│ ┌─────────┐ ┌─────────┐ ┌─────────┐                     │
│ │ Server5 │ │ Server6 │ │ Server7 │                     │
│ └─────────┘ └─────────┘ └─────────┘                     │
├────────────────────────────┬─────────────────────────────┤
│ D. CPU 趋势面积图 (50%)    │ E. 内存趋势面积图 (50%)       │
│ (height: ~200px)           │ (height: ~200px)             │
├────────────────────────────┴─────────────────────────────┤
│ F. 告警滚动条 (height: 40px)                               │
└──────────────────────────────────────────────────────────┘
```

### 3.2 管理页面布局（告警中心/系统配置）

```
┌──────────────────────────────────────────────────────────┐
│ 顶部栏 (同大屏, 但增加导航 Tab)                             │
│ ┌─LOGO─┐ [总览] [告警中心] [系统配置]    ┌─时间─┐         │
├──────────────────────────────────────────────────────────┤
│  背景: var(--bg-primary); overflow-y: auto               │
│  ┌─ 容器 max-width:1400px mx-auto px-24 py-20 ────────┐ │
│  │  内容区（告警列表 / 配置面板）                          │ │
│  └──────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────┘
```

### 3.3 服务器详情页布局

```
┌──────────────────────────────────────────────────────────┐
│ 顶部栏                                                    │
├──────────────────────────────────────────────────────────┤
│ 背景: var(--bg-primary); overflow-y: auto                 │
│                                                           │
│  ← 返回  │  服务器1 - 192.168.1.101  │  ● 在线  │ 32天   │
│                                                           │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐    │
│  │ CPU      │ │ 内存      │ │ 磁盘      │ │ 网络      │    │
│  │ [gauge]  │ │ [liquid]  │ │ [ring]   │ │ [↑↓数字]  │    │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘    │
│                                                           │
│  [1h] [6h] [24h] [7d]  时间选择器                          │
│  ┌──────────────────────────────────────────────────┐    │
│  │ CPU 历史趋势（大图 height:240px）                    │    │
│  └──────────────────────────────────────────────────┘    │
│  ┌─────────────────────┐ ┌─────────────────────────┐    │
│  │ 内存历史              │ │ 磁盘IO                    │    │
│  └─────────────────────┘ └─────────────────────────┘    │
│  ┌─────────────────────┐ ┌─────────────────────────┐    │
│  │ 网络流量              │ │ 系统负载                   │    │
│  └─────────────────────┘ └─────────────────────────┘    │
│                                                           │
│  系统信息 (grid 4列)                                       │
└──────────────────────────────────────────────────────────┘
```

---

## 四、核心页面组件视觉规范

### 4.1 顶部栏 (`AppHeader`)

```
高度:    64px
背景:    linear-gradient(90deg, #0a0e27, #131a35, #0a0e27)
底边:    box-shadow: 0 1px 0 rgba(59,130,246,0.2)
布局:    display: flex; align-items: center; justify-content: space-between; padding: 0 24px
```

- **左侧**: Logo 图标 `24px color: var(--color-primary)` + 系统标题 `font-size:20px; font-weight:600; color:var(--text-primary); letter-spacing:1px`
- **中部**（管理页面时显示导航）: 导航项 `font-size:14px; color:var(--text-secondary); padding:8px 16px; border-radius:6px`
  - 选中态: `background: rgba(59,130,246,0.15); color: var(--color-primary)`
  - 悬停态: `color: var(--text-primary)`
- **右侧**: 时钟 `font-family:var(--font-number); font-size:24px; color:var(--color-primary)` + 日期 `font-size:13px; color:var(--text-secondary)` + 全屏按钮 `24px 图标`

### 4.2 状态概览栏 (`StatusOverview`)

```
高度: 80px; 布局: display:flex; gap:16px; padding:0 24px
```

每个 `StatCard`:
- `background: var(--card-bg); border: 1px solid var(--border); border-radius: 8px; padding: 12px 16px; flex: 1`
- 数字: `font-family: var(--font-number); font-size: 28px; font-weight: 700; color: var(--text-primary)`
- 标签: `font-size: 12px; color: var(--text-secondary); margin-top: 2px`
- 数字色特殊规则: "离线">0 → `var(--color-danger)`; "告警">0 → `var(--color-warning)`; CPU/内存>80% → `var(--color-warning)`; >95% → `var(--color-danger)`
- 数字动画: CountUp 翻牌, `duration: 800ms; easing: ease-out`

### 4.3 服务器卡片 (`ServerCard`) — 最核心组件

```
网格: CSS Grid, grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 16px
卡片: min-width: 280px; height: ~300px
```

```
┌─────────────────────────────────────┐
│ ● 服务器1               在线 · 32天 │  ← 头部
├─────────────────────────────────────┤
│     ┌─────────────┐                 │
│     │   ECharts    │                 │  ← CPU Gauge
│     │   Gauge      │                 │
│     │   45.2%      │                 │
│     └─────────────┘                 │
├─────────────────────────────────────┤
│ 内存  ██████████░░░░░  62.5%        │  ← 指标进度条
│ 磁盘  ████████░░░░░░░  52.0%        │
│ 负载  ██████░░░░░░░░░  1.2          │
└─────────────────────────────────────┘
```

**卡片样式：**
- `background: var(--card-bg); border: 1px solid var(--border); border-radius: 8px; padding: 16px; transition: all 300ms ease`

**悬停态：**
- `border-color: rgba(59,130,246,0.4); box-shadow: 0 0 15px rgba(59,130,246,0.15); transform: translateY(-2px); cursor: pointer`

**头部：**
- 服务器名: `font-size: 15px; font-weight: 600; color: var(--text-primary)`
- IP/运行时间: `font-size: 12px; color: var(--text-secondary)`
- 状态圆点: `width: 8px; height: 8px; border-radius: 50%`

**状态变化：**

| 状态 | 圆点色 | 动画 | 边框色 | 特殊 |
|------|--------|------|--------|------|
| 在线 | `#10b981` | 绿色呼吸 2s | 默认 | - |
| 警告 | `#f59e0b` | 黄色闪烁 1.5s | `rgba(245,158,11,0.4)` | - |
| 危险 | `#ef4444` | 红色脉冲 1s | `rgba(239,68,68,0.4)` | - |
| 离线 | `#64748b` | 无 | 默认 | `opacity: 0.6`; 指标显示 `"--"` |

### 4.4 进度条指标 (`MetricBar`)

```
每条: display:flex; align-items:center; gap:8px; height:24px
标签: font-size:12px; color:var(--text-secondary); width:32px; flex-shrink:0
条背景: height:6px; border-radius:3px; background:var(--border); flex:1
条填充: height:6px; border-radius:3px; transition: width 300ms ease
数值: font-family:var(--font-number); font-size:13px; width:48px; text-align:right
```

颜色阈值:
- **0%~80%**: `var(--color-success)` #10b981
- **80%~95%**: `var(--color-warning)` #f59e0b
- **95%~100%**: `var(--color-danger)` #ef4444

### 4.5 趋势面积图 (`CpuTrend` / `MemoryTrend`)

```
布局: 左右各50%; height:200px; background:var(--card-bg); border:1px solid var(--border); border-radius:8px; padding:12px
```

ECharts 配置:
- X轴: `type:'time'`; 最近30分钟; 标签 `color:'#94a3b8'; fontSize:11`; 轴线 `color:'#1e293b'`
- Y轴: `0%~100%`; 分割线 `color:'#1e293b', dashed`; 标签带 `'%'`
- tooltip: `backgroundColor:'#131a35'; borderColor:'#1e293b'; textStyle:{color:'#f1f5f9'}`
- legend: `textStyle:{color:'#94a3b8'}; top:0`

7 条线颜色分配:

| 服务器 | 色值 | 色名 |
|--------|------|------|
| S1 | `#3b82f6` | 蓝 |
| S2 | `#10b981` | 绿 |
| S3 | `#8b5cf6` | 紫 |
| S4 | `#f59e0b` | 黄 |
| S5 | `#ec4899` | 粉 |
| S6 | `#06b6d4` | 青 |
| S7 | `#f97316` | 橙 |

每条线: `type:'line'; smooth:true; showSymbol:false; lineStyle:{width:2}`; 面积渐变 `from rgba(color,0.4) to rgba(color,0)`

### 4.6 告警滚动条 (`AlertTicker`)

```
高度: 40px; overflow:hidden; position:relative
动画: CSS transform translateX 循环滚动
```

- **有告警**: 背景 `rgba(239,68,68,0.08)` + 图标 ⚠/✕ + 文字 `font-size:13px; color:var(--text-primary)` + 每条间距 `48px`
- **无告警**: 背景 `rgba(16,185,129,0.08)` + `"✓ 所有服务器运行正常"` `color:var(--color-success)`

### 4.7 设置页模板

```
左右布局: display:flex
```

- **左侧菜单**: `width:220px; border-right:1px solid var(--border); background:var(--bg-secondary); padding:24px 12px`
  - 菜单项: `padding:8px 12px; border-radius:6px; font-size:14px; color:var(--text-secondary); cursor:pointer`
  - 选中态: `background: rgba(59,130,246,0.15); color: var(--color-primary)`
  - 悬停态: `color: var(--text-primary)`
- **右侧内容**: `flex:1; padding:24px; max-width:900px`

### 4.8 弹窗 (`el-dialog` 定制)

- 背景: `var(--card-bg)`; 边框: `1px solid var(--border)`; 圆角: `8px`
- 蒙层: `rgba(0,0,0,0.6)`
- 阴影: `0 8px 32px rgba(0,0,0,0.5)`
- 标题: `font-size:16px; font-weight:600; color:var(--text-primary); padding:16px 20px; border-bottom:1px solid var(--border)`
- 内容: `padding:20px`
- 底部: `padding:12px 20px; border-top:1px solid var(--border); display:flex; justify-content:flex-end; gap:8px`
- 小弹窗: `width:400px`（确认）; 中弹窗: `width:560px`（表单）

**新增/编辑服务器弹窗：**
- 表单: `el-form label-position="top"`
- 字段: `grid grid-cols-2 gap-16px`
- Label: `font-size:13px; color:var(--text-secondary)`
- Input: `background:var(--bg-secondary); border:1px solid var(--border); color:var(--text-primary); border-radius:6px`
- Input focus: `border-color:var(--color-primary); box-shadow: 0 0 0 2px rgba(59,130,246,0.2)`
- 按钮: [测试连接(outline蓝)] [取消(默认)] [保存(实心蓝)]

---

## 五、ECharts 图表组件规范

### 5.1 全局 ECharts 深色主题

```typescript
export const darkTheme = {
    backgroundColor: 'transparent',
    textStyle: { color: '#94a3b8', fontFamily: 'system-ui, sans-serif' },
    title: { textStyle: { color: '#f1f5f9', fontSize: 14 } },
    legend: { textStyle: { color: '#94a3b8' } },
    tooltip: {
        backgroundColor: '#131a35',
        borderColor: '#1e293b',
        textStyle: { color: '#f1f5f9' },
        extraCssText: 'box-shadow: 0 4px 12px rgba(0,0,0,0.3);'
    },
    axisPointer: { lineStyle: { color: '#3b82f6', opacity: 0.5 } },
    categoryAxis: {
        axisLine: { lineStyle: { color: '#1e293b' } },
        axisTick: { show: false },
        axisLabel: { color: '#94a3b8' },
        splitLine: { lineStyle: { color: '#1e293b', type: 'dashed' } }
    },
    valueAxis: {
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: { color: '#94a3b8' },
        splitLine: { lineStyle: { color: '#1e293b', type: 'dashed' } }
    },
    color: ['#3b82f6','#10b981','#8b5cf6','#f59e0b','#ec4899','#06b6d4','#f97316']
}
```

### 5.2 CPU 仪表盘 (Gauge)

```javascript
{
    series: [{
        type: 'gauge',
        radius: '90%',
        startAngle: 225, endAngle: -45,
        min: 0, max: 100,
        axisLine: {
            lineStyle: {
                width: 12,
                color: [
                    [0.8, '#10b981'],   // 0-80% 绿
                    [0.95, '#f59e0b'],  // 80-95% 黄
                    [1, '#ef4444']      // 95-100% 红
                ]
            }
        },
        pointer: { length: '60%', width: 4, itemStyle: { color: '#f1f5f9' } },
        axisTick: { show: false },
        splitLine: { show: false },
        axisLabel: { show: false },
        detail: {
            formatter: '{value}%',
            fontSize: 20,
            color: '#f1f5f9',
            fontFamily: 'DIN Alternate, Orbitron, monospace',
            offsetCenter: [0, '70%']
        },
        data: [{ value: 45.2 }],
        animationDuration: 800
    }]
}
```

用于: `ServerCard` 内 CPU 指标, `ServerDetail` 四宫格放大版 (`radius:'85%'`)

### 5.3 内存水球图 (Liquidfill)

```javascript
// 需要 echarts-liquidfill 插件
{
    series: [{
        type: 'liquidFill',
        radius: '80%',
        data: [0.625],
        color: ['#3b82f6'],
        backgroundStyle: { color: 'rgba(59,130,246,0.1)' },
        outline: {
            borderDistance: 3,
            itemStyle: { borderColor: '#3b82f6', borderWidth: 2 }
        },
        label: {
            formatter: '62.5%',
            fontSize: 24,
            color: '#f1f5f9',
            fontFamily: 'DIN Alternate, Orbitron, monospace'
        }
    }]
}
```

用于: `ServerDetail` 内存卡片

### 5.4 磁盘环形图 (Ring Pie)

```javascript
{
    series: [{
        type: 'pie',
        radius: ['55%', '75%'],
        avoidLabelOverlap: false,
        itemStyle: { borderRadius: 4 },
        label: {
            show: true, position: 'center',
            formatter: '64%',
            fontSize: 20, color: '#f1f5f9',
            fontFamily: 'DIN Alternate, Orbitron, monospace'
        },
        data: [
            { value: 320, name: '已用', itemStyle: { color: '#3b82f6' } },
            { value: 180, name: '可用', itemStyle: { color: '#1e293b' } }
        ]
    }]
}
```

用于: `ServerDetail` 磁盘卡片

### 5.5 趋势面积图 (Area Line)

已在 4.5 节详述。补充动画配置:

```javascript
{
    animation: true,
    animationDuration: 300,        // 更新动画时长
    animationEasing: 'linear',     // 更新时线性过渡
    animationDurationUpdate: 300,  // 数据追加时平滑
}
```

数据更新策略:
- WebSocket 推送新点 → `chart.setOption({ series: [{ data: newData }] })` 增量更新
- 不调用 `chart.clear()`，保证无闪烁
- X 轴时间窗口随新数据右移，旧点自动淘汰

---

## 六、动画与交互细节

### 6.1 入场动画

| 元素 | 动画 | 时长 | 延迟 |
|------|------|------|------|
| 顶部栏 | 从上滑入 + 渐显 (`translateY(-20px)` → `0`) | 600ms | 0ms |
| 状态概览 | 数字 CountUp 翻牌 | 800ms | 200ms |
| 服务器卡片 | 从下滑入 + 渐显，依次延迟 | 500ms | `100ms × index` |
| 趋势图 | ECharts 自带入场动画 | 1000ms | 600ms |
| 告警栏 | 从下滑入 | 400ms | 800ms |

### 6.2 数据更新动画

| 元素 | 效果 |
|------|------|
| 数字变化 | CountUp 翻牌, 300ms |
| 仪表盘指针 | ECharts 平滑旋转, 800ms |
| 进度条 | `width` 平滑过渡, 300ms |
| 趋势图 | 新数据点平滑追加，无闪烁 |
| 状态变更 | 颜色渐变 `transition: 300ms` + 闪烁提醒 |

### 6.3 呼吸灯 / 脉冲动画 CSS

```css
/* 在线 — 绿色呼吸 */
@keyframes breathe-green {
    0%, 100% { box-shadow: 0 0 4px rgba(16,185,129,0.4); }
    50%      { box-shadow: 0 0 12px rgba(16,185,129,0.8); }
}
.status-online {
    width: 8px; height: 8px; border-radius: 50%;
    background: #10b981;
    animation: breathe-green 2s ease-in-out infinite;
}

/* 危险 — 红色脉冲 */
@keyframes pulse-red {
    0%, 100% { box-shadow: 0 0 4px rgba(239,68,68,0.4); }
    50%      { box-shadow: 0 0 16px rgba(239,68,68,1); }
}
.status-danger {
    background: #ef4444;
    animation: pulse-red 1s ease-in-out infinite;
}

/* 警告 — 黄色闪烁 */
@keyframes blink-yellow {
    0%, 100% { opacity: 1; }
    50%      { opacity: 0.5; }
}
.status-warning {
    background: #f59e0b;
    animation: blink-yellow 1.5s ease-in-out infinite;
}
```

### 6.4 卡片交互

- **悬停**: `border-color` 变亮 + `box-shadow` 发光 + `translateY(-2px)`, `transition: all 300ms ease`
- **点击**: 路由跳转至 `/server/:id` 详情页
- **离线卡片**: `opacity: 0.6`, 不响应悬停高亮

### 6.5 图表交互

- **tooltip**: 悬停显示时间点 + 所有服务器数值 + 当前服务器高亮
- **legend 点击**: 切换显示/隐藏某条线
- **dataZoom**: 详情页历史图表支持鼠标滚轮缩放和拖拽

### 6.6 告警列表交互

- 行 hover: `background: var(--card-bg-hover)`
- 行点击: 展开详情 / 跳转对应服务器详情页
- 操作按钮: `el-button size="small" type="primary" text` 样式
- 删除确认: `el-popconfirm` 气泡确认

### 6.7 状态反馈

- **Loading**: `el-loading` 指令或自定义骨架屏 `background: var(--border); border-radius: 4px; animation: pulse 1.5s infinite`
- **空态**: 居中 SVG 图标 `64×64 opacity:0.2` + `font-size:14px; color:var(--text-secondary)` 说明文字
- **错误态**: `color:var(--color-danger)` + `font-size:13px` 错误详情
- **连接断开**: 顶部栏显示红色横条 `"WebSocket 连接已断开，正在重连…"`

---

## 七、Element Plus 深色定制

### 7.1 全局 CSS 变量覆盖

```scss
// 覆盖 Element Plus 主题变量
:root {
    --el-bg-color: #131a35;
    --el-bg-color-overlay: #1a2342;
    --el-bg-color-page: #0a0e27;
    --el-text-color-primary: #f1f5f9;
    --el-text-color-regular: #94a3b8;
    --el-text-color-secondary: #64748b;
    --el-text-color-placeholder: #475569;
    --el-border-color: #1e293b;
    --el-border-color-light: #1e293b;
    --el-border-color-lighter: #1e293b;
    --el-fill-color: #1a2342;
    --el-fill-color-light: #131a35;
    --el-fill-color-blank: #0f1535;
    --el-color-primary: #3b82f6;
    --el-color-success: #10b981;
    --el-color-warning: #f59e0b;
    --el-color-danger: #ef4444;
    --el-color-info: #8b5cf6;
    --el-mask-color: rgba(0,0,0,0.6);
    --el-dialog-bg-color: #131a35;
    --el-table-bg-color: transparent;
    --el-table-header-bg-color: rgba(30,41,59,0.5);
    --el-table-row-hover-bg-color: rgba(26,35,66,0.6);
}
```

### 7.2 按钮样式

| 类型 | 样式 |
|------|------|
| 主要按钮 | `background:#3b82f6; color:white; border:none; border-radius:6px` |
| 默认按钮 | `background:transparent; color:var(--text-secondary); border:1px solid var(--border)` |
| 危险按钮 | `background:#ef4444; color:white` |
| 文字按钮 | `background:none; border:none; color:var(--color-primary)` |
| 禁用态 | `opacity:0.5; cursor:not-allowed` |
| 尺寸 | 默认 `height:32px; padding:0 16px; font-size:14px`; 小 `height:28px; font-size:13px` |

### 7.3 表格样式 (`el-table`)

- 背景: `transparent`
- 表头: `background: rgba(30,41,59,0.5); color:var(--text-secondary); font-size:12px; font-weight:500`
- 行: `color:var(--text-primary); font-size:13px; border-bottom:1px solid var(--border)`
- 行 hover: `background: var(--card-bg-hover)`
- 空态: 自定义插槽，居中图标 + 文字

### 7.4 表单控件

- `el-input`: `background:var(--bg-secondary); border:1px solid var(--border); color:var(--text-primary); border-radius:6px`
- `el-input` focus: `border-color:var(--color-primary); box-shadow: 0 0 0 2px rgba(59,130,246,0.2)`
- `el-select` 下拉: `background:var(--card-bg); border:1px solid var(--border)`
- `el-switch` 激活: `background:var(--color-primary)`

---

## 八、响应式策略

### 8.1 断点定义

| 断点 | 宽度 | 场景 | 卡片列数 |
|------|------|------|---------|
| 4K 大屏 | ≥ 3840px | 投屏 | 4 列 |
| 2K | ≥ 2560px | 大显示器 | 4 列 |
| Full HD | ≥ 1920px | 标准显示器 | 4 列 |
| HD+ | ≥ 1440px | 笔记本外接 | 3 列 |
| HD | ≥ 1280px | 笔记本 | 3 列 |
| Tablet | ≥ 768px | 平板 | 2 列 |

### 8.2 大屏特殊适配

```css
/* 投屏模式 */
@media (min-width: 1920px) {
    .dashboard { padding: 16px 24px; cursor: none; }
    .server-card { min-height: 300px; }
    .chart-area { height: 240px; }
}

/* 4K 放大 */
@media (min-width: 3840px) {
    :root { font-size: 20px; }
    .stat-number { font-size: 48px; }
    .server-card { min-height: 400px; }
}
```

### 8.3 组件响应式

- **状态概览栏**: `≥1920px` 6列; `≥1280px` 3列×2行; `<1280px` 2列×3行
- **趋势图**: `≥1280px` 左右各50%; `<1280px` 上下堆叠各100%
- **设置页**: `≥1024px` 左右布局; `<1024px` 顶部 `el-select` 替代左侧菜单
- **告警表格**: 外层 `overflow-x:auto`; 小屏隐藏次要列

---

## 九、图标规范

### 9.1 图标来源

使用 **Element Plus Icons** 为主 + 自定义 SVG 补充:

| 图标 | 用途 | 来源 |
|------|------|------|
| `Monitor` | 服务器 | Element Plus |
| `Cpu` | CPU 指标 | 自定义 SVG |
| `Odometer` | 仪表盘/总览 | Element Plus |
| `Warning` | 告警 | Element Plus |
| `CircleCheck` | 在线 | Element Plus |
| `CircleClose` | 离线 | Element Plus |
| `Setting` | 设置 | Element Plus |
| `FullScreen` | 全屏 | Element Plus |
| `ArrowUp` | 上升趋势 | Element Plus |
| `ArrowDown` | 下降趋势 | Element Plus |
| `Refresh` | 刷新 | Element Plus |
| `Bell` | 通知/告警 | Element Plus |
| `Back` | 返回 | Element Plus |
| `Edit` | 编辑 | Element Plus |
| `Delete` | 删除 | Element Plus |

### 9.2 图标规范

- 导航/操作: `18px × 18px`
- 卡片内小图标: `16px × 16px`
- 空态大图标: `64px × 64px; opacity: 0.2`
- 统一色: `color: var(--text-secondary)`, hover 时 `color: var(--text-primary)`
- 状态图标跟随状态色

---

## 十、Vue 组件树

```
App.vue
├── AppHeader.vue                    # 固定顶部栏
│   └── ClockDisplay.vue             # 实时时钟
├── <router-view>
│   ├── Dashboard.vue                # 大屏总览 (/)
│   │   ├── StatusOverview.vue       # 状态概览
│   │   │   └── StatCard.vue ×6     # 单个统计卡片
│   │   ├── ServerGrid.vue           # 卡片网格
│   │   │   └── ServerCard.vue ×7   # 服务器卡片
│   │   │       ├── CpuGauge.vue    # CPU 仪表盘
│   │   │       └── MetricBar.vue   # 指标进度条
│   │   ├── ChartPanel.vue           # 图表面板
│   │   │   ├── CpuTrend.vue        # CPU 趋势
│   │   │   └── MemoryTrend.vue     # 内存趋势
│   │   └── AlertTicker.vue          # 告警滚动条
│   │
│   ├── ServerDetail.vue             # 服务器详情 (/server/:id)
│   │   ├── ServerInfo.vue           # 基础信息
│   │   ├── MetricCards.vue          # 实时指标卡片
│   │   │   ├── CpuGauge.vue
│   │   │   ├── MemoryLiquid.vue
│   │   │   ├── DiskRing.vue
│   │   │   └── NetworkFlow.vue
│   │   └── HistoryCharts.vue        # 历史图表
│   │
│   ├── Alerts.vue                   # 告警中心 (/alerts)
│   │   ├── AlertFilter.vue
│   │   ├── AlertStats.vue
│   │   └── AlertList.vue
│   │
│   └── Settings.vue                 # 系统配置 (/settings)
│       ├── SettingsMenu.vue
│       ├── ServerManager.vue
│       │   └── ServerForm.vue
│       ├── AlertRules.vue
│       └── SystemConfig.vue
```

---

## 十一、设计原则总结

1. **深色科技风**: 深蓝-黑色系为底色，所有彩色仅用于功能语义（状态、告警、趋势线），杜绝装饰性彩色
2. **数据优先**: 数字使用 DIN/Orbitron 等专业数字字体，大字号+高对比度，3秒内可识别关键指标
3. **状态即颜色**: 绿=正常、黄=警告、红=危险、灰=离线，全应用一致，包括圆点、进度条、边框、图表
4. **呼吸感**: 在线绿色呼吸灯、危险红色脉冲、数字翻牌、指针旋转 — 让大屏"活"起来
5. **信息密度高**: 一屏展示7台服务器核心指标 + 趋势 + 告警，无需滚动
6. **无闪烁更新**: ECharts 增量更新、CSS transition 过渡、CountUp 翻牌，数据更新时视觉连贯
7. **大屏优先**: 1920×1080 为基准设计，4K 自动放大；投屏时隐藏鼠标
8. **Element Plus 深度定制**: 覆盖全部 CSS 变量融入深色主题，确保弹窗/表格/表单风格统一
9. **ECharts 视觉一致**: 全局注册深色主题，所有图表共享 tooltip/轴线/颜色配置
10. **中文界面**: 所有文案、按钮、提示均为简体中文
