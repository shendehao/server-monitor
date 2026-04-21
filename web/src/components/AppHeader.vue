<template>
  <header class="hdr">
    <div class="hdr-accent"></div>
    <div class="hdr-inner">
      <div class="hdr-brand">
        <svg class="hdr-logo" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
          <rect x="2" y="4" width="20" height="14" rx="2"/>
          <path d="M7 11h2m3 0h2m-7 3h1m3 0h1m3 0h1" stroke-linecap="round"/>
          <circle cx="12" cy="8" r="1.5"/>
          <path d="M8 22h8M12 18v4"/>
        </svg>
        <span class="hdr-name">SERVER<span class="hdr-name-dim">MONITOR</span></span>
      </div>

      <nav class="hdr-nav">
        <router-link
          v-for="item in navItems"
          :key="item.path"
          :to="item.path"
          class="nav-link"
          :class="{ current: isActive(item.path) }"
        >
          {{ item.label }}
          <span v-if="item.badge && item.badge > 0" class="nav-badge">{{ item.badge }}</span>
        </router-link>
      </nav>

      <div class="hdr-end">
        <div class="hdr-indicator">
          <span class="status-dot" :class="store.wsConnected ? 'online' : 'offline'"></span>
          <span class="ind-label font-num">{{ store.wsConnected ? 'LIVE' : 'OFF' }}</span>
        </div>
        <span class="hdr-time font-num">{{ time }}</span>
        <button class="hdr-theme" @click="toggleTheme" :title="themeRef === 'dark' ? '切换亮色' : '切换暗色'">
          <svg v-if="themeRef === 'dark'" viewBox="0 0 20 20" fill="currentColor"><path d="M10 2a1 1 0 011 1v1a1 1 0 11-2 0V3a1 1 0 011-1zm4 8a4 4 0 11-8 0 4 4 0 018 0zm-.464 4.95l.707.707a1 1 0 001.414-1.414l-.707-.707a1 1 0 00-1.414 1.414zm2.12-10.607a1 1 0 010 1.414l-.706.707a1 1 0 11-1.414-1.414l.707-.707a1 1 0 011.414 0zM17 11a1 1 0 100-2h-1a1 1 0 100 2h1zm-7 4a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM5.05 6.464A1 1 0 106.465 5.05l-.708-.707a1 1 0 00-1.414 1.414l.707.707zm1.414 8.486l-.707.707a1 1 0 01-1.414-1.414l.707-.707a1 1 0 011.414 1.414zM4 11a1 1 0 100-2H3a1 1 0 000 2h1z"/></svg>
          <svg v-else viewBox="0 0 20 20" fill="currentColor"><path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z"/></svg>
        </button>
        <button class="hdr-logout" @click="logout" title="退出登录">
          <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round">
            <path d="M13 3h3a1 1 0 011 1v12a1 1 0 01-1 1h-3M8 15l5-5-5-5M13 10H3"/>
          </svg>
        </button>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMonitorStore } from '@/stores/monitor'
import { useTheme } from '@/composables/useTheme'

const route = useRoute()
const router = useRouter()
const store = useMonitorStore()
const { theme: themeRef, toggle: toggleTheme } = useTheme()
const time = ref('')
let timer: ReturnType<typeof setInterval>

const navItems = computed(() => [
  { path: '/', label: '总览', badge: 0 },
  { path: '/alerts', label: '告警', badge: store.alertCount.total },
  { path: '/stress', label: '压测', badge: 0 },
  { path: '/settings', label: '设置', badge: 0 },
])

function isActive(path: string) {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}

function updateTime() {
  time.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
}

function logout() {
  localStorage.removeItem('token')
  localStorage.removeItem('username')
  store.disconnectWebSocket()
  router.push('/login')
}

onMounted(() => { updateTime(); timer = setInterval(updateTime, 1000) })
onUnmounted(() => clearInterval(timer))
</script>

<style scoped lang="scss">
.hdr {
  position: sticky;
  top: 0;
  z-index: 100;
  background: rgba(6, 10, 22, 0.82);
  backdrop-filter: blur(16px) saturate(1.4);
  transition: background 0.3s;
}

html.light .hdr {
  background: rgba(255, 255, 255, 0.95);
  box-shadow: 0 1px 6px rgba(0,0,0,0.1);
  border-bottom: 1px solid rgba(0,0,0,0.08);
}

.hdr-accent {
  height: 1px;
  background: linear-gradient(
    90deg,
    transparent 0%,
    rgba(45, 124, 246, 0.4) 20%,
    rgba(6, 182, 212, 0.3) 50%,
    rgba(45, 124, 246, 0.4) 80%,
    transparent 100%
  );
}

.hdr-inner {
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
}

.hdr-brand {
  display: flex;
  align-items: center;
  gap: 10px;
}

.hdr-logo {
  width: 20px;
  height: 20px;
  color: var(--c-blue);
  opacity: 0.8;
}

.hdr-name {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 2.5px;
  color: var(--t1);
  font-family: 'SF Mono', 'Courier New', monospace;
}

.hdr-name-dim {
  color: var(--t3);
  font-weight: 400;
}

.hdr-nav {
  display: flex;
  gap: 2px;
}

.nav-link {
  position: relative;
  padding: 6px 14px;
  font-size: 12px;
  font-weight: 500;
  color: var(--t2);
  text-decoration: none;
  border-radius: 6px;
  transition: color 0.2s, background 0.2s;
  letter-spacing: 0.5px;

  &:hover {
    color: var(--t1);
    background: rgba(127, 127, 127, 0.08);
  }

  &.current {
    color: var(--t1);
    background: rgba(45, 124, 246, 0.1);

    &::after {
      content: '';
      position: absolute;
      bottom: -1px;
      left: 50%;
      transform: translateX(-50%);
      width: 16px;
      height: 2px;
      border-radius: 1px;
      background: var(--c-blue);
    }
  }
}

.nav-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 15px;
  height: 15px;
  padding: 0 4px;
  margin-left: 4px;
  border-radius: 7px;
  background: var(--c-red);
  color: white;
  font-size: 9px;
  font-weight: 700;
  line-height: 1;
}

.hdr-end {
  display: flex;
  align-items: center;
  gap: 14px;
}

.hdr-indicator {
  display: flex;
  align-items: center;
  gap: 5px;
}

.ind-label {
  font-size: 10px;
  letter-spacing: 1px;
  color: var(--t3);
}

.hdr-time {
  font-size: 12px;
  color: var(--t3);
  letter-spacing: 0.5px;
}

.hdr-theme,
.hdr-logout {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  border-radius: 6px;
  background: transparent;
  cursor: pointer;
  color: var(--t3);
  transition: color 0.2s, background 0.2s;

  svg { width: 15px; height: 15px; }
}

.hdr-theme:hover {
  color: var(--c-amber);
  background: rgba(245, 158, 11, 0.1);
}

.hdr-logout:hover {
  color: var(--c-red);
  background: rgba(239, 68, 68, 0.1);
}

@media (max-width: 768px) {
  .hdr-inner { padding: 0 10px; height: 42px; }
  .hdr-brand { gap: 6px; }
  .hdr-logo { width: 16px; height: 16px; }
  .hdr-name { font-size: 10px; letter-spacing: 1.5px; }
  .hdr-name-dim { display: none; }
  .nav-link { padding: 4px 8px; font-size: 11px; }
  .hdr-end { gap: 8px; }
  .hdr-time { display: none; }
  .hdr-indicator .ind-label { display: none; }
  .hdr-theme svg, .hdr-logout svg { width: 14px; height: 14px; }
  .hdr-theme, .hdr-logout { width: 24px; height: 24px; }
}

@media (max-width: 480px) {
  .hdr-nav { gap: 0; }
  .nav-link { padding: 4px 6px; font-size: 10px; letter-spacing: 0; }
}
</style>
