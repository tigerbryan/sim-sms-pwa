import { readonly, ref, shallowRef } from 'vue'

interface InstallPrompt extends Event {
  prompt(): Promise<{ outcome: 'accepted' | 'dismissed' } | void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

const pendingPrompt = shallowRef<InstallPrompt | null>(null)
const installed = ref(false)
const prompting = ref(false)
let initialized = false

// Capture this before login/routing: the browser may offer installation before
// the inbox is mounted. An absent prompt does not mean the app is installed.
export function initPWAInstall() {
  if (initialized || typeof window === 'undefined') return
  initialized = true
  const standalone = window.matchMedia?.('(display-mode: standalone)')
  const syncDisplayMode = () => {
    installed.value = Boolean(standalone?.matches || (navigator as Navigator & { standalone?: boolean }).standalone)
  }
  syncDisplayMode()
  standalone?.addEventListener('change', syncDisplayMode)
  window.addEventListener('beforeinstallprompt', (event) => {
    event.preventDefault()
    pendingPrompt.value = event as InstallPrompt
  })
  window.addEventListener('appinstalled', () => {
    installed.value = true
    pendingPrompt.value = null
  })
}

export function usePWAInstall() {
  async function promptInstall(): Promise<boolean> {
    const prompt = pendingPrompt.value
    if (!prompt || prompting.value) return false
    pendingPrompt.value = null // Browser prompts are single-use, even if dismissed.
    prompting.value = true
    let timeout: ReturnType<typeof setTimeout> | undefined
    try {
      // Invoke synchronously while the button's user activation is still valid.
      // Modern browsers return the choice; older implementations return void.
      const choice = Promise.resolve(prompt.prompt()).then(result => result ?? prompt.userChoice)
      // Embedded browsers may expose the event without displaying install UI.
      // Stop showing a spinner and offer guidance; never infer installation.
      return await Promise.race([
        choice.then(() => true),
        new Promise<boolean>(resolve => { timeout = setTimeout(() => resolve(false), 8000) })
      ])
    } catch {
      return false
    } finally {
      clearTimeout(timeout)
      prompting.value = false
    }
  }
  return { installed: readonly(installed), prompting: readonly(prompting), promptInstall }
}
