<script setup lang="ts">
import PushSettings from '../components/PushSettings.vue'
import { onMounted, ref } from 'vue'
import { getNotifications, sendPushPlusTest } from '../api/notifications'
import type { NotificationsResponse } from '../types/api'
import { Bell, AlertCircle, RefreshCw, CheckCircle2, Send, Clock, SendIcon } from 'lucide-vue-next'

const data = ref<NotificationsResponse | null>(null)
const error = ref('')
const result = ref('')
const testing = ref(false)
const loading = ref(true)

async function load() {
  loading.value = true
  try {
    data.value = await getNotifications()
    error.value = ''
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : 'Failed to read notification status.'
  } finally {
    loading.value = false
  }
}

async function testPush() {
  testing.value = true
  result.value = ''
  error.value = ''
  try {
    await sendPushPlusTest()
    result.value = 'PushPlus test notification queued successfully.'
    setTimeout(() => { result.value = '' }, 5000)
    await load()
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : 'PushPlus test failed.'
  } finally {
    testing.value = false
  }
}

onMounted(() => { void load() })
</script>

<template>
  <div class="page-container">
    <PushSettings />
    

    <div v-if="error" class="alert-banner error">
      <AlertCircle :size="18" />
      <span>{{ error }}</span>
    </div>
    
    <div v-if="result" class="alert-banner success">
      <CheckCircle2 :size="18" />
      <span>{{ result }}</span>
    </div>

    <div v-if="!data && loading" class="loading-state">
      <div class="skeleton-card"></div>
    </div>

    <template v-else-if="data">
      <div class="panel summary-card">
        <div class="summary-content">
          <div class="summary-icon" :class="data.pushplus.enabled ? 'bg-success' : 'bg-neutral'">
            <Bell :size="24" :class="data.pushplus.enabled ? 'text-success' : 'text-muted'" />
          </div>
          <div class="summary-info">
            <h2 class="flex items-center gap-2">
              PushPlus 
              <span class="badge" :class="data.pushplus.enabled ? 'safe' : 'neutral'">
                {{ data.pushplus.enabled ? 'Enabled' : '已禁用' }}
              </span>
            </h2>
            <p class="text-muted">
              Token: <strong :class="data.pushplus.token_configured ? 'text-success' : 'text-warning'">
                {{ data.pushplus.token_configured ? 'Configured' : 'Not Configured' }}
              </strong>
              <span class="divider">•</span>
              Template: <strong>{{ data.pushplus.template || 'Default' }}</strong>
            </p>
          </div>
        </div>
        
        <button class="btn btn-primary" :disabled="testing || !data.pushplus.enabled" @click="testPush">
          <component :is="testing ? RefreshCw : SendIcon" :size="16" :class="{ 'spin': testing }" />
          {{ testing ? 'Sending...' : 'Send Test Notification' }}
        </button>
      </div>

      <h2 class="section-title mt-6">Delivery Queue</h2>
      <div class="panel delivery-list">
        <div v-if="!data.items.length" class="empty-state">
          <Clock :size="40" class="text-muted mb-4 opacity-50" />
          <p>No notifications in queue.</p>
        </div>
        
        <div v-else class="list-container">
          <article v-for="item in data.items" :key="item.id" class="list-item">
            <div class="item-content">
              <strong>{{ item.title }}</strong>
              <p class="item-meta">
                <span class="kind-tag">{{ item.kind }}</span>
                <span class="divider">•</span>
                <span>Attempts: {{ item.attempts }}</span>
                <span class="divider">•</span>
                <span>{{ new Date(item.updated_at).toLocaleString() }}</span>
              </p>
            </div>
            <span class="status-badge" :class="item.status === 'success' ? 'success' : 'warning'">
              {{ item.status.toUpperCase() }}
            </span>
          </article>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.page-container {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 16px;
}

.page-title {
  font-size: 1.5rem;
  font-weight: 700;
  margin-bottom: 4px;
}

.page-description {
  color: var(--text-muted);
  font-size: 0.95rem;
}

.alert-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px;
  border-radius: var(--radius-md);
}
.alert-banner.error {
  background-color: var(--danger-bg);
  color: var(--danger);
  border: 1px solid var(--danger);
}
.alert-banner.success {
  background-color: var(--success-bg);
  color: var(--success);
  border: 1px solid var(--success);
}

.panel {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
}

.summary-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 24px;
  padding: 24px;
}

.summary-content {
  display: flex;
  align-items: center;
  gap: 20px;
}

.summary-icon {
  width: 56px;
  height: 56px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
}
.bg-success { background-color: var(--success-bg); }
.bg-neutral { background-color: var(--bg-app); }

.summary-info h2 {
  font-size: 1.25rem;
  font-weight: 600;
  margin: 0 0 8px 0;
}
.summary-info p {
  margin: 0;
  font-size: 0.95rem;
}

.badge {
  padding: 4px 10px;
  border-radius: 99px;
  font-size: 0.75rem;
  font-weight: 600;
}
.badge.safe { background-color: var(--success-bg); color: var(--success); }
.badge.neutral { background-color: var(--bg-app); color: var(--text-secondary); border: 1px solid var(--border-strong); }

.flex { display: flex; }
.items-center { align-items: center; }
.gap-2 { gap: 8px; }
.text-muted { color: var(--text-muted); }
.text-success { color: var(--success); }
.text-warning { color: var(--warning); }
.divider { color: var(--border-strong); margin: 0 6px; }
.mt-6 { margin-top: 24px; }
.mb-4 { margin-bottom: 16px; }
.opacity-50 { opacity: 0.5; }

.section-title {
  font-size: 1.1rem;
  font-weight: 600;
  margin: 0 0 -8px 0;
}

.delivery-list {
  overflow: hidden;
}

.list-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 24px;
  border-bottom: 1px solid var(--border-subtle);
  transition: background-color 0.2s;
}
.list-item:last-child {
  border-bottom: none;
}
.list-item:hover {
  background-color: rgba(0,0,0,0.02);
}
:root.dark .list-item:hover {
  background-color: rgba(255,255,255,0.02);
}

.item-content strong {
  font-size: 1rem;
  color: var(--text-primary);
  display: block;
  margin-bottom: 4px;
}

.item-meta {
  font-size: 0.85rem;
  color: var(--text-muted);
  margin: 0;
}

.kind-tag {
  background-color: var(--bg-app);
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid var(--border-strong);
  font-size: 0.75rem;
  font-weight: 500;
  text-transform: uppercase;
}

.status-badge {
  padding: 6px 12px;
  border-radius: 99px;
  font-size: 0.8rem;
  font-weight: 600;
}
.status-badge.success { background-color: var(--success-bg); color: var(--success); }
.status-badge.warning { background-color: var(--warning-bg); color: var(--warning); }

.empty-state {
  padding: 60px 24px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 10px 20px;
  border-radius: var(--radius-md);
  font-weight: 500;
  font-size: 0.95rem;
  cursor: pointer;
  transition: all 0.2s;
  border: 1px solid transparent;
}
.btn-primary {
  background-color: var(--accent);
  color: var(--accent-text);
}
.btn-primary:hover:not(:disabled) {
  background-color: var(--accent-hover);
}
.btn-outline {
  background: transparent;
  border-color: var(--border-strong);
  color: var(--text-primary);
}
.btn-outline:hover:not(:disabled) {
  background-color: var(--bg-surface);
}
.btn:disabled {
  opacity: 0.7;
  cursor: not-allowed;
}

.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

.loading-state {
  display: flex;
}
.skeleton-card {
  width: 100%;
  height: 120px;
  background: var(--bg-surface);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-subtle);
  animation: pulse 1.5s infinite;
}

@media (max-width: 640px) {
  .summary-card {
    flex-direction: column;
    align-items: flex-start;
  }
  .summary-card .btn {
    width: 100%;
  }
}
</style>
