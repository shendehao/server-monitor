<template>
  <div
    class="scard"
    :class="[statusClass, { dead: !server.isOnline }]"
    @click="goDetail"
  >
    <div class="scard-top">
      <span class="status-dot" :class="statusClass"></span>
      <span class="scard-name">{{ server.name }}</span>
      <span class="scard-tag" :class="statusClass">{{ statusLabel }}</span>
    </div>

    <div class="scard-body">
      <div class="scard-gauge">
        <div ref="gaugeRef" class="gauge-el"></div>
        <span class="gauge-lbl">CPU</span>
      </div>
      <div class="scard-bars">
        <div class="bar-row">
          <span class="bar-label">MEM</span>
          <div class="bar-track"><div class="bar-fill" :style="barStyle(server.memUsage)"></div></div>
          <span class="bar-val font-num">{{ server.memUsage.toFixed(1) }}%</span>
        </div>
        <div class="bar-row">
          <span class="bar-label">DISK</span>
          <div class="bar-track"><div class="bar-fill" :style="barStyle(server.diskUsage)"></div></div>
          <span class="bar-val font-num">{{ server.diskUsage.toFixed(1) }}%</span>
        </div>
      </div>
    </div>
    <span v-if="server.agentVersion" class="scard-version">v{{ server.agentVersion }}</span>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import * as echarts from 'echarts'
import type { ServerSummary } from '@/api'

const props = defineProps<{ server: ServerSummary }>()
const router = useRouter()
const gaugeRef = ref<HTMLElement>()
let chart: echarts.ECharts | null = null

const statusClass = computed(() => props.server.isOnline ? props.server.status : 'offline')
const statusLabel = computed(() => {
  const m: Record<string, string> = { normal: '正常', warning: '警告', danger: '危险', offline: '离线' }
  return m[statusClass.value] || '—'
})

function goDetail() {
  if (props.server.isOnline) router.push(`/server/${props.server.id}`)
}

function barStyle(v: number) {
  const c = v >= 95 ? 'var(--c-red)' : v >= 80 ? 'var(--c-amber)' : 'var(--c-green)'
  return { width: Math.min(v, 100) + '%', background: c }
}

function initChart() {
  if (!gaugeRef.value) return
  chart = echarts.init(gaugeRef.value)
  updateChart()
}

function isLight() {
  return document.documentElement.classList.contains('light')
}

function updateChart() {
  if (!chart) return
  const val = props.server.cpuUsage
  const light = isLight()
  chart.setOption({
    series: [{
      type: 'gauge',
      radius: '88%',
      center: ['50%', '55%'],
      startAngle: 220,
      endAngle: -40,
      min: 0, max: 100,
      axisLine: {
        lineStyle: {
          width: 8,
          color: light
            ? [[0.8, '#d1fae5'], [0.95, '#fef3c7'], [1, '#fee2e2']]
            : [[0.8, '#1a5c45'], [0.95, '#5c4a1a'], [1, '#5c1a1a']],
        },
      },
      progress: {
        show: true,
        width: 8,
        itemStyle: {
          color: val >= 95 ? (light ? '#dc2626' : '#ef4444') : val >= 80 ? (light ? '#d97706' : '#f59e0b') : (light ? '#059669' : '#10b981'),
        },
      },
      pointer: { show: false },
      axisTick: { show: false },
      splitLine: { show: false },
      axisLabel: { show: false },
      detail: {
        formatter: '{value}%',
        fontSize: 16,
        fontWeight: 600,
        color: light ? '#1e293b' : '#e8edf5',
        fontFamily: "'SF Mono','Courier New',monospace",
        offsetCenter: [0, '65%'],
      },
      data: [{ value: +val.toFixed(1) }],
      animationDuration: 600,
      animationEasingUpdate: 'cubicOut',
    }],
  })
}

watch(() => props.server.cpuUsage, updateChart)

onMounted(() => {
  initChart()
  window.addEventListener('resize', () => chart?.resize())
})
onUnmounted(() => chart?.dispose())
</script>

<style scoped lang="scss">
.scard {
  background: var(--card-bg);
  backdrop-filter: blur(8px);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 14px 16px;
  cursor: pointer;
  position: relative;
  transition: border-color 0.3s, background 0.3s;

  &::before {
    content: '';
    position: absolute;
    inset: -1px;
    border-radius: 11px;
    padding: 1px;
    background: linear-gradient(160deg, rgba(45,124,246,0.15), transparent 40%, transparent 60%, rgba(6,182,212,0.1));
    -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
    -webkit-mask-composite: xor;
    mask-composite: exclude;
    pointer-events: none;
    opacity: 0;
    transition: opacity 0.35s;
  }

  &:hover:not(.dead) {
    background: var(--card-bg-hover);
    &::before { opacity: 1; }
  }

  &.dead { opacity: 0.45; cursor: default; }
  &.warning { border-color: rgba(245,158,11,0.2); }
  &.danger { border-color: rgba(239,68,68,0.2); }
}

.scard-top {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.scard-name {
  flex: 1;
  font-size: 13px;
  font-weight: 600;
  color: var(--t1);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.scard-tag {
  font-size: 10px;
  padding: 1px 7px;
  border-radius: 4px;
  font-weight: 600;
  letter-spacing: 0.5px;
  text-transform: uppercase;

  &.normal { background: rgba(16,185,129,0.12); color: #34d399; }
  &.warning { background: rgba(245,158,11,0.12); color: #fbbf24; }
  &.danger { background: rgba(239,68,68,0.12); color: #f87171; }
  &.offline { background: rgba(61,79,106,0.15); color: var(--t3); }
}

.scard-body {
  display: flex;
  gap: 10px;
  align-items: center;
}

.scard-gauge {
  width: 88px;
  flex-shrink: 0;
  text-align: center;
}

.gauge-el {
  width: 88px;
  height: 72px;
}

.gauge-lbl {
  display: block;
  font-size: 10px;
  color: var(--t3);
  margin-top: -2px;
  letter-spacing: 1px;
  text-transform: uppercase;
}

.scard-bars {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
}

.bar-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.bar-label {
  width: 30px;
  font-size: 10px;
  color: var(--t3);
  letter-spacing: 0.8px;
  flex-shrink: 0;
}

.bar-track {
  flex: 1;
  height: 4px;
  background: rgba(255,255,255,0.04);
  border-radius: 2px;
  overflow: hidden;
}

.bar-fill {
  height: 100%;
  border-radius: 2px;
  transition: width 0.5s cubic-bezier(0.16,1,0.3,1), background 0.3s;
}

.bar-val {
  width: 42px;
  text-align: right;
  font-size: 11px;
  font-weight: 600;
  color: var(--t2);
  flex-shrink: 0;
}

.scard-version {
  position: absolute;
  bottom: 6px;
  right: 10px;
  font-size: 10px;
  color: #64748b;
  opacity: 0.7;
  font-family: 'SF Mono', 'Courier New', monospace;
}

@media (max-width: 768px) {
  .scard { padding: 10px 12px; }
  .scard-top { margin-bottom: 8px; }
  .scard-name { font-size: 12px; }
  .scard-gauge { width: 70px; }
  .gauge-el { width: 70px; height: 58px; }
  .bar-label { width: 26px; font-size: 9px; }
  .bar-val { width: 38px; font-size: 10px; }
  .scard-version { font-size: 9px; bottom: 4px; right: 8px; }
}

@media (max-width: 480px) {
  .scard { padding: 10px; }
  .scard-body { gap: 8px; }
  .scard-gauge { width: 60px; }
  .gauge-el { width: 60px; height: 50px; }
  .gauge-lbl { font-size: 9px; }
  .scard-bars { gap: 4px; }
  .bar-row { gap: 4px; }
  .bar-label { width: 24px; font-size: 8px; }
  .bar-val { width: 34px; font-size: 9px; }
}
</style>

<style lang="scss">
/* ===== ServerCard Light Theme ===== */
html.light .scard {
  background: rgba(255,255,255,0.95);
  border-color: rgba(0,0,0,0.12);
  box-shadow: 0 1px 6px rgba(0,0,0,0.08);
  &:hover:not(.dead) {
    background: #ffffff;
    box-shadow: 0 4px 16px rgba(0,0,0,0.12);
  }
  &.warning { border-color: rgba(217,119,6,0.3); }
  &.danger { border-color: rgba(220,38,38,0.3); }
}

html.light .scard-tag {
  &.normal { background: rgba(5,150,105,0.1); color: #047857; }
  &.warning { background: rgba(217,119,6,0.1); color: #b45309; }
  &.danger { background: rgba(220,38,38,0.1); color: #dc2626; }
  &.offline { background: rgba(148,163,184,0.15); color: #64748b; }
}

html.light .bar-track {
  background: rgba(0,0,0,0.1);
}
</style>
