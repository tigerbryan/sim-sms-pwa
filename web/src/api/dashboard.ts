import type { Dashboard } from '../types/api'
import { api } from './client'

export function getDashboard(): Promise<Dashboard> {
  return api<Dashboard>('/api/v1/dashboard')
}
