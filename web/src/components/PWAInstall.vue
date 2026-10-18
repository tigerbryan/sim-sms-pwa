<script setup lang="ts">
import { computed, onBeforeUnmount, ref, useId, watch } from 'vue'
import { Download, Copy, X } from 'lucide-vue-next'
import { usePWAInstall } from '../composables/pwaInstall'

defineProps<{ wide?: boolean }>()
const { installed, prompting, promptInstall } = usePWAInstall()
const dialog = ref<HTMLDialogElement | null>(null)
const titleId = useId()
const copyStatus = ref('')
const ua = navigator.userAgent
const ios = /iPad|iPhone|iPod/.test(ua) || (/Macintosh/.test(ua) && navigator.maxTouchPoints > 1)
const android = /Android/.test(ua)
const mobile = ios || android
const mac = !ios && /Macintosh|Mac OS X/.test(ua)
const safari = /Safari/.test(ua) && !/Chrome|Chromium|CriOS|Edg|OPR|FxiOS|Firefox/.test(ua)
const edge = /Edg\//.test(ua)
const chrome = /Chrome|Chromium/.test(ua) && !edge
const label = mobile ? '添加到主屏幕' : '安装到桌面'
const appURL = `${window.location.origin}/sms`
const steps = computed(() => {
  if (ios) return ['用 Safari 打开下面的网址并登录。', '点“分享”，选择“添加到主屏幕”。', '名称保留为“信息”，点“添加”。']
  if (mac && safari) return ['在 Safari 菜单栏点“文件” → “添加到程序坞”。', '名称保留为“信息”，点“添加”。', '以后从程序坞打开“信息”即可。']
  if (android) return ['用 Chrome 打开下面的网址并登录。', '点右上角菜单，选择“安装应用”或“添加到主屏幕”。', '按提示确认添加“信息”。']
  if (edge) return ['点 Edge 右上角“…”菜单。', '在“应用”中选择“将此站点作为应用安装”。', '名称保留为“信息”，确认安装。']
  if (chrome) return ['点地址栏右侧的安装图标，或打开 Chrome 菜单。', '在“投放、保存和分享”中选择“将网页安装为应用”。', '名称保留为“信息”，确认安装。']
  return ['复制下面的网址，在 Safari、Chrome 或 Edge 中打开。', 'Safari：菜单“文件” → “添加到程序坞”。', 'Chrome / Edge：点击地址栏的安装图标，按提示安装。']
})
async function install() {
  if (prompting.value || installed.value) return
  copyStatus.value = ''
  if (!await promptInstall() && !installed.value) dialog.value?.showModal()
}
async function copyURL() {
  try {
    await navigator.clipboard.writeText(appURL)
    copyStatus.value = '网址已复制'
  } catch {
    copyStatus.value = '请长按或选中上方网址复制'
  }
}
watch(installed, (value) => { if (value && dialog.value?.open) dialog.value.close() })
onBeforeUnmount(() => { if (dialog.value?.open) dialog.value.close() })
</script>

<template>
  <button v-if="!installed" type="button" class="pwa-install" :class="{ wide }" :disabled="prompting" @click="install">
    <Download :size="16" aria-hidden="true" />{{ prompting ? '正在打开…' : label }}
  </button>
  <dialog ref="dialog" class="install-dialog" :aria-labelledby="titleId" @keydown.esc.stop @click="($event.target === dialog) && dialog?.close()">
    <header><img src="/icons/messages-192.png" width="52" height="52" alt="" /><button type="button" class="close-install" aria-label="关闭安装说明" @click="dialog?.close()"><X :size="22" /></button></header>
    <h2 :id="titleId">把“信息”{{ mobile ? '添加到主屏幕' : '安装到桌面' }}</h2>
    <p class="intro">像 App 一样打开短信，也能接收新短信提醒。</p>
    <p v-if="!ios && !(mac && safari)" class="install-explanation" role="status">当前浏览器没有打开安装窗口。若在应用内浏览器中，请复制网址，用完整的 Chrome 或 Edge 打开后再点“安装到桌面”。</p>
    <ol><li v-for="step in steps" :key="step">{{ step }}</li></ol>
    <p v-if="mac && safari" class="hint">需要 macOS 14 或更新版本。Safari 的安装需要在浏览器菜单中确认。</p>
    <p v-else class="hint">若已安装，可直接从主屏幕或应用列表打开。在应用内浏览器中，请复制网址到上述浏览器。</p>
    <div class="copy-url"><input :value="appURL" readonly aria-label="短信网页地址" /><button type="button" @click="copyURL"><Copy :size="15" />复制网址</button></div>
    <p v-if="copyStatus" class="copy-status" role="status">{{ copyStatus }}</p>
    <p class="hint">安装后在“信息”的设置里启用通知，并允许系统通知权限。</p>
    <button type="button" class="done-install" @click="dialog?.close()">知道了</button>
  </dialog>
</template>

<style scoped>
.pwa-install { display:inline-flex; align-items:center; justify-content:center; gap:5px; flex-shrink:0; color:var(--blue,#007aff); background:var(--field,#efeff0)!important; border:0; border-radius:99px; padding:8px 11px; font-size:13px!important; font-weight:600!important; cursor:pointer; }
.pwa-install.wide { width:100%; background:var(--paper,#fff)!important; border-radius:12px; padding:14px; margin-top:16px; font-size:16px!important; }
.pwa-install:disabled { opacity:.55; }
.install-dialog { color:var(--ink,#000); background:var(--paper,#fff); width:min(420px,calc(100vw - 32px)); max-height:calc(100dvh - 48px); box-sizing:border-box; overflow:auto; border:0; border-radius:24px; margin:auto; padding:24px; box-shadow:0 24px 80px #0003; font:15px/1.5 -apple-system,BlinkMacSystemFont,'PingFang SC',sans-serif; }
.install-dialog::backdrop { background:#0006; }
.install-dialog header { display:flex; justify-content:space-between; align-items:flex-start; }
.install-dialog img { border-radius:13px; }
.install-dialog button { cursor:pointer; font:inherit; border:0; }
.install-dialog button:focus-visible,.install-dialog input:focus-visible { outline:2px solid var(--blue,#007aff); outline-offset:2px; }
.close-install { color:var(--secondary,#8e8e93); background:var(--field,#efeff0)!important; display:grid; place-items:center; width:32px; height:32px; border-radius:50%; }
.install-dialog h2 { color:var(--ink,#000); font-size:23px; font-weight:700; letter-spacing:-.5px; margin:20px 0 8px; }
.intro,.hint { color:var(--secondary,#8e8e93); }
.intro { margin:0 0 22px; }
.install-explanation { color:var(--ink,#000); background:var(--field,#efeff0); padding:12px; border-radius:12px; font-size:13px; margin-bottom:18px; }
.install-dialog ol { padding-left:23px; margin:0 0 20px; list-style:decimal; }
.install-dialog li { margin:12px 0; padding-left:3px; }
.hint { font-size:12px; line-height:1.6; margin:14px 0; }
.copy-url { display:flex; flex-wrap:wrap; align-items:center; gap:8px; background:var(--field,#efeff0); border-radius:12px; padding:10px 12px; }
.copy-url input { flex:1; width:100%; min-width:150px; color:var(--ink,#000); font-family:inherit; font-size:12px; line-height:1.5; background:none; border:0; }
.copy-url button { color:var(--blue,#007aff); display:flex; align-items:center; gap:4px; font-size:12px; padding:3px 0; white-space:nowrap; }
.copy-status { color:var(--blue,#007aff); font-size:12px; margin:8px 0; }
.done-install { width:100%; border-radius:12px; background:var(--blue,#007aff)!important; color:#fff; padding:12px; margin-top:6px; font-weight:600!important; }
</style>
