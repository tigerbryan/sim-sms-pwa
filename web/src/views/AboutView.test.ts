import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AboutView from './AboutView.vue'

vi.mock('../api/version', () => ({ getVersion: vi.fn() }))

import { getVersion } from '../api/version'

describe('AboutView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(getVersion).mockReset()
  })

  it('shows the real version, Go version, and MIT license from /version', async () => {
    vi.mocked(getVersion).mockResolvedValue({
      version: '1.0.0', commit: 'abc1234', build_time: '2026-08-11T00:00:00Z',
      go_version: 'go1.26.3', qmi_go_version: 'v0.6.4', smscodec_version: 'v0.1.0',
      license: 'MIT', sms_only: true
    })
    const wrapper = mount(AboutView)
    await flushPromises()

    expect(getVersion).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain('v1.0.0')
    expect(wrapper.text()).toContain('go1.26.3')
    expect(wrapper.text()).toContain('MIT')
  })
})
