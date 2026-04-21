import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: () => import('@/views/Login.vue'),
      meta: { public: true },
    },
    {
      path: '/',
      name: 'Dashboard',
      component: () => import('@/views/Dashboard.vue'),
    },
    {
      path: '/server/:id',
      name: 'ServerDetail',
      component: () => import('@/views/ServerDetail.vue'),
    },
    {
      path: '/alerts',
      name: 'Alerts',
      component: () => import('@/views/Alerts.vue'),
    },
    {
      path: '/stress',
      name: 'StressTest',
      component: () => import('@/views/StressTest.vue'),
    },
    {
      path: '/settings',
      name: 'Settings',
      component: () => import('@/views/Settings.vue'),
    },
  ],
})

router.beforeEach((to, _from, next) => {
  const token = localStorage.getItem('token')
  if (!to.meta.public && !token) {
    next('/login')
  } else if (to.path === '/login' && token) {
    next('/')
  } else {
    next()
  }
})

export default router
