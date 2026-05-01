import { reactive } from 'vue'

// IP → 国家代码缓存（内存 + localStorage）
const cache = reactive<Record<string, string>>({})
const LS_KEY = 'ip_country_cache'
const pending = new Set<string>()

// 初始化：从 localStorage 恢复缓存
try {
  const saved = localStorage.getItem(LS_KEY)
  if (saved) Object.assign(cache, JSON.parse(saved))
} catch {}

function saveCache() {
  try { localStorage.setItem(LS_KEY, JSON.stringify(cache)) } catch {}
}

// 国家代码 → 国旗 emoji（Regional Indicator Symbol）
function codeToFlag(cc: string): string {
  if (!cc || cc.length !== 2) return ''
  const base = 0x1F1E6 - 65
  return String.fromCodePoint(cc.charCodeAt(0) + base, cc.charCodeAt(1) + base)
}

// 批量查询 IP 的国家（用免费 API）
let batchTimer: ReturnType<typeof setTimeout> | null = null
const batchQueue: string[] = []

function flushBatch() {
  batchTimer = null
  const ips = [...new Set(batchQueue)]
  batchQueue.length = 0
  // ip-api.com 支持批量查询（最多 100 个）
  const toQuery = ips.filter(ip => !cache[ip] && !pending.has(ip))
  if (toQuery.length === 0) return

  toQuery.forEach(ip => pending.add(ip))

  // 批量 API
  fetch('http://ip-api.com/batch?fields=query,countryCode', {
    method: 'POST',
    body: JSON.stringify(toQuery.map(ip => ({ query: ip }))),
  })
    .then(r => r.json())
    .then((results: any[]) => {
      for (const r of results) {
        if (r.countryCode && r.query) {
          cache[r.query] = r.countryCode
        }
        pending.delete(r.query)
      }
      saveCache()
    })
    .catch(() => {
      toQuery.forEach(ip => pending.delete(ip))
    })
}

export function lookupIp(ip: string) {
  if (!ip) return
  if (cache[ip]) return
  batchQueue.push(ip)
  if (!batchTimer) batchTimer = setTimeout(flushBatch, 300)
}

export function getFlag(ip: string): string {
  const cc = cache[ip]
  return cc ? codeToFlag(cc) : ''
}

export function useIpFlag() {
  return { cache, lookupIp, getFlag }
}
