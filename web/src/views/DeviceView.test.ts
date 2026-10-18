import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import DeviceView from './DeviceView.vue'

vi.mock('../api/devices', () => ({
  getDevices: vi.fn(),
  scanDevices: vi.fn()
}))

import { getDevices } from '../api/devices'

describe('DeviceView', () => {
  beforeEach(() => { vi.mocked(getDevices).mockReset() })

  it('renders the friendly empty state from the devices endpoint, never version JSON', async () => {
    vi.mocked(getDevices).mockResolvedValue({
      devices: [], backend: 'auto', scan_time: '2026-08-10T00:00:00Z'
    })
    const wrapper = mount(DeviceView)
    await flushPromises()

    expect(getDevices).toHaveBeenCalledOnce()
    expect(wrapper.text()).not.toContain('v0.1.1')
    expect(wrapper.find('pre').exists()).toBe(false)
  })
})
