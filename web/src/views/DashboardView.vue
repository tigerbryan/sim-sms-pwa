<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, computed } from 'vue'
import { getDashboard } from '../api/dashboard'
import { getSMSPage } from '../api/sms'
import type { Dashboard, SMSMessage } from '../types/api'
import { 
  Router, Radio, Signal, MapPin, 
  CreditCard, MessageSquare, ShieldCheck,
  Activity, Navigation, Hash, Clock, RefreshCw
} from 'lucide-vue-next'

const data = ref<Dashboard | null>(null)
const recentSms = ref<SMSMessage[]>([])
const error = ref('')
const isOffline = computed(() => !data.value || data.value.device_status !== 'connected')
const waitingForSIM = computed(() => data.value?.qmi_validation.device_open === 'pass' &&
  ['SIM_ABSENT', 'SIM_NOT_READY', 'SIM_PIN_REQUIRED'].includes(data.value?.qmi_validation.detail ?? ''))
const offlineTitle = computed(() => waitingForSIM.value ? '设备已识别，SIM 卡未就绪' : '暂未连接蜂窝设备')
const offlineDetail = computed(() => {
  if (!waitingForSIM.value) return '请连接兼容的 QMI 模组，系统将在识别后自动显示设备状态。'
  if (data.value?.qmi_validation.detail === 'SIM_PIN_REQUIRED') return 'EC25 已连接，但 SIM 卡需要解锁。当前还无法接收短信。'
  return 'EC25 已连接，SIM 卡尚未通过检查。请关机断电后检查卡槽、插卡方向和接触，再开机。当前还无法接收短信。'
})
let timer: number | undefined
const isRefreshing = ref(false)

async function load() {
  isRefreshing.value = true
  try {
    const [dashboardData, smsData] = await Promise.all([
      getDashboard(),
      getSMSPage(1, 5, '')
    ])
    data.value = dashboardData
    recentSms.value = smsData.items || []
    error.value = ''
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '无法连接到设备服务'
  } finally {
    setTimeout(() => { isRefreshing.value = false }, 500)
  }
}

onMounted(() => {
  void load()
  timer = window.setInterval(() => { void load() }, 5000)
})

onBeforeUnmount(() => { if (timer) window.clearInterval(timer) })

function getSignalLevel(rsrpStr: string | undefined): { label: string, percent: number, class: string } {
  if (!rsrpStr) return { label: '未获取', percent: 0, class: 'neutral' }
  const val = parseInt(rsrpStr.replace(' dBm', ''))
  if (isNaN(val)) return { label: '未获取', percent: 0, class: 'neutral' }
  if (val >= -80) return { label: '极佳', percent: 100, class: 'success' }
  if (val >= -90) return { label: '良好', percent: 75, class: 'info' }
  if (val >= -100) return { label: '一般', percent: 50, class: 'warning' }
  return { label: '较差', percent: 25, class: 'danger' }
}

function maskSender(sender: string) {
  if (!sender) return '未知号码'
  if (sender.length > 5) {
    return sender.slice(0, -4) + '****'
  }
  return sender
}

function formatTimeOnly(iso: string) {
  if (!iso) return ''
  try {
    const date = new Date(iso)
    const now = new Date()
    const diffMs = now.getTime() - date.getTime()
    const diffMins = Math.floor(diffMs / 60000)
    if (diffMins < 60) return `${diffMins || 1} 分钟前`
    const diffHours = Math.floor(diffMins / 60)
    if (diffHours < 24) return `${diffHours} 小时前`
    return date.toLocaleDateString()
  } catch {
    return iso
  }
}
</script>

<template>
  <div class="dashboard-v3">
    
    <div class="action-bar">
      <span class="update-time text-muted">实时监控运行中</span>
      <button class="btn-v2 btn-secondary btn-refresh" :class="{ 'is-loading': isRefreshing }" @click="load">
        <RefreshCw :size="16" />
        刷新状态
      </button>
    </div>

    <div v-if="error" class="error-banner card-base">
      <Activity :size="24" class="text-danger" />
      <div class="error-content">
        <h3>设备状态加载失败</h3>
        <p>{{ error }}</p>
      </div>
      <button class="btn-v2 btn-secondary retry-btn" @click="load">重试</button>
    </div>

    <!-- Skeletons -->
    <div v-else-if="!data" class="grid-12 skeleton-grid">
      <div class="skeleton-box s-hero"></div>
      <div class="skeleton-box s-signal"></div>
      <div class="skeleton-box s-4"></div>
      <div class="skeleton-box s-4"></div>
      <div class="skeleton-box s-4"></div>
    </div>

    <!-- Offline State -->
    <div v-else-if="isOffline" class="offline-hero card-hero">
      <div class="offline-content">
        <div class="offline-icon-wrapper">
          <Router :size="64" class="text-muted" />
        </div>
        <h2>{{ offlineTitle }}</h2>
        <p class="text-secondary">{{ offlineDetail }}</p>
        
        <div class="offline-details text-mono">
          <span class="detail-item"><Hash :size="14"/> /dev/cdc-wdm0</span>
          <span class="detail-item"><Activity :size="14"/> {{ data.backend }}</span>
        </div>
        
        <button class="btn-v2 btn-primary" @click="load">
          重新检测
        </button>
      </div>
    </div>

    <!-- Online State -->
    <div v-else class="grid-12">
      
      <!-- 1. Device Hero (Col 1-5) -->
      <section class="card-hero device-hero">
        <div class="hero-bg-accent"></div>
        <div class="hero-header">
          <div class="status-indicator">
            <span class="dot pulse-success"></span>
            设备在线
          </div>
          <span class="backend-tag text-mono">{{ data.backend }}</span>
        </div>
        
        <div class="hero-main">
          <div class="device-icon">
            <Router :size="40" class="text-accent" />
          </div>
          <div class="device-meta">
            <h2>{{ data.sim.operator || '蜂窝网络' }} 设备</h2>
            <div class="meta-row text-mono text-secondary">
              <Hash :size="14" /> /dev/cdc-wdm0
            </div>
            <div class="meta-row text-mono text-secondary">
              <Navigation :size="14" /> wwan0
            </div>
          </div>
        </div>
      </section>

      <!-- 2. Signal / Network (Col 6-12) -->
      <section class="card-base signal-card">
        <div class="card-title-row">
          <h3>信号质量</h3>
          <span class="badge-v2" :class="`badge-${getSignalLevel(data.signal.rsrp).class}`">
            {{ getSignalLevel(data.signal.rsrp).label }}
          </span>
        </div>
        
        <div class="signal-visualizer">
          <div class="metric-group">
            <div class="metric-header">
              <span class="metric-name">RSRP</span>
              <span class="metric-val text-mono">{{ data.signal.rsrp || '--' }}</span>
            </div>
            <div class="progress-track">
              <div class="progress-fill fill-accent" :style="{ width: getSignalLevel(data.signal.rsrp).percent + '%' }"></div>
            </div>
          </div>
          
          <div class="metric-group">
            <div class="metric-header">
              <span class="metric-name">RSRQ</span>
              <span class="metric-val text-mono">{{ data.signal.rsrq || '--' }}</span>
            </div>
            <div class="progress-track">
              <div class="progress-fill fill-info" :style="{ width: (data.signal.rsrq ? 70 : 0) + '%' }"></div>
            </div>
          </div>
          
          <div class="metric-group">
            <div class="metric-header">
              <span class="metric-name">SINR</span>
              <span class="metric-val text-mono">{{ data.signal.sinr || '--' }}</span>
            </div>
            <div class="progress-track">
              <div class="progress-fill fill-info" :style="{ width: (data.signal.sinr ? 85 : 0) + '%' }"></div>
            </div>
          </div>
        </div>
      </section>

      <!-- 3. SIM (Col 1-3) -->
      <section class="card-base stat-tile">
        <div class="tile-icon bg-info-light">
          <CreditCard :size="24" class="text-info" />
        </div>
        <div class="tile-info">
          <span class="tile-label">SIM 状态</span>
          <span class="tile-value" :class="data.sim.ready ? 'text-primary' : 'text-warning'">
            {{ data.sim.ready ? 'Ready' : '未就绪' }}
          </span>
          <span class="tile-sub text-muted">{{ data.sim.operator || '未知运营商' }}</span>
        </div>
      </section>

      <!-- 4. Network (Col 4-6) -->
      <section class="card-base stat-tile">
        <div class="tile-icon bg-accent-light">
          <MapPin :size="24" class="text-accent" />
        </div>
        <div class="tile-info">
          <span class="tile-label">蜂窝网络</span>
          <span class="tile-value">{{ data.signal.technology || 'LTE' }}</span>
          <span class="tile-sub text-muted">{{ data.signal.registered ? '已注册网络' : '未注册' }}</span>
        </div>
      </section>

      <!-- 5. SMS (Col 7-9) -->
      <section class="card-base stat-tile">
        <div class="tile-icon bg-success-light">
          <MessageSquare :size="24" class="text-success" />
        </div>
        <div class="tile-info">
          <span class="tile-label">短信总数</span>
          <span class="tile-value">{{ data.sms.total || 0 }}</span>
          <span class="tile-sub text-muted">{{ data.sms.unread }} 条未读</span>
        </div>
      </section>

      <!-- 6. PushPlus (Col 10-12) -->
      <section class="card-base stat-tile">
        <div class="tile-icon bg-warning-light">
          <Bell :size="24" class="text-warning" />
        </div>
        <div class="tile-info">
          <span class="tile-label">PushPlus 推送</span>
          <span class="tile-value">未配置</span>
          <span class="tile-sub text-muted">前往设置开启</span>
        </div>
      </section>

      <!-- 7. Recent SMS (Col 1-8) -->
      <section class="card-base recent-sms-card">
        <div class="card-title-row">
          <h3>最近短信</h3>
          <RouterLink to="/sms" class="link-btn">查看全部</RouterLink>
        </div>
        
        <div v-if="recentSms.length === 0" class="empty-list">
          <MessageSquare :size="32" class="text-muted" style="opacity:0.3"/>
          <p>暂无新短信，收到的短信会显示在这里</p>
        </div>
        
        <div v-else class="sms-list">
          <div v-for="msg in recentSms" :key="msg.id" class="sms-item">
            <div class="sms-avatar">
              <MessageSquare :size="16" />
            </div>
            <div class="sms-content">
              <div class="sms-header">
                <span class="sms-sender" :class="{'text-primary': msg.status === 'unread'}">{{ maskSender(msg.sender) }}</span>
                <span class="sms-time text-muted">{{ formatTimeOnly(msg.received_at) }}</span>
              </div>
              <div class="sms-preview text-secondary" :class="{'font-bold': msg.status === 'unread'}">
                {{ msg.body }}
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- 8. QMI Health & Security (Col 9-12) -->
      <section class="card-base side-panel-wrapper">
        
        <div class="side-panel">
          <div class="panel-header">
            <h3>QMI 健康度</h3>
            <Activity :size="18" class="text-success" />
          </div>
          <div class="health-metrics">
            <div class="metric-row">
              <span class="text-secondary">守护进程</span>
              <span class="badge-v2 badge-success">运行中</span>
            </div>
            <div class="metric-row">
              <span class="text-secondary">设备锁</span>
              <span class="badge-v2 badge-neutral">已释放</span>
            </div>
            <div class="metric-row">
              <span class="text-secondary">内存占用</span>
              <span class="text-mono text-primary">-- MB</span>
            </div>
          </div>
        </div>
        
        <div class="panel-divider"></div>
        
        <div class="side-panel">
          <div class="panel-header">
            <h3>安全边界</h3>
            <ShieldCheck :size="18" class="text-success" />
          </div>
          <ul class="security-list">
            <li>
              <span class="check text-success">✓</span>
              <span>SMS-only 模式已启用</span>
            </li>
            <li>
              <span class="check text-success">✓</span>
              <span>未启用蜂窝默认路由</span>
            </li>
            <li>
              <span class="check" :class="data.data_guard.state === 'safe' ? 'text-success' : 'text-danger'">
                {{ data.data_guard.state === 'safe' ? '✓' : '!' }}
              </span>
              <span>WDS 数据连接受限</span>
            </li>
          </ul>
        </div>
        
      </section>

    </div>
  </div>
</template>

<style scoped>
.dashboard-v3 {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.action-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.update-time {
  font-size: 0.9rem;
}

.btn-refresh {
  height: 36px;
}
.btn-refresh .lucide {
  transition: transform 0.5s ease;
}
.btn-refresh.is-loading .lucide {
  transform: rotate(180deg);
}

/* Offline Hero */
.offline-hero {
  grid-column: span 12;
  height: 60vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 40px;
}

.offline-icon-wrapper {
  width: 120px;
  height: 120px;
  background-color: var(--bg-interactive);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto 32px;
}

.offline-content h2 {
  font-size: 1.75rem;
  margin-bottom: 12px;
}

.offline-content p {
  font-size: 1.1rem;
  max-width: 400px;
  margin: 0 auto 32px;
}

.offline-details {
  display: flex;
  gap: 24px;
  justify-content: center;
  margin-bottom: 40px;
}
.detail-item {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--text-muted);
}

/* 1. Device Hero */
.device-hero {
  grid-column: span 5;
  padding: 32px;
  display: flex;
  flex-direction: column;
}

.hero-bg-accent {
  position: absolute;
  top: 0;
  right: 0;
  width: 150px;
  height: 150px;
  background: radial-gradient(circle at top right, var(--accent-light) 0%, transparent 70%);
  pointer-events: none;
}

.hero-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  position: relative;
  z-index: 1;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary);
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
.pulse-success {
  background-color: var(--success);
  box-shadow: 0 0 0 0 rgba(5, 150, 105, 0.4);
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(5, 150, 105, 0.4); }
  70% { transform: scale(1); box-shadow: 0 0 0 6px rgba(5, 150, 105, 0); }
  100% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(5, 150, 105, 0); }
}

.backend-tag {
  color: var(--text-muted);
}

.hero-main {
  display: flex;
  align-items: center;
  gap: 24px;
  margin-top: auto;
  margin-bottom: auto;
  position: relative;
  z-index: 1;
}

.device-icon {
  width: 72px;
  height: 72px;
  background-color: var(--bg-surface);
  box-shadow: var(--shadow-md);
  border-radius: var(--radius-lg);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.device-meta h2 {
  font-size: 1.5rem;
  margin-bottom: 12px;
}

.meta-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

/* 2. Signal Card */
.signal-card {
  grid-column: span 7;
  padding: 32px;
  display: flex;
  flex-direction: column;
}

.card-title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.card-title-row h3 {
  font-size: 1.15rem;
  margin: 0;
}

.signal-visualizer {
  display: flex;
  flex-direction: column;
  gap: 20px;
  flex: 1;
  justify-content: center;
}

.metric-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.metric-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
}

.metric-name {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-secondary);
}

.metric-val {
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--text-primary);
}

.progress-track {
  width: 100%;
  height: 8px;
  background-color: var(--bg-interactive);
  border-radius: 99px;
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  border-radius: 99px;
  transition: width 0.6s cubic-bezier(0.4, 0, 0.2, 1);
}

.fill-accent { background-color: var(--accent); }
.fill-info { background-color: var(--info); }

/* 3-6. Stat Tiles */
.stat-tile {
  grid-column: span 3;
  padding: 24px;
  display: flex;
  align-items: center;
  gap: 16px;
}

.tile-icon {
  width: 48px;
  height: 48px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.bg-info-light { background-color: var(--info-bg); }
.bg-accent-light { background-color: var(--accent-light); }
.bg-success-light { background-color: var(--success-bg); }
.bg-warning-light { background-color: var(--warning-bg); }

.tile-info {
  display: flex;
  flex-direction: column;
}

.tile-label {
  font-size: 0.85rem;
  color: var(--text-secondary);
  font-weight: 600;
  margin-bottom: 2px;
}

.tile-value {
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--text-primary);
}

.tile-sub {
  font-size: 0.8rem;
  margin-top: 2px;
}

/* 7. Recent SMS */
.recent-sms-card {
  grid-column: span 8;
  padding: 32px;
}

.link-btn {
  font-size: 0.9rem;
  font-weight: 500;
  color: var(--accent);
}
.link-btn:hover { text-decoration: underline; }

.empty-list {
  padding: 40px 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.sms-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.sms-item {
  display: flex;
  gap: 16px;
  padding: 16px;
  background-color: var(--bg-interactive);
  border-radius: var(--radius-md);
}

.sms-avatar {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background-color: var(--accent-light);
  color: var(--accent);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.sms-content {
  flex: 1;
  min-width: 0;
}

.sms-header {
  display: flex;
  justify-content: space-between;
  margin-bottom: 4px;
}

.sms-sender {
  font-weight: 600;
  font-size: 0.95rem;
}

.sms-time {
  font-size: 0.8rem;
}

.sms-preview {
  font-size: 0.95rem;
  line-height: 1.5;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.font-bold {
  font-weight: 600;
  color: var(--text-primary) !important;
}

/* 8. Health & Security Panel */
.side-panel-wrapper {
  grid-column: span 4;
  display: flex;
  flex-direction: column;
}

.side-panel {
  padding: 24px;
  flex: 1;
}

.panel-divider {
  height: 1px;
  background-color: var(--border-subtle);
  margin: 0 24px;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.panel-header h3 {
  font-size: 1.05rem;
  margin: 0;
}

.health-metrics {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.metric-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.9rem;
}

.security-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  list-style: none;
}

.security-list li {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 0.9rem;
  color: var(--text-secondary);
}

.check {
  font-weight: bold;
}

/* Skeletons */
.skeleton-grid {
  margin-top: 24px;
}
.skeleton-box {
  background-color: var(--bg-surface);
  border-radius: var(--radius-lg);
  animation: pulse-skel 1.5s infinite;
}
.s-hero { grid-column: span 5; height: 280px; }
.s-signal { grid-column: span 7; height: 280px; }
.s-4 { grid-column: span 4; height: 120px; }

@keyframes pulse-skel {
  0% { opacity: 1; }
  50% { opacity: 0.6; }
  100% { opacity: 1; }
}

@media (max-width: 1200px) {
  .device-hero { grid-column: span 12; }
  .signal-card { grid-column: span 12; }
  .stat-tile { grid-column: span 6; }
  .recent-sms-card { grid-column: span 12; }
  .side-panel-wrapper { grid-column: span 12; }
}

@media (max-width: 768px) {
  .device-hero { padding: 24px; }
  .signal-card { padding: 24px; }
  .recent-sms-card { padding: 24px; }
}

@media (max-width: 480px) {
  .stat-tile { grid-column: span 12; }
}
</style>
