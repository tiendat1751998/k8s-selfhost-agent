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
@import '../../assets/styles/components/pwa-install-banner.css';
</style>
