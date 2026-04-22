<template>
  <div class="stress-page">
    <div class="stress-header">
      <h2 class="stress-title">压力测试</h2>
      <span class="stress-desc">分布式流量压测 — 批量控制服务器对目标发起高并发请求</span>
    </div>

    <!-- 攻击模式选择 -->
    <div class="mode-section">
      <div class="mode-grid">
        <label v-for="m in modes" :key="m.value" class="mode-card" :class="{ active: config.mode === m.value }" @click="!running && (config.mode = m.value)">
          <span class="mode-icon">{{ m.icon }}</span>
          <span class="mode-name">{{ m.label }}</span>
          <span class="mode-desc">{{ m.desc }}</span>
        </label>
      </div>
    </div>

    <!-- 配置区域 -->
    <div class="config-section">
      <div class="config-row">
        <div class="config-field url-field">
          <label>目标 URL / IP</label>
          <input v-model="config.url" placeholder="https://example.com 或 http://1.2.3.4:80" class="input" :disabled="running" />
        </div>
        <div class="config-field method-field" v-if="config.mode !== 'tcp_flood'">
          <label>请求方法</label>
          <select v-model="config.method" class="input" :disabled="running">
            <option value="GET">GET</option>
            <option value="POST">POST</option>
            <option value="PUT">PUT</option>
            <option value="DELETE">DELETE</option>
            <option value="HEAD">HEAD</option>
          </select>
        </div>
      </div>

      <div class="config-row">
        <div class="config-field">
          <label>并发数 (最高50000)</label>
          <input v-model.number="config.concurrency" type="number" min="1" max="50000" class="input" :disabled="running" />
        </div>
        <div class="config-field">
          <label>持续时间(秒)</label>
          <input v-model.number="config.duration" type="number" min="1" max="3600" class="input" :disabled="running" />
        </div>
        <div class="config-field" v-if="config.mode === 'bandwidth'">
          <label>包大小 (KB)</label>
          <input v-model.number="config.bodySize" type="number" min="1" max="10240" class="input" :disabled="running" />
        </div>
        <div class="config-field" v-else>
          <label>总请求数 (0=按时间)</label>
          <input v-model.number="config.totalReqs" type="number" min="0" class="input" :disabled="running" />
        </div>
      </div>

      <div class="config-row">
        <div class="config-field toggle-field">
          <label>连接复用 (KeepAlive)</label>
          <label class="toggle">
            <input type="checkbox" v-model="config.keepAlive" :disabled="running" />
            <span class="toggle-slider"></span>
            <span class="toggle-text">{{ config.keepAlive ? '开启' : '关闭' }}</span>
          </label>
        </div>
      </div>

      <div class="config-row" v-if="(config.method === 'POST' || config.method === 'PUT') && config.mode !== 'bandwidth' && config.mode !== 'tcp_flood'">
        <div class="config-field" style="flex: 1">
          <label>请求体 (Body)</label>
          <textarea v-model="config.body" class="input textarea" rows="3" placeholder='{"key":"value"}' :disabled="running"></textarea>
        </div>
      </div>
    </div>

    <!-- 服务器选择 -->
    <div class="server-section">
      <div class="section-header">
        <h3>选择服务器 <span class="server-count">{{ selectedCount }}/{{ agents.length }} 台</span></h3>
        <div class="section-actions">
          <button class="btn btn-sm" @click="selectAll" :disabled="running">全选</button>
          <button class="btn btn-sm" @click="selectNone" :disabled="running">全不选</button>
          <button class="btn btn-sm" @click="loadAgents" :disabled="running">刷新</button>
        </div>
      </div>
      <div class="agent-grid" v-if="agents.length > 0">
        <label v-for="agent in agents" :key="agent.id" class="agent-item" :class="{ offline: !agent.online, checked: selectedIds.has(agent.id) }">
          <input type="checkbox" :value="agent.id" :checked="selectedIds.has(agent.id)" @change="toggleAgent(agent.id)" :disabled="running || !agent.online" />
          <span class="agent-dot" :class="agent.online ? 'on' : 'off'"></span>
          <span class="agent-name">{{ agent.name }}</span>
          <span class="agent-status">{{ agent.online ? '在线' : '离线' }}</span>
        </label>
      </div>
      <div v-else class="no-agents">暂无 Agent/插件 服务器</div>
    </div>

    <!-- 操作按钮 -->
    <div class="action-bar">
      <button class="btn btn-primary btn-lg" @click="startTest" :disabled="running || selectedCount === 0 || !config.url">
        {{ running ? '攻击中...' : '开始压力测试' }}
      </button>
      <button class="btn btn-danger btn-lg" @click="stopTest" :disabled="!running">
        停止测试
      </button>
      <div class="attack-info" v-if="running">
        <span class="pulse-dot"></span>
        <span>{{ selectedCount }} 台服务器 × {{ config.concurrency }} 并发 = {{ selectedCount * config.concurrency }} 总并发</span>
      </div>
    </div>

    <!-- 实时进度 -->
    <div class="result-section" v-if="taskId">
      <!-- 核心指标卡片 -->
      <div class="summary-cards">
        <div class="sum-card">
          <div class="sum-val">{{ formatNum(totalSent) }}</div>
          <div class="sum-label">总请求</div>
        </div>
        <div class="sum-card success">
          <div class="sum-val">{{ formatNum(totalSuccess) }}</div>
          <div class="sum-label">成功</div>
        </div>
        <div class="sum-card error">
          <div class="sum-val">{{ formatNum(totalErrors) }}</div>
          <div class="sum-label">失败</div>
        </div>
        <div class="sum-card rps">
          <div class="sum-val">{{ totalRPS.toFixed(0) }}</div>
          <div class="sum-label">总 RPS</div>
        </div>
      </div>

      <!-- 带宽和连接数 -->
      <div class="summary-cards bw-cards">
        <div class="sum-card bw-send">
          <div class="sum-val">{{ totalMbpsSent.toFixed(1) }}</div>
          <div class="sum-label">发送 Mbps</div>
        </div>
        <div class="sum-card bw-recv">
          <div class="sum-val">{{ totalMbpsRecv.toFixed(1) }}</div>
          <div class="sum-label">接收 Mbps</div>
        </div>
        <div class="sum-card bw-total">
          <div class="sum-val">{{ formatBytes(totalBytesSent) }}</div>
          <div class="sum-label">已发送</div>
        </div>
        <div class="sum-card conn">
          <div class="sum-val">{{ formatNum(totalActiveConn) }}</div>
          <div class="sum-label">活跃连接</div>
        </div>
      </div>

      <!-- RPS 趋势 -->
      <div class="chart-section" v-if="rpsHistory.length > 0">
        <h3>RPS / 带宽 趋势</h3>
        <div class="rps-chart" ref="rpsChartRef"></div>
      </div>

      <!-- 各节点详情 -->
      <div class="agents-detail">
        <h3>各节点详情</h3>
        <div class="detail-table">
          <div class="table-head">
            <span class="col-name">服务器</span>
            <span class="col-num">已发送</span>
            <span class="col-num">成功</span>
            <span class="col-num">失败</span>
            <span class="col-num">RPS</span>
            <span class="col-num">延迟</span>
            <span class="col-num">发送Mbps</span>
            <span class="col-num">连接数</span>
            <span class="col-status">状态</span>
          </div>
          <div v-for="agent in agentProgress" :key="agent.serverId" class="table-row">
            <span class="col-name">{{ agent.serverName }}</span>
            <span class="col-num">{{ formatNum(agent.sent) }}</span>
            <span class="col-num t-green">{{ formatNum(agent.success) }}</span>
            <span class="col-num t-red">{{ formatNum(agent.errors) }}</span>
            <span class="col-num t-blue">{{ agent.rps.toFixed(0) }}</span>
            <span class="col-num">{{ agent.avgLatency.toFixed(1) }}ms</span>
            <span class="col-num t-orange">{{ agent.mbpsSent.toFixed(1) }}</span>
            <span class="col-num">{{ agent.activeConn }}</span>
            <span class="col-status">
              <span class="status-badge" :class="agent.running ? 'active' : 'done'">
                {{ agent.running ? '运行中' : '已完成' }}
              </span>
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { stressApi } from '@/api'
import * as echarts from 'echarts'

interface AgentInfo {
  id: string
  name: string
  online: boolean
}

interface AgentProgressItem {
  serverId: string
  serverName: string
  sent: number
  success: number
  errors: number
  rps: number
  avgLatency: number
  minLatency: number
  maxLatency: number
  bytesSent: number
  bytesRecv: number
  mbpsSent: number
  mbpsRecv: number
  activeConn: number
  running: boolean
}

const modes = [
  { value: 'http_flood', icon: '⚡', label: 'HTTP Flood', desc: '海量 HTTP 请求，压测 Web 服务' },
  { value: 'https_flood', icon: '🔒', label: 'HTTPS Flood', desc: 'HTTP/2 + TLS 握手耗尽，专攻 HTTPS' },
  { value: 'cc', icon: '🎯', label: 'CC 攻击', desc: '缓存穿透，每次请求唯一 URL' },
  { value: 'bandwidth', icon: '📡', label: '带宽洪水', desc: '大包 POST ，压测上行带宽' },
  { value: 'tcp_flood', icon: '🔌', label: 'TCP 洪水', desc: 'TCP 连接洪水，耗尽连接数' },
]

const config = ref({
  url: '',
  method: 'GET',
  mode: 'http_flood',
  concurrency: 500,
  duration: 30,
  totalReqs: 0,
  body: '',
  bodySize: 64,
  keepAlive: false,
  headers: {} as Record<string, string>,
})

const agents = ref<AgentInfo[]>([])
const selectedIds = ref(new Set<string>())
const running = ref(false)
const taskId = ref('')

const totalSent = ref(0)
const totalSuccess = ref(0)
const totalErrors = ref(0)
const totalRPS = ref(0)
const totalBytesSent = ref(0)
const totalBytesRecv = ref(0)
const totalMbpsSent = ref(0)
const totalMbpsRecv = ref(0)
const totalActiveConn = ref(0)
const agentProgress = ref<AgentProgressItem[]>([])
const rpsHistory = ref<{ t: string; v: number; mbps: number }[]>([])

const rpsChartRef = ref<HTMLDivElement>()
let rpsChart: echarts.ECharts | null = null
let stressWs: WebSocket | null = null
let chartUpdatePending = false
let lastChartUpdate = 0
let autoFinishTimer: ReturnType<typeof setTimeout> | null = null
const handleResize = () => rpsChart?.resize()

const selectedCount = computed(() => selectedIds.value.size)

function toggleAgent(id: string) {
  const s = new Set(selectedIds.value)
  if (s.has(id)) s.delete(id)
  else s.add(id)
  selectedIds.value = s
}

function selectAll() {
  selectedIds.value = new Set(agents.value.filter(a => a.online).map(a => a.id))
}

function selectNone() {
  selectedIds.value = new Set()
}

async function loadAgents() {
  try {
    const res: any = await stressApi.getAgents()
    agents.value = res || []
    selectedIds.value = new Set(agents.value.filter(a => a.online).map(a => a.id))
  } catch {
    agents.value = []
  }
}

function clearAutoFinishTimer() {
  if (autoFinishTimer) {
    clearTimeout(autoFinishTimer)
    autoFinishTimer = null
  }
}

function scheduleAutoFinish() {
  clearAutoFinishTimer()
  if (config.value.duration > 0) {
    autoFinishTimer = setTimeout(() => {
      running.value = false
      totalActiveConn.value = 0
      totalRPS.value = 0
      totalMbpsSent.value = 0
      totalMbpsRecv.value = 0
      agentProgress.value = agentProgress.value.map(item => ({ ...item, running: false, activeConn: 0 }))
    }, (config.value.duration + 5) * 1000)
  }
}

function resetMetrics() {
  totalSent.value = 0
  totalSuccess.value = 0
  totalErrors.value = 0
  totalRPS.value = 0
  totalBytesSent.value = 0
  totalBytesRecv.value = 0
  totalMbpsSent.value = 0
  totalMbpsRecv.value = 0
  totalActiveConn.value = 0
  agentProgress.value = []
  rpsHistory.value = []
  chartUpdatePending = false
  lastChartUpdate = 0
}

function handleStressMessage(data: any) {
  if (!taskId.value || data.taskId !== taskId.value) return

  totalSent.value = data.totalSent || 0
  totalSuccess.value = data.totalSuccess || 0
  totalErrors.value = data.totalErrors || 0
  totalRPS.value = data.totalRPS || 0
  totalBytesSent.value = data.totalBytesSent || 0
  totalBytesRecv.value = data.totalBytesRecv || 0
  totalMbpsSent.value = data.totalMbpsSent || 0
  totalMbpsRecv.value = data.totalMbpsRecv || 0
  totalActiveConn.value = data.totalActiveConn || 0

  const nextAgents = Array.isArray(data.agents) ? data.agents as AgentProgressItem[] : []
  agentProgress.value = nextAgents

  const hasRunningAgents = nextAgents.some(item => item.running)
  running.value = Boolean(data.running) || hasRunningAgents
  if (!running.value) {
    clearAutoFinishTimer()
    totalActiveConn.value = 0
  }

  rpsHistory.value.push({
    t: new Date().toLocaleTimeString('zh-CN', { hour12: false }),
    v: Math.round(data.totalRPS || 0),
    mbps: Math.round((data.totalMbpsSent || 0) * 10) / 10,
  })
  if (rpsHistory.value.length > 120) rpsHistory.value.shift()

  const now = Date.now()
  if (now - lastChartUpdate >= 1000 && !chartUpdatePending) {
    chartUpdatePending = true
    nextTick(() => {
      updateRpsChart()
      lastChartUpdate = Date.now()
      chartUpdatePending = false
    })
  }
}

function connectWs() {
  if (stressWs && (stressWs.readyState === WebSocket.OPEN || stressWs.readyState === WebSocket.CONNECTING)) return
  const token = localStorage.getItem('token')
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const url = `${proto}://${location.host}/ws/stress?token=${token}`

  stressWs = new WebSocket(url)
  stressWs.onmessage = (e) => {
    try {
      handleStressMessage(JSON.parse(e.data))
    } catch {
    }
  }
  stressWs.onclose = () => {
    stressWs = null
  }
}

async function startTest() {
  if (!config.value.url || selectedCount.value === 0) return

  taskId.value = ''
  resetMetrics()
  clearAutoFinishTimer()

  if (!stressWs || stressWs.readyState !== WebSocket.OPEN) {
    connectWs()
  }

  try {
    const res: any = await stressApi.start({
      url: config.value.url,
      method: config.value.method,
      mode: config.value.mode,
      concurrency: config.value.concurrency,
      duration: config.value.duration,
      totalReqs: config.value.totalReqs,
      body: config.value.body,
      bodySize: config.value.bodySize,
      keepAlive: config.value.keepAlive,
      headers: config.value.headers,
      serverIds: Array.from(selectedIds.value),
    })
    taskId.value = res.taskId || ''
    running.value = true
    scheduleAutoFinish()
  } catch (err: any) {
    clearAutoFinishTimer()
    alert(err.response?.data?.error || '启动失败')
  }
}

async function stopTest() {
  if (!taskId.value) return
  try {
    await stressApi.stop(taskId.value)
  } catch {
  }
  clearAutoFinishTimer()
  running.value = false
  totalActiveConn.value = 0
  totalRPS.value = 0
  totalMbpsSent.value = 0
  totalMbpsRecv.value = 0
  agentProgress.value = agentProgress.value.map(item => ({ ...item, running: false, activeConn: 0 }))
}

function formatNum(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

function formatBytes(b: number): string {
  if (b >= 1_000_000_000) return (b / 1_000_000_000).toFixed(2) + ' GB'
  if (b >= 1_000_000) return (b / 1_000_000).toFixed(1) + ' MB'
  if (b >= 1_000) return (b / 1_000).toFixed(1) + ' KB'
  return b + ' B'
}

function initRpsChart() {
  if (!rpsChartRef.value) return
  rpsChart = echarts.init(rpsChartRef.value)
  const light = document.documentElement.classList.contains('light')
  const axisColor = light ? '#94a3b8' : '#3d4f6a'
  const splitColor = light ? 'rgba(0,0,0,0.04)' : 'rgba(255,255,255,0.03)'
  rpsChart.setOption({
    grid: { top: 30, right: 60, bottom: 24, left: 60 },
    legend: { data: ['RPS', 'Mbps'], top: 0, textStyle: { color: axisColor, fontSize: 10 } },
    xAxis: {
      type: 'category',
      data: [],
      axisLabel: { color: axisColor, fontSize: 10 },
      axisLine: { lineStyle: { color: light ? '#e2e8f0' : 'rgba(255,255,255,0.06)' } },
    },
    yAxis: [
      {
        type: 'value', name: 'RPS', position: 'left',
        axisLabel: { color: axisColor, fontSize: 10 },
        splitLine: { lineStyle: { color: splitColor } },
        nameTextStyle: { color: axisColor, fontSize: 10 },
      },
      {
        type: 'value', name: 'Mbps', position: 'right',
        axisLabel: { color: axisColor, fontSize: 10 },
        splitLine: { show: false },
        nameTextStyle: { color: axisColor, fontSize: 10 },
      },
    ],
    series: [
      {
        name: 'RPS', type: 'line', yAxisIndex: 0, data: [], smooth: true, symbol: 'none',
        lineStyle: { width: 2, color: '#10b981' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(16,185,129,0.2)' },
            { offset: 1, color: 'rgba(16,185,129,0)' },
          ]),
        },
      },
      {
        name: 'Mbps', type: 'line', yAxisIndex: 1, data: [], smooth: true, symbol: 'none',
        lineStyle: { width: 2, color: '#f59e0b' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(245,158,11,0.15)' },
            { offset: 1, color: 'rgba(245,158,11,0)' },
          ]),
        },
      },
    ],
    animation: true,
    animationDuration: 300,
  })
}

function updateRpsChart() {
  if (!rpsChart) {
    initRpsChart()
    if (!rpsChart) return
  }
  rpsChart.setOption({
    xAxis: { data: rpsHistory.value.map(d => d.t) },
    series: [
      { data: rpsHistory.value.map(d => d.v) },
      { data: rpsHistory.value.map(d => d.mbps) },
    ],
  })
}

onMounted(() => {
  loadAgents()
  connectWs()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  clearAutoFinishTimer()
  if (stressWs) {
    stressWs.onclose = null
    stressWs.close()
    stressWs = null
  }
  window.removeEventListener('resize', handleResize)
  rpsChart?.dispose()
  rpsChart = null
})
</script>

<style scoped lang="scss">
.stress-page {
  padding: 16px 20px;
  max-width: 1100px;
  margin: 0 auto;
}

.stress-header {
  margin-bottom: 16px;
}

.stress-title {
  font-size: 18px;
  font-weight: 700;
  color: var(--t1);
  margin: 0 0 4px;
}

.stress-desc {
  font-size: 12px;
  color: var(--t3);
}

/* 攻击模式 */
.mode-section {
  margin-bottom: 12px;
}

.mode-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
}

.mode-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 14px 8px;
  background: var(--card-bg);
  border: 2px solid var(--border);
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s;
  text-align: center;

  &:hover { border-color: var(--c-blue); }
  &.active {
    border-color: #10b981;
    background: rgba(16, 185, 129, 0.06);
    box-shadow: 0 0 12px rgba(16, 185, 129, 0.15);
  }
}

.mode-icon { font-size: 22px; line-height: 1; }
.mode-name { font-size: 12px; font-weight: 700; color: var(--t1); }
.mode-desc { font-size: 10px; color: var(--t3); line-height: 1.3; }

/* Toggle 开关 */
.toggle-field {
  flex-direction: row !important;
  align-items: center;
  gap: 10px !important;
}

.toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;

  input { display: none; }
}

.toggle-slider {
  width: 36px;
  height: 20px;
  background: var(--border);
  border-radius: 10px;
  position: relative;
  transition: background 0.2s;

  &::after {
    content: '';
    position: absolute;
    top: 2px;
    left: 2px;
    width: 16px;
    height: 16px;
    background: white;
    border-radius: 50%;
    transition: transform 0.2s;
  }
}

.toggle input:checked + .toggle-slider {
  background: #10b981;
  &::after { transform: translateX(16px); }
}

.toggle-text {
  font-size: 12px;
  color: var(--t2);
}

/* 配置区域 */
.config-section {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 16px;
  margin-bottom: 12px;
}

.config-row {
  display: flex;
  gap: 12px;
  margin-bottom: 10px;
  &:last-child { margin-bottom: 0; }
}

.config-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;

  label {
    font-size: 11px;
    color: var(--t3);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.3px;
  }
}

.url-field { flex: 3; }
.method-field { flex: 1; }

.input {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 8px 10px;
  font-size: 13px;
  color: var(--t1);
  outline: none;
  transition: border-color 0.2s;
  font-family: inherit;

  &:focus { border-color: var(--c-blue); }
  &:disabled { opacity: 0.5; cursor: not-allowed; }
}

.textarea {
  resize: vertical;
  font-family: 'Cascadia Code', 'SF Mono', monospace;
  font-size: 12px;
}

select.input {
  cursor: pointer;
  appearance: auto;
}

/* 服务器选择 */
.server-section {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 16px;
  margin-bottom: 12px;
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;

  h3 {
    font-size: 13px;
    font-weight: 600;
    color: var(--t1);
    margin: 0;
  }
}

.server-count {
  font-size: 11px;
  color: var(--t3);
  font-weight: 400;
  margin-left: 6px;
}

.section-actions {
  display: flex;
  gap: 6px;
}

.btn {
  padding: 6px 14px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--card-bg);
  color: var(--t2);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;

  &:hover:not(:disabled) { border-color: var(--c-blue); color: var(--c-blue); }
  &:disabled { opacity: 0.4; cursor: not-allowed; }
}

.btn-sm { padding: 4px 10px; font-size: 11px; }

.btn-primary {
  background: linear-gradient(135deg, #10b981, #059669);
  border-color: transparent;
  color: #fff;
  font-weight: 600;

  &:hover:not(:disabled) { filter: brightness(1.1); color: #fff; border-color: transparent; }
}

.btn-danger {
  background: linear-gradient(135deg, #ef4444, #dc2626);
  border-color: transparent;
  color: #fff;
  font-weight: 600;

  &:hover:not(:disabled) { filter: brightness(1.1); color: #fff; border-color: transparent; }
}

.btn-lg { padding: 10px 24px; font-size: 14px; }

.agent-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 6px;
}

.agent-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
  font-size: 12px;

  &:hover { border-color: var(--c-blue); }
  &.checked { border-color: #10b981; background: rgba(16, 185, 129, 0.05); }
  &.offline { opacity: 0.5; cursor: not-allowed; }

  input[type="checkbox"] { accent-color: #10b981; }
}

.agent-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  &.on { background: #10b981; box-shadow: 0 0 4px rgba(16,185,129,0.5); }
  &.off { background: #64748b; }
}

.agent-name {
  flex: 1;
  color: var(--t1);
  font-weight: 500;
}

.agent-status {
  font-size: 10px;
  color: var(--t3);
}

.no-agents {
  text-align: center;
  padding: 20px;
  color: var(--t3);
  font-size: 13px;
}

/* 操作按钮 */
.action-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  align-items: center;
}

.attack-info {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--t2);
  font-family: 'Cascadia Code', 'SF Mono', monospace;
}

.pulse-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #ef4444;
  animation: pulse 1s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; box-shadow: 0 0 0 0 rgba(239,68,68,0.4); }
  50% { opacity: 0.7; box-shadow: 0 0 0 6px rgba(239,68,68,0); }
}

/* 结果区域 */
.result-section {
  animation: fadeIn 0.3s;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(8px); }
  to { opacity: 1; transform: translateY(0); }
}

.summary-cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
  margin-bottom: 12px;
}

.sum-card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 14px;
  text-align: center;
}

.sum-val {
  font-size: 24px;
  font-weight: 700;
  color: var(--t1);
  line-height: 1;
  font-family: 'Cascadia Code', 'SF Mono', monospace;
}

.sum-label {
  font-size: 10px;
  color: var(--t3);
  margin-top: 6px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.sum-card.success .sum-val { color: #10b981; }
.sum-card.error .sum-val { color: #ef4444; }
.sum-card.rps .sum-val { color: #3b82f6; }
.sum-card.bw-send .sum-val { color: #f59e0b; }
.sum-card.bw-recv .sum-val { color: #8b5cf6; }
.sum-card.bw-total .sum-val { color: #ec4899; }
.sum-card.conn .sum-val { color: #06b6d4; }

.bw-cards { margin-bottom: 12px; }

.chart-section {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 14px 16px;
  margin-bottom: 12px;

  h3 {
    font-size: 12px;
    font-weight: 600;
    color: var(--t2);
    margin: 0 0 8px;
  }
}

.rps-chart {
  width: 100%;
  height: 180px;
}

.agents-detail {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 14px 16px;

  h3 {
    font-size: 12px;
    font-weight: 600;
    color: var(--t2);
    margin: 0 0 8px;
  }
}

.detail-table {
  font-size: 12px;
}

.table-head {
  display: flex;
  padding: 6px 0;
  border-bottom: 1px solid var(--border);
  color: var(--t3);
  font-weight: 600;
  text-transform: uppercase;
  font-size: 10px;
  letter-spacing: 0.3px;
}

.table-row {
  display: flex;
  padding: 8px 0;
  border-bottom: 1px solid rgba(127,127,127,0.06);
  align-items: center;
  color: var(--t1);

  &:last-child { border-bottom: none; }
}

.col-name { flex: 2; }
.col-num { flex: 1; text-align: right; font-family: 'Cascadia Code', 'SF Mono', monospace; }
.col-status { flex: 1; text-align: right; }

.t-green { color: #10b981; }
.t-red { color: #ef4444; }
.t-blue { color: #3b82f6; }
.t-orange { color: #f59e0b; }

.status-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 10px;
  font-size: 10px;
  font-weight: 600;

  &.active { background: rgba(16,185,129,0.1); color: #10b981; }
  &.done { background: rgba(100,116,139,0.1); color: #64748b; }
}

/* 移动端适配 */
@media (max-width: 768px) {
  .stress-page { padding: 10px 12px; }
  .mode-grid { grid-template-columns: repeat(2, 1fr); }
  .config-row { flex-direction: column; gap: 8px; }
  .summary-cards { grid-template-columns: repeat(2, 1fr); }
  .agent-grid { grid-template-columns: repeat(2, 1fr); }
  .action-bar { flex-direction: column; align-items: stretch; }
  .btn-lg { width: 100%; text-align: center; }
  .attack-info { justify-content: center; }
  .table-head, .table-row { font-size: 10px; }
  .col-name { flex: 1.5; }
  .rps-chart { height: 140px; }
}

@media (max-width: 480px) {
  .stress-page { padding: 8px; }
  .summary-cards { grid-template-columns: repeat(2, 1fr); gap: 6px; }
  .sum-val { font-size: 18px; }
  .agent-grid { grid-template-columns: 1fr; }
}
</style>

<style lang="scss">
/* StressTest Light Theme */
html.light .mode-card,
html.light .config-section,
html.light .server-section,
html.light .sum-card,
html.light .chart-section,
html.light .agents-detail {
  background: rgba(255,255,255,0.95);
  box-shadow: 0 1px 6px rgba(0,0,0,0.08);
  border-color: rgba(0,0,0,0.1);
}
</style>
