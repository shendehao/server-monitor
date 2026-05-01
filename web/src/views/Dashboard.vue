<template>
  <div class="dash">
    <StatusOverview :loading="store.loading" />

    <div v-if="store.loading && store.servers.length === 0" class="dash-grid">
      <div v-for="n in 8" :key="n" class="dash-skeleton"></div>
    </div>

    <div v-else class="dash-grid">
      <ServerCard
        v-for="s in onlineServers"
        :key="s.id"
        :server="s"
      />
    </div>

    <div class="dash-charts">
      <TrendChart title="CPU 使用率趋势" :series="store.cpuSeries" :loading="store.seriesLoading" :error="store.seriesError" />
      <TrendChart title="内存使用率趋势" :series="store.memSeries" :loading="store.seriesLoading" :error="store.seriesError" />
    </div>

    <AlertTicker />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { useMonitorStore } from '@/stores/monitor'
import StatusOverview from '@/components/StatusOverview.vue'
import ServerCard from '@/components/ServerCard.vue'
import TrendChart from '@/components/TrendChart.vue'
import AlertTicker from '@/components/AlertTicker.vue'

const store = useMonitorStore()
let timer: ReturnType<typeof setInterval> | null = null

const onlineServers = computed(() => store.servers.filter(s => s.isOnline))

onMounted(async () => {
  // 并行加载所有数据
  await Promise.all([
    store.fetchOverview(),
    store.fetchRealtimeSeries('cpu'),
    store.fetchRealtimeSeries('memory'),
  ])

  timer = setInterval(() => {
    store.fetchRealtimeSeries('cpu')
    store.fetchRealtimeSeries('memory')
  }, 15000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped lang="scss">
.dash {
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-height: calc(100vh - 50px);
}

.dash-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 10px;
}

.dash-skeleton {
  height: 108px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: linear-gradient(90deg, rgba(127,127,127,0.06) 25%, rgba(127,127,127,0.1) 50%, rgba(127,127,127,0.06) 75%);
  background-size: 200% 100%;
  animation: dashSkeleton 1.2s ease-in-out infinite;
}

@keyframes dashSkeleton {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

@media (max-width: 1440px) { .dash-grid { grid-template-columns: repeat(3, 1fr); } }
@media (max-width: 1024px) { .dash-grid { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 640px)  { .dash-grid { grid-template-columns: 1fr; } }

.dash-charts {
  display: flex;
  gap: 10px;
}

@media (max-width: 1280px) { .dash-charts { flex-direction: column; } }

@media (max-width: 768px) {
  .dash { padding: 10px 12px; gap: 10px; }
  .dash-grid { gap: 8px; }
  .dash-charts { gap: 8px; }
}

@media (max-width: 480px) {
  .dash { padding: 8px; gap: 8px; }
}
</style>
