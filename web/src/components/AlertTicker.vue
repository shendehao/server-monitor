<template>
  <div class="aticker" v-if="displayAlerts.length > 0" title="点击查看告警中心" @click="goAlerts">
    <span class="aticker-tag">ALERT</span>
    <div class="aticker-track">
      <div class="aticker-scroll">
        <span v-for="(a, i) in displayAlerts" :key="`${a.id || a.createdAt || a.message}-${i}`" class="aticker-item" :title="`${a.serverName} ${a.message}`">
          <span class="adot" :class="a.severity"></span>
          <span class="aserver">{{ a.serverName }}</span>
          <span class="amsg">{{ a.message }}</span>
          <span class="atime font-num">{{ fmtTime(a.createdAt) }}</span>
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useMonitorStore } from '@/stores/monitor'
const router = useRouter()
const store = useMonitorStore()
const displayAlerts = computed(() => {
  const list = store.latestAlerts.slice(0, 10)
  return list.length > 1 ? [...list, ...list] : list
})
function goAlerts() {
  router.push('/alerts')
}
function fmtTime(t: string) {
  return t ? new Date(t).toLocaleTimeString('zh-CN', { hour12: false }) : ''
}
</script>

<style scoped lang="scss">
.aticker {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  height: 32px;
  display: flex;
  align-items: center;
  overflow: hidden;
  cursor: pointer;
}

.aticker-tag {
  padding: 0 10px;
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 1.5px;
  color: var(--c-amber);
  border-right: 1px solid var(--border);
  height: 100%;
  display: flex;
  align-items: center;
  flex-shrink: 0;
}

.aticker-track {
  flex: 1;
  overflow: hidden;
}

.aticker-scroll {
  display: flex;
  white-space: nowrap;
  animation: ticker 35s linear infinite;
}

.aticker:hover .aticker-scroll {
  animation-play-state: paused;
}

.aticker-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 0 20px;
  font-size: 11px;
}

.adot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  flex-shrink: 0;
  &.critical { background: var(--c-red); }
  &.warning { background: var(--c-amber); }
  &.info { background: var(--c-violet); }
}

.aserver { color: var(--t1); font-weight: 500; }
.amsg { color: var(--t2); max-width: 260px; overflow: hidden; text-overflow: ellipsis; }
.atime { color: var(--t3); font-size: 10px; }
</style>
