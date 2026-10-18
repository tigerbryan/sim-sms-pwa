<script setup lang="ts">
import { useRoute } from 'vue-router'
import { 
  LayoutDashboard, Router, CreditCard, 
  Signal, MessageSquare, Bell, ScrollText, 
  Activity, Settings, LogOut, X, Sun, Moon, Info, ShieldCheck
} from 'lucide-vue-next'
import { useSessionStore } from '../../stores/session'
import { useThemeStore } from '../../stores/theme'
import { ref, onMounted, computed } from 'vue'

const props = defineProps<{
  isOpen: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const route = useRoute()
const session = useSessionStore()
const themeStore = useThemeStore()

// For the UI to show sun/moon, we determine actual dark state 
const isDark = computed(() => {
  if (themeStore.mode === 'dark') return true
  if (themeStore.mode === 'light') return false
  return themeStore.isSystemDark
})

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

import { onUnmounted } from 'vue'
onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.isOpen) {
    emit('close')
  }
}

function toggleTheme() {
  themeStore.toggle()
}

const navGroups = [
  {
    title: '工作台',
    items: [
      { name: '运行概览', path: '/dashboard', icon: LayoutDashboard },
      { name: '设备管理', path: '/devices', icon: Router },
      { name: 'SIM', path: '/sim', icon: CreditCard },
      { name: '蜂窝网络', path: '/signal', icon: Signal },
      { name: '短信服务', path: '/sms', icon: MessageSquare }
    ]
  },
  {
    title: '系统',
    items: [
      { name: '通知提醒', path: '/notifications', icon: Bell },
      { name: '系统日志', path: '/logs', icon: ScrollText },
      { name: '运行诊断', path: '/diagnostics', icon: Activity },
      { name: '设置', path: '/settings', icon: Settings }
    ]
  }
]
</script>

<template>
  <aside class="sidebar-v3" :class="{ 'is-open': isOpen }">
    <div class="sidebar-header">
      <div class="logo-area">
        <div class="logo-icon">Q</div>
        <div class="logo-text">QMI Web</div>
      </div>
      <button v-if="isOpen" class="mobile-close" @click="emit('close')">
        <X :size="20" />
      </button>
    </div>

    <div class="sidebar-nav-scroll">
      <nav class="nav-groups">
        <div v-for="group in navGroups" :key="group.title" class="nav-group">
          <div class="group-title">{{ group.title }}</div>
          <RouterLink 
            v-for="item in group.items" 
            :key="item.path" 
            :to="item.path"
            class="nav-item"
            :class="{ active: route.path === item.path }"
            @click="emit('close')"
          >
            <component :is="item.icon" :size="20" class="nav-icon" />
            <span class="nav-label">{{ item.name }}</span>
          </RouterLink>
        </div>
      </nav>
    </div>

    <div class="sidebar-footer">
      <RouterLink to="/about" class="nav-item" :class="{ active: route.path === '/about' }" @click="emit('close')">
        <Info :size="20" class="nav-icon" />
        <span class="nav-label">关于系统</span>
      </RouterLink>
      
      <div class="footer-divider"></div>
      
      <div class="user-actions">
        <div class="user-profile">
          <div class="avatar">
            <span v-if="session.username">{{ session.username.charAt(0).toUpperCase() }}</span>
            <span v-else>A</span>
          </div>
          <span class="username">{{ session.username || 'admin' }}</span>
        </div>
        <div class="action-buttons">
          <button class="action-btn" @click="toggleTheme" title="切换主题">
            <Moon v-if="isDark" :size="16" />
            <Sun v-else :size="16" />
          </button>
          <button class="action-btn danger" @click="session.logout" title="退出登录">
            <LogOut :size="16" />
          </button>
        </div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.sidebar-v3 {
  width: 240px;
  background-color: var(--bg-surface);
  border-right: 1px solid var(--border-subtle);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  z-index: 50;
}

.sidebar-header {
  padding: 24px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.logo-area {
  display: flex;
  align-items: center;
  gap: 12px;
}

.logo-icon {
  width: 32px;
  height: 32px;
  background: linear-gradient(135deg, var(--accent), #0284c7);
  color: white;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 1.1rem;
  box-shadow: 0 4px 10px rgba(0, 157, 245, 0.3);
}

.logo-text {
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: -0.02em;
}

.mobile-close {
  display: none;
  background: transparent;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  padding: 4px;
}

.sidebar-nav-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 0 16px;
}

.nav-groups {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.nav-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.group-title {
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  padding: 0 12px;
  margin-bottom: 8px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  text-decoration: none;
  font-weight: 500;
  font-size: 0.95rem;
  transition: all 0.2s;
}

.nav-icon {
  opacity: 0.7;
  transition: opacity 0.2s, color 0.2s;
}

.nav-item:hover {
  background-color: var(--bg-interactive);
  color: var(--text-primary);
}

.nav-item:hover .nav-icon {
  opacity: 1;
}

.nav-item.active {
  background-color: var(--accent-light);
  color: var(--accent);
}

.nav-item.active .nav-icon {
  color: var(--accent);
  opacity: 1;
}

.sidebar-footer {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-top: auto;
}

.footer-divider {
  height: 1px;
  background-color: var(--border-subtle);
  margin: 0 8px;
}

.user-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px;
}

.user-profile {
  display: flex;
  align-items: center;
  gap: 10px;
}

.avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background-color: var(--bg-interactive);
  border: 1px solid var(--border-strong);
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 600;
  color: var(--text-secondary);
}

.username {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary);
}

.action-buttons {
  display: flex;
  gap: 4px;
}

.action-btn {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-sm);
  background: transparent;
  border: none;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s;
}

.action-btn:hover {
  background-color: var(--bg-interactive);
  color: var(--text-primary);
}

.action-btn.danger:hover {
  background-color: var(--danger-bg);
  color: var(--danger);
}

@media (max-width: 768px) {
  .sidebar-v3 {
    position: fixed;
    top: 0;
    left: 0;
    bottom: 0;
    transform: translateX(-100%);
  }
  
  .sidebar-v3.is-open {
    transform: translateX(0);
  }
  
  .mobile-close {
    display: block;
  }
}
</style>
