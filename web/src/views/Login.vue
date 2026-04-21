<template>
  <div class="login-scene">
    <canvas ref="gridCanvas" class="grid-bg"></canvas>
    <div class="noise-overlay"></div>

    <div class="login-container" :class="{ 'is-ready': mounted }">
      <div class="brand">
        <div class="brand-icon">
          <svg viewBox="0 0 32 32" fill="none">
            <rect x="2" y="6" width="28" height="20" rx="2" stroke="currentColor" stroke-width="1.5" fill="none"/>
            <path d="M8 14h4m4 0h4m-12 4h2m4 0h2m4 0h2" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
            <circle cx="16" cy="11" r="2" stroke="currentColor" stroke-width="1.5"/>
          </svg>
        </div>
        <div class="brand-text">
          <span class="brand-title">SERVER MONITOR</span>
          <span class="brand-sub">运维监控管理平台</span>
        </div>
      </div>

      <form class="login-form" @submit.prevent="handleLogin">
        <div class="field" :class="{ focused: userFocus, filled: form.username }">
          <label>账号</label>
          <input
            v-model="form.username"
            type="text"
            autocomplete="username"
            spellcheck="false"
            @focus="userFocus = true"
            @blur="userFocus = false"
          />
          <div class="field-line"></div>
        </div>

        <div class="field" :class="{ focused: passFocus, filled: form.password }">
          <label>密码</label>
          <input
            v-model="form.password"
            type="password"
            autocomplete="current-password"
            @focus="passFocus = true"
            @blur="passFocus = false"
          />
          <div class="field-line"></div>
        </div>

        <p class="error-msg" v-if="errorMsg">{{ errorMsg }}</p>

        <button class="login-btn" type="submit" :disabled="loading">
          <span v-if="!loading">进入系统</span>
          <span v-else class="btn-loading">
            <i></i><i></i><i></i>
          </span>
        </button>
      </form>

      <div class="login-footer">
        <span>默认账号 admin / admin123</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { authApi } from '@/api'
import { useTheme } from '@/composables/useTheme'

const router = useRouter()
const mounted = ref(false)
const loading = ref(false)
const errorMsg = ref('')
const userFocus = ref(false)
const passFocus = ref(false)
const gridCanvas = ref<HTMLCanvasElement>()
const { theme: themeRef } = useTheme()
let animId = 0

const form = reactive({ username: '', password: '' })

async function handleLogin() {
  errorMsg.value = ''
  if (!form.username || !form.password) {
    errorMsg.value = '请输入账号和密码'
    return
  }
  loading.value = true
  try {
    const res: any = await authApi.login(form.username, form.password)
    if (res.success) {
      localStorage.setItem('token', res.data.token)
      localStorage.setItem('username', res.data.username)
      router.push('/')
    } else {
      errorMsg.value = res.error || '登录失败'
    }
  } catch (e: any) {
    errorMsg.value = e.response?.data?.error || '网络错误，请重试'
  } finally {
    loading.value = false
  }
}

function drawGrid() {
  const canvas = gridCanvas.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const dpr = window.devicePixelRatio || 1
  canvas.width = window.innerWidth * dpr
  canvas.height = window.innerHeight * dpr
  canvas.style.width = window.innerWidth + 'px'
  canvas.style.height = window.innerHeight + 'px'
  ctx.scale(dpr, dpr)

  const w = window.innerWidth
  const h = window.innerHeight
  const step = 60
  const time = Date.now() * 0.0003

  ctx.clearRect(0, 0, w, h)

  const isLight = themeRef.value === 'light'
  const gridColor = isLight ? '37,99,235' : '59,130,246'
  const baseAlpha = isLight ? 0.06 : 0.04
  const varAlpha = isLight ? 0.03 : 0.02

  for (let x = 0; x <= w; x += step) {
    const offset = Math.sin(x * 0.01 + time) * 2
    const alpha = baseAlpha + Math.sin(x * 0.005 + time * 0.7) * varAlpha
    ctx.strokeStyle = `rgba(${gridColor},${alpha})`
    ctx.lineWidth = 0.5
    ctx.beginPath()
    ctx.moveTo(x, 0)
    ctx.lineTo(x + offset, h)
    ctx.stroke()
  }
  for (let y = 0; y <= h; y += step) {
    const offset = Math.cos(y * 0.01 + time) * 2
    const alpha = (baseAlpha * 0.75) + Math.sin(y * 0.008 + time * 0.5) * (varAlpha * 0.75)
    ctx.strokeStyle = `rgba(${gridColor},${alpha})`
    ctx.lineWidth = 0.5
    ctx.beginPath()
    ctx.moveTo(0, y + offset)
    ctx.lineTo(w, y)
    ctx.stroke()
  }

  const grd = ctx.createRadialGradient(w / 2, h * 0.4, 0, w / 2, h * 0.4, w * 0.5)
  grd.addColorStop(0, isLight ? 'rgba(37,99,235,0.08)' : 'rgba(59,130,246,0.06)')
  grd.addColorStop(0.5, isLight ? 'rgba(8,145,178,0.03)' : 'rgba(6,182,212,0.02)')
  grd.addColorStop(1, 'transparent')
  ctx.fillStyle = grd
  ctx.fillRect(0, 0, w, h)

  animId = requestAnimationFrame(drawGrid)
}

onMounted(() => {
  setTimeout(() => (mounted.value = true), 50)
  drawGrid()
  window.addEventListener('resize', drawGrid)
})

onUnmounted(() => {
  cancelAnimationFrame(animId)
  window.removeEventListener('resize', drawGrid)
})
</script>

<style lang="scss">
.login-scene {
  position: fixed;
  inset: 0;
  background: var(--bg-deep);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.grid-bg {
  position: absolute;
  inset: 0;
  z-index: 0;
}

.noise-overlay {
  position: absolute;
  inset: 0;
  z-index: 1;
  opacity: 0.35;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 256 256' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='0.5'/%3E%3C/svg%3E");
  background-repeat: repeat;
  background-size: 128px;
  pointer-events: none;
}

html.light .noise-overlay {
  opacity: 0.05;
}

.login-container {
  position: relative;
  z-index: 2;
  width: 380px;
  padding: 40px 36px 32px;
  background: var(--card-bg);
  backdrop-filter: blur(24px) saturate(1.2);
  border: 1px solid var(--border);
  border-radius: 16px;

  opacity: 0;
  transform: translateY(20px) scale(0.98);
  transition: all 0.6s cubic-bezier(0.16, 1, 0.3, 1);

  &.is-ready {
    opacity: 1;
    transform: translateY(0) scale(1);
  }

  &::before {
    content: '';
    position: absolute;
    inset: -1px;
    border-radius: 17px;
    padding: 1px;
    background: linear-gradient(
      160deg,
      rgba(59, 130, 246, 0.25) 0%,
      transparent 40%,
      transparent 60%,
      rgba(6, 182, 212, 0.15) 100%
    );
    -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
    -webkit-mask-composite: xor;
    mask-composite: exclude;
    pointer-events: none;
  }
}

html.light .login-container {
  background: #ffffff;
  border-color: rgba(0, 0, 0, 0.12);
  box-shadow: 0 8px 40px rgba(0, 0, 0, 0.12), 0 1px 3px rgba(0, 0, 0, 0.08);
}

.brand {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 36px;
}

.brand-icon {
  width: 40px;
  height: 40px;
  color: var(--c-blue);
  flex-shrink: 0;
  svg { width: 100%; height: 100%; }
}

.brand-text {
  display: flex;
  flex-direction: column;
}

.brand-title {
  font-size: 15px;
  font-weight: 700;
  letter-spacing: 3px;
  color: var(--t1);
  font-family: 'Courier New', monospace;
}

.brand-sub {
  font-size: 11px;
  color: var(--t3);
  margin-top: 2px;
  letter-spacing: 2px;
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.field {
  position: relative;

  label {
    position: absolute;
    left: 0;
    top: 16px;
    font-size: 13px;
    color: var(--t2);
    transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
    pointer-events: none;
    letter-spacing: 1px;
  }

  input {
    width: 100%;
    border: none;
    outline: none;
    background: transparent;
    color: var(--t1);
    font-size: 15px;
    padding: 16px 0 8px;
    caret-color: var(--c-blue);
    font-family: inherit;
  }

  .field-line {
    height: 1px;
    background: var(--border);
    position: relative;
    transition: background 0.3s;

    &::after {
      content: '';
      position: absolute;
      bottom: 0;
      left: 50%;
      width: 0;
      height: 2px;
      background: linear-gradient(90deg, var(--c-blue), var(--c-cyan));
      transition: all 0.35s cubic-bezier(0.4, 0, 0.2, 1);
      transform: translateX(-50%);
    }
  }

  &.focused .field-line::after,
  &.filled .field-line::after {
    width: 100%;
  }

  &.focused label,
  &.filled label {
    top: 0;
    font-size: 11px;
    color: var(--c-blue);
  }
}

html.light .field .field-line {
  background: #cbd5e1;
}

.error-msg {
  color: var(--c-red);
  font-size: 12px;
  margin: -8px 0 0;
  padding-left: 2px;
}

.login-btn {
  width: 100%;
  height: 44px;
  border: none;
  border-radius: 8px;
  background: linear-gradient(135deg, #1d4ed8 0%, #2563eb 50%, #0ea5e9 100%);
  color: white;
  font-size: 14px;
  font-weight: 600;
  letter-spacing: 2px;
  cursor: pointer;
  position: relative;
  overflow: hidden;
  transition: all 0.3s;
  margin-top: 4px;
  box-shadow: 0 2px 8px rgba(37, 99, 235, 0.3);

  &::before {
    content: '';
    position: absolute;
    inset: 0;
    background: linear-gradient(135deg, transparent 40%, rgba(255,255,255,0.1) 50%, transparent 60%);
    transform: translateX(-100%);
    transition: transform 0.5s;
  }

  &:hover::before {
    transform: translateX(100%);
  }

  &:hover {
    box-shadow: 0 4px 24px rgba(37, 99, 235, 0.4);
  }

  &:active {
    transform: scale(0.98);
  }

  &:disabled {
    opacity: 0.7;
    cursor: wait;
  }
}

.btn-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;

  i {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: white;
    animation: dotPulse 0.8s ease-in-out infinite;

    &:nth-child(2) { animation-delay: 0.15s; }
    &:nth-child(3) { animation-delay: 0.3s; }
  }
}

@keyframes dotPulse {
  0%, 80%, 100% { transform: scale(0.4); opacity: 0.4; }
  40% { transform: scale(1); opacity: 1; }
}

.login-footer {
  text-align: center;
  margin-top: 24px;
  font-size: 11px;
  color: var(--t3);
  letter-spacing: 0.5px;
}
</style>
