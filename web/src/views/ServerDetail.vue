<template>
  <div class="server-detail" v-loading="loading">
    <div class="detail-header">
      <el-button text @click="router.push('/')">
        <el-icon><Back /></el-icon>
        返回总览
      </el-button>
      <h2 class="detail-title" v-if="detail">
        <span class="status-dot" :class="detail.isOnline ? 'online' : 'offline'"></span>
        {{ detail.name }}
      </h2>
      <span class="uptime font-num" v-if="detail?.uptime">运行 {{ detail.uptime }}</span>
    </div>

    <template v-if="detail">
      <div class="info-grid">
        <div class="info-item">
          <span class="info-label">主机</span>
          <span class="info-value">{{ detail.host }}:{{ detail.port }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">系统</span>
          <span class="info-value">{{ detail.osType }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">分组</span>
          <span class="info-value">{{ detail.group || '未分组' }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">负载</span>
          <span class="info-value font-num" v-if="detail.latestMetrics">
            {{ detail.latestMetrics.load1m }} / {{ detail.latestMetrics.load5m }} / {{ detail.latestMetrics.load15m }}
          </span>
        </div>
        <div class="info-item">
          <span class="info-label">进程数</span>
          <span class="info-value font-num">{{ detail.latestMetrics?.processCount ?? '-' }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">网络 I/O</span>
          <span class="info-value font-num" v-if="detail.latestMetrics">
            ↑{{ formatBytes(detail.latestMetrics.netOut) }}/s ↓{{ formatBytes(detail.latestMetrics.netIn) }}/s
          </span>
        </div>
      </div>

      <div class="metric-cards">
        <div class="metric-card">
          <div class="metric-card-title">CPU</div>
          <div ref="cpuGaugeRef" class="metric-chart"></div>
        </div>
        <div class="metric-card">
          <div class="metric-card-title">内存</div>
          <div class="metric-big-num">
            <span class="font-num">{{ (detail.latestMetrics?.memUsage ?? 0).toFixed(1) }}%</span>
            <span class="metric-sub">{{ detail.latestMetrics?.memUsed ?? 0 }} / {{ detail.latestMetrics?.memTotal ?? 0 }} MB</span>
          </div>
        </div>
        <div class="metric-card">
          <div class="metric-card-title">磁盘</div>
          <div class="metric-big-num">
            <span class="font-num">{{ (detail.latestMetrics?.diskUsage ?? 0).toFixed(1) }}%</span>
            <span class="metric-sub">{{ detail.latestMetrics?.diskUsed ?? 0 }} / {{ detail.latestMetrics?.diskTotal ?? 0 }} GB</span>
          </div>
        </div>
      </div>

      <div class="history-section">
        <div class="history-header">
          <span class="section-title">历史趋势</span>
          <el-radio-group v-model="period" size="small" @change="fetchHistory">
            <el-radio-button value="1h">1小时</el-radio-button>
            <el-radio-button value="6h">6小时</el-radio-button>
            <el-radio-button value="24h">24小时</el-radio-button>
          </el-radio-group>
        </div>
        <div ref="historyChartRef" class="history-chart"></div>
      </div>

      <!-- 交互式终端 -->
      <div class="terminal-section">
        <div class="terminal-header">
          <span class="section-title">远程终端</span>
          <span class="term-status" :class="termStatus">{{ termStatusText }}</span>
          <div class="term-actions">
            <button class="term-btn" @click="connectTerminal" :disabled="termStatus === 'connecting'" v-if="termStatus !== 'connected'">连接</button>
            <button class="term-btn danger" @click="disconnectTerminal" v-if="termStatus === 'connected'">断开</button>
          </div>
        </div>
        <div ref="xtermRef" class="xterm-container"></div>
      </div>

      <!-- 桌面查看器 -->
      <div class="screen-section">
        <div class="screen-header">
          <span class="section-title">桌面查看</span>
          <span class="term-status" :class="screenStatus">{{ screenStatusText }}</span>
          <div class="screen-controls" v-if="screenStatus === 'connected'">
            <select v-model="screenFps" @change="updateScreenConfig" class="screen-select">
              <option :value="1">1 FPS</option>
              <option :value="2">2 FPS</option>
              <option :value="5">5 FPS</option>
              <option :value="10">10 FPS</option>
              <option :value="15">15 FPS</option>
            </select>
            <select v-model="screenQuality" @change="updateScreenConfig" class="screen-select">
              <option :value="30">低画质</option>
              <option :value="50">中画质</option>
              <option :value="70">高画质</option>
            </select>
            <select v-model="screenScale" @change="updateScreenConfig" class="screen-select">
              <option :value="30">30%</option>
              <option :value="50">50%</option>
              <option :value="75">75%</option>
              <option :value="100">100%</option>
            </select>
          </div>
          <div class="term-actions">
            <button class="term-btn" @click="connectScreen" v-if="screenStatus !== 'connected'">查看</button>
            <button class="term-btn danger" @click="disconnectScreen" v-if="screenStatus === 'connected'">停止</button>
          </div>
        </div>
        <div class="screen-viewer" v-if="screenStatus === 'connected' || screenFrame">
          <img v-if="screenFrame" :src="screenFrame" class="screen-img" />
          <div v-else class="screen-placeholder">等待截图...</div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as echarts from 'echarts'
import { serverApi, metricApi } from '@/api'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { ClipboardAddon } from '@xterm/addon-clipboard'
import '@xterm/xterm/css/xterm.css'

const route = useRoute()
const router = useRouter()
const loading = ref(true)
const detail = ref<any>(null)
const period = ref('1h')
const cpuGaugeRef = ref<HTMLElement>()
const historyChartRef = ref<HTMLElement>()
const xtermRef = ref<HTMLElement>()
let gaugeChart: echarts.ECharts | null = null
let historyChart: echarts.ECharts | null = null

// 交互式终端
const termStatus = ref<'disconnected' | 'connecting' | 'connected'>('disconnected')
const termStatusText = computed(() => {
  switch (termStatus.value) {
    case 'connected': return '已连接'
    case 'connecting': return '连接中...'
    default: return '未连接'
  }
})

let term: Terminal | null = null
let fitAddon: FitAddon | null = null
let termWs: WebSocket | null = null
let resizeObserver: ResizeObserver | null = null
let pipeMode = false // agent 管道模式下本地回显
let pipeInputLen = 0 // 管道模式：当前行已输入字符数（防止退格删提示符）

// 桌面查看器
const screenStatus = ref<'disconnected' | 'connected'>('disconnected')
const screenStatusText = computed(() => screenStatus.value === 'connected' ? '实时查看中' : '未连接')
const screenFrame = ref('')
const screenFps = ref(2)
const screenQuality = ref(50)
const screenScale = ref(50)
let screenWs: WebSocket | null = null

function initXterm() {
  if (!xtermRef.value || term) return
  const isMobile = window.innerWidth <= 768
  term = new Terminal({
    cursorBlink: true,
    cursorStyle: 'bar',
    fontSize: isMobile ? 11 : 13,
    fontFamily: "'Cascadia Code', 'SF Mono', 'Menlo', 'Courier New', monospace",
    lineHeight: 1.3,
    theme: {
      background: '#0b0e17',
      foreground: '#c8d6e5',
      cursor: '#10b981',
      cursorAccent: '#0b0e17',
      selectionBackground: 'rgba(59,130,246,0.3)',
      black: '#0b0e17',
      red: '#f87171',
      green: '#10b981',
      yellow: '#fbbf24',
      blue: '#60a5fa',
      magenta: '#c084fc',
      cyan: '#22d3ee',
      white: '#e2e8f0',
      brightBlack: '#3d4f6a',
      brightRed: '#fca5a5',
      brightGreen: '#34d399',
      brightYellow: '#fde68a',
      brightBlue: '#93bbfc',
      brightMagenta: '#d8b4fe',
      brightCyan: '#67e8f9',
      brightWhite: '#f8fafc',
    },
    scrollback: 5000,
    allowProposedApi: true,
  })

  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.loadAddon(new ClipboardAddon())
  term.open(xtermRef.value)
  fitAddon.fit()

  // 键盘输入 → WebSocket
  term.onData((data: string) => {
    if (termWs && termWs.readyState === WebSocket.OPEN) {
      // 管道模式：本地回显（agent 管道模式下 PowerShell 不回显 stdin）
      if (pipeMode) {
        if (data === '\r') {
          term!.write('\r\n')
          pipeInputLen = 0
        } else if (data === '\x7f' || data === '\x08') {
          if (pipeInputLen > 0) {
            term!.write('\b \b')
            pipeInputLen--
          }
        } else if (data >= ' ') {
          term!.write(data)
          pipeInputLen += data.length
        }
      }
      termWs.send(data)
    }
  })

  // 右键粘贴
  xtermRef.value.addEventListener('contextmenu', async (e: MouseEvent) => {
    e.preventDefault()
    try {
      const text = await navigator.clipboard.readText()
      if (text && termWs && termWs.readyState === WebSocket.OPEN) {
        termWs.send(text)
      }
    } catch {}
  })

  // 自动调整大小
  resizeObserver = new ResizeObserver(() => {
    fitAddon?.fit()
    if (termWs && termWs.readyState === WebSocket.OPEN && term) {
      termWs.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
    }
  })
  resizeObserver.observe(xtermRef.value)

  term.writeln('\x1b[90m点击「连接」按钮打开交互式终端\x1b[0m')
}

function connectTerminal() {
  if (termStatus.value === 'connected' || termStatus.value === 'connecting') return
  if (!term) return

  termStatus.value = 'connecting'
  // agent/plugin 连接默认启用本地回显（管道模式），收到 pty_mode=conpty 时关闭
  const cm = detail.value?.connectMethod
  pipeMode = (cm === 'agent' || cm === 'plugin')
  term.clear()
  term.writeln('\x1b[33m正在连接...\x1b[0m')

  const token = localStorage.getItem('token') || ''
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const wsUrl = `${proto}://${location.host}/ws/terminal/${route.params.id}?token=${token}`

  termWs = new WebSocket(wsUrl)

  termWs.onopen = () => {
    termStatus.value = 'connected'
    term!.clear()
    termWs!.send(JSON.stringify({ type: 'resize', cols: term!.cols, rows: term!.rows }))
    term!.focus()
  }

  termWs.onmessage = (ev) => {
    if (term && typeof ev.data === 'string') {
      // 检查是否是 pty_mode JSON 消息
      if (ev.data.startsWith('{')) {
        try {
          const msg = JSON.parse(ev.data)
          if (msg.type === 'pty_mode') {
            // conpty 模式下关闭本地回显（远端已回显）
            pipeMode = msg.mode === 'pipe'
            return
          }
        } catch {}
      }
      term.write(ev.data)
      // 服务端输出后重置输入计数（新提示符出现）
      if (pipeMode) pipeInputLen = 0
    }
  }

  termWs.onclose = () => {
    if (termStatus.value === 'connected') {
      termStatus.value = 'disconnected'
      term?.writeln('\r\n\x1b[31m连接已断开\x1b[0m')
    } else {
      termStatus.value = 'disconnected'
    }
    termWs = null
  }

  termWs.onerror = () => {
    termStatus.value = 'disconnected'
    term?.writeln('\r\n\x1b[31m连接失败，请检查服务器SSH配置\x1b[0m')
    termWs = null
  }
}

function disconnectTerminal() {
  if (termWs) {
    termWs.close()
    termWs = null
  }
  termStatus.value = 'disconnected'
}

function cleanupTerminal() {
  disconnectTerminal()
  disconnectScreen()
  resizeObserver?.disconnect()
  term?.dispose()
  term = null
  fitAddon = null
}

function connectScreen() {
  if (screenStatus.value === 'connected') return
  const token = localStorage.getItem('token') || ''
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const wsUrl = `${proto}://${location.host}/ws/screen/${route.params.id}?token=${token}`
  screenWs = new WebSocket(wsUrl)
  screenWs.onopen = () => { screenStatus.value = 'connected' }
  screenWs.binaryType = 'blob'
  screenWs.onmessage = (ev) => {
    if (ev.data instanceof Blob) {
      // 二进制 JPEG 帧 → Blob URL（比 base64 高效 33%）
      const url = URL.createObjectURL(ev.data)
      const old = screenFrame.value
      screenFrame.value = url
      if (old && old.startsWith('blob:')) URL.revokeObjectURL(old)
    }
    // JSON 元数据（宽高等）忽略，只用图片
  }
  screenWs.onclose = () => {
    screenStatus.value = 'disconnected'
    screenWs = null
  }
  screenWs.onerror = () => {
    screenStatus.value = 'disconnected'
    screenWs = null
  }
}

function disconnectScreen() {
  if (screenWs) {
    screenWs.close()
    screenWs = null
  }
  screenStatus.value = 'disconnected'
  if (screenFrame.value && screenFrame.value.startsWith('blob:')) {
    URL.revokeObjectURL(screenFrame.value)
  }
  screenFrame.value = ''
}

function updateScreenConfig() {
  if (screenWs && screenWs.readyState === WebSocket.OPEN) {
    screenWs.send(JSON.stringify({
      type: 'config',
      fps: screenFps.value,
      quality: screenQuality.value,
      scale: screenScale.value,
    }))
  }
}

async function fetchDetail() {
  loading.value = true
  try {
    const res: any = await serverApi.getById(route.params.id as string)
    if (res.success) detail.value = res.data
  } finally {
    loading.value = false
  }
}

async function fetchHistory() {
  const res: any = await metricApi.history(route.params.id as string, period.value)
  if (!res.success || !historyChart) return

  const metrics = res.data || []
  const times = metrics.map((m: any) => new Date(m.collectedAt).toLocaleTimeString('zh-CN', { hour12: false }))
  const light = document.documentElement.classList.contains('light')

  const colors = {
    cpu: '#3b82f6',
    mem: '#10b981',
    disk: '#8b5cf6',
    tooltipBg: light ? 'rgba(255,255,255,0.96)' : '#131a35',
    tooltipBorder: light ? 'rgba(0,0,0,0.1)' : '#1e293b',
    tooltipText: light ? '#1e293b' : '#f1f5f9',
    axisLine: light ? '#e2e8f0' : '#1e293b',
    axisLabel: light ? '#64748b' : '#94a3b8',
    splitLine: light ? 'rgba(0,0,0,0.06)' : '#1e293b',
    legendText: light ? '#475569' : '#94a3b8',
  }

  function areaStyle(color: string) {
    return {
      color: {
        type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
        colorStops: [
          { offset: 0, color: color + (light ? '30' : '40') },
          { offset: 1, color: color + '05' },
        ],
      },
    }
  }

  const showSymbol = metrics.length < 30

  historyChart.setOption({
    tooltip: {
      trigger: 'axis',
      backgroundColor: colors.tooltipBg,
      borderColor: colors.tooltipBorder,
      textStyle: { color: colors.tooltipText, fontSize: 12 },
      axisPointer: { lineStyle: { color: colors.axisLine } },
    },
    legend: { bottom: 0, textStyle: { color: colors.legendText, fontSize: 11 }, icon: 'circle', itemWidth: 8 },
    grid: { left: 44, right: 16, top: 16, bottom: 40 },
    xAxis: {
      type: 'category',
      data: times,
      boundaryGap: false,
      axisLine: { lineStyle: { color: colors.axisLine } },
      axisTick: { show: false },
      axisLabel: { color: colors.axisLabel, fontSize: 11 },
    },
    yAxis: {
      type: 'value', min: 0, max: 100,
      axisLabel: { color: colors.axisLabel, fontSize: 11, formatter: '{value}%' },
      splitLine: { lineStyle: { color: colors.splitLine, type: 'dashed' } },
    },
    series: [
      { name: 'CPU', type: 'line', smooth: 0.4, showSymbol, symbolSize: 4, lineStyle: { width: 2.5 }, itemStyle: { color: colors.cpu }, areaStyle: areaStyle(colors.cpu), data: metrics.map((m: any) => m.cpuUsage) },
      { name: '内存', type: 'line', smooth: 0.4, showSymbol, symbolSize: 4, lineStyle: { width: 2.5 }, itemStyle: { color: colors.mem }, areaStyle: areaStyle(colors.mem), data: metrics.map((m: any) => m.memUsage) },
      { name: '磁盘', type: 'line', smooth: 0.4, showSymbol, symbolSize: 4, lineStyle: { width: 2.5 }, itemStyle: { color: colors.disk }, areaStyle: areaStyle(colors.disk), data: metrics.map((m: any) => m.diskUsage) },
    ],
  }, true)
}

function initGauge() {
  if (!cpuGaugeRef.value || !detail.value?.latestMetrics) return
  gaugeChart = echarts.init(cpuGaugeRef.value)
  gaugeChart.setOption({
    series: [{
      type: 'gauge', radius: '85%', startAngle: 225, endAngle: -45, min: 0, max: 100,
      axisLine: { lineStyle: { width: 14, color: [[0.8, '#10b981'], [0.95, '#f59e0b'], [1, '#ef4444']] } },
      pointer: { length: '55%', width: 4, itemStyle: { color: '#f1f5f9' } },
      axisTick: { show: false }, splitLine: { show: false }, axisLabel: { show: false },
      detail: { formatter: '{value}%', fontSize: 22, fontWeight: 600, color: '#e8edf5', fontFamily: "'SF Mono','Courier New',monospace", offsetCenter: [0, '70%'] },
      data: [{ value: detail.value.latestMetrics.cpuUsage.toFixed(1) }],
    }],
  })
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / 1048576).toFixed(1) + ' MB'
}

onMounted(async () => {
  await fetchDetail()
  setTimeout(() => {
    initGauge()
    if (historyChartRef.value) {
      historyChart = echarts.init(historyChartRef.value)
      fetchHistory()
    }
    initXterm()
  }, 100)
  window.addEventListener('resize', () => { gaugeChart?.resize(); historyChart?.resize() })
})

onUnmounted(() => {
  gaugeChart?.dispose()
  historyChart?.dispose()
  cleanupTerminal()
})
</script>

<style scoped lang="scss">
.server-detail {
  padding: 16px 20px;
  max-width: 1200px;
  margin: 0 auto;
}

.detail-header {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 18px;
}

.detail-title {
  font-size: 15px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
}

.uptime {
  margin-left: auto;
  font-size: 12px;
  color: var(--t3);
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  margin-bottom: 16px;
}

.info-item {
  background: var(--card-bg);
  backdrop-filter: blur(8px);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.info-label {
  font-size: 10px;
  color: var(--t3);
  letter-spacing: 0.5px;
  text-transform: uppercase;
}

.info-value {
  font-size: 13px;
  font-weight: 500;
  color: var(--t1);
}

.metric-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  margin-bottom: 16px;
}

.metric-card {
  background: var(--card-bg);
  backdrop-filter: blur(8px);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 14px;
  text-align: center;
}

.metric-card-title {
  font-size: 11px;
  color: var(--t3);
  margin-bottom: 8px;
  letter-spacing: 0.5px;
  text-transform: uppercase;
}

.metric-chart {
  width: 100%;
  height: 150px;
}

.metric-big-num {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 150px;

  .font-num {
    font-size: 32px;
    font-weight: 700;
    color: var(--t1);
  }

  .metric-sub {
    font-size: 11px;
    color: var(--t3);
    margin-top: 4px;
  }
}

.history-section {
  background: var(--card-bg);
  backdrop-filter: blur(8px);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 16px;
}

.history-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.section-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--t2);
  letter-spacing: 0.5px;
  text-transform: uppercase;
}

.history-chart {
  width: 100%;
  height: 280px;
}

/* ===== 交互式终端 ===== */
.terminal-section {
  margin-top: 16px;
  background: var(--card-bg);
  backdrop-filter: blur(8px);
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
}

.terminal-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--border);
}

.term-status {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  letter-spacing: 0.5px;
}

.term-status.connected {
  background: rgba(5,150,105,0.15);
  color: #10b981;
}

.term-status.connecting {
  background: rgba(251,191,36,0.12);
  color: #fbbf24;
}

.term-status.disconnected {
  background: rgba(100,116,139,0.1);
  color: #64748b;
}

.term-actions {
  margin-left: auto;
}

.term-btn {
  padding: 4px 14px;
  background: rgba(59,130,246,0.15);
  border: 1px solid rgba(59,130,246,0.3);
  border-radius: 4px;
  color: #60a5fa;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.term-btn:hover:not(:disabled) {
  background: rgba(59,130,246,0.25);
}

.term-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.term-btn.danger {
  background: rgba(239,68,68,0.12);
  border-color: rgba(239,68,68,0.3);
  color: #f87171;
}

.term-btn.danger:hover {
  background: rgba(239,68,68,0.2);
}

.xterm-container {
  height: 420px;
  background: #0b0e17;
  padding: 4px 0 4px 4px;
}

.xterm-container :deep(.xterm) {
  height: 100%;
}

.xterm-container :deep(.xterm-viewport) {
  &::-webkit-scrollbar { width: 6px; }
  &::-webkit-scrollbar-track { background: transparent; }
  &::-webkit-scrollbar-thumb {
    background: rgba(255,255,255,0.1);
    border-radius: 3px;
  }
}

/* ===== 桌面查看器 ===== */
.screen-section {
  margin-top: 16px;
  background: var(--card-bg);
  backdrop-filter: blur(8px);
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
}

.screen-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--border);
}

.screen-controls {
  display: flex;
  gap: 6px;
}

.screen-select {
  padding: 2px 6px;
  background: rgba(255,255,255,0.05);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--t2);
  font-size: 10px;
  cursor: pointer;
}

.screen-viewer {
  display: flex;
  justify-content: center;
  align-items: center;
  background: #0b0e17;
  min-height: 200px;
  padding: 8px;
}

.screen-img {
  max-width: 100%;
  height: auto;
  border-radius: 4px;
  image-rendering: auto;
}

.screen-placeholder {
  color: var(--t3);
  font-size: 12px;
}

/* ===== 移动端适配 ===== */
@media (max-width: 768px) {
  .server-detail { padding: 10px 12px; }
  .detail-header { flex-wrap: wrap; gap: 8px; margin-bottom: 12px; }
  .detail-title { font-size: 14px; }
  .uptime { margin-left: 0; flex-basis: 100%; font-size: 11px; }
  .info-grid { grid-template-columns: repeat(2, 1fr); gap: 6px; margin-bottom: 10px; }
  .info-item { padding: 8px 10px; }
  .info-value { font-size: 12px; }
  .metric-cards { grid-template-columns: 1fr; gap: 8px; margin-bottom: 10px; }
  .metric-chart { height: 120px; }
  .metric-big-num { height: 80px; .font-num { font-size: 24px; } }
  .history-section { padding: 10px; }
  .history-header { flex-direction: column; align-items: flex-start; gap: 8px; }
  .history-chart { height: 200px; }
  .terminal-header { padding: 8px 12px; gap: 8px; }
  .xterm-container { height: 300px; padding: 2px 0 2px 2px; }
  .term-btn { padding: 4px 10px; font-size: 10px; }
}

@media (max-width: 480px) {
  .server-detail { padding: 8px; }
  .info-grid { grid-template-columns: 1fr 1fr; gap: 4px; }
  .xterm-container { height: 260px; }
}
</style>

<style lang="scss">
/* ServerDetail Light Theme */
html.light .info-item,
html.light .metric-card,
html.light .history-section,
html.light .terminal-section {
  background: rgba(255,255,255,0.95);
  box-shadow: 0 1px 6px rgba(0,0,0,0.08);
  border-color: rgba(0,0,0,0.1);
}
html.light .term-status.connected {
  background: rgba(5,150,105,0.1);
  color: #059669;
}
html.light .term-status.disconnected {
  background: rgba(100,116,139,0.08);
  color: #94a3b8;
}
</style>
