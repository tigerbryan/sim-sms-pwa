<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getDevices } from '../api/devices'
import type { Device, DevicesResponse } from '../types/api'
import { HardDrive, Cpu, Hexagon, Info, AlertCircle, RefreshCw } from 'lucide-vue-next'
import { computed } from 'vue'

const res = ref<DevicesResponse | null>(null)
const dev = computed(() => res.value?.devices[0] || null)
const error = ref('')
const busy = ref(false)

async function load() {
  busy.value = true
  error.value = ''
  try {
    res.value = await getDevices()
  } catch (e) {
    error.value = e instanceof Error ? e.message : '获取设备信息失败'
  } finally {
    busy.value = false
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <div class="device-view page-animate">
    <div class="page-header">
      <div>
        <h1>模组设备管理</h1>
        <p class="text-muted">查看当前连接的蜂窝硬件信息与固件版本</p>
      </div>
      <button class="btn-v2 btn-ghost" @click="load" :disabled="busy">
        <RefreshCw :size="18" :class="{'spin': busy}" /> 刷新状态
      </button>
    </div>

    <div v-if="error" class="error-banner card-base">
      <AlertCircle :size="24" />
      <div>
        <h3>加载设备信息失败</h3>
        <p>{{ error }}</p>
      </div>
    </div>

    <div v-else-if="!dev && busy" class="skeleton-grid">
      <div class="skeleton-card" v-for="i in 4" :key="i"></div>
    </div>

    <div v-else-if="dev" class="device-grid">
      
      <div class="card-base primary-card">
        <div class="card-icon-header">
          <div class="icon-wrap bg-accent-light text-accent">
            <Cpu :size="32" />
          </div>
          <span class="status-badge bg-success-light text-success">已连接</span>
        </div>
        <div class="card-info">
          <h2>{{ dev.manufacturer }} {{ dev.product }}</h2>
          <p class="text-muted">当前工作设备</p>
        </div>
      </div>

      <div class="card-base info-card">
        <div class="icon-wrap bg-interactive text-secondary">
          <HardDrive :size="24" />
        </div>
        <div class="card-info">
          <h3>设备路径</h3>
          <div class="value-large text-mono">{{ dev.control_path || '未知' }}</div>
        </div>
      </div>

      <div class="card-base info-card">
        <div class="icon-wrap bg-purple-light text-purple">
          <Hexagon :size="24" />
        </div>
        <div class="card-info">
          <h3>驱动程序</h3>
          <div class="value-large text-mono">{{ dev.driver || '未知' }}</div>
          <p class="hint">当前使用的内核模块</p>
        </div>
      </div>
      
      <div class="card-base info-card">
        <div class="icon-wrap bg-warning-light text-warning">
          <Info :size="24" />
        </div>
        <div class="card-info">
          <h3>USB VID/PID</h3>
          <div class="value-large text-mono">{{ dev.usb_vid }}:{{ dev.usb_pid }}</div>
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
.page-header h1 { margin: 0 0 8px 0; font-size: 1.75rem; }

.device-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 24px;
}

.primary-card {
  grid-column: 1 / -1;
  padding: 40px;
  display: flex;
  flex-direction: column;
  gap: 24px;
}
@media (min-width: 1024px) {
  .primary-card { grid-column: span 2; }
}

.card-icon-header { display: flex; justify-content: space-between; align-items: flex-start; }
.icon-wrap { width: 56px; height: 56px; border-radius: var(--radius-lg); display: flex; align-items: center; justify-content: center; }

.primary-card h2 { font-size: 2rem; margin: 0 0 8px 0; }
.status-badge { padding: 6px 12px; border-radius: 99px; font-size: 0.85rem; font-weight: 600; text-transform: uppercase; }

.info-card {
  padding: 32px;
  display: flex;
  flex-direction: column;
  gap: 24px;
}
.info-card h3 { font-size: 0.95rem; color: var(--text-secondary); font-weight: 600; margin: 0 0 12px 0; }
.value-large { font-size: 1.5rem; font-weight: 700; color: var(--text-primary); margin-bottom: 8px; word-break: break-all; }
.hint { font-size: 0.85rem; color: var(--text-muted); margin: 0; }
.bg-purple-light { background-color: #f3e8ff; } .text-purple { color: #9333ea; }

.skeleton-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 24px; }
.skeleton-card { height: 200px; background-color: var(--bg-surface); border-radius: var(--radius-xl); animation: pulse 1.5s infinite; }
</style>
