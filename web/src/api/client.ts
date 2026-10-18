import type { APIErrorResponse, AuthStatus } from '../types/api'

export class APIRequestError extends Error {
  constructor(readonly status: number, message: string) {
    super(message)
    this.name = 'APIRequestError'
  }
}

let csrfToken = typeof sessionStorage === 'undefined' ? '' : (sessionStorage.getItem('qmi-web-csrf') ?? '')

export function setCSRF(token: string) {
  csrfToken = token
  sessionStorage.setItem('qmi-web-csrf', token)
}

export function clearCSRF() {
  csrfToken = ''
  sessionStorage.removeItem('qmi-web-csrf')
}

export async function request(path: string, init: RequestInit = {}): Promise<Response> {
  const method = (init.method ?? 'GET').toUpperCase()
  const headers = new Headers(init.headers)
  headers.set('Accept', 'application/json')
  if (init.body) headers.set('Content-Type', 'application/json')
  if (!['GET', 'HEAD', 'OPTIONS'].includes(method) && csrfToken) headers.set('X-CSRF-Token', csrfToken)
  const response = await fetch(path, { ...init, method, headers, credentials: 'same-origin' })
  if (response.ok) return response

  const body = await response.json().catch(() => ({})) as APIErrorResponse
  if (response.status === 401 && typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent('qmi-web:unauthorized'))
  }
  throw new APIRequestError(response.status, body.error?.message ?? `请求失败 (${response.status})`)
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  return (await request(path, init)).json() as Promise<T>
}

export async function getAuthStatus(): Promise<AuthStatus> {
  const response = await request('/api/v1/auth/me')
  const csrf = response.headers.get('X-CSRF-Token')
  if (csrf) setCSRF(csrf)
  return response.json() as Promise<AuthStatus>
}

export function jsonBody(method: string, value: unknown): RequestInit {
  return { method, body: JSON.stringify(value) }
}
