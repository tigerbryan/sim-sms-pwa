import { describe, expect, it } from 'vitest'

describe('QMI Web API client', () => {
  it('keeps API errors typed and human-readable', () => {
    const error = { error: { code: 'not_ready', message: 'database is not ready' } }
    expect(error.error.code).toBe('not_ready')
    expect(error.error.message).toContain('database')
  })
})
