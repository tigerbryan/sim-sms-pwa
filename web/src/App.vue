<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useSessionStore } from './stores/session'
import { useVersionStore } from './stores/version'
import { useThemeStore } from './stores/theme'
import AppShell from './components/layout/AppShell.vue'
import { Hexagon } from 'lucide-vue-next'

const session = useSessionStore()
const version = useVersionStore()
const themeStore = useThemeStore()
const route = useRoute()
const router = useRouter()

onMounted(async () => {
  themeStore.applyTheme()
  void version.load()
  await session.bootstrap()
  
  if (session.checked && !session.authenticated && route.path !== '/login') {
    router.replace('/login')
  } else if (session.checked && session.authenticated && route.path === '/login') {
    router.replace('/sms')
  }
})

watch(() => session.authenticated, (isAuthenticated) => {
  if (session.checked && !isAuthenticated && route.path !== '/login') {
    router.replace('/login')
  } else if (session.checked && isAuthenticated && route.path === '/login') {
    router.replace('/sms')
  }
})

watch(() => route.path, (newPath) => {
  if (session.checked && !session.authenticated && newPath !== '/login') {
    router.replace('/login')
  }
})
</script>

<template>
  <main v-if="!session.checked" class="splash">
    <div class="loader-container">
      <div class="brand-icon pulse">
        <Hexagon :size="40" stroke-width="2.5" />
      </div>
      <p class="text-muted mt-2">Connecting to QMI Web...</p>
    </div>
  </main>
  
  <router-view v-else-if="route.path === '/login'" />
  
  <router-view v-else-if="route.path === '/sms'" />
  <AppShell v-else />
</template>

<style scoped>
.splash {
  min-height: 100dvh;
  display: grid;
  place-items: center;
  background-color: var(--bg-app);
  background-image: radial-gradient(circle at center, var(--accent-light) 0%, transparent 50%);
}

.loader-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
}

.brand-icon {
  width: 80px;
  height: 80px;
  background-color: var(--accent-light);
  color: var(--accent);
  border-radius: var(--radius-lg);
  display: flex;
  align-items: center;
  justify-content: center;
}

.pulse {
  animation: pulse-soft 2s infinite;
}

.mt-2 {
  margin-top: 8px;
  font-weight: 500;
  letter-spacing: 0.05em;
}

@keyframes pulse-soft {
  0% { transform: scale(0.98); box-shadow: 0 0 0 0 var(--accent-light); }
  50% { transform: scale(1.02); box-shadow: 0 0 0 20px rgba(0, 157, 245, 0); }
  100% { transform: scale(0.98); box-shadow: 0 0 0 0 rgba(0, 157, 245, 0); }
}
</style>
