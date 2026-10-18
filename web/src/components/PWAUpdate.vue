<script setup lang="ts">
import { computed } from 'vue'
import { RefreshCw, Download } from 'lucide-vue-next'
import { usePWAUpdate } from '../composables/pwaUpdate'

const { status, message, currentVersion, checkForUpdates, applyUpdate } = usePWAUpdate()
const busy = computed(() => status.value === 'checking' || status.value === 'updating')
</script>

<template>
  <section class="pwa-update" aria-label="应用更新">
    <div class="update-heading"><h3>应用更新</h3><span v-if="currentVersion">{{ currentVersion }}</span></div>
    <p role="status" aria-live="polite" :class="{ error: status === 'error' }">{{ message }}</p>
    <button type="button" :disabled="busy" @click="status === 'available' ? applyUpdate() : checkForUpdates()">
      <Download v-if="status === 'available'" :size="17" aria-hidden="true" />
      <RefreshCw v-else :size="17" :class="{ spin: busy }" aria-hidden="true" />
      {{ status === 'checking' ? '正在检查…' : status === 'updating' ? '正在更新…' : status === 'available' ? '立即更新' : '检查更新' }}
    </button>
    <p class="icon-note">更新网页和图标资源；不能强制替换系统保存的主屏幕或桌面图标。</p>
  </section>
</template>

<style scoped>
.pwa-update { margin-top:16px; padding:16px; border-radius:12px; background:var(--paper,var(--bg-secondary,#fff)); color:var(--ink,var(--text-primary,#111)); }
.update-heading { display:flex; align-items:center; justify-content:space-between; gap:10px; }
.update-heading h3 { margin:0; font-size:16px; font-weight:600; }
.update-heading span { font-size:12px; color:var(--secondary,#8e8e93); }
.pwa-update p { margin:8px 0 12px; font-size:13px; line-height:1.5; }
.pwa-update button { display:flex; align-items:center; justify-content:center; gap:7px; width:100%; min-height:44px; padding:10px 12px; border:0; border-radius:10px; background:var(--blue,#007aff)!important; color:#fff; font:600 15px/1.4 -apple-system,BlinkMacSystemFont,'PingFang SC',sans-serif; cursor:pointer; }
.pwa-update button:disabled { opacity:.6; cursor:default; }
.pwa-update button:focus-visible { outline:2px solid var(--blue,#007aff); outline-offset:3px; }
.pwa-update .icon-note { margin-bottom:0; color:var(--secondary,#8e8e93); font-size:12px; }
.pwa-update .error { color:#b42318; }
.spin { animation:turn 1s linear infinite; } @keyframes turn { to { transform:rotate(360deg); } }
@media (prefers-reduced-motion:reduce) { .spin { animation:none; } }
</style>
