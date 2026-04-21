<template>
  <div class="alerts-page">
    <!-- 顶部统计卡片 -->
    <div class="stats-grid">
      <div class="stat-card" :class="{ highlight: alertCount.critical > 0 }">
        <div class="stat-icon critical">
          <el-icon><CircleCloseFilled /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-label">严重告警</div>
          <div class="stat-value font-num">{{ alertCount.critical }}</div>
        </div>
      </div>

      <div class="stat-card" :class="{ highlight: alertCount.warning > 0 }">
        <div class="stat-icon warning">
          <el-icon><WarningFilled /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-label">警告</div>
          <div class="stat-value font-num">{{ alertCount.warning }}</div>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon info">
          <el-icon><InfoFilled /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-label">信息</div>
          <div class="stat-value font-num">{{ alertCount.info }}</div>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon total">
          <el-icon><Bell /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-label">总告警数</div>
          <div class="stat-value font-num">{{ alertCount.total }}</div>
        </div>
      </div>
    </div>

    <!-- 工具栏 -->
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>告警中心</h2>
        <span class="alert-count" v-if="total > 0">共 {{ total }} 条</span>
      </div>
      <div class="toolbar-right">
        <el-radio-group v-model="viewMode" size="small">
          <el-radio-button value="list">列表</el-radio-button>
          <el-radio-button value="timeline">时间轴</el-radio-button>
        </el-radio-group>
        <el-select v-model="filters.severity" placeholder="严重级别" clearable size="small" style="width:120px" @change="fetchAlerts">
          <el-option label="严重" value="critical">
            <span class="sev-dot critical" />严重
          </el-option>
          <el-option label="警告" value="warning">
            <span class="sev-dot warning" />警告
          </el-option>
          <el-option label="信息" value="info">
            <span class="sev-dot info" />信息
          </el-option>
        </el-select>
        <el-select v-model="filters.resolved" placeholder="状态" clearable size="small" style="width:110px" @change="fetchAlerts">
          <el-option label="未解决" value="false" />
          <el-option label="已解决" value="true" />
        </el-select>
        <el-button size="small" @click="fetchAlerts" :loading="loading">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
        <el-button size="small" type="primary" v-if="hasUnresolved" @click="resolveAll">
          <el-icon><Check /></el-icon>
          全部标记已解决
        </el-button>
      </div>
    </div>

    <!-- 列表视图 -->
    <div v-if="viewMode === 'list'" class="alerts-list" v-loading="loading">
      <div v-if="!loading && alerts.length === 0" class="empty-state">
        <el-icon><Bell /></el-icon>
        <p>当前没有告警记录</p>
        <span>系统运行正常</span>
      </div>
      <div
        v-for="alert in alerts"
        :key="alert.id"
        class="alert-item"
        :class="[alert.severity, { resolved: alert.isResolved }]"
      >
        <div class="alert-severity-icon" :class="alert.severity">
          <el-icon v-if="alert.severity === 'critical'"><CircleCloseFilled /></el-icon>
          <el-icon v-else-if="alert.severity === 'warning'"><WarningFilled /></el-icon>
          <el-icon v-else><InfoFilled /></el-icon>
        </div>
        <div class="alert-body">
          <div class="alert-top">
            <span class="severity-tag" :class="alert.severity">
              {{ severityMap[alert.severity] || alert.severity }}
            </span>
            <span class="alert-server">
              <el-icon><Monitor /></el-icon>
              {{ alert.serverName }}
            </span>
            <span class="alert-time">{{ formatTime(alert.createdAt) }}</span>
            <span v-if="alert.isResolved" class="status-badge resolved">
              <el-icon><CircleCheckFilled /></el-icon>
              已解决
            </span>
            <span v-else class="status-badge pending">
              <span class="pulse-dot" />
              未解决
            </span>
          </div>
          <div class="alert-message">{{ alert.message }}</div>
        </div>
        <div class="alert-actions">
          <el-button
            v-if="!alert.isResolved"
            type="primary"
            size="small"
            plain
            @click="resolveAlert(alert.id)"
          >
            <el-icon><Check /></el-icon>
            标记解决
          </el-button>
        </div>
      </div>
    </div>

    <!-- 时间轴视图 -->
    <div v-else class="timeline" v-loading="loading">
      <div v-if="!loading && alerts.length === 0" class="empty-state">
        <el-icon><Bell /></el-icon>
        <p>当前没有告警记录</p>
        <span>系统运行正常</span>
      </div>
      <div v-for="(group, date) in groupedAlerts" :key="date" class="timeline-group">
        <div class="timeline-date">{{ date }}</div>
        <div class="timeline-line">
          <div
            v-for="alert in group"
            :key="alert.id"
            class="timeline-item"
            :class="[alert.severity, { resolved: alert.isResolved }]"
          >
            <div class="timeline-dot" :class="alert.severity" />
            <div class="timeline-content">
              <div class="timeline-header">
                <span class="severity-tag" :class="alert.severity">{{ severityMap[alert.severity] }}</span>
                <span class="timeline-server">{{ alert.serverName }}</span>
                <span class="timeline-time">{{ formatHour(alert.createdAt) }}</span>
              </div>
              <div class="timeline-message">{{ alert.message }}</div>
              <div class="timeline-footer">
                <span v-if="alert.isResolved" class="status-badge resolved">
                  <el-icon><CircleCheckFilled /></el-icon>
                  已解决
                </span>
                <span v-else class="status-badge pending">
                  <span class="pulse-dot" />
                  未解决
                </span>
                <el-button
                  v-if="!alert.isResolved"
                  type="primary"
                  size="small"
                  text
                  @click="resolveAlert(alert.id)"
                >标记解决</el-button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="pagination" v-if="total > pageSize">
      <el-pagination
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next, total"
        @current-change="fetchAlerts"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { alertApi } from '@/api'
import type { AlertItem, AlertCount } from '@/api'
import { useMonitorStore } from '@/stores/monitor'
import {
  CircleCloseFilled, WarningFilled, InfoFilled, Bell,
  Monitor, CircleCheckFilled, Check, Refresh,
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

const monitorStore = useMonitorStore()

const loading = ref(false)
const alerts = ref<AlertItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const viewMode = ref<'list' | 'timeline'>('list')
const alertCount = ref<AlertCount>({ total: 0, critical: 0, warning: 0, info: 0 })
const filters = reactive({ severity: '', resolved: '' })

const severityMap: Record<string, string> = {
  critical: '严重',
  warning: '警告',
  info: '信息',
}

const hasUnresolved = computed(() => alerts.value.some(a => !a.isResolved))

const groupedAlerts = computed(() => {
  const map: Record<string, AlertItem[]> = {}
  for (const a of alerts.value) {
    const d = new Date(a.createdAt).toLocaleDateString('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' })
    if (!map[d]) map[d] = []
    map[d].push(a)
  }
  return map
})

async function fetchAlerts() {
  loading.value = true
  try {
    const res: any = await alertApi.list({
      page: page.value,
      page_size: pageSize,
      severity: filters.severity || undefined,
      resolved: filters.resolved || undefined,
    })
    if (res.success) {
      alerts.value = res.data.list || []
      total.value = res.data.total
    }
  } finally {
    loading.value = false
  }
}

async function fetchCount() {
  const res: any = await alertApi.count()
  if (res.success) alertCount.value = res.data
}

async function resolveAlert(id: number) {
  await alertApi.resolve(id)
  ElMessage.success('已标记为解决')
  fetchAlerts()
  fetchCount()
}

async function resolveAll() {
  try {
    await ElMessageBox.confirm('确认将当前所有未解决的告警标记为已解决？', '批量操作', { type: 'warning' })
    const unresolvedIds = alerts.value.filter(a => !a.isResolved).map(a => a.id)
    for (const id of unresolvedIds) {
      await alertApi.resolve(id)
    }
    ElMessage.success(`已解决 ${unresolvedIds.length} 条告警`)
    fetchAlerts()
    fetchCount()
  } catch { /* cancelled */ }
}

function formatTime(t: string) {
  if (!t) return ''
  const d = new Date(t)
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return `${Math.floor(diff / 60000)} 分钟前`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)} 小时前`
  return d.toLocaleString('zh-CN', { hour12: false })
}

function formatHour(t: string) {
  if (!t) return ''
  return new Date(t).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false })
}

onMounted(() => {
  monitorStore.clearAlertBadge()
  fetchAlerts()
  fetchCount()
})
</script>

<style scoped lang="scss">
.alerts-page {
  padding: 20px 24px;
  max-width: 1300px;
  margin: 0 auto;
}

/* ===== 统计卡片 ===== */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 14px;
  margin-bottom: 20px;
}

.stat-card {
  position: relative;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 18px 20px;
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 12px;
  overflow: hidden;
  transition: all .2s;

  &:hover {
    transform: translateY(-2px);
    box-shadow: 0 8px 24px rgba(0,0,0,0.25);
  }

  &.highlight {
    animation: cardPulse 2s ease-in-out infinite;
  }
}

@keyframes cardPulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(239, 68, 68, 0); }
  50% { box-shadow: 0 0 0 4px rgba(239, 68, 68, 0.12); }
}

.stat-icon {
  width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  font-size: 22px;

  &.critical { background: rgba(239, 68, 68, 0.15); color: #f87171; }
  &.warning { background: rgba(245, 158, 11, 0.15); color: #fbbf24; }
  &.info { background: rgba(139, 92, 246, 0.15); color: #a78bfa; }
  &.total { background: rgba(14, 165, 233, 0.15); color: #38bdf8; }
}

.stat-content {
  flex: 1;
  min-width: 0;
}

.stat-label {
  font-size: 12px;
  color: var(--t3);
  margin-bottom: 4px;
}

.stat-value {
  font-size: 26px;
  font-weight: 700;
  line-height: 1;
  color: var(--t1);
}

/* ===== 工具栏 ===== */
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.toolbar-left {
  display: flex;
  align-items: baseline;
  gap: 10px;

  h2 {
    font-size: 16px;
    font-weight: 600;
    margin: 0;
  }
}

.alert-count {
  font-size: 12px;
  color: var(--t3);
  padding: 2px 8px;
  background: rgba(255,255,255,0.05);
  border-radius: 10px;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.sev-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 6px;

  &.critical { background: #f87171; }
  &.warning { background: #fbbf24; }
  &.info { background: #a78bfa; }
}

/* ===== 列表视图 ===== */
.alerts-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 200px;
}

.alert-item {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  padding: 16px 18px;
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  transition: all .2s;

  &:hover {
    border-color: rgba(255,255,255,0.15);
    transform: translateX(2px);
  }

  &.critical { border-left: 3px solid #ef4444; }
  &.warning { border-left: 3px solid #f59e0b; }
  &.info { border-left: 3px solid #8b5cf6; }

  &.resolved {
    opacity: 0.6;
    border-left-color: #10b981 !important;
  }
}

.alert-severity-icon {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  font-size: 20px;
  flex-shrink: 0;

  &.critical { background: rgba(239, 68, 68, 0.12); color: #f87171; }
  &.warning { background: rgba(245, 158, 11, 0.12); color: #fbbf24; }
  &.info { background: rgba(139, 92, 246, 0.12); color: #a78bfa; }
}

.alert-body {
  flex: 1;
  min-width: 0;
}

.alert-top {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 6px;
  flex-wrap: wrap;
}

.severity-tag {
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;

  &.critical { background: rgba(239, 68, 68, 0.15); color: #f87171; }
  &.warning { background: rgba(245, 158, 11, 0.15); color: #fbbf24; }
  &.info { background: rgba(139, 92, 246, 0.15); color: #a78bfa; }
}

.alert-server {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  font-weight: 500;
  color: var(--t2);

  .el-icon { font-size: 13px; }
}

.alert-time {
  font-size: 12px;
  color: var(--t3);
}

.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 500;
  border-radius: 10px;

  &.resolved {
    background: rgba(16, 185, 129, 0.12);
    color: #34d399;
  }
  &.pending {
    background: rgba(239, 68, 68, 0.12);
    color: #f87171;
  }

  .el-icon { font-size: 12px; }
}

.pulse-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
  animation: pulse 1.5s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.5; transform: scale(1.3); }
}

.alert-message {
  font-size: 13px;
  color: var(--t1);
  line-height: 1.5;
  word-break: break-word;
}

.alert-actions {
  flex-shrink: 0;
}

/* ===== 时间轴视图 ===== */
.timeline {
  padding: 0 10px;
}

.timeline-group {
  margin-bottom: 28px;
}

.timeline-date {
  display: inline-block;
  padding: 4px 12px;
  margin-bottom: 14px;
  font-size: 12px;
  font-weight: 600;
  color: var(--t1);
  background: rgba(255,255,255,0.06);
  border-radius: 12px;
}

.timeline-line {
  position: relative;
  padding-left: 28px;

  &::before {
    content: '';
    position: absolute;
    left: 10px;
    top: 6px;
    bottom: 6px;
    width: 1px;
    background: linear-gradient(180deg, var(--border), transparent);
  }
}

.timeline-item {
  position: relative;
  margin-bottom: 14px;
  padding: 12px 14px;
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  transition: all .2s;

  &:hover {
    transform: translateX(3px);
    border-color: rgba(255,255,255,0.15);
  }

  &.resolved { opacity: 0.6; }
}

.timeline-dot {
  position: absolute;
  left: -23px;
  top: 14px;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  box-shadow: 0 0 0 3px var(--bg);

  &.critical { background: #f87171; box-shadow: 0 0 0 3px var(--bg), 0 0 0 5px rgba(239,68,68,0.3); }
  &.warning { background: #fbbf24; box-shadow: 0 0 0 3px var(--bg), 0 0 0 5px rgba(245,158,11,0.3); }
  &.info { background: #a78bfa; box-shadow: 0 0 0 3px var(--bg), 0 0 0 5px rgba(139,92,246,0.3); }
}

.timeline-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 6px;
  flex-wrap: wrap;
}

.timeline-server {
  font-size: 12px;
  font-weight: 500;
  color: var(--t2);
}

.timeline-time {
  margin-left: auto;
  font-size: 11px;
  color: var(--t3);
  font-family: 'SF Mono', 'Courier New', monospace;
}

.timeline-message {
  font-size: 13px;
  color: var(--t1);
  line-height: 1.5;
  margin-bottom: 6px;
}

.timeline-footer {
  display: flex;
  align-items: center;
  gap: 10px;
}

/* ===== 空状态 ===== */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80px 20px;
  color: var(--t3);

  .el-icon {
    font-size: 56px;
    margin-bottom: 16px;
    opacity: 0.3;
  }

  p {
    margin: 0 0 6px;
    font-size: 15px;
    font-weight: 500;
    color: var(--t2);
  }

  span {
    font-size: 12px;
    color: var(--t3);
  }
}

.pagination {
  display: flex;
  justify-content: center;
  margin-top: 20px;
}

/* ===== 移动端适配 ===== */
@media (max-width: 900px) {
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
  .toolbar { flex-direction: column; align-items: stretch; }
  .toolbar-right { justify-content: flex-end; }
}

@media (max-width: 500px) {
  .alerts-page { padding: 12px; }
  .stats-grid { grid-template-columns: 1fr 1fr; gap: 8px; }
  .stat-card { padding: 12px; }
  .stat-icon { width: 36px; height: 36px; font-size: 18px; }
  .stat-value { font-size: 20px; }
  .alert-item { padding: 12px; }
  .alert-severity-icon { width: 30px; height: 30px; font-size: 16px; }
  .alert-top { gap: 6px; }
}
</style>

<style lang="scss">
/* Alerts Light Theme */
html.light .alerts-page .stat-card {
  background: rgba(255,255,255,0.95);
  box-shadow: 0 1px 6px rgba(0,0,0,0.06);
  border-color: rgba(0,0,0,0.08);

  &:hover { box-shadow: 0 8px 24px rgba(0,0,0,0.1); }
}
html.light .alerts-page .alert-item {
  background: rgba(255,255,255,0.95);
  box-shadow: 0 1px 4px rgba(0,0,0,0.04);
  border-color: rgba(0,0,0,0.08);

  &:hover { border-color: rgba(0,0,0,0.12); }
}
html.light .alerts-page .timeline-item {
  background: rgba(255,255,255,0.95);
  box-shadow: 0 1px 4px rgba(0,0,0,0.04);
  border-color: rgba(0,0,0,0.08);
}
html.light .alerts-page .timeline-date {
  background: rgba(0,0,0,0.05);
}
html.light .alerts-page .alert-count {
  background: rgba(0,0,0,0.05);
}
html.light .alerts-page .severity-tag {
  &.critical { background: rgba(220,38,38,0.1); color: #dc2626; }
  &.warning { background: rgba(217,119,6,0.1); color: #b45309; }
  &.info { background: rgba(124,58,237,0.1); color: #7c3aed; }
}
html.light .alerts-page .stat-icon {
  &.critical { background: rgba(220,38,38,0.1); color: #dc2626; }
  &.warning { background: rgba(217,119,6,0.1); color: #d97706; }
  &.info { background: rgba(124,58,237,0.1); color: #7c3aed; }
  &.total { background: rgba(2,132,199,0.1); color: #0284c7; }
}
html.light .alerts-page .alert-severity-icon {
  &.critical { background: rgba(220,38,38,0.1); color: #dc2626; }
  &.warning { background: rgba(217,119,6,0.1); color: #d97706; }
  &.info { background: rgba(124,58,237,0.1); color: #7c3aed; }
}
html.light .alerts-page .status-badge {
  &.resolved { background: rgba(5,150,105,0.1); color: #059669; }
  &.pending { background: rgba(220,38,38,0.1); color: #dc2626; }
}
</style>
