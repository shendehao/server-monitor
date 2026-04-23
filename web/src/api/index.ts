import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  timeout: 10000,
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

api.interceptors.response.use(
  (res: any) => res.data,
  (err: any) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('token')
      window.location.hash = ''
      window.location.href = '/login'
    }
    return Promise.reject(err)
  }
)

export interface ServerInfo {
  id: string
  name: string
  host: string
  port: number
  username: string
  authType: string
  connectMethod: string
  agentToken: string
  osType: string
  group: string
  sortOrder: number
  isActive: boolean
  createdAt: string
  updatedAt: string
}

export interface ServerSummary {
  id: string
  name: string
  isOnline: boolean
  cpuUsage: number
  memUsage: number
  diskUsage: number
  status: string
  agentVersion?: string
}

export interface Overview {
  serverCount: number
  onlineCount: number
  offlineCount: number
  warningCount: number
  avgCpu: number
  avgMemory: number
  avgDisk: number
  activeAlerts: number
  servers: ServerSummary[]
}

export interface Metric {
  id: number
  serverId: string
  cpuUsage: number
  memTotal: number
  memUsed: number
  memUsage: number
  diskTotal: number
  diskUsed: number
  diskUsage: number
  netIn: number
  netOut: number
  load1m: number
  load5m: number
  load15m: number
  processCount: number
  uptime: string
  collectedAt: string
}

export interface AlertItem {
  id: number
  serverId: string
  serverName: string
  alertType: string
  message: string
  severity: string
  isResolved: boolean
  createdAt: string
  resolvedAt: string | null
}

export interface AlertCount {
  total: number
  critical: number
  warning: number
  info: number
}

export interface DataPoint {
  t: string
  v: number
}

export interface SeriesItem {
  serverId: string
  serverName: string
  color: string
  data: DataPoint[]
}

export interface RealtimeSeries {
  metric: string
  minutes: number
  series: SeriesItem[]
}

export interface ServerDetail extends ServerInfo {
  isOnline: boolean
  uptime: string
  latestMetrics: Metric | null
}

// 服务器 API
export const serverApi = {
  list: () => api.get('/servers'),
  getById: (id: string) => api.get(`/servers/${id}`),
  create: (data: any) => api.post('/servers', data),
  update: (id: string, data: any) => api.put(`/servers/${id}`, data),
  remove: (id: string) => api.delete(`/servers/${id}`),
  test: (id: string) => api.post(`/servers/${id}/test`),
  testNew: (data: any) => api.post('/servers/test', data),
  exec: (id: string, command: string) => api.post(`/servers/${id}/exec`, { command }),
  agentStatus: (id: string) => api.get(`/servers/${id}/agent-status`),
}

// 指标 API
export const metricApi = {
  overview: () => api.get('/metrics/overview'),
  realtime: (metric: string = 'cpu', minutes: number = 30) =>
    api.get('/metrics/realtime', { params: { metric, minutes }, timeout: 30000 }),
  history: (serverId: string, period: string = '1h') =>
    api.get(`/metrics/${serverId}`, { params: { period } }),
}

// 告警 API
export const alertApi = {
  list: (params: any = {}) => api.get('/alerts', { params }),
  count: () => api.get('/alerts/count'),
  resolve: (id: number) => api.put(`/alerts/${id}/resolve`),
  batchResolve: (ids: number[]) => api.put('/alerts/batch-resolve', { ids }),
}

// 告警规则 API
export const alertRuleApi = {
  list: () => api.get('/alert-rules'),
  update: (id: number, data: any) => api.put(`/alert-rules/${id}`, data),
}

// 压力测试 API
export const stressApi = {
  getAgents: () => api.get('/stress/agents'),
  start: (data: any) => api.post('/stress/start', data),
  stop: (id: string) => api.post(`/stress/stop/${id}`),
}

// 认证 API
export const authApi = {
  login: (username: string, password: string) =>
    api.post('/login', { username, password }),
  userInfo: () => api.get('/user/info'),
  changePassword: (oldPassword: string, newPassword: string) =>
    api.put('/user/password', { oldPassword, newPassword }),
}

// 通知推送 API
export const notifyApi = {
  getConfig: () => api.get('/notify/config'),
  updateConfig: (data: any) => api.put('/notify/config', data),
  test: () => api.post('/notify/test'),
}

// Agent 更新 API
export const agentUpdateApi = {
  info: () => api.get('/agent/info'),
  upload: (file: File, platform: string = 'linux') => {
    const form = new FormData()
    form.append('file', file)
    return api.post(`/agent/upload?platform=${platform}`, form, { timeout: 120000, headers: { 'Content-Type': 'multipart/form-data' } })
  },
  pushUpdate: (platform: string = 'linux', serverIds?: string[]) =>
    api.post('/agent/push-update', { serverIds: serverIds || [], platform }),
}

// 系统配置 API
export const systemApi = {
  getConfig: () => api.get('/config'),
}

// 安全管理 API
export const securityApi = {
  getBlacklist: () => api.get('/security/blacklist'),
  addBlacklist: (data: { ip: string; reason: string; duration: number }) =>
    api.post('/security/blacklist', data),
  removeBlacklist: (id: number) => api.delete(`/security/blacklist/${id}`),
  getLogs: () => api.get('/security/logs'),
  getLoginAttempts: () => api.get('/security/login-attempts'),
}

export default api
