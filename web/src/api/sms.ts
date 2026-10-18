import type { SMSMessage, SMSPage } from '../types/api'
import { api, jsonBody } from './client'

export function getSMSPage(page: number, pageSize: number, query: string): Promise<SMSPage> {
  const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) })
  if (query.trim()) params.set('q', query.trim())
  return api<SMSPage>(`/api/v1/sms?${params}`)
}

export function getSMS(id: number): Promise<SMSMessage> {
  return api<SMSMessage>(`/api/v1/sms/${id}`)
}

export function setSMSRead(id: number, read: boolean): Promise<{ updated: boolean }> {
  return api<{ updated: boolean }>(`/api/v1/sms/${id}/read`, jsonBody('PATCH', { read }))
}
