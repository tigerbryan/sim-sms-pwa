import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('./client', () => ({
  api: vi.fn(),
  jsonBody: vi.fn((method: string, value: unknown) => ({ method, body: JSON.stringify(value) }))
}))

import { api } from './client'
import { getDevices } from './devices'
import { getSIM } from './sim'
import { getSignal } from './signal'
import { getVersion } from './version'
import { getDashboard } from './dashboard'
import { getNotifications } from './notifications'
import { getSettings } from './settings'
import { getDiagnostics } from './diagnostics'
import { getLogs } from './logs'

describe('endpoint-specific API modules', () => {
  beforeEach(() => { vi.mocked(api).mockReset() })

  it('keeps every normal page on its own endpoint', async () => {
    vi.mocked(api).mockResolvedValue({})
    await getDashboard()
    await getDevices()
    await getSIM()
    await getSignal()
    await getNotifications()
    await getSettings()
    await getLogs()
    await getDiagnostics()
    await getVersion()

    expect(api).toHaveBeenNthCalledWith(1, '/api/v1/dashboard')
    expect(api).toHaveBeenNthCalledWith(2, '/api/v1/devices')
    expect(api).toHaveBeenNthCalledWith(3, '/api/v1/sim')
    expect(api).toHaveBeenNthCalledWith(4, '/api/v1/signal')
    expect(api).toHaveBeenNthCalledWith(5, '/api/v1/notifications')
    expect(api).toHaveBeenNthCalledWith(6, '/api/v1/settings')
    expect(api).toHaveBeenNthCalledWith(7, '/api/v1/logs?limit=200')
    expect(api).toHaveBeenNthCalledWith(8, '/api/v1/diagnostics')
    expect(api).toHaveBeenNthCalledWith(9, '/version')
  })
})
