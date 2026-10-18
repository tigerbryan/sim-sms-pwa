<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { getSMSPage, setSMSRead } from '../api/sms'
import type { SMSMessage } from '../types/api'
import PushSettings from '../components/PushSettings.vue'
import PWAInstall from '../components/PWAInstall.vue'
import PWAUpdate from '../components/PWAUpdate.vue'
import { usePWAInstall } from '../composables/pwaInstall'
import { Search, Settings, ChevronLeft, ChevronRight, MessageCircle, Copy, X } from 'lucide-vue-next'
const items = ref<SMSMessage[]>([])
const query = ref(''), selected = ref(''), error = ref('')
const busy = ref(false), settingsOpen = ref(false)
const copied = ref<number | null>(null), total = ref(0), page = ref(1)
const initialLoading = ref(true), loadingOlder = ref(false)
const serverUnread = ref(0)
const { installed: pwaInstalled } = usePWAInstall()
const timeline = ref<HTMLElement | null>(null)
let timer: ReturnType<typeof setInterval> | undefined
let searchTimer: ReturnType<typeof setTimeout> | undefined
let copyTimer: ReturnType<typeof setTimeout> | undefined
let generation = 0, disposed = false, pendingRefresh = false
let marking: Promise<void> | undefined
const keyOf = (m: SMSMessage) => JSON.stringify([m.device_id, m.sender])
const threads = computed(() => {
  const groups = new Map<string, SMSMessage[]>()
  for (const m of items.value) { const key = keyOf(m); groups.set(key, [...(groups.get(key) || []), m]) }
  return [...groups].map(([key, messages]) => {
    messages.sort((a,b) => Date.parse(b.received_at) - Date.parse(a.received_at) || b.id - a.id)
    return { key, messages, latest: messages[0]!, unread: messages.some(m => m.status === 'unread') }
  }).sort((a,b) => Date.parse(b.latest.received_at) - Date.parse(a.latest.received_at) || b.latest.id - a.latest.id)
})
const active = computed(() => threads.value.find(t => t.key === selected.value))
const messages = computed(() => [...(active.value?.messages || [])].reverse())
const hasMore = computed(() => items.value.length < total.value)
async function updateBadge() {
  const badge = navigator as Navigator & {setAppBadge?: (count:number)=>Promise<void>;clearAppBadge?:()=>Promise<void>}
  try { const count=serverUnread.value; if(count) await badge.setAppBadge?.(count); else await badge.clearAppBadge?.() } catch {}
}
async function markActiveRead(): Promise<void> {
  if (marking) return marking
  if (document.hidden || !selected.value || disposed) return
  marking = (async () => {
    try {
      // Re-read the active thread after each request: the user may switch conversations
      // or a refresh may replace its message objects while the read request is in flight.
      while (!document.hidden && !disposed && selected.value) {
        const message = active.value?.messages.find(m => m.status === 'unread')
        if (!message) break
        await setSMSRead(message.id, true)
        const current = items.value.find(m => m.id === message.id)
        if (current?.status === 'unread') {
          current.status = 'read'
          serverUnread.value = Math.max(0, serverUnread.value - 1)
        }
      }
    } catch (e) { if (!disposed) error.value = e instanceof Error ? e.message : '未能更新已读状态' }
  })().finally(() => { marking = undefined; void updateBadge() })
  return marking
}
async function load(mode: 'reset' | 'latest' | 'older' = 'latest') {
  if (disposed) return
  if (busy.value) { if (mode === 'latest') pendingRefresh = true; return }
  busy.value = true
  loadingOlder.value = mode === 'older'
  const version = generation, target = mode === 'older' ? page.value + 1 : 1
  const nearBottom = !timeline.value || timeline.value.scrollHeight - timeline.value.scrollTop - timeline.value.clientHeight < 90
  try {
    const res = await getSMSPage(target, 100, query.value)
    if (disposed || version !== generation) return
    const merged = new Map((mode === 'reset' ? [] : items.value).map(m => [m.id, m]))
    for (const m of res.items || []) { const prior=merged.get(m.id); merged.set(m.id, prior && JSON.stringify(prior) === JSON.stringify(m) ? prior : m) }
    const fresh = [...merged.values()]
    if (JSON.stringify(fresh) !== JSON.stringify(items.value)) items.value = fresh
    total.value = res.total
    serverUnread.value = res.unread_total ?? fresh.filter(m=>m.status==='unread').length
    if (mode !== 'latest') page.value = target
    error.value = ''; await markActiveRead(); await nextTick()
    void updateBadge()
    if (nearBottom && mode !== 'older' && timeline.value) timeline.value.scrollTop = timeline.value.scrollHeight
  } catch (e) { error.value = e instanceof Error ? e.message : '暂时无法连接，请重试' }
  finally {
    busy.value = false; initialLoading.value = false; loadingOlder.value = false
    if (pendingRefresh) {
      pendingRefresh = false
      // A push received during this request must not wait for the next polling tick.
      if (!disposed && !document.hidden && !searchTimer) void load()
    }
  }
}
async function openThread(key: string) {
  selected.value = key; await nextTick()
  if (timeline.value) timeline.value.scrollTop = timeline.value.scrollHeight
  void markActiveRead()
}
async function copyMessage(m: SMSMessage) {
  try {
    await navigator.clipboard.writeText(m.body); copied.value = m.id
    clearTimeout(copyTimer); copyTimer = setTimeout(() => copied.value = null, 1800)
  } catch { error.value = '复制失败，可长按短信文字进行复制' }
}
function listTime(iso: string) {
  const date = new Date(iso), now = new Date()
  if (date.toDateString() === now.toDateString()) return date.toLocaleTimeString('zh-CN', {hour:'2-digit', minute:'2-digit', hour12:false})
  const yesterday = new Date(); yesterday.setDate(yesterday.getDate()-1)
  if (date.toDateString() === yesterday.toDateString()) return '昨天'
  return date.toLocaleDateString('zh-CN', {month:'numeric', day:'numeric'})
}
function messageTime(iso: string) { return new Date(iso).toLocaleString('zh-CN', {month:'long', day:'numeric', hour:'2-digit', minute:'2-digit', hour12:false}) }
function onPush(event: MessageEvent) { if(event.data?.type === 'sms-push' && !document.hidden) void load() }
function onVisible() { if (!document.hidden) void load() }
function onKey(e: KeyboardEvent) { if (e.key === 'Escape') { settingsOpen.value = false; selected.value = '' } }
watch(query, () => {
  generation++; selected.value = ''; clearTimeout(searchTimer)
  const update = () => {
    if (disposed) return
    if (busy.value) { searchTimer = setTimeout(update, 100); return }
    searchTimer = undefined; void load('reset')
  }
  searchTimer = setTimeout(update, 250)
})
onMounted(() => {
  document.documentElement.classList.add('messages-route')
  void load('reset')
  navigator.serviceWorker?.addEventListener('message', onPush)
  timer = setInterval(() => { if (!document.hidden && !searchTimer) void load() }, 5000)
  document.addEventListener('visibilitychange', onVisible); window.addEventListener('online', onVisible); window.addEventListener('keydown', onKey)
})
onBeforeUnmount(() => {
  document.documentElement.classList.remove('messages-route')
  disposed = true; navigator.serviceWorker?.removeEventListener('message', onPush); clearInterval(timer); clearTimeout(searchTimer); clearTimeout(copyTimer)
  document.removeEventListener('visibilitychange', onVisible); window.removeEventListener('online', onVisible); window.removeEventListener('keydown', onKey)
})
</script>
<template>
  <main class="messages-app" :class="{ 'conversation-open': selected }">
    <section class="inbox" aria-label="短信会话列表">
      <header class="inbox-header">
        <div class="inbox-title">
          <h1>信息</h1>
          <button class="icon-button" @click="settingsOpen = true" aria-label="打开设置"><Settings :size="22" /></button>
        </div>
        <div v-if="!pwaInstalled" class="install-shortcut"><PWAInstall /></div>
        <label class="search"><Search :size="19" /><input v-model="query" type="search" placeholder="搜索号码或短信" aria-label="搜索号码或短信" /></label>
      </header>
      <div v-if="error" class="connection-error" role="alert">{{ error }}<button @click="load()">重试</button></div>
      <div class="thread-list">
        <button v-for="thread in threads" :key="thread.key" class="thread" :class="{ selected: selected === thread.key, unread: thread.unread }" @click="openThread(thread.key)">
          <span class="unread-dot" :class="{ visible: thread.unread }" aria-hidden="true"></span><span class="avatar"><svg width="48" height="48" viewBox="0 0 48 48" aria-hidden="true"><circle cx="24" cy="17" r="7" fill="currentColor"/><path d="M11 37c0-6 5-10 13-10s13 4 13 10c-7 5-19 5-26 0Z" fill="currentColor"/></svg></span>
          <span class="thread-copy"><span class="thread-top"><strong>{{ thread.latest.sender || '未知号码' }}<span v-if="thread.unread" class="sr-only">（未读）</span></strong><time>{{ listTime(thread.latest.received_at) }}</time><ChevronRight :size="16" /></span><span class="preview">{{ thread.latest.body || '（空短信）' }}</span></span>
        </button>
        <div v-if="!threads.length" class="empty-list"><MessageCircle :size="46" :stroke-width="1.3" /><h2>{{ initialLoading ? '正在加载…' : query ? '没有搜索结果' : '暂无信息' }}</h2><p v-if="!initialLoading && !query">收到的短信会自动显示在这里</p></div>
        <button v-if="hasMore" class="load-older" :disabled="loadingOlder" @click="load('older')">{{ loadingOlder ? '正在加载…' : '加载更早的信息' }}</button>
      </div>
      
    </section>
    <section class="conversation" aria-label="短信内容">
      <template v-if="active">
        <header class="conversation-header"><button class="back" @click="selected = ''"><ChevronLeft :size="28" />信息</button><div class="contact"><span class="avatar"><svg width="44" height="44" viewBox="0 0 48 48" aria-hidden="true"><circle cx="24" cy="17" r="7" fill="currentColor"/><path d="M11 37c0-6 5-10 13-10s13 4 13 10c-7 5-19 5-26 0Z" fill="currentColor"/></svg></span><h2>{{ active.latest.sender || '未知号码' }}</h2></div></header>
        <div ref="timeline" class="timeline">
          <button v-if="hasMore" class="load-older" :disabled="loadingOlder" @click="load('older')">加载更早的信息</button>
          <article v-for="m in messages" :key="m.id" class="message"><time>{{ messageTime(m.received_at) }}</time><div class="bubble">{{ m.body || '（空短信）' }}</div><button class="copy" @click="copyMessage(m)" :aria-label="copied === m.id ? '已复制短信' : '复制短信'"><Copy :size="12" />{{ copied === m.id ? '已复制' : '复制' }}</button></article>
        </div>
        
      </template>
      <div v-else class="empty-conversation"><MessageCircle :size="72" :stroke-width="1.1" /><h2>你的信息，在这里</h2><p>选择一个会话，查看收到的短信</p></div>
    </section>
    <div v-if="settingsOpen" class="sheet-backdrop" @click.self="settingsOpen = false">
      <section class="settings-sheet" role="dialog" aria-modal="true" aria-labelledby="settings-title">
        <header><h2 id="settings-title">设置</h2><button class="icon-button" @click="settingsOpen = false" aria-label="关闭设置"><X :size="22" /></button></header>
        <nav><RouterLink to="/notifications">通知提醒<ChevronRight :size="18" /></RouterLink><RouterLink to="/devices">设备状态<ChevronRight :size="18" /></RouterLink><RouterLink to="/settings">账户与设置<ChevronRight :size="18" /></RouterLink><RouterLink to="/dashboard">运行概览<ChevronRight :size="18" /></RouterLink></nav>
        <PushSettings />
        <PWAUpdate />
        <PWAInstall wide />
        <p class="settings-note">仅接收短信，页面静默同步。通知显示发送方和短信内容，锁屏也可能看到。</p>
      </section>
    </div>
  </main>
</template>
<style scoped>
.messages-app { --paper:#fff; --ink:#17171a; --secondary:#707078; --line:#e9e9ed; --field:#f2f2f5; --bubble:#e9e9eb; --blue:#007aff; --safe-top:env(safe-area-inset-top,0px); --safe-bottom:env(safe-area-inset-bottom,0px); position:fixed; inset:0; isolation:isolate; font-family:-apple-system,BlinkMacSystemFont,'SF Pro Text','PingFang SC',sans-serif; color:var(--ink); background:var(--paper); display:grid; grid-template-columns:360px minmax(0,1fr); width:100%; min-height:0; line-height:1.4; overflow:hidden; }
.messages-app button,.messages-app input { font:inherit; }
.messages-app button { cursor:pointer; border:0; background:none; }
.messages-app button:focus-visible,.messages-app a:focus-visible,.messages-app input:focus-visible { outline:2px solid var(--blue); outline-offset:-2px; }
.messages-app button:disabled { opacity:.55; cursor:default; }
.inbox { display:flex; flex-direction:column; min-width:0; min-height:0; overflow:hidden; border-right:1px solid var(--line); }
.inbox-header { position:relative; z-index:1; flex-shrink:0; background:var(--paper); padding:calc(10px + var(--safe-top)) 18px 12px; }
.icon-button { display:grid; place-items:center; width:44px; height:44px; color:var(--blue); border-radius:50%; flex-shrink:0; }
h1 { color:var(--ink); font-size:32px; font-weight:700; letter-spacing:-.8px; margin:0; line-height:44px; }
.inbox-title { display:flex; align-items:center; justify-content:space-between; gap:10px; margin-bottom:14px; }
.install-shortcut { margin:-2px 0 12px; }
.search { display:flex; align-items:center; gap:8px; background:var(--field); color:var(--secondary); border-radius:12px; min-height:44px; padding:10px 12px; }
.search input { min-width:0; width:100%; background:none; border:0; outline:none; color:var(--ink); font-size:16px; }
.search input::placeholder { color:var(--secondary); }
.thread-list { flex:1; min-height:0; overflow:auto; overscroll-behavior:contain; padding:0 0 calc(12px + var(--safe-bottom)); scroll-padding-bottom:calc(12px + var(--safe-bottom)); }
.thread { display:flex; width:100%; align-items:center; padding:0 18px 0 10px; text-align:left; color:var(--ink); min-height:84px; -webkit-tap-highlight-color:transparent; }
.thread.selected { background:var(--field); }
@media(hover:hover) and (pointer:fine) { .thread:hover { background:var(--field); } }
.unread-dot { width:7px; height:7px; border-radius:50%; background:var(--blue); flex-shrink:0; margin-right:6px; visibility:hidden; }
.unread-dot.visible { visibility:visible; }
.avatar { width:42px; height:42px; border-radius:50%; display:grid; place-items:center; background:#afb2ba; color:white; flex-shrink:0; }
.avatar svg { width:40px; height:40px; }
.thread-copy { display:block; flex:1; min-width:0; margin-left:12px; padding:11px 0; border-bottom:1px solid var(--line); }
.thread-top { display:flex; align-items:center; gap:5px; }
.thread-top strong { flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font-size:16px; line-height:21px; font-weight:600; font-variant-numeric:tabular-nums; }
.thread.unread .thread-top strong { font-weight:700; }
.thread-top time { font-size:12px; color:var(--secondary); flex-shrink:0; font-variant-numeric:tabular-nums; }
.thread-top svg { color:#b8b8c0; flex-shrink:0; width:13px; }
.preview { margin-top:3px; color:var(--secondary); font-size:14px; line-height:1.4; display:-webkit-box; -webkit-line-clamp:2; -webkit-box-orient:vertical; overflow:hidden; overflow-wrap:anywhere; min-height:39.2px; }
.empty-list { padding:70px 24px; text-align:center; color:var(--secondary); }
.empty-list h2 { color:var(--ink); font-size:20px; font-weight:600; margin:15px 0 8px; }
.empty-list p { font-size:14px; }
.load-older { color:var(--blue); display:block; margin:12px auto; padding:12px; font-size:14px!important; }
.conversation { display:flex; flex-direction:column; min-width:0; min-height:0; overflow:hidden; }
.conversation-header { position:relative; flex-shrink:0; padding:calc(12px + var(--safe-top)) 16px 12px; border-bottom:1px solid var(--line); background:var(--field); }
.contact { display:flex; flex-direction:column; align-items:center; gap:5px; }
.contact .avatar { width:44px; height:44px; }
.contact h2 { color:var(--ink); font-size:14px; font-weight:500; line-height:1.3; max-width:60%; overflow-wrap:anywhere; }
.back { position:absolute; left:10px; top:calc(24px + var(--safe-top)); min-height:44px; color:var(--blue); align-items:center; font-size:17px!important; display:none; }
.timeline { flex:1; min-height:0; overflow:auto; padding:20px 26px calc(20px + var(--safe-bottom)); overscroll-behavior:contain; }
.message { margin-bottom:17px; display:flex; flex-direction:column; align-items:flex-start; }
.message time { align-self:center; color:var(--secondary); font-size:12px; margin:6px 0 16px; }
.bubble { position:relative; background:var(--bubble); border-radius:19px; padding:10px 15px; max-width:78%; white-space:pre-wrap; overflow-wrap:anywhere; font-size:17px; line-height:1.4; user-select:text; -webkit-user-select:text; }
.bubble::before { content:''; position:absolute; bottom:0; left:-6px; width:13px; height:16px; background:var(--bubble); border-bottom-right-radius:12px; }
.bubble::after { content:''; position:absolute; bottom:0; left:-10px; width:10px; height:19px; background:var(--paper); border-bottom-right-radius:9px; }
.copy { display:flex; align-items:center; gap:4px; margin:7px 5px 0; font-size:11px!important; color:var(--secondary); padding:3px; }
.empty-conversation { display:grid; align-content:center; justify-items:center; height:100%; gap:12px; color:var(--secondary); }
.empty-conversation h2 { font-weight:600; font-size:22px; color:var(--ink); }
.empty-conversation p { font-size:14px; }
.connection-error { background:#fff2f0; color:#b42318; padding:10px 16px; font-size:13px; }
.connection-error button { color:var(--blue); margin-left:8px; }
.sheet-backdrop { position:fixed; inset:0; background:#0005; z-index:100; display:flex; align-items:center; justify-content:center; padding:calc(12px + var(--safe-top)) 20px calc(12px + var(--safe-bottom)); }
.settings-sheet { max-height:100%; overflow-y:auto; background:var(--field); width:100%; max-width:400px; border-radius:22px; padding:20px; box-shadow:0 20px 80px #0003; }
.settings-sheet header { display:flex; align-items:center; justify-content:space-between; margin-bottom:18px; }
.settings-sheet h2 { font-size:22px; font-weight:700; }
.settings-sheet nav { background:var(--paper); border-radius:12px; overflow:hidden; }
.settings-sheet a { display:flex; justify-content:space-between; align-items:center; color:var(--ink); padding:14px 16px; font-size:16px; border-bottom:1px solid var(--line); }
.settings-sheet a:last-child { border:0; }
.settings-sheet a svg { color:var(--secondary); }
.settings-note { color:var(--secondary); font-size:13px; line-height:1.6; padding:14px 4px 0; }
.sr-only { position:absolute; width:1px; height:1px; overflow:hidden; clip-path:inset(50%); }
.dark .messages-app { --paper:#000; --ink:#fff; --secondary:#98989e; --line:#38383a; --field:#1c1c1e; --bubble:#262628; --blue:#0a84ff; }
@media(max-width:700px) { .messages-app { grid-template-columns:1fr; } .inbox { border:0; } .conversation { display:none; } .conversation-open .inbox { display:none; } .conversation-open .conversation { display:flex; } .back { display:flex; } .timeline { padding:20px 22px calc(20px + var(--safe-bottom)); } .bubble { max-width:86%; } .sheet-backdrop { align-items:flex-end; padding-left:12px; padding-right:12px; } }
</style>
<style>
html.messages-route, html.messages-route body, html.messages-route #app { width:100%; height:100%; margin:0; overflow:hidden; overscroll-behavior:none; background:#fff; }
html.dark.messages-route, html.dark.messages-route body, html.dark.messages-route #app { background:#000; }
</style>
