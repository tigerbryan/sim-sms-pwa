import type { NotificationsResponse } from '../types/api'
import { api, jsonBody } from './client'

export function getNotifications(): Promise<NotificationsResponse> {
  return api<NotificationsResponse>('/api/v1/notifications')
}

export function sendPushPlusTest(): Promise<{ sent: boolean }> {
  return api<{ sent: boolean }>('/api/v1/notifications/pushplus/test', jsonBody('POST', {}))
}
