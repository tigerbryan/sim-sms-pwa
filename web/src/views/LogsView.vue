<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { getLogs } from '../api/logs'
import type { LogEntry } from '../types/api'
import { Terminal, Download, RefreshCw, AlertCircle, Trash2 } from 'lucide-vue-next'

const logs = ref<LogEntry[]>([])
const error = ref('')
const busy = ref(false)
const terminalRef = ref<HTMLElement | null>(null)
let timer: ReturnType<typeof setInterval>

async function load() {
  error.value = ''
  try {
    const res = await getLogs()
    logs.value = res.items || []
    // scroll to bottom
    setTimeout(() => {
      if (terminalRef.value) {
        terminalRef.value.scrollTop = terminalRef.value.scrollHeight
      }
    }, 50)
  } catch (e) {
    error.value = e instanceof Error ? e.message : '获取日志失败'
  }
}

function doDownload() {
  if (!logs.value || logs.value.length === 0) return
  
  const content = logs.value.map(l => `[${l.time}] ${l.level.toUpperCase()} ${l.message}`).join('\n')
  const blob = new Blob([content], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  
  const date = new Date().toISOString().replace(/T/, '-').replace(/:/g, '').split('.')[0]
  a.download = `qmi-web-logs-${date}.txt`
  
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

onMounted(() => {
  void load()
  timer = setInterval(load, 5000)
})

onUnmounted(() => {
  clearInterval(timer)
})
</script>

<template>
  <div class="logs-view page-animate">
    
    <div class="page-header">
      <div>
        <h1>系统日志</h1>
        <p class="text-muted">查看 QMI Web 守护进程运行日志，方便排查故障</p>
      </div>
      <div class="actions">
        <button class="btn-v2 btn-ghost" @click="load">
          <RefreshCw :size="18" /> 刷新
        </button>
        <button class="btn-v2 btn-primary" @click="doDownload">
          <Download :size="18" /> 导出日志
        </button>
      </div>
    </div>

    <div v-if="error" class="error-banner card-base">
      <AlertCircle :size="24" />
      <div>
        <h3>无法读取日志</h3>
        <p>{{ error }}</p>
      </div>
    </div>

    <div class="terminal-card card-base" v-else>
      <div class="terminal-header">
        <div class="window-controls">
          <div class="dot red"></div>
          <div class="dot yellow"></div>
          <div class="dot green"></div>
        </div>
        <div class="terminal-title"><Terminal :size="14" /> qmi-web.log</div>
      </div>
      <div class="terminal-body" ref="terminalRef">
        <pre class="log-content">{{ logs.map(l => `[${l.time}] ${l.level.toUpperCase()} ${l.message}`).join('\n') || 'No logs available.' }}</pre>
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
.actions { display: flex; gap: 12px; }

.terminal-card {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 200px);
  min-height: 500px;
  overflow: hidden;
  padding: 0;
}

.terminal-header {
  background-color: var(--bg-interactive);
  padding: 12px 16px;
  display: flex;
  align-items: center;
  border-bottom: 1px solid var(--border-subtle);
}

.window-controls {
  display: flex;
  gap: 8px;
  width: 60px;
}

.dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
}
.red { background-color: #ff5f56; }
.yellow { background-color: #ffbd2e; }
.green { background-color: #27c93f; }

.terminal-title {
  flex: 1;
  text-align: center;
  font-size: 0.85rem;
  font-family: var(--font-mono);
  color: var(--text-muted);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-right: 60px; /* offset the window controls to truly center */
}

.terminal-body {
  flex: 1;
  padding: 24px;
  overflow-y: auto;
  background-color: var(--bg-surface);
}

.log-content {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 0.9rem;
  line-height: 1.6;
  color: var(--text-secondary);
  white-space: pre-wrap;
  word-break: break-all;
}

/* In dark mode, we might want to override the surface color for terminal to be truly black */
:global(.dark) .terminal-body {
  background-color: #050505;
}
@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 16px;
  }
  .actions {
    flex-wrap: wrap;
    width: 100%;
  }
  .actions button {
    flex: 1;
  }
}
</style>
