import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'

const id = 'a'.repeat(20), newer = 'b'.repeat(20)
let fetchMock: ReturnType<typeof vi.fn>
beforeEach(() => {
  vi.resetModules()
  document.head.innerHTML = `<meta name="app-release" content="${id}"><meta name="app-version" content="1.0.1">`
  fetchMock = vi.fn()
  vi.stubGlobal('fetch', fetchMock)
  Object.defineProperty(window, 'isSecureContext', { configurable: true, value: true })
  Object.defineProperty(navigator, 'serviceWorker', { configurable: true, value: undefined })
})
afterEach(() => vi.unstubAllGlobals())
const release = (value: string) => new Response(JSON.stringify({ id: value, version: '1.0.1' }), { headers: { 'Content-Type': 'application/json' } })

describe('PWA application updates', () => {
  it('compares deployed content, rather than only the backend version label', async () => {
    fetchMock.mockResolvedValueOnce(release(id)).mockResolvedValueOnce(release(newer))
    const pwa = await import('./pwaUpdate')
    await pwa.checkForUpdates()
    expect(pwa.usePWAUpdate().status.value).toBe('current')
    await pwa.checkForUpdates()
    expect(pwa.usePWAUpdate().status.value).toBe('available')
    expect(fetchMock).toHaveBeenCalledWith('/pwa-release.json', expect.objectContaining({ cache: 'no-store' }))
  })

  it('does not mistake a Cloudflare sign-in HTML response for an update', async () => {
    fetchMock.mockResolvedValue(new Response('<html>Sign in</html>', { headers: { 'Content-Type': 'text/html' } }))
    const pwa = await import('./pwaUpdate')
    await pwa.checkForUpdates()
    expect(pwa.usePWAUpdate().status.value).toBe('error')
    expect(pwa.usePWAUpdate().message.value).toContain('登录状态')
  })

  it('leaves stored sessions untouched and offers retry on network failure', async () => {
    const values = new Map<string, string>()
    const storage = { setItem: (key: string, value: string) => values.set(key, value), getItem: (key: string) => values.get(key), clear: vi.fn(), removeItem: vi.fn() }
    vi.stubGlobal('localStorage', storage)
    vi.stubGlobal('sessionStorage', storage)
    window.localStorage.setItem('retained-setting', 'keep')
    window.sessionStorage.setItem('qmi-web-csrf', 'keep')
    fetchMock.mockRejectedValue(new TypeError('Failed to fetch'))
    const pwa = await import('./pwaUpdate')
    await pwa.checkForUpdates()
    expect(pwa.usePWAUpdate().status.value).toBe('error')
    expect(pwa.usePWAUpdate().message.value).toContain('重试')
    expect(window.localStorage.getItem('retained-setting')).toBe('keep')
    expect(window.sessionStorage.getItem('qmi-web-csrf')).toBe('keep')
  })

  it('does not activate a waiting worker until the user chooses to update', async () => {
    const waiting = { postMessage: vi.fn() }
    const reg = { update: vi.fn().mockResolvedValue(undefined), waiting, installing: null }
    const serviceWorker = new EventTarget() as EventTarget & { register: ReturnType<typeof vi.fn> }
    serviceWorker.register = vi.fn().mockResolvedValue(reg)
    Object.defineProperty(navigator, 'serviceWorker', { configurable: true, value: serviceWorker })
    fetchMock.mockResolvedValue(release(newer))
    const pwa = await import('./pwaUpdate')
    await pwa.checkForUpdates()
    expect(pwa.usePWAUpdate().status.value).toBe('available')
    expect(waiting.postMessage).not.toHaveBeenCalled()
    expect(serviceWorker.register).toHaveBeenCalledWith('/sw.js', { updateViaCache: 'none' })
  })

  it('stops an update when connectivity is lost before application', async () => {
    fetchMock.mockResolvedValueOnce(release(newer)).mockRejectedValue(new TypeError('Offline'))
    const pwa = await import('./pwaUpdate')
    await pwa.checkForUpdates()
    await pwa.applyUpdate()
    expect(pwa.usePWAUpdate().status.value).toBe('error')
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })
})
