import type { DiagnosticsResponse } from '../types/api'
import { api } from './client'

export function getDiagnostics(): Promise<DiagnosticsResponse> {
  return api<DiagnosticsResponse>('/api/v1/diagnostics')
}
