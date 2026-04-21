<template>
  <div class="aticker" v-if="store.latestAlerts.length > 0">
    <span class="aticker-tag">ALERT</span>
    <div class="aticker-track">
      <div class="aticker-scroll">
        <span v-for="(a, i) in store.latestAlerts.slice(0, 10)" :key="i" class="aticker-item">
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
import { useMonitorStore } from '@/stores/monitor'
const store = useMonitorStore()
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
.amsg { color: var(--t2); }
.atime { color: var(--t3); font-size: 10px; }
</style>
