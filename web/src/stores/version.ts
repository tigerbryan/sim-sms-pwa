import { defineStore } from 'pinia'
import { getVersion } from '../api/version'
import type { VersionInfo } from '../types/api'

type VersionStatus = 'idle' | 'loading' | 'ready' | 'error'

// Build metadata belongs to the backend. The frontend only caches the single
// authoritative /version response for the current application session.
export const useVersionStore = defineStore('version', {
  state: () => ({
    info: null as VersionInfo | null,
    status: 'idle' as VersionStatus,
    error: ''
  }),
  getters: {
    headerLabel: state => state.info ? `V1.0` : 'version unavailable'
  },
  actions: {
    async load() {
      if (this.status === 'loading' || this.status === 'ready') return
      this.status = 'loading'
      try {
        this.info = await getVersion()
        this.status = 'ready'
        this.error = ''
      } catch (cause) {
        this.info = null
        this.status = 'error'
        this.error = cause instanceof Error ? cause.message : '无法读取版本信息。'
      }
    }
  }
})
