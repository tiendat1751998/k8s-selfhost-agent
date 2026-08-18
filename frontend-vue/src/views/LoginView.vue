<template>
  <div class="login-page">
    <div class="login-background-glow"></div>

    <div class="login-card glass-panel animate-fade-in">
      <!-- Brand Header -->
      <div class="brand-section">
        <div class="brand-icon-wrapper">
          <div class="brand-icon">⎈</div>
          <div class="brand-glow"></div>
        </div>
        <h1 class="brand-title">K8S<span>CONTROL</span></h1>
        <p class="brand-subtitle">Enterprise Hybrid Control Plane</p>
      </div>

      <!-- Environment Badge -->
      <div class="env-pill">
        <span class="pulse-dot pulse-dot-emerald"></span>
        <span class="env-text">Air-Gapped ZeroTrust Enforced</span>
        <span class="env-chip">TLS v1.3</span>
      </div>

      <!-- Error Box -->
      <div v-if="errorMessage" class="error-banner animate-fade-in">
        <span class="error-icon">⚠️</span>
        <span class="error-text">{{ errorMessage }}</span>
      </div>

      <!-- Login Form -->
      <form class="login-form" @submit.prevent="handleSubmit">
        <div class="form-group">
          <label for="email" class="form-label">Email Address</label>
          <input
            id="email"
            v-model="email"
            type="email"
            required
            autocomplete="email"
            placeholder="admin@k8s.local"
            class="input-glass form-input"
            :disabled="authStore.loading"
          />
        </div>

        <div class="form-group">
          <label for="password" class="form-label">Password</label>
          <input
            id="password"
            v-model="password"
            type="password"
            required
            autocomplete="current-password"
            placeholder="••••••••••••"
            class="input-glass form-input"
            :disabled="authStore.loading"
          />
        </div>

        <button
          type="submit"
          class="btn btn-primary login-btn"
          :disabled="authStore.loading || !email || !password"
        >
          <span v-if="authStore.loading" class="spinner"></span>
          <span>{{ authStore.loading ? 'Authenticating...' : 'Sign In to Console' }}</span>
        </button>
      </form>

      <!-- Footer Info -->
      <div class="login-footer">
        <span>Dual-Sync DR • Trivy Gate • Real-Time Stream</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/authStore'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const email = ref('')
const password = ref('')
const errorMessage = ref('')

async function handleSubmit() {
  if (!email.value || !password.value) return
  errorMessage.value = ''

  try {
    await authStore.login(email.value, password.value)
    const redirect = (route.query.redirect as string) || '/backup'
    router.push(redirect)
  } catch (err: any) {
    errorMessage.value = err.message || 'Authentication failed. Please check your credentials.'
  }
}
</script>

<style scoped>
.login-page {
  width: 100vw;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: var(--bg-app);
  position: relative;
  overflow: hidden;
  padding: 20px;
}

.login-background-glow {
  position: absolute;
  width: 600px;
  height: 600px;
  background: radial-gradient(circle, rgba(6, 182, 212, 0.12) 0%, rgba(99, 102, 241, 0.05) 50%, transparent 70%);
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  pointer-events: none;
}

.login-card {
  width: 100%;
  max-width: 440px;
  padding: 36px 32px;
  background: rgba(11, 15, 25, 0.9);
  border: 1px solid var(--border-medium);
  border-radius: 20px;
  box-shadow: 0 25px 60px -15px rgba(0, 0, 0, 0.8), 0 0 40px rgba(6, 182, 212, 0.1);
  position: relative;
  z-index: 10;
}

.brand-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  margin-bottom: 20px;
}

.brand-icon-wrapper {
  position: relative;
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 12px;
}

.brand-icon {
  width: 48px;
  height: 48px;
  background: var(--grad-cyan);
  color: #fff;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 26px;
  font-weight: bold;
  box-shadow: 0 4px 20px rgba(6, 182, 212, 0.4);
  position: relative;
  z-index: 2;
}

.brand-glow {
  position: absolute;
  width: 48px;
  height: 48px;
  background: var(--accent-cyan);
  filter: blur(14px);
  opacity: 0.6;
}

.brand-title {
  font-size: 22px;
  font-weight: 800;
  letter-spacing: -0.03em;
  color: #fff;
}

.brand-title span {
  color: var(--accent-cyan);
  margin-left: 2px;
}

.brand-subtitle {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
  margin-top: 2px;
}

.env-pill {
  margin-bottom: 20px;
  padding: 8px 12px;
  background: rgba(16, 185, 129, 0.08);
  border: 1px solid rgba(16, 185, 129, 0.2);
  border-radius: 10px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.env-text {
  font-size: 11px;
  font-weight: 600;
  color: #34d399;
  flex: 1;
}

.env-chip {
  font-size: 10px;
  background: rgba(16, 185, 129, 0.2);
  color: #a7f3d0;
  padding: 2px 6px;
  border-radius: 6px;
  font-weight: 700;
  font-family: var(--font-mono);
}

.error-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.3);
  border-radius: 10px;
  color: #fda4af;
  font-size: 12px;
  margin-bottom: 18px;
}

.error-icon {
  font-size: 14px;
  flex-shrink: 0;
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.form-input {
  width: 100%;
  padding: 10px 14px;
  font-size: 13px;
}

.login-btn {
  width: 100%;
  padding: 11px;
  font-size: 14px;
  margin-top: 8px;
}

.spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.login-footer {
  margin-top: 24px;
  padding-top: 16px;
  border-top: 1px solid var(--border-subtle);
  text-align: center;
  font-size: 11px;
  color: var(--text-muted);
}
</style>
