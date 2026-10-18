<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getSIM } from '../api/sim'
import type { SIMInfo } from '../types/api'
import { CreditCard, Hash, Activity, Lock, RefreshCw, AlertCircle } from 'lucide-vue-next'

const sim = ref<SIMInfo | null>(null)
const error = ref('')
const busy = ref(false)

async function load() {
  busy.value = true
  error.value = ''
  try {
    sim.value = await getSIM()
  } catch (e) {
    error.value = e instanceof Error ? e.message : '获取SIM卡状态失败'
  } finally {
    busy.value = false
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <div class="sim-view page-animate">
    <div class="page-header">
      <div>
        <h1>SIM 卡状态</h1>
        <p class="text-muted">实时监控当前插入的 SIM 卡信息与 PIN 锁状态</p>
      </div>
      <button class="btn-v2 btn-ghost" @click="load" :disabled="busy">
        <RefreshCw :size="18" :class="{'spin': busy}" /> 刷新数据
      </button>
    </div>

    <div v-if="error" class="error-banner card-base">
      <AlertCircle :size="24" />
      <div>
        <h3>加载失败</h3>
        <p>{{ error }}</p>
      </div>
    </div>

    <div v-else-if="!sim && busy" class="skeleton-grid">
      <div class="skeleton-card" v-for="i in 4" :key="i"></div>
    </div>

    <div v-else-if="sim" class="sim-grid">
      
      <!-- Primary Info -->
      <div class="card-base feature-card primary-card">
        <div class="card-icon-header">
          <div class="icon-wrap bg-accent-light text-accent">
            <CreditCard :size="28" />
          </div>
          <span class="status-badge" :class="{'bg-success-light text-success': sim.ready, 'bg-warning-light text-warning': !sim.ready}">
            {{ sim.ready ? '就绪' : '未就绪' }}
          </span>
        </div>
        <div class="card-info">
          <h3>ICCID (集成电路卡识别码)</h3>
          <div class="value-large text-mono">{{ sim.iccid || '未知' }}</div>
          <p class="hint">全球唯一的 SIM 卡物理标识符</p>
        </div>
      </div>

      <!-- IMSI -->
      <div class="card-base feature-card">
        <div class="card-icon-header">
          <div class="icon-wrap bg-purple-light text-purple">
            <Hash :size="24" />
          </div>
        </div>
        <div class="card-info">
          <h3>IMSI (国际移动用户识别码)</h3>
          <div class="value-large text-mono">{{ sim.imsi || '未知' }}</div>
          <p class="hint">用于在蜂窝网络中识别用户的身份</p>
        </div>
      </div>

      <!-- PIN Status -->
      <div class="card-base feature-card">
        <div class="card-icon-header">
          <div class="icon-wrap bg-warning-light text-warning">
            <Lock :size="24" />
          </div>
        </div>
        <div class="card-info">
          <h3>PIN 状态</h3>
          <div class="value-large">{{ sim.pin_status || '未知' }}</div>
          <p class="hint">当前 SIM 卡的安全锁定状态</p>
        </div>
      </div>

      <!-- App State -->
      <div class="card-base feature-card">
        <div class="card-icon-header">
          <div class="icon-wrap bg-interactive text-secondary">
            <Activity :size="24" />
          </div>
        </div>
        <div class="card-info">
          <h3>运营商 (MCC/MNC)</h3>
          <div class="value-large">{{ sim.operator || '未知' }}</div>
          <p class="hint">MCC: {{ sim.mcc || '-' }} / MNC: {{ sim.mnc || '-' }}</p>
        </div>
      </div>

    </div>
  </div>
</template>

<style scoped>
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: 24px;
}

.page-header h1 {
  font-size: 1.75rem;
  margin: 0 0 8px 0;
}

.sim-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 24px;
}

.feature-card {
  padding: 32px;
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.primary-card {
  grid-column: 1 / -1;
  border-left: 4px solid var(--accent);
}

@media (min-width: 1024px) {
  .primary-card {
    grid-column: span 2;
  }
}

.card-icon-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.icon-wrap {
  width: 56px;
  height: 56px;
  border-radius: var(--radius-lg);
  display: flex;
  align-items: center;
  justify-content: center;
}

.bg-purple-light { background-color: #f3e8ff; }
.text-purple { color: #9333ea; }

.status-badge {
  padding: 6px 12px;
  border-radius: 99px;
  font-size: 0.85rem;
  font-weight: 600;
  text-transform: uppercase;
}

.card-info h3 {
  font-size: 0.95rem;
  color: var(--text-secondary);
  font-weight: 600;
  margin: 0 0 12px 0;
}

.value-large {
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--text-primary);
  margin-bottom: 12px;
  word-break: break-all;
}

.hint {
  font-size: 0.85rem;
  color: var(--text-muted);
  margin: 0;
}

.skeleton-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 24px;
}
.skeleton-card {
  height: 200px;
  background-color: var(--bg-surface);
  border-radius: var(--radius-xl);
  animation: pulse 1.5s infinite;
}

.error-banner {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 24px;
  background-color: var(--danger-bg);
  color: var(--danger);
}
.error-banner h3 { margin: 0 0 4px 0; }
.error-banner p { margin: 0; opacity: 0.9; }
</style>
