import type { SignalInfo } from '../types/api'
import { api } from './client'

export function getSignal(): Promise<SignalInfo> {
  return api<SignalInfo>('/api/v1/signal')
}
