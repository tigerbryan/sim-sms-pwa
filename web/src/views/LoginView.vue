<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSessionStore } from '../stores/session'
import { Hexagon, Lock, User, LogIn, Loader2 } from 'lucide-vue-next'

const username = ref('admin')
const password = ref('')
const busy = ref(false)
const error = ref('')
const session = useSessionStore()
const router = useRouter()

async function submit() {
  error.value = ''
  busy.value = true
  try {
    await session.login(username.value, password.value)
    await router.replace('/')
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '登录失败，请检查账号密码'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="auth-screen">
    <div class="auth-container card-hero">
      <div class="auth-header">
        <div class="brand-icon">
          <Hexagon :size="32" class="text-accent" stroke-width="2.5" />
        </div>
        <h1>QMI Web</h1>
        <p>你的私人短信收件箱</p>
      </div>

      <form class="auth-form" @submit.prevent="submit">
        <div class="form-group">
          <label for="username">用户名</label>
          <div class="input-wrapper">
            <User class="input-icon" :size="18" />
            <input 
              id="username"
              v-model="username" 
              autocomplete="username" 
              placeholder="请输入管理员账号"
              required 
            />
          </div>
        </div>

        <div class="form-group">
          <label for="password">密码</label>
          <div class="input-wrapper">
            <Lock class="input-icon" :size="18" />
            <input 
              id="password"
              v-model="password" 
              type="password" 
              autocomplete="current-password" 
              placeholder="请输入密码"
              required 
            />
          </div>
        </div>

        <div v-if="error" class="error-banner">
          {{ error }}
        </div>

        <button type="submit" class="submit-btn" :disabled="busy">
          <Loader2 v-if="busy" class="spin" :size="18" />
          <LogIn v-else :size="18" />
          {{ busy ? '正在登录...' : '登 录' }}
        </button>
      </form>
      
      <div class="auth-footer text-muted">
        &copy; 2026 QMI Web System
      </div>
    </div>
  </section>
</template>

<style scoped>
.auth-screen {
  min-height: 100dvh;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: var(--bg-app);
  background-image: radial-gradient(circle at center, var(--accent-light) 0%, transparent 50%);
  padding: 24px;
}

.auth-container {
  width: 100%;
  max-width: 420px;
  background-color: var(--bg-surface);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-modal);
  padding: 48px 40px;
  display: flex;
  flex-direction: column;
  gap: 32px;
}

.auth-header {
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.brand-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  border-radius: var(--radius-lg);
  background-color: var(--accent-light);
  color: var(--accent);
}

.auth-header h1 {
  font-size: 1.75rem;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
  letter-spacing: -0.02em;
}

.auth-header p {
  color: var(--text-secondary);
  font-size: 0.95rem;
  margin: 0;
}

.auth-form {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

label {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-secondary);
}

.input-wrapper {
  position: relative;
  display: flex;
  align-items: center;
}

.input-icon {
  position: absolute;
  left: 14px;
  color: var(--text-muted);
}

input {
  width: 100%;
  height: 44px;
  padding: 0 14px 0 42px;
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-md);
  background-color: var(--bg-surface);
  color: var(--text-primary);
  font-size: 0.95rem;
  transition: all 0.2s;
}

input:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-light);
}

.error-banner {
  padding: 12px 16px;
  background-color: var(--danger-bg);
  color: var(--danger);
  border-radius: var(--radius-md);
  font-size: 0.9rem;
  text-align: center;
}

.submit-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 48px;
  background-color: var(--accent);
  color: var(--accent-text);
  border: none;
  border-radius: var(--radius-md);
  font-weight: 600;
  font-size: 1rem;
  cursor: pointer;
  transition: all 0.2s;
  margin-top: 8px;
  box-shadow: 0 4px 14px var(--accent-light);
}

.submit-btn:hover:not(:disabled) {
  background-color: var(--accent-hover);
  transform: translateY(-1px);
  box-shadow: 0 6px 20px var(--accent-light);
}

.submit-btn:disabled {
  opacity: 0.7;
  cursor: not-allowed;
  transform: none;
}

.auth-footer {
  text-align: center;
  font-size: 0.85rem;
  margin-top: 16px;
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
