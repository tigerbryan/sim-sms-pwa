import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../api/version', () => ({ getVersion: vi.fn() }))

import { getVersion } from '../api/version'
import { useVersionStore } from './version'

describe('version store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(getVersion).mockReset()
  })

  it('uses /version as the only build metadata source', async () => {
    vi.mocked(getVersion).mockResolvedValue({
      version: '1.0.0', commit: 'abc1234', build_time: '2026-08-11T00:00:00Z',
      go_version: 'go1.26.3', qmi_go_version: 'v0.6.4', smscodec_version: 'v0.1.0',
      license: 'MIT', sms_only: true
    })
    const store = useVersionStore()

    await store.load()

    expect(getVersion).toHaveBeenCalledOnce()
    expect(store.info?.version).toBe('1.0.0')
    expect(store.headerLabel).toBe('V1.0')
  })

  it('does not fall back to a hardcoded version when /version fails', async () => {
    vi.mocked(getVersion).mockRejectedValue(new Error('offline'))
    const store = useVersionStore()

    await store.load()

    expect(store.info).toBeNull()
    expect(store.headerLabel).toBe('version unavailable')
  })
})
