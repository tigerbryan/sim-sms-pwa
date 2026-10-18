<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getDiagnostics } from '../api/diagnostics'
import type { DiagnosticsResponse } from '../types/api'
import { Server, Activity, Shield, Code, RefreshCw, AlertCircle, HardDrive } from 'lucide-vue-next'

const diag = ref<DiagnosticsResponse | null>(null)
const error = ref('')
const busy = ref(false)

async function load() {
  busy.value = true
  error.value = ''
  try {
    diag.value = await getDiagnostics()
  } catch (e) {
    error.value = e instanceof Error ? e.message : '获取诊断信息失败'
  } finally {
    busy.value = false
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <div class="diag-view page-animate">
    <div class="page-header">
      <div>
        <h1>系统诊断</h1>
        <p class="text-muted">查看守护进程、主机环境及依赖服务的健康状态</p>
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

    <div v-else-if="!diag && busy" class="skeleton-grid">
      <div class="skeleton-card" v-for="i in 3" :key="i"></div>
    </div>

    <div v-else-if="diag" class="diag-grid">
      
      <div class="card-base feature-card primary-card">
        <div class="card-icon-header">
          <div class="icon-wrap bg-accent-light text-accent">
            <Server :size="28" />
          </div>
          <span class="status-badge bg-success-light text-success">
            {{ diag.backend }} 正常
          </span>
        </div>
        <div class="card-info">
          <h3>主机环境</h3>
          <div class="value-large text-mono">{{ diag.os }} / {{ diag.architecture }}</div>
          <p class="hint">守护进程已运行 {{ diag.uptime_seconds }} 秒</p>
        </div>
      </div>

      <div class="card-base feature-card">
        <div class="card-icon-header">
          <div class="icon-wrap bg-purple-light text-purple">
            <Code :size="24" />
          </div>
        </div>
        <div class="card-info">
          <h3>系统版本</h3>
          <div class="value-large text-mono">{{ diag.version.version || '未知' }}</div>
          <p class="hint">Go: {{ diag.version.go_version }}</p>
        </div>
      </div>

      <div class="card-base feature-card">
        <div class="card-icon-header">
          <div class="icon-wrap bg-interactive text-secondary">
            <Shield :size="24" />
          </div>
        </div>
        <div class="card-info">
          <h3>QMI 数据安全监控 (Data Guard)</h3>
          <div class="value-large" :class="{'text-success': diag.guard.state === 'pass', 'text-warning': diag.guard.state !== 'pass'}">{{ diag.guard.state || '未知' }}</div>
          <p class="hint">最后检查: {{ new Date(diag.guard.checked_at).toLocaleTimeString() }}</p>
        </div>
      </div>
      
      <div class="card-base feature-card wide-card">
        <div class="card-icon-header">
          <div class="icon-wrap bg-interactive text-secondary">
            <HardDrive :size="24" />
          </div>
        </div>
        <div class="card-info">
          <h3>QMI 核心子系统连通性</h3>
          <div class="subsystem-grid mt-4 text-mono">
            <div class="subsys-item">
              <span>WDS (数据)</span>
              <span :class="{'text-success': diag.qmi_validation.wds === 'OK', 'text-danger': diag.qmi_validation.wds !== 'OK'}">{{ diag.qmi_validation.wds }}</span>
            </div>
            <div class="subsys-item">
              <span>DMS (设备管理)</span>
              <span :class="{'text-success': diag.qmi_validation.dms === 'OK', 'text-danger': diag.qmi_validation.dms !== 'OK'}">{{ diag.qmi_validation.dms }}</span>
            </div>
            <div class="subsys-item">
              <span>UIM (SIM卡)</span>
              <span :class="{'text-success': diag.qmi_validation.uim === 'OK', 'text-danger': diag.qmi_validation.uim !== 'OK'}">{{ diag.qmi_validation.uim }}</span>
            </div>
            <div class="subsys-item">
              <span>NAS (网络接入)</span>
              <span :class="{'text-success': diag.qmi_validation.nas === 'OK', 'text-danger': diag.qmi_validation.nas !== 'OK'}">{{ diag.qmi_validation.nas }}</span>
            </div>
          </div>
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

.diag-grid {
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
}

.wide-card {
  grid-column: 1 / -1;
}

@media (min-width: 1024px) {
  .primary-card { grid-column: span 2; }
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

.subsystem-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.subsys-item {
  display: flex;
  justify-content: space-between;
  padding: 12px 16px;
  background-color: var(--bg-interactive);
  border-radius: var(--radius-md);
  font-size: 0.95rem;
}

.mt-4 { margin-top: 16px; }

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
</style>
