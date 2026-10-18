<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getSettings, updateSettings } from '../api/settings'
import type { SettingsResponse, SettingsUpdate } from '../types/api'
import { useThemeStore } from '../stores/theme'
import PWAUpdate from '../components/PWAUpdate.vue'
import { Save, Loader2, Bell, Shield, Palette, Layout, Server, AlertCircle } from 'lucide-vue-next'

const settings = ref<SettingsResponse | null>(null)
const themeStore = useThemeStore()
const ppToken = ref('') // store token locally
const busy = ref(false)
const error = ref('')
const success = ref(false)
const activeTab = ref('general')

async function load() {
  try {
    settings.value = await getSettings()
    ppToken.value = '' // clear on load
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载设置失败'
  }
}

async function save() {
  if (!settings.value) return
  busy.value = true
  error.value = ''
  success.value = false
  try {
    const update: SettingsUpdate = {
      general: {
        scan_interval: settings.value.general.scan_interval
      },
      pushplus: {
        enabled: settings.value.pushplus.enabled,
        token: ppToken.value || '',
        template: settings.value.pushplus.template
      },
      logging: settings.value.logging
    }
    await updateSettings(update)
    success.value = true
    ppToken.value = ''
    setTimeout(() => { success.value = false }, 3000)
  } catch (e) {
    error.value = e instanceof Error ? e.message : '保存失败'
  } finally {
    busy.value = false
  }
}

onMounted(() => {
  void load()
})

const tabs = [
  { id: 'general', name: '常规设置', icon: Layout },
  { id: 'appearance', name: '外观个性化', icon: Palette },
  { id: 'notifications', name: '通知推送', icon: Bell },
  { id: 'security', name: '安全与访问', icon: Shield },
  { id: 'advanced', name: '高级选项', icon: Server }
]
</script>

<template>
  <div class="settings-v3">
    
    <div class="settings-sidebar card-base">
      <nav class="settings-nav">
        <button 
          v-for="tab in tabs" 
          :key="tab.id"
          class="nav-tab"
          :class="{ active: activeTab === tab.id }"
          @click="activeTab = tab.id"
        >
          <component :is="tab.icon" :size="18" />
          {{ tab.name }}
        </button>
      </nav>
    </div>
    
    <div class="settings-content card-base">
      <div v-if="!settings" class="loading-state">
        <Loader2 class="spin text-accent" :size="32" />
        <p class="text-muted mt-4">正在加载设置...</p>
      </div>
      
      <form v-else class="settings-form" @submit.prevent="save">
        
        <div class="form-header">
          <h2>{{ tabs.find(t => t.id === activeTab)?.name }}</h2>
          <p class="text-muted">管理 QMI Web 的系统偏好与运行参数。</p>
        </div>

        <div v-show="activeTab === 'general'" class="form-section">
          <PWAUpdate />
          <div class="field-group">
            <label>扫描间隔</label>
            <input type="text" v-model="settings.general.scan_interval" class="input-v3 text-mono" />
            <span class="field-hint">默认 30s。</span>
          </div>
          <div class="field-group">
            <label>当前后端</label>
            <input type="text" :value="settings.general.backend" class="input-v3 text-mono" disabled />
          </div>
        </div>
        
        <div v-show="activeTab === 'appearance'" class="form-section">
          <div class="field-group">
            <label>系统主题</label>
            <div class="theme-selector">
              <label class="theme-option" :class="{active: themeStore.mode === 'light'}">
                <input type="radio" v-model="themeStore.mode" value="light" class="sr-only" />
                <div class="theme-preview light-preview"></div>
                <span>浅色 (Light)</span>
              </label>
              <label class="theme-option" :class="{active: themeStore.mode === 'dark'}">
                <input type="radio" v-model="themeStore.mode" value="dark" class="sr-only" />
                <div class="theme-preview dark-preview"></div>
                <span>深色 (Dark)</span>
              </label>
              <label class="theme-option" :class="{active: themeStore.mode === 'auto'}">
                <input type="radio" v-model="themeStore.mode" value="auto" class="sr-only" />
                <div class="theme-preview auto-preview"></div>
                <span>跟随系统 (Auto)</span>
              </label>
            </div>
          </div>
        </div>
        
        <div v-show="activeTab === 'notifications'" class="form-section">
          <div class="field-group">
            <label class="toggle-label">
              <span class="label-text">
                <span class="title">PushPlus 微信推送</span>
                <span class="desc">将收到的短信自动推送到微信。</span>
              </span>
              <div class="toggle-switch">
                <input type="checkbox" v-model="settings.pushplus.enabled" class="sr-only" />
                <div class="slider" :class="{on: settings.pushplus.enabled}"></div>
              </div>
            </label>
          </div>
          
          <div class="field-group" :class="{disabled: !settings.pushplus.enabled}">
            <label>PushPlus Token {{ settings.pushplus.token_configured ? '(已配置)' : '' }}</label>
            <input type="password" v-model="ppToken" class="input-v3 text-mono" placeholder="输入 Token (留空不修改)" :disabled="!settings.pushplus.enabled" />
            <span class="field-hint">请前往 PushPlus 官网获取您的专属 Token。</span>
          </div>
        </div>

        <div v-show="activeTab === 'security'" class="form-section">
          <div class="field-group">
            <label class="toggle-label">
              <span class="label-text">
                <span class="title">SMS-only 模式</span>
                <span class="desc">只读模式，避免干扰其他程序。</span>
              </span>
              <div class="toggle-switch">
                <input type="checkbox" v-model="settings.security.sms_only" class="sr-only" disabled />
                <div class="slider" :class="{on: settings.security.sms_only}"></div>
              </div>
            </label>
            <span class="field-hint text-warning">此设置只能通过环境变量修改。</span>
          </div>
        </div>

        <div v-show="activeTab === 'advanced'" class="form-section">
          <div class="field-group">
            <label>日志级别</label>
            <select v-model="settings.logging.level" class="input-v3 select-v3">
              <option value="debug">Debug</option>
              <option value="info">Info</option>
              <option value="warn">Warn</option>
              <option value="error">Error</option>
            </select>
          </div>
          <div class="field-group">
            <label class="toggle-label">
              <span class="label-text">
                <span class="title">保存 PDU 原文</span>
                <span class="desc">在日志中记录短信的原始 PDU 编码（用于调试）。</span>
              </span>
              <div class="toggle-switch">
                <input type="checkbox" v-model="settings.sms.store_raw_pdu" class="sr-only" disabled />
                <div class="slider" :class="{on: settings.sms.store_raw_pdu}"></div>
              </div>
            </label>
            <span class="field-hint text-warning">此设置只能通过环境变量修改。</span>
          </div>
        </div>

        <div class="form-footer">
          <div v-if="error" class="error-text text-danger">{{ error }}</div>
          <div v-if="success" class="success-text text-success">
            <Shield :size="16" /> 设置已成功保存
          </div>
          
          <button type="submit" class="btn-v2 btn-primary submit-btn" :disabled="busy">
            <Loader2 v-if="busy" class="spin" :size="18" />
            <Save v-else :size="18" />
            保存更改
          </button>
        </div>

      </form>
    </div>
  </div>
</template>

<style scoped>
.settings-v3 {
  display: grid;
  grid-template-columns: 280px 1fr;
  gap: 32px;
  align-items: start;
}

.settings-sidebar {
  padding: 16px;
}

.settings-nav {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.nav-tab {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border-radius: var(--radius-md);
  background: transparent;
  border: none;
  font-size: 0.95rem;
  font-weight: 500;
  color: var(--text-secondary);
  cursor: pointer;
  text-align: left;
  transition: all 0.2s;
}

.nav-tab:hover {
  background-color: var(--bg-interactive);
  color: var(--text-primary);
}

.nav-tab.active {
  background-color: var(--accent-light);
  color: var(--accent);
}

.settings-content {
  padding: 40px;
  min-height: 500px;
}

.loading-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 300px;
}
.mt-4 { margin-top: 16px; }

.form-header {
  margin-bottom: 40px;
  padding-bottom: 24px;
  border-bottom: 1px solid var(--border-subtle);
}

.form-header h2 {
  font-size: 1.5rem;
  margin: 0 0 8px 0;
}

.form-section {
  display: flex;
  flex-direction: column;
  gap: 32px;
}

.field-group {
  display: flex;
  flex-direction: column;
  gap: 10px;
  transition: opacity 0.3s;
}
.field-group.disabled { opacity: 0.5; pointer-events: none; }

.field-group > label {
  font-weight: 600;
  font-size: 0.95rem;
}

.input-v3 {
  height: 44px;
  padding: 0 16px;
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-md);
  background-color: var(--bg-interactive);
  color: var(--text-primary);
  font-size: 0.95rem;
  transition: all 0.2s;
  width: 100%;
  max-width: 480px;
}
.input-v3:focus {
  outline: none;
  background-color: var(--bg-surface);
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-light);
}

.select-v3 {
  appearance: none;
  background-image: url("data:image/svg+xml;charset=utf-8,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%23475569' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 16px center;
  padding-right: 40px;
}

.field-hint {
  font-size: 0.85rem;
  color: var(--text-muted);
}

/* Radios */
.radio-group {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-width: 480px;
}
.radio-label {
  display: flex;
  cursor: pointer;
}
.radio-label input {
  display: none;
}
.radio-box {
  flex: 1;
  padding: 16px;
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-md);
  background-color: var(--bg-surface);
  transition: all 0.2s;
}
.radio-title {
  display: block;
  font-weight: 600;
  margin-bottom: 4px;
}
.radio-desc {
  display: block;
  font-size: 0.85rem;
  color: var(--text-secondary);
}
.radio-label input:checked + .radio-box {
  border-color: var(--accent);
  background-color: var(--accent-light);
}

/* Theme selector */
.theme-selector {
  display: flex;
  gap: 24px;
}
.theme-option {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  cursor: pointer;
}
.theme-preview {
  width: 100px;
  height: 72px;
  border-radius: var(--radius-md);
  border: 2px solid var(--border-strong);
  transition: all 0.2s;
}
.light-preview { background: #f8fafc; }
.dark-preview { background: #0f172a; }
.auto-preview { background: linear-gradient(135deg, #f8fafc 50%, #0f172a 50%); }

.theme-option.active .theme-preview {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-light);
}
.theme-option span {
  font-size: 0.9rem;
  font-weight: 500;
}
.sr-only { display: none; }

/* Toggles */
.toggle-label {
  display: flex;
  justify-content: space-between;
  align-items: center;
  cursor: pointer;
  max-width: 480px;
}
.label-text {
  display: flex;
  flex-direction: column;
}
.label-text .title { font-weight: 600; }
.label-text .desc { font-size: 0.85rem; color: var(--text-muted); margin-top: 2px; }

.toggle-switch {
  position: relative;
  width: 44px;
  height: 24px;
}
.slider {
  position: absolute;
  cursor: pointer;
  top: 0; left: 0; right: 0; bottom: 0;
  background-color: var(--border-strong);
  border-radius: 24px;
  transition: .4s;
}
.slider:before {
  position: absolute;
  content: "";
  height: 18px;
  width: 18px;
  left: 3px;
  bottom: 3px;
  background-color: white;
  border-radius: 50%;
  transition: .4s;
}
.slider.on {
  background-color: var(--success);
}
.slider.on:before {
  transform: translateX(20px);
}

.warning-box {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  background-color: var(--warning-bg);
  color: var(--warning);
  border-radius: var(--radius-md);
  font-size: 0.9rem;
  max-width: 480px;
}

.form-footer {
  margin-top: 48px;
  padding-top: 24px;
  border-top: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 16px;
}

.success-text {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 500;
  font-size: 0.95rem;
}

.submit-btn {
  min-width: 120px;
}

@media (max-width: 768px) {
  .settings-v3 {
    grid-template-columns: 1fr;
    gap: 16px;
  }
  .settings-nav {
    flex-direction: row;
    overflow-x: auto;
  }
  .nav-tab {
    white-space: nowrap;
  }
  .settings-content {
    padding: 24px;
  }
}
</style>
