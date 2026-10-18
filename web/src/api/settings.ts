import type { SettingsResponse, SettingsUpdate } from '../types/api'
import { api, jsonBody } from './client'

export function getSettings(): Promise<SettingsResponse> {
  return api<SettingsResponse>('/api/v1/settings')
}

export function updateSettings(update: SettingsUpdate): Promise<SettingsResponse> {
  return api<SettingsResponse>('/api/v1/settings', jsonBody('PATCH', update))
}
