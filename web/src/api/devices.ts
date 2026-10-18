import type { DevicesResponse } from '../types/api'
import { api, jsonBody } from './client'

export function getDevices(): Promise<DevicesResponse> {
  return api<DevicesResponse>('/api/v1/devices')
}

export function scanDevices(): Promise<DevicesResponse> {
  return api<DevicesResponse>('/api/v1/devices/scan', jsonBody('POST', {}))
}
