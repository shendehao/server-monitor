<template>
  <div class="kpi-strip">
    <div class="kpi" v-for="item in stats" :key="item.label">
      <span class="kpi-val font-num" :class="{ placeholder: item.placeholder }" :style="{ color: item.accent }">{{ item.value }}</span>
      <span class="kpi-label">{{ item.label }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, defineProps } from 'vue'
import { useMonitorStore } from '@/stores/monitor'

const props = defineProps<{
  loading?: boolean
}>()

const store = useMonitorStore()

const stats = computed(() => {
  const o = store.overview
  const loading = props.loading && !o
  const placeholder = (suffix = '') => loading ? `--${suffix}` : suffix === '%' ? '0.0%' : '0'
  return [
    { label: '服务器', value: loading ? placeholder() : (o?.serverCount ?? 0), accent: 'var(--t1)', placeholder: loading },
    { label: '在线', value: loading ? placeholder() : (o?.onlineCount ?? 0), accent: 'var(--c-green)', placeholder: loading },
    { label: '离线', value: loading ? placeholder() : (o?.offlineCount ?? 0), accent: 'var(--t3)', placeholder: loading },
    { label: 'CPU 均值', value: loading ? placeholder('%') : (o?.avgCpu ?? 0).toFixed(1) + '%', accent: 'var(--c-cyan)', placeholder: loading },
    { label: '内存均值', value: loading ? placeholder('%') : (o?.avgMemory ?? 0).toFixed(1) + '%', accent: 'var(--c-blue)', placeholder: loading },
    { label: '告警', value: loading ? placeholder() : (o?.activeAlerts ?? 0), accent: loading ? 'var(--t3)' : (o?.activeAlerts ? 'var(--c-amber)' : 'var(--t3)'), placeholder: loading },
  ]
})
</script>

<style scoped lang="scss">
.kpi-strip {
  display: flex;
  gap: 2px;
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
}

.kpi {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 14px 8px 12px;
  position: relative;
  transition: background 0.25s;

  &:hover { background: rgba(127, 127, 127, 0.04); }

  &:not(:last-child)::after {
    content: '';
    position: absolute;
    right: 0;
    top: 20%;
    height: 60%;
    width: 1px;
    background: var(--border);
  }
}

.kpi-val {
  font-size: 20px;
  font-weight: 700;
  line-height: 1;
  &.placeholder { color: var(--t3) !important; opacity: 0.7; }
}

.kpi-label {
  font-size: 10px;
  color: var(--t3);
  margin-top: 6px;
  letter-spacing: 0.5px;
}

@media (max-width: 768px) {
  .kpi-strip { flex-wrap: wrap; border-radius: 8px; }
  .kpi {
    flex: 0 0 33.33%;
    padding: 10px 6px 8px;
    &:not(:last-child)::after { display: none; }
  }
  .kpi-val { font-size: 16px; }
  .kpi-label { font-size: 9px; margin-top: 4px; }
}

@media (max-width: 480px) {
  .kpi { flex: 0 0 33.33%; }
  .kpi-val { font-size: 14px; }
}
</style>

<style lang="scss">
/* StatusOverview Light Theme */
html.light .kpi-strip {
  background: rgba(255,255,255,0.95);
  box-shadow: 0 1px 6px rgba(0,0,0,0.08);
  border-color: rgba(0,0,0,0.12);
}
html.light .kpi-label {
  color: #64748b;
}
</style>
