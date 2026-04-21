import { defineStore } from 'pinia'
import { ref } from 'vue'
import { metricApi, alertApi } from '@/api'
import type { Overview, ServerSummary, AlertCount, RealtimeSeries } from '@/api'

export const useMonitorStore = defineStore('monitor', () => {
  const overview = ref<Overview | null>(null)
  const servers = ref<ServerSummary[]>([])
  const alertCount = ref<AlertCount>({ total: 0, critical: 0, warning: 0, info: 0 })
  const cpuSeries = ref<RealtimeSeries | null>(null)
  const memSeries = ref<RealtimeSeries | null>(null)
  const wsConnected = ref(false)
  const loading = ref(false)
  const seriesLoading = ref(false)
  const seriesError = ref('')
  const latestAlerts = ref<any[]>([])

  let ws: WebSocket | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null

  async function fetchOverview() {
    loading.value = true
    try {
      const res: any = await metricApi.overview()
      if (res.success) {
        overview.value = res.data
        servers.value = res.data.servers || []
      }
    } catch (e) {
      console.error('获取概览失败:', e)
    } finally {
      loading.value = false
    }
  }

  async function fetchAlertCount() {
    try {
      const res: any = await alertApi.count()
      if (res.success) {
        alertCount.value = res.data
      }
    } catch (e) {
      console.error('获取告警计数失败:', e)
    }
  }

  async function fetchRealtimeSeries(metric: string = 'cpu') {
    seriesLoading.value = true
    seriesError.value = ''
    try {
      const res: any = await metricApi.realtime(metric, 30)
      if (res.success) {
        if (metric === 'cpu') cpuSeries.value = res.data
        else if (metric === 'memory') memSeries.value = res.data
      } else {
        console.warn('[趋势] API 返回失败:', metric, res)
        seriesError.value = res.error || '数据加载失败'
      }
    } catch (e: any) {
      console.error('[趋势] 请求异常:', metric, e?.message || e)
      seriesError.value = e?.message?.includes('timeout') ? '请求超时' : '网络错误'
    } finally {
      seriesLoading.value = false
    }
  }

  function connectWebSocket() {
    if (ws && ws.readyState === WebSocket.OPEN) return

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    ws = new WebSocket(`${protocol}//${host}/ws`)

    ws.onopen = () => {
      wsConnected.value = true
      console.log('WebSocket 已连接')
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        handleWsMessage(msg)
      } catch (e) {
        console.error('WebSocket 消息解析失败:', e)
      }
    }

    ws.onclose = () => {
      wsConnected.value = false
      console.log('WebSocket 断开，5秒后重连...')
      reconnectTimer = setTimeout(connectWebSocket, 5000)
    }

    ws.onerror = () => {
      ws?.close()
    }
  }

  function disconnectWebSocket() {
    if (reconnectTimer) clearTimeout(reconnectTimer)
    ws?.close()
    ws = null
  }

  function handleWsMessage(msg: any) {
    switch (msg.type) {
      case 'metrics_update':
        if (msg.data?.servers && Array.isArray(servers.value)) {
          // 更新服务器列表指标
          const newServers = msg.data.servers as any[]
          servers.value = servers.value.map((s) => {
            const updated = newServers.find((ns: any) => ns.serverId === s.id)
            if (updated) {
              return {
                ...s,
                cpuUsage: updated.cpuUsage,
                memUsage: updated.memUsage,
                diskUsage: updated.diskUsage,
                isOnline: updated.isOnline,
                status: getStatus(updated),
              }
            }
            return s
          })
          // 更新概览
          if (msg.data.overview && overview.value) {
            overview.value = {
              ...overview.value,
              ...msg.data.overview,
              servers: servers.value,
            }
          }
        }
        break

      case 'alert':
        latestAlerts.value.unshift(msg.data)
        if (latestAlerts.value.length > 50) {
          latestAlerts.value = latestAlerts.value.slice(0, 50)
        }
        fetchAlertCount()
        break

      case 'ping':
        break
    }
  }

  function getStatus(m: any): string {
    if (!m.isOnline) return 'offline'
    if (m.cpuUsage >= 95 || m.memUsage >= 95) return 'danger'
    if (m.cpuUsage >= 80 || m.memUsage >= 80) return 'warning'
    return 'normal'
  }

  function clearAlertBadge() {
    alertCount.value = { total: 0, critical: 0, warning: 0, info: 0 }
  }

  return {
    overview,
    servers,
    alertCount,
    cpuSeries,
    memSeries,
    wsConnected,
    loading,
    latestAlerts,
    seriesLoading,
    seriesError,
    fetchOverview,
    fetchAlertCount,
    fetchRealtimeSeries,
    connectWebSocket,
    disconnectWebSocket,
    clearAlertBadge,
  }
})
