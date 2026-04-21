<template>
  <div class="tchart">
    <div class="tchart-hd">
      <span class="tchart-title">{{ title }}</span>
      <span v-if="loading" class="tchart-status">加载中...</span>
      <span v-else-if="error" class="tchart-status tchart-err">{{ error }}</span>
    </div>
    <div ref="chartRef" class="tchart-canvas"></div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import * as echarts from 'echarts'
import type { RealtimeSeries } from '@/api'

const props = defineProps<{
  title: string
  series: RealtimeSeries | null
  unit?: string
  loading?: boolean
  error?: string
}>()

const chartRef = ref<HTMLElement>()
let chart: echarts.ECharts | null = null
let resizeObserver: ResizeObserver | null = null
let isFirstRender = true
let userSelected: Record<string, boolean> | null = null

function initChart() {
  if (!chartRef.value) return
  if (chart) chart.dispose()
  chart = echarts.init(chartRef.value)
  isFirstRender = true
  userSelected = null

  // 监听用户手动切换图例
  chart.on('legendselectchanged', (params: any) => {
    userSelected = { ...params.selected }
  })
  chart.on('legendselectall', () => { userSelected = null })
  chart.on('legendinverseselect', () => { userSelected = null })

  updateChart()
}

function isLight() {
  return document.documentElement.classList.contains('light')
}

// 16 色高对比度配色
const palette = [
  '#3b82f6', '#10b981', '#f59e0b', '#ef4444',
  '#8b5cf6', '#06b6d4', '#ec4899', '#f97316',
  '#14b8a6', '#6366f1', '#84cc16', '#e11d48',
  '#0ea5e9', '#a855f7', '#22c55e', '#eab308',
]

const MAX_DEFAULT_SHOW = 3

function updateChart() {
  if (!chart || !props.series) return
  const light = isLight()
  const seriesData = props.series.series || []

  // 只保留有数据的服务器
  const validSeries = seriesData.filter((s: any) => Array.isArray(s.data) && s.data.length > 0)

  // 空数据状态
  if (validSeries.length === 0) {
    chart.setOption({
      graphic: {
        type: 'text',
        left: 'center', top: 'middle',
        style: { text: '暂无数据', fill: light ? '#94a3b8' : '#3d4f6a', fontSize: 13 },
      },
      xAxis: { show: false }, yAxis: { show: false }, series: [], legend: { show: false },
    }, true)
    return
  }

  // 按最新值排序
  const sorted = [...validSeries].sort((a: any, b: any) => {
    const aLast = a.data?.[a.data.length - 1]?.v ?? 0
    const bLast = b.data?.[b.data.length - 1]?.v ?? 0
    return bLast - aLast
  })

  // 首次渲染：默认只显示前 N 台；后续刷新：保留用户的图例选择
  let legendSelected: Record<string, boolean> | undefined
  if (isFirstRender) {
    legendSelected = {}
    sorted.forEach((s: any, i: number) => {
      legendSelected![s.serverName] = i < MAX_DEFAULT_SHOW
    })
    isFirstRender = false
  } else if (userSelected) {
    legendSelected = userSelected
  }

  const sd = sorted.map((s: any, i: number) => {
    const color = palette[i % palette.length]
    return {
      name: s.serverName,
      type: 'line' as const,
      smooth: 0.4,
      symbol: 'none',
      sampling: 'lttb',
      lineStyle: { width: 1.5, color },
      itemStyle: { color },
      emphasis: { focus: 'series' as const, lineStyle: { width: 2.5 } },
      blur: { lineStyle: { width: 0.5, opacity: 0.1 } },
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: color + '18' },
          { offset: 1, color: color + '02' },
        ]),
      },
      data: (s.data || []).map((d: any) => [new Date(d.t).getTime(), d.v]),
    }
  })

  const option: any = {
    graphic: [],
    tooltip: {
      trigger: 'axis',
      backgroundColor: light ? 'rgba(255,255,255,0.96)' : 'rgba(15,19,32,0.96)',
      borderColor: light ? 'rgba(0,0,0,0.06)' : 'rgba(255,255,255,0.06)',
      textStyle: { color: light ? '#1e293b' : '#e8edf5', fontSize: 11 },
      extraCssText: 'backdrop-filter:blur(12px);box-shadow:0 4px 20px rgba(0,0,0,0.15);border-radius:8px;padding:8px 12px;max-height:220px;overflow-y:auto;',
      axisPointer: {
        lineStyle: { color: light ? 'rgba(0,0,0,0.08)' : 'rgba(255,255,255,0.06)', type: 'dashed' },
      },
      formatter: (params: any) => {
        if (!Array.isArray(params) || !params.length) return ''
        const t = new Date(params[0].data[0]).toLocaleTimeString('zh-CN', { hour12: false })
        const sub = light ? '#64748b' : '#7a8ba8'
        const txt = light ? '#1e293b' : '#e8edf5'
        let h = `<div style="color:${sub};font-size:10px;margin-bottom:4px;font-weight:500">${t}</div>`
        const items = [...params].sort((a: any, b: any) => b.data[1] - a.data[1])
        items.forEach((p: any) => {
          h += `<div style="display:flex;align-items:center;gap:5px;margin:1px 0;min-width:140px"><span style="width:8px;height:2px;border-radius:1px;background:${p.color};flex-shrink:0"></span><span style="font-size:10px;color:${txt};flex:1;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;max-width:100px">${p.seriesName}</span><span style="font-family:monospace;font-weight:600;font-size:10px;color:${txt}">${p.data[1].toFixed(1)}%</span></div>`
        })
        return h
      },
    },
    legend: {
      show: true,
      type: 'scroll',
      bottom: 0,
      textStyle: { color: light ? '#64748b' : '#7a8ba8', fontSize: 10 },
      icon: 'roundRect',
      itemWidth: 14,
      itemHeight: 3,
      itemGap: 10,
      pageIconColor: light ? '#64748b' : '#7a8ba8',
      pageIconInactiveColor: light ? '#cbd5e1' : '#2a3548',
      pageTextStyle: { color: light ? '#64748b' : '#7a8ba8', fontSize: 10 },
      selector: [
        { type: 'all' as const, title: '全选' },
        { type: 'inverse' as const, title: '反选' },
      ],
      selectorLabel: { fontSize: 10, color: light ? '#64748b' : '#7a8ba8' },
    },
    grid: { left: 40, right: 12, top: 8, bottom: 48 },
    xAxis: {
      type: 'time',
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: { color: light ? '#94a3b8' : '#3d4f6a', fontSize: 10 },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value', min: 0, max: 100,
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: { color: light ? '#94a3b8' : '#3d4f6a', fontSize: 10, formatter: '{value}%' },
      splitLine: { lineStyle: { color: light ? 'rgba(0,0,0,0.04)' : 'rgba(255,255,255,0.03)' } },
    },
    series: sd,
    animation: true,
    animationDuration: 400,
    animationDurationUpdate: 250,
    animationEasingUpdate: 'cubicOut',
  }

  // 只在首次渲染时设置 legend.selected，后续用 merge 模式保留用户选择
  if (legendSelected) {
    option.legend.selected = legendSelected
  }

  chart.setOption(option, isFirstRender)
}

watch(() => props.series, () => {
  if (!chart && chartRef.value) initChart()
  else updateChart()
}, { deep: true })

onMounted(() => {
  nextTick(() => {
    initChart()
    if (chartRef.value) {
      resizeObserver = new ResizeObserver(() => chart?.resize())
      resizeObserver.observe(chartRef.value)
    }
  })
})

onUnmounted(() => {
  resizeObserver?.disconnect()
  chart?.dispose()
})
</script>

<style scoped lang="scss">
.tchart {
  background: var(--card-bg);
  backdrop-filter: blur(8px);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 14px 16px;
  flex: 1;
  min-width: 0;
}

.tchart-hd {
  margin-bottom: 6px;
}

.tchart-title {
  font-size: 11px;
  font-weight: 600;
  color: var(--t3);
  letter-spacing: 0.5px;
  text-transform: uppercase;
}

.tchart-hd {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tchart-status {
  font-size: 10px;
  color: var(--t3);
  opacity: 0.7;
}

.tchart-err {
  color: #ef4444;
  opacity: 1;
}

.tchart-canvas {
  width: 100%;
  height: 240px;
}

@media (max-width: 768px) {
  .tchart { padding: 10px 12px; border-radius: 8px; }
  .tchart-canvas { height: 160px; }
  .tchart-title { font-size: 10px; }
}
</style>

<style lang="scss">
/* TrendChart Light Theme */
html.light .tchart {
  background: rgba(255,255,255,0.95);
  box-shadow: 0 1px 6px rgba(0,0,0,0.08);
  border-color: rgba(0,0,0,0.12);
}
</style>
