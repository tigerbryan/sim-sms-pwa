import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: '../internal/web/dist',
    emptyOutDir: true
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:7580',
      '/health': 'http://127.0.0.1:7580',
      '/ready': 'http://127.0.0.1:7580',
      '/version': 'http://127.0.0.1:7580'
    }
  },
  test: { environment: 'jsdom' }
})
