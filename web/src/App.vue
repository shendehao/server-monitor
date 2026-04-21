<template>
  <div class="app" :class="{ 'is-login': isLoginPage }">
    <AppHeader v-if="!isLoginPage" />
    <main class="app-main">
      <router-view v-slot="{ Component }">
        <transition name="page" mode="out-in">
          <component :is="Component" />
        </transition>
      </router-view>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import AppHeader from '@/components/AppHeader.vue'
import { useMonitorStore } from '@/stores/monitor'

const route = useRoute()
const store = useMonitorStore()
const isLoginPage = computed(() => route.path === '/login')

watch(isLoginPage, (isLogin) => {
  if (!isLogin && localStorage.getItem('token')) {
    store.fetchOverview()
    store.fetchAlertCount()
    store.connectWebSocket()
  } else if (isLogin) {
    store.disconnectWebSocket()
  }
}, { immediate: true })
</script>

<style scoped>
.app {
  min-height: 100vh;
  background: var(--bg-primary);
}

.app.is-login {
  background: transparent;
}

.app-main {
  position: relative;
}

.page-enter-active {
  transition: opacity 0.25s ease, transform 0.25s ease;
}
.page-leave-active {
  transition: opacity 0.15s ease;
}
.page-enter-from {
  opacity: 0;
  transform: translateY(6px);
}
.page-leave-to {
  opacity: 0;
}
</style>
