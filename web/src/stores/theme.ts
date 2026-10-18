import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

export type ThemeMode = 'auto' | 'light' | 'dark'

export const useThemeStore = defineStore('theme', () => {
  // Read initial from localStorage, default to 'auto'
  // Handle old 'true'/'false'/'dark'/'light' values for migration
  let initialTheme = localStorage.getItem('theme') as ThemeMode | string | null
  
  // Migration logic
  if (initialTheme === 'dark' || initialTheme === 'light' || initialTheme === 'auto') {
    // Valid new format
  } else if (initialTheme === 'true') {
    initialTheme = 'dark'
  } else if (initialTheme === 'false') {
    initialTheme = 'light'
  } else {
    initialTheme = 'auto'
  }
  
  const mode = ref<ThemeMode>(initialTheme as ThemeMode)
  const isSystemDark = ref(window.matchMedia('(prefers-color-scheme: dark)').matches)

  // Listen to system changes
  const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  const mediaHandler = (e: MediaQueryListEvent) => {
    isSystemDark.value = e.matches
    applyTheme()
  }
  
  // Setup listener
  mediaQuery.addEventListener('change', mediaHandler)
  
  function applyTheme() {
    let shouldBeDark = false
    if (mode.value === 'dark') {
      shouldBeDark = true
    } else if (mode.value === 'light') {
      shouldBeDark = false
    } else {
      shouldBeDark = isSystemDark.value
    }

    if (shouldBeDark) {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }
    
    // Save preference
    localStorage.setItem('theme', mode.value)
  }

  // Watch for manual changes
  watch(mode, () => {
    applyTheme()
  })

  // Expose a helper to toggle (for Sidebar)
  function toggle() {
    // If currently dark (either auto+dark or forced dark), toggle to light
    const currentlyDark = document.documentElement.classList.contains('dark')
    mode.value = currentlyDark ? 'light' : 'dark'
  }

  return {
    mode,
    isSystemDark,
    applyTheme,
    toggle
  }
})
