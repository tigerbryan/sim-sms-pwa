import type { SIMInfo } from '../types/api'
import { api } from './client'

export function getSIM(): Promise<SIMInfo> {
  return api<SIMInfo>('/api/v1/sim')
}
