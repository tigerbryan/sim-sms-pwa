<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Menu, ShieldCheck } from 'lucide-vue-next'

const emit = defineEmits<{
  (e: 'toggle-menu'): void
}>()

const route = useRoute()

const pageTitle = computed(() => {
  switch (route.path) {
    case '/': return '运行概览'
    case '/device': return '设备管理'
    case '/sim': return 'SIM 卡'
    case '/signal': return '蜂窝网络'
    case '/sms': return '短信服务'
    case '/notifications': return '通知提醒'
    case '/logs': return '系统日志'
    case '/diagnostics': return '运行诊断'
    case '/settings': return '系统设置'
    case '/about': return '关于'
    default: return 'QMI Web'
  }
})

const pageDesc = computed(() => {
  switch (route.path) {
    case '/': return '实时查看设备、SIM、蜂窝网络与短信状态'
    case '/device': return '管理已连接的底层物理与网络设备'
    case '/sms': return '查看和管理短信收件箱'
    case '/settings': return '系统偏好、安全与高级选项配置'
    default: return ''
  }
})
</script>

<template>
  <header class="topbar-v3">
    <div class="topbar-left">
      <button class="mobile-menu-btn" @click="emit('toggle-menu')">
        <Menu :size="20" />
      </button>
      
      <div class="page-header-info">
        <h1 class="page-title">{{ pageTitle }}</h1>
        <p v-if="pageDesc" class="page-desc">{{ pageDesc }}</p>
      </div>
    </div>
    
    <div class="topbar-right">
      <div class="status-badge" title="系统正处于安全模式，不干扰其他守护进程">
        <ShieldCheck :size="14" class="text-success" />
        <span>SMS-only 安全模式</span>
      </div>
    </div>
  </header>
</template>

<style scoped>
.topbar-v3 {
  height: 80px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 32px;
  background-color: var(--bg-app);
  flex-shrink: 0;
}

.topbar-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.mobile-menu-btn {
  display: none;
  background: transparent;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  padding: 8px;
  margin-left: -8px;
  border-radius: var(--radius-sm);
}

.mobile-menu-btn:hover {
  background-color: var(--bg-interactive);
  color: var(--text-primary);
}

.page-header-info {
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.page-title {
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
  letter-spacing: -0.01em;
}

.page-desc {
  font-size: 0.9rem;
  color: var(--text-muted);
  margin-top: 2px;
}

.topbar-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.status-badge {
  display: flex;
  align-items: center;
  gap: 6px;
  background-color: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  padding: 6px 12px;
  border-radius: 99px;
  font-size: 0.85rem;
  font-weight: 500;
  color: var(--text-secondary);
  box-shadow: var(--shadow-sm);
}

@media (max-width: 768px) {
  .topbar-v3 {
    height: 64px;
    padding: 0 16px;
    border-bottom: 1px solid var(--border-subtle);
    background-color: var(--bg-surface);
  }
  
  .mobile-menu-btn {
    display: block;
  }
  
  .page-desc {
    display: none;
  }
  
  .page-title {
    font-size: 1.25rem;
  }
  
  .status-badge span {
    display: none;
  }
  .status-badge {
    padding: 6px;
  }
}
</style>
