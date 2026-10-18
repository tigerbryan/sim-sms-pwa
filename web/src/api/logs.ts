import type { LogsResponse } from '../types/api'
import { api } from './client'

export function getLogs(limit = 200): Promise<LogsResponse> {
  return api<LogsResponse>(`/api/v1/logs?limit=${limit}`)
}
