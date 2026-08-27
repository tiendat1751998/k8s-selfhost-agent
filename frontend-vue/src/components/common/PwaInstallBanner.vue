<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { usePwaInstall } from '../../registerServiceWorker'

const { isInstallable, isInstalled, promptInstall } = usePwaInstall()

const STORAGE_KEY = 'k8scontrol_pwa_banner_dismissed_until'
const isDismissed = ref(false)
const showIosInstructions = ref(false)

// iOS Detection
const isIos = computed(() => {
  if (typeof window === 'undefined') return false
  const ua = window.navigator.userAgent || ''
  return /iPad|iPhone|iPod/.test(ua) && !(window as unknown as { MSStream?: boolean }).MSStream
})

const isIosStandalone = computed(() => {
  if (typeof window === 'undefined') return false
  return (window.navigator as unknown as { standalone?: boolean }).standalone === true
})

function checkDismissalStatus() {
  if (typeof window === 'undefined') return
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      const until = parseInt(raw, 10)
      if (Date.now() < until) {
        isDismissed.value = true
        return
      }
    }
  } catch (e) {
    console.warn('[PWA] Read storage dismissal failed:', e)
  }
  isDismissed.value = false
}

function dismissBanner() {
  isDismissed.value = true
  showIosInstructions.value = false
  try {
    const sevenDaysFromNow = Date.now() + 7 * 24 * 60 * 60 * 1000
    localStorage.setItem(STORAGE_KEY, sevenDaysFromNow.toString())
  } catch (e) {
    console.warn('[PWA] Set storage dismissal failed:', e)
  }
}

onMounted(() => {
  checkDismissalStatus()
})

const shouldShow = computed(() => {
  if (isDismissed.value) return false
  if (isInstalled.value || isIosStandalone.value) return false
  // Show if browser fired beforeinstallprompt or on iOS mobile devices
  return isInstallable.value || isIos.value
})

async function handleInstallClick() {
  if (isIos.value && !isInstallable.value) {
    showIosInstructions.value = !showIosInstructions.value
    return
  }
  await promptInstall()
}
</script>

<template>
  <transition name="banner-slide">
    <div v-if="shouldShow" class="pwa-install-banner glass-panel">
      <!-- Glow Accent Line -->
      <div class="banner-accent-line"></div>

      <div class="banner-content">
        <div class="banner-icon-box">
          <span class="banner-icon">☸️</span>
          <span class="banner-pulse"></span>
        </div>

        <div class="banner-text">
          <div class="banner-title-row">
            <span class="banner-title">Install K8sControl App</span>
            <span class="banner-tag font-mono">PWA</span>
          </div>
          <p class="banner-desc">Run as standalone desktop &amp; mobile app with offline shell</p>
        </div>

        <div class="banner-actions">
          <button 
            class="btn-install" 
            @click="handleInstallClick" 
            title="Install K8sControl to your device"
          >
            <span>📲 Install Now</span>
          </button>
          <button 
            class="btn-dismiss" 
            @click="dismissBanner" 
            title="Dismiss installation banner for 7 days"
            aria-label="Dismiss banner"
          >
            <span>✕</span>
          </button>
        </div>
      </div>

      <!-- iOS Instruction Tooltip / Dropdown -->
      <div v-if="showIosInstructions" class="ios-instructions-box animate-fade-in">
        <div class="ios-header">
          <span class="ios-icon">🍎</span>
          <span class="ios-title font-mono">iOS Installation Guide</span>
          <button class="ios-close" @click="showIosInstructions = false">✕</button>
        </div>
        <ol class="ios-steps font-mono">
          <li>1. Tap Safari's Share button <span class="share-icon">⎙ / 📤</span> at the bottom bar.</li>
          <li>2. Scroll down and tap <strong class="text-cyan">"Add to Home Screen"</strong> (➕).</li>
          <li>3. Tap <strong class="text-cyan">"Add"</strong> in the top-right corner to launch fullscreen.</li>
        </ol>
      </div>
    </div>
  </transition>
</template>

<style scoped>
.pwa-install-banner {
  position: fixed;
  bottom: 24px;
  right: 24px;
  z-index: 95;
  width: calc(100% - 48px);
  max-width: 440px;
  background: rgba(11, 17, 30, 0.92);
  backdrop-filter: blur(16px);
  border: 1px solid rgba(56, 189, 248, 0.35);
  border-radius: 16px;
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.6), 0 0 24px rgba(6, 182, 212, 0.2);
  overflow: hidden;
}

.banner-accent-line {
  height: 2px;
  width: 100%;
  background: linear-gradient(90deg, #06b6d4, #3b82f6, #8b5cf6);
}

.banner-content {
  padding: 14px 16px;
  display: flex;
  align-items: center;
  gap: 12px;
}

.banner-icon-box {
  position: relative;
  width: 40px;
  height: 40px;
  background: rgba(6, 182, 212, 0.12);
  border: 1px solid rgba(56, 189, 248, 0.3);
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.banner-icon {
  font-size: 20px;
}

.banner-pulse {
  position: absolute;
  top: -2px;
  right: -2px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #10b981;
  box-shadow: 0 0 8px #10b981;
}

.banner-text {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.banner-title-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.banner-title {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.banner-tag {
  font-size: 9px;
  font-weight: 700;
  background: rgba(56, 189, 248, 0.2);
  color: #38bdf8;
  padding: 1px 5px;
  border-radius: 4px;
  border: 1px solid rgba(56, 189, 248, 0.4);
}

.banner-desc {
  font-size: 11px;
  color: var(--text-secondary);
  line-height: 1.3;
}

.banner-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.btn-install {
  background: linear-gradient(135deg, #06b6d4 0%, #3b82f6 100%);
  color: #fff;
  border: none;
  font-size: 11px;
  font-weight: 700;
  padding: 7px 12px;
  border-radius: 8px;
  cursor: pointer;
  box-shadow: 0 2px 8px rgba(6, 182, 212, 0.4);
  transition: transform 0.15s ease, box-shadow 0.15s ease;
  white-space: nowrap;
}

.btn-install:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(6, 182, 212, 0.6);
}

.btn-dismiss {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--border-subtle);
  color: var(--text-muted);
  width: 28px;
  height: 28px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  font-size: 12px;
  transition: all 0.15s ease;
}

.btn-dismiss:hover {
  background: rgba(244, 63, 94, 0.2);
  color: #fb7185;
  border-color: rgba(244, 63, 94, 0.4);
}

/* iOS instructions */
.ios-instructions-box {
  margin: 0 12px 12px 12px;
  padding: 10px 12px;
  background: rgba(6, 10, 20, 0.95);
  border: 1px solid rgba(56, 189, 248, 0.25);
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.ios-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.ios-title {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-primary);
  display: flex;
  align-items: center;
  gap: 6px;
}

.ios-close {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 11px;
  cursor: pointer;
}

.ios-steps {
  font-size: 10px;
  color: var(--text-secondary);
  line-height: 1.5;
  padding-left: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.share-icon {
  color: var(--accent-cyan);
}

.text-cyan {
  color: var(--accent-sky);
}

/* Transitions */
.banner-slide-enter-active,
.banner-slide-leave-active {
  transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

.banner-slide-enter-from,
.banner-slide-leave-to {
  opacity: 0;
  transform: translateY(20px) scale(0.96);
}

@media (max-width: 640px) {
  .pwa-install-banner {
    bottom: calc(64px + var(--sab) + 12px);
    right: 12px;
    left: 12px;
    width: calc(100% - 24px);
    max-width: none;
  }
}
</style>
