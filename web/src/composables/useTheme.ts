import { ref, watch } from 'vue'

const STORAGE_KEY = 'theme'

type Theme = 'dark' | 'light'

const theme = ref<Theme>((localStorage.getItem(STORAGE_KEY) as Theme) || 'dark')

function applyTheme(t: Theme) {
  document.documentElement.classList.toggle('light', t === 'light')
}

// Apply on load
applyTheme(theme.value)

watch(theme, (val) => {
  localStorage.setItem(STORAGE_KEY, val)
  applyTheme(val)
})

export function useTheme() {
  function toggle() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
  }

  return {
    theme,
    toggle,
    isDark: () => theme.value === 'dark',
  }
}
