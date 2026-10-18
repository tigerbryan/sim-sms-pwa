import type { AuthStatus, LoginReply } from '../types/api'
import { api, getAuthStatus, jsonBody } from './client'

export function authStatus(): Promise<AuthStatus> {
  return getAuthStatus()
}

export function login(username: string, password: string): Promise<LoginReply> {
  return api<LoginReply>('/api/v1/auth/login', jsonBody('POST', { username, password }))
}

export function logout(): Promise<{ logged_out: boolean }> {
  return api<{ logged_out: boolean }>('/api/v1/auth/logout', jsonBody('POST', {}))
}

export function changePassword(currentPassword: string, newPassword: string, confirmation: string): Promise<{ password_changed: boolean }> {
  return api<{ password_changed: boolean }>('/api/v1/auth/password', jsonBody('POST', {
    current_password: currentPassword,
    new_password: newPassword,
    confirmation
  }))
}
