import type { VersionInfo } from '../types/api'
import { api } from './client'

export function getVersion(): Promise<VersionInfo> {
  return api<VersionInfo>('/version')
}
