import { defineStore } from 'pinia'
import { authStatus, changePassword, login, logout } from '../api/auth'
import { APIRequestError, clearCSRF, setCSRF } from '../api/client'

export const useSessionStore = defineStore('session', {
  state: () => ({ checked: false, authenticated: false, username: '', error: '' }),
  actions: {
    markUnauthenticated() {
      clearCSRF()
      this.authenticated = false
      this.username = ''
    },
    async bootstrap() {
      this.checked = false
      try {
        const state = await authStatus()
        this.authenticated = state.authenticated
        this.username = state.username
        this.error = ''
        if (!state.authenticated) this.markUnauthenticated()
      } catch (cause) {
        if (cause instanceof APIRequestError && cause.status === 401) {
          this.markUnauthenticated()
          this.error = ''
        } else {
          this.error = cause instanceof Error ? cause.message : '后端暂时不可用。'
        }
      } finally {
        this.checked = true
      }
    },
    async login(username: string, password: string) {
      const reply = await login(username, password)
      setCSRF(reply.csrf_token)
      this.authenticated = reply.authenticated
      this.username = 'admin'
      this.error = ''
    },
    async logout() {
      try {
        await logout()
      } finally {
        this.markUnauthenticated()
      }
    },
    async changePassword(currentPassword: string, newPassword: string, confirmation: string) {
      await changePassword(currentPassword, newPassword, confirmation)
      this.markUnauthenticated()
    }
  }
})
