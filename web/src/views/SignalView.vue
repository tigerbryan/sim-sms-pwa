<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { getSignal } from '../api/signal'
import type { SignalInfo } from '../types/api'
import { Radio, Activity, MapPin, Zap, AlertCircle } from 'lucide-vue-next'

const signal = ref<SignalInfo | null>(null)
const error = ref('')
const busy = ref(false)
let timer: ReturnType<typeof setInterval>

async function load() {
  error.value = ''
  try {
    signal.value = await getSignal()
  } catch (e) {
    error.value = e instanceof Error ? e.message : '获取信号状态失败'
  }
}

onMounted(() => {
  void load()
  timer = setInterval(load, 5000)
})

onUnmounted(() => {
  clearInterval(timer)
})

function getSignalQuality(dbm: number): { label: string, color: string, percentage: number } {
  if (dbm === 0) return { label: '未知', color: 'var(--text-muted)', percentage: 0 }
  if (dbm > -70) return { label: '极佳', color: 'var(--success)', percentage: 100 }
  if (dbm > -85) return { label: '良好', color: 'var(--success)', percentage: 75 }
  if (dbm > -100) return { label: '一般', color: 'var(--warning)', percentage: 50 }
  return { label: '微弱', color: 'var(--danger)', percentage: 25 }
}
</script>

<template>
  <div class="signal-view page-animate">
    <div class="page-header">
      <div>
        <h1>蜂窝网络与信号</h1>
        <p class="text-muted">实时监控物理层网络注册状态与射频指标 (自动刷新)</p>
      </div>
    </div>

    <div v-if="error" class="error-banner card-base">
      <AlertCircle :size="24" />
      <div>
        <h3>无法获取信号数据</h3>
        <p>{{ error }}</p>
      </div>
    </div>

    <div v-else-if="!signal" class="skeleton-grid">
      <div class="skeleton-card" v-for="i in 3" :key="i"></div>
    </div>

    <div v-else class="signal-dashboard">
      
      <!-- Top Row: Registration & Operator -->
      <div class="top-metrics">
        <div class="metric-card card-base">
          <div class="icon-wrap bg-accent-light text-accent">
            <Radio :size="24" />
          </div>
          <div class="metric-info">
            <span class="label">运营商</span>
            <span class="value">{{ signal.plmn || '未注册' }}</span>
          </div>
        </div>
        
        <div class="metric-card card-base">
          <div class="icon-wrap bg-success-light text-success">
            <Activity :size="24" />
          </div>
          <div class="metric-info">
            <span class="label">注册状态</span>
            <span class="value">{{ signal.registered ? '已注册' : '未注册' }}</span>
          </div>
        </div>
        
        <div class="metric-card card-base">
          <div class="icon-wrap bg-purple-light text-purple">
            <Zap :size="24" />
          </div>
          <div class="metric-info">
            <span class="label">网络制式 (Radio)</span>
            <span class="value">{{ signal.technology || '未知' }}</span>
          </div>
        </div>
      </div>

      <!-- Main Signal Strength -->
      <div class="card-base rf-card">
        <div class="rf-header">
          <h2>射频信号强度 (RSSI)</h2>
          <span 
            class="quality-badge" 
            :style="{ color: getSignalQuality(parseInt(signal.rssi)).color, backgroundColor: getSignalQuality(parseInt(signal.rssi)).color + '20' }"
          >
            {{ getSignalQuality(parseInt(signal.rssi)).label }}
          </span>
        </div>
        
        <div class="signal-meter">
          <div class="dbm-value">
            {{ signal.rssi ? signal.rssi + ' dBm' : 'N/A' }}
          </div>
          <div class="progress-track">
            <div 
              class="progress-fill" 
              :style="{ 
                width: getSignalQuality(parseInt(signal.rssi)).percentage + '%',
                backgroundColor: getSignalQuality(parseInt(signal.rssi)).color
              }"
            ></div>
          </div>
          <div class="scale-labels text-muted">
            <span>-120</span>
            <span>-100</span>
            <span>-80</span>
            <span>-60</span>
            <span>-40</span>
          </div>
        </div>
      </div>

      <!-- Detail Grid -->
      <div class="detail-grid">
        <div class="detail-card card-base">
          <h3>RSRP (参考信号接收功率)</h3>
          <div class="detail-val text-mono">{{ signal.rsrp ? signal.rsrp + ' dBm' : '-' }}</div>
          <p class="text-muted">衡量 LTE 信号覆盖质量的核心指标</p>
        </div>
        
        <div class="detail-card card-base">
          <h3>RSRQ (参考信号接收质量)</h3>
          <div class="detail-val text-mono">{{ signal.rsrq ? signal.rsrq + ' dB' : '-' }}</div>
          <p class="text-muted">反映当前频段干扰水平</p>
        </div>
        
        <div class="detail-card card-base">
          <h3>SINR (信噪比)</h3>
          <div class="detail-val text-mono">{{ signal.sinr ? signal.sinr + ' dB' : '-' }}</div>
          <p class="text-muted">直接影响数据传输速率</p>
        </div>
        
        <div class="detail-card card-base">
          <h3>漫游状态</h3>
          <div class="detail-val text-mono">{{ signal.roaming ? '漫游中' : '本地' }}</div>
          <p class="text-muted">当前是否处于漫游网络</p>
        </div>
      </div>

    </div>
  </div>
</template>

<style scoped>
.page-header {
  margin-bottom: 24px;
}
.page-header h1 { margin: 0 0 8px 0; font-size: 1.75rem; }

.signal-dashboard {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.top-metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 24px;
}

.metric-card {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 24px;
}

.icon-wrap {
  width: 56px; height: 56px;
  border-radius: var(--radius-lg);
  display: flex; align-items: center; justify-content: center;
}
.bg-purple-light { background-color: #f3e8ff; }
.text-purple { color: #9333ea; }

.metric-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.label { font-size: 0.9rem; color: var(--text-secondary); font-weight: 600; }
.value { font-size: 1.25rem; font-weight: 700; color: var(--text-primary); }

.rf-card {
  padding: 40px;
}

.rf-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 32px;
}
.rf-header h2 { margin: 0; font-size: 1.25rem; }
.quality-badge {
  padding: 6px 16px;
  border-radius: 99px;
  font-weight: 700;
  font-size: 0.9rem;
}

.signal-meter {
  max-width: 800px;
  margin: 0 auto;
  text-align: center;
}

.dbm-value {
  font-size: 3rem;
  font-weight: 800;
  font-family: var(--font-mono);
  margin-bottom: 24px;
  letter-spacing: -1px;
}

.progress-track {
  height: 12px;
  background-color: var(--bg-interactive);
  border-radius: 99px;
  overflow: hidden;
  margin-bottom: 12px;
}
.progress-fill {
  height: 100%;
  border-radius: 99px;
  transition: width 0.5s ease-out, background-color 0.3s;
}
.scale-labels {
  display: flex;
  justify-content: space-between;
  font-size: 0.8rem;
  font-family: var(--font-mono);
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 24px;
}

.detail-card {
  padding: 24px;
  display: flex;
  flex-direction: column;
}
.detail-card h3 {
  font-size: 0.95rem; color: var(--text-secondary); font-weight: 600; margin: 0 0 16px 0;
}
.detail-val {
  font-size: 1.75rem; font-weight: 700; margin-bottom: 8px;
}
.detail-card p {
  margin: 0; font-size: 0.85rem;
}

.cell-info {
  display: flex;
  flex-direction: column;
  gap: 12px;
  font-size: 0.95rem;
  flex: 1;
  justify-content: center;
}
.cell-row {
  display: flex; justify-content: space-between; align-items: center;
  border-bottom: 1px dashed var(--border-subtle);
  padding-bottom: 4px;
}
.cell-row strong { color: var(--text-primary); }

.skeleton-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 24px;
}
.skeleton-card { height: 120px; background-color: var(--bg-surface); border-radius: var(--radius-xl); animation: pulse 1.5s infinite; }
</style>
