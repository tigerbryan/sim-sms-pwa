import { readonly, ref } from 'vue'

type UpdateStatus = 'idle' | 'checking' | 'current' | 'available' | 'updating' | 'error'
interface Release { id: string; version: string }
class UpdateError extends Error {}
const loadedRelease = document.querySelector<HTMLMetaElement>('meta[name="app-release"]')?.content ?? ''
const currentVersion = document.querySelector<HTMLMetaElement>('meta[name="app-version"]')?.content ?? ''
const status = ref<UpdateStatus>('idle')
const message = ref('检查已发布的应用和图标更新。')
let registration: Promise<ServiceWorkerRegistration | null> | undefined
let latestRelease: Release | undefined

function registerWorker(): Promise<ServiceWorkerRegistration | null> {
  if (!window.isSecureContext || typeof navigator.serviceWorker?.register !== 'function') return Promise.resolve(null)
  registration ??= navigator.serviceWorker.register('/sw.js', { updateViaCache: 'none' }).catch(error => {
    registration = undefined
    throw error
  })
  return registration
}

async function fetchRelease(): Promise<Release> {
  const response = await fetch('/pwa-release.json', {
    cache: 'no-store', credentials: 'same-origin', signal: AbortSignal.timeout(12000), headers: { Accept: 'application/json' }
  })
  if (!response.ok || !response.headers.get('content-type')?.includes('application/json')) throw new UpdateError('无法检查更新，请确认网络连接和网页登录状态。')
  const release = await response.json() as Release
  if (!/^[a-f0-9]{20}$/.test(release.id) || typeof release.version !== 'string') throw new UpdateError('更新信息暂不可用，请稍后重试。')
  return release
}

export async function checkForUpdates(quiet = false) {
  if (status.value === 'checking' || status.value === 'updating') return
  status.value = 'checking'
  message.value = '正在检查更新…'
  try {
    latestRelease = await fetchRelease()
    const worker = await registerWorker()
    await worker?.update()
    status.value = latestRelease.id === loadedRelease ? 'current' : 'available'
    message.value = status.value === 'current' ? '已经是最新版本。' : `发现新版本 ${latestRelease.version}，更新后将重新打开页面。`
  } catch (error) {
    status.value = quiet ? 'idle' : 'error'
    message.value = quiet ? '检查已发布的应用和图标更新。' : error instanceof UpdateError ? error.message : '无法连接更新服务，请确认网络后重试。'
  }
}

function waitForInstall(worker: ServiceWorkerRegistration): Promise<void> {
  const installing = worker.installing
  if (!installing) return Promise.resolve()
  return new Promise((resolve, reject) => {
    const finish = () => {
      if (installing.state !== 'installed' && installing.state !== 'activated' && installing.state !== 'redundant') return
      clearTimeout(timeout)
      installing.removeEventListener('statechange', finish)
      if (installing.state === 'redundant') reject(new UpdateError('新版下载未完成，请重新检查更新。'))
      else resolve()
    }
    const timeout = setTimeout(() => {
      installing.removeEventListener('statechange', finish)
      reject(new UpdateError('更新下载超时，请稍后重试。'))
    }, 15000)
    installing.addEventListener('statechange', finish)
    finish()
  })
}

async function activateWorker(worker: ServiceWorkerRegistration) {
  await waitForInstall(worker)
  if (!worker.waiting) return
  await new Promise<void>((resolve, reject) => {
    const cleanup = () => { clearTimeout(timeout); navigator.serviceWorker.removeEventListener('controllerchange', changed) }
    const changed = () => { cleanup(); resolve() }
    const timeout = setTimeout(() => { cleanup(); reject(new UpdateError('更新尚未完成，请稍后再试。')) }, 12000)
    navigator.serviceWorker.addEventListener('controllerchange', changed)
    worker.waiting!.postMessage({ type: 'SKIP_WAITING' })
  })
}

export async function applyUpdate() {
  if (status.value !== 'available') return
  status.value = 'updating'
  message.value = '正在更新应用与图标…'
  try {
    // Verify connectivity again before navigation. Never delete login data or unregister push.
    latestRelease = await fetchRelease()
    const worker = await registerWorker()
    if (worker) { await worker.update(); await activateWorker(worker) }
    const assets = ['/manifest.webmanifest', '/icons/messages-192.png', '/icons/messages-512.png', '/icons/apple-touch-icon.png']
    await Promise.all(assets.map(async path => {
      const response = await fetch(`${path}?v=${latestRelease!.id}`, { cache: 'reload', credentials: 'same-origin', signal: AbortSignal.timeout(12000) })
      if (!response.ok || response.headers.get('content-type')?.includes('text/html')) throw new UpdateError('图标资源未能更新，请重试。')
      await response.arrayBuffer()
    }))
    window.location.reload()
  } catch (error) {
    status.value = 'error'
    message.value = error instanceof UpdateError ? error.message : '更新未完成，请确认网络后重试。'
  }
}

export function initPWAUpdates() {
  void registerWorker().then(() => checkForUpdates(true)).catch(() => {})
}

export function usePWAUpdate() {
  return { status: readonly(status), message: readonly(message), currentVersion, checkForUpdates, applyUpdate }
}
