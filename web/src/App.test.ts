import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App.vue'

const { route, replace, session, version } = vi.hoisted(() => ({
  route: { path: '/' },
  replace: vi.fn(),
  session: {
    checked: true,
    authenticated: true,
    username: 'admin',
    error: '',
    bootstrap: vi.fn(),
    logout: vi.fn(),
    markUnauthenticated: vi.fn()
  },
  version: { headerLabel: 'v1.0.0', load: vi.fn() }
}))

vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({ replace })
}))
vi.mock('./stores/session', () => ({ useSessionStore: () => session }))
vi.mock('./stores/version', () => ({ useVersionStore: () => version }))
vi.mock('./stores/theme', () => ({
  useThemeStore: () => ({ mode: 'auto', applyTheme: vi.fn(), isSystemDark: false, toggle: vi.fn() })
}))

describe('App shell build label', () => {
  beforeEach(() => {
    route.path = '/'
    replace.mockReset()
    session.bootstrap.mockReset().mockResolvedValue(undefined)
    session.logout.mockReset().mockResolvedValue(undefined)
    session.markUnauthenticated.mockReset()
    version.load.mockReset().mockResolvedValue(undefined)
  })

  it('renders the /version-backed header label', async () => {
    const wrapper = mount(App, {
      global: {
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          RouterView: { template: '<div />' }
        }
      }
    })
    await flushPromises()

    expect(version.load).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain('SMS-only')
  })
})
