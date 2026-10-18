<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { api, jsonBody } from '../api/client'
import { Bell, BellRing } from 'lucide-vue-next'
const supported = typeof Notification !== 'undefined' && 'serviceWorker' in navigator && 'PushManager' in window && window.isSecureContext
const ios = /iPhone|iPad|iPod/.test(navigator.userAgent) || (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
const mac = !ios && /Macintosh|Mac OS X|MacIntel/.test(`${navigator.userAgent} ${navigator.platform}`)
const standalone = window.matchMedia('(display-mode: standalone)').matches || !!(navigator as Navigator & {standalone?:boolean}).standalone
const needsInstall = ios && !standalone
const loading = ref(true), busy = ref(false), subscribed = ref(false), status = ref(''), error = ref('')
let publicKey = '', subscriptionID = '', disposed = false
let statusTimer: ReturnType<typeof setTimeout> | undefined
function fromBase64(value: string): ArrayBuffer {
  const raw = atob(value.replace(/-/g,'+').replace(/_/g,'/'))
  const bytes = new Uint8Array(raw.length); for(let i=0;i<raw.length;i++) bytes[i]=raw.charCodeAt(i)
  return bytes.buffer
}
async function registration() {
  let timeout: ReturnType<typeof setTimeout> | undefined
  try { return await Promise.race([navigator.serviceWorker.ready, new Promise<never>((_, reject) => { timeout = setTimeout(() => reject(new Error('请刷新页面后重试启用通知')), 10000) })]) }
  finally { clearTimeout(timeout) }
}
async function save(sub: PushSubscription) {
  const reply = await api<{id:string}>('/api/v1/webpush/subscribe', jsonBody('POST', sub.toJSON()))
  subscriptionID = reply.id; subscribed.value = true
}
async function enable() {
  if (busy.value || !supported || needsInstall) return
  busy.value = true; error.value = ''; status.value = ''
  // Permission must be requested directly from this tap, before network awaits on iOS.
  try {
    const permission = await Notification.requestPermission()
    if (permission !== 'granted') throw new Error(permission === 'denied' ? '通知已被拒绝，请在浏览器或此设备的系统通知设置中允许“信息”发送通知。' : '尚未允许通知，请再点一次启用。')
    const worker = await registration()
    const sub = await worker.pushManager.getSubscription() || await worker.pushManager.subscribe({userVisibleOnly:true, applicationServerKey:fromBase64(publicKey)})
    await save(sub); status.value = '此设备已订阅通知，请发送测试通知确认能收到提醒。'
  } catch(e) {error.value = e instanceof Error ? (/permission denied/i.test(e.message) ? (ios ? '系统未允许推送。请从主屏幕打开“信息”，并在“设置 → 通知”中允许通知。' : '系统未允许推送，请检查浏览器和此设备的系统通知权限，再重新打开“信息”重试。') : /Registration failed/i.test(e.message) ? '系统推送注册失败，请重新打开“信息”后重试。' : e.message) : '启用通知失败，请重试'}
  finally {busy.value=false}
}
async function test() {
  busy.value=true; error.value=''
  try {
    await api('/api/v1/webpush/test',jsonBody('POST',{id:subscriptionID}))
    status.value='测试通知已排队，请查看系统通知。'
    statusTimer = setTimeout(async () => {
      try {
        const res=await api<{last_delivery?:string}>(`/api/v1/webpush/status?id=${subscriptionID}`)
        if(disposed)return
        if(res.last_delivery==='success') status.value='推送服务已接收测试通知，请确认此设备是否出现提醒。'
        else if(res.last_delivery==='failed') error.value='推送未送达，请关闭后重新启用通知。'
        else if(res.last_delivery==='retry') status.value='推送服务暂时不可达，系统会自动重试。'
      } catch {/* Retain the queue confirmation if the status request fails. */}
    },2500)
  } catch(e){error.value=e instanceof Error?e.message:'发送测试失败'} finally{busy.value=false}
}
async function disable() {
  busy.value=true; error.value=''
  try {
    const worker=await registration(), sub=await worker.pushManager.getSubscription()
    if(sub && !await sub.unsubscribe()) throw new Error('关闭通知失败，请重试')
    await api('/api/v1/webpush/subscribe',jsonBody('DELETE',{id:subscriptionID}))
    subscribed.value=false; status.value='这台设备的推送已关闭。'
  }catch(e){error.value=e instanceof Error?e.message:'关闭通知失败'}finally{busy.value=false}
}
onMounted(async () => {
  try {
    const res=await api<{public_key:string}>('/api/v1/webpush/status');publicKey=res.public_key
    if(supported&&!needsInstall&&Notification.permission==='granted') {
      const worker=await registration(), sub=await worker.pushManager.getSubscription()
      if(sub) await save(sub)
    }
  }catch(e){error.value=e instanceof Error?e.message:'通知服务暂时不可用'}finally{loading.value=false}
})
onBeforeUnmount(()=>{disposed=true;clearTimeout(statusTimer)})
</script>
<template>
  <section class="push-settings" aria-label="此设备系统通知">
    <div class="push-heading"><BellRing v-if="subscribed" :size="19"/><Bell v-else :size="19"/><strong>此设备系统通知</strong><span>{{ subscribed ? '已启用' : '未启用' }}</span></div>
    <p v-if="needsInstall">先用 Safari 将网页“添加到主屏幕”，从主屏幕打开后，再启用通知。</p>
    <p v-else-if="!supported">请使用支持系统通知的浏览器；iPhone 或 iPad 需从主屏幕打开“信息”。</p>
    <p v-else>允许后，新短信可通过此设备的系统通知提醒。每台设备需要分别启用。</p>
    <p v-if="mac">Mac：打开“系统设置 → 通知”，选择由此网页安装的“信息”，开启“允许通知”和“标记应用程序图标”。若没有“信息”，请检查用于安装它的浏览器的通知设置。桌面未读角标需要安装此网页 App，并在系统中允许显示。</p>
    <div class="push-actions"><button v-if="!subscribed" @click="enable" :disabled="loading||busy||needsInstall||!supported||!publicKey">{{ loading ? '准备中…' : busy ? '正在启用…' : '启用系统通知' }}</button><template v-else><button @click="test" :disabled="busy">发送测试通知</button><button class="secondary" @click="disable" :disabled="busy">关闭</button></template></div>
    <p v-if="status" class="push-result" role="status">{{ status }}</p><p v-if="error" class="push-error" role="alert">{{ error }}</p>
  </section>
</template>
<style scoped>
.push-settings {background:var(--paper,var(--bg-surface));color:var(--ink,var(--text-primary));border-radius:12px;padding:16px;margin:16px 0;line-height:1.5;font-family:-apple-system,BlinkMacSystemFont,'PingFang SC',sans-serif;}
.push-heading {display:flex;align-items:center;gap:8px;font-size:15px}.push-heading svg {color:#007aff}.push-heading span {margin-left:auto;color:var(--secondary,var(--text-muted));font-size:12px}.push-settings p {font-size:13px;color:var(--secondary,var(--text-secondary));margin:10px 0 0}.push-actions{display:flex;gap:10px;margin-top:14px}.push-actions button {border:0!important;border-radius:10px!important;padding:10px 14px!important;font-size:14px!important;background:#007aff!important;color:white!important;cursor:pointer}.push-actions button.secondary{background:var(--field,var(--bg-interactive))!important;color:#007aff!important}.push-actions button:disabled{opacity:.5;cursor:default}.push-settings .push-result{color:#21804a}.push-settings .push-error{color:#cc342d}
</style>
