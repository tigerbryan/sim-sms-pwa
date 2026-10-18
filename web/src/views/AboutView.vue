<script setup lang="ts">
import { onMounted } from 'vue'
import { useVersionStore } from '../stores/version'
import { Activity, ShieldCheck, Github, Code2, BookOpen } from 'lucide-vue-next'

const version = useVersionStore()

onMounted(() => { void version.load() })
</script>

<template>
  <div class="page-container">
    

    <div v-if="version.error" class="error-banner">
      {{ version.error }}
    </div>

    <div v-else-if="!version.info" class="loading-state">
      <div class="skeleton-card"></div>
    </div>

    <div v-else class="about-grid">
      <div class="panel version-card">
        <div class="brand-large">
          <Activity :size="48" class="brand-icon" />
          <div class="brand-text">
            <h2>QMI Web</h2>
            <div class="version-number">v{{ version.info.version }}</div>
          </div>
        </div>
        
        <div class="security-badge">
          <ShieldCheck :size="18" class="text-success" />
          <span>Running in <strong>SMS-only Secure Mode</strong></span>
        </div>
      </div>
      
      <div class="panel build-details">
        <h3 class="section-title">Build Information</h3>
        <dl class="details-list">
          <div class="detail-row">
            <dt>Commit Hash</dt>
            <dd class="font-mono bg-app p-1 rounded">{{ version.info.commit }}</dd>
          </div>
          <div class="detail-row">
            <dt>Build Time (UTC)</dt>
            <dd>{{ version.info.build_time }}</dd>
          </div>
          <div class="detail-row">
            <dt>Go Version</dt>
            <dd>{{ version.info.go_version }}</dd>
          </div>
        </dl>
      </div>

      <div class="panel dependencies-details">
        <h3 class="section-title">Dependencies</h3>
        <dl class="details-list">
          <div class="detail-row">
            <dt>qmi-go</dt>
            <dd>{{ version.info.qmi_go_version }}</dd>
          </div>
          <div class="detail-row">
            <dt>smscodec</dt>
            <dd>{{ version.info.smscodec_version }}</dd>
          </div>
          <div class="detail-row">
            <dt>License</dt>
            <dd>{{ version.info.license }}</dd>
          </div>
        </dl>
      </div>
    </div>
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

.error-banner {
  padding: 16px;
  background-color: var(--danger-bg);
  color: var(--danger);
  border-radius: var(--radius-md);
  border: 1px solid var(--danger);
}

.panel {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: 24px;
  box-shadow: var(--shadow-sm);
}

.about-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 24px;
}

.version-card {
  grid-column: 1 / -1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 48px 24px;
  background: linear-gradient(145deg, var(--bg-surface), var(--bg-app));
}

.brand-large {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  margin-bottom: 24px;
}

.brand-icon {
  color: var(--info);
  padding: 12px;
  background-color: var(--info-bg);
  border-radius: var(--radius-lg);
  width: 72px;
  height: 72px;
}

.brand-text h2 {
  font-size: 2rem;
  font-weight: 800;
  margin: 0 0 8px 0;
  letter-spacing: -0.02em;
}

.version-number {
  font-size: 1.25rem;
  color: var(--text-secondary);
  font-weight: 600;
}

.security-badge {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  background-color: var(--success-bg);
  border-radius: 99px;
  color: var(--text-primary);
  font-size: 0.95rem;
}

.text-success { color: var(--success); }

.section-title {
  font-size: 1.15rem;
  font-weight: 600;
  margin: 0 0 16px 0;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border-subtle);
}

.details-list {
  display: flex;
  flex-direction: column;
}

.detail-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid var(--border-subtle);
}
.detail-row:last-child {
  border-bottom: none;
}

.detail-row dt {
  color: var(--text-secondary);
  font-weight: 500;
  font-size: 0.95rem;
}

.detail-row dd {
  margin: 0;
  color: var(--text-primary);
  font-weight: 500;
}

.font-mono {
  font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
  font-size: 0.85rem;
}

.bg-app { background-color: var(--bg-app); }
.p-1 { padding: 4px 8px; }
.rounded { border-radius: var(--radius-sm); border: 1px solid var(--border-strong); }

.loading-state {
  display: flex;
}
.skeleton-card {
  width: 100%;
  height: 200px;
  background: var(--bg-surface);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-subtle);
  animation: pulse 1.5s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}
</style>
