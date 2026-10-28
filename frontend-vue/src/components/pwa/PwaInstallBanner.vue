<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { deferredPrompt, usePwaInstall } from '../../registerServiceWorker'
import BaseIcon from '../ui/BaseIcon.vue'

const { isInstalled, promptInstall } = usePwaInstall()

const STORAGE_KEY = 'k8scontrol_pwa_banner_dismissed_until'
const isDismissed = ref(false)
const showIosInstructions = ref(false)

// iOS Standalone Detection

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
  // ONLY render when a genuine browser beforeinstallprompt event has fired
  return deferredPrompt.value !== null
})

async function handleInstallClick() {
  await promptInstall()
}
</script>

<template>
  <transition name="banner-slide">
    <div v-if="shouldShow" class="pwa-install-banner">
      <div class="banner-content">
        <div class="banner-top-row">
          <div class="banner-title-group">
            <div class="banner-icon-box">
              <BaseIcon name="anchor" size="xs" />
            </div>
            <span class="banner-title">Install App</span>
            <span class="banner-tag font-mono">PWA</span>
          </div>
          <button 
            class="btn-dismiss" 
            @click="dismissBanner" 
            title="Dismiss installation banner for 7 days"
            aria-label="Dismiss banner"
          >
            <BaseIcon name="x" size="xs" />
          </button>
        </div>

        <p class="banner-desc">
          Standalone app with offline shell
        </p>

        <div class="banner-actions">
          <button 
            class="btn-install" 
            @click="handleInstallClick" 
            title="Install K8sControl to your device"
          >
            <BaseIcon name="download" size="xs" />
            <span>Install Now</span>
          </button>
        </div>
      </div>

      <!-- iOS Instruction Tooltip / Dropdown -->
      <div v-if="showIosInstructions" class="ios-instructions-box animate-fade-in">
        <div class="ios-header">
          <span class="ios-icon"><BaseIcon name="info" size="xs" /></span>
          <span class="ios-title font-mono">iOS Installation Guide</span>
          <button class="ios-close" @click="showIosInstructions = false" aria-label="Close iOS guide"><BaseIcon name="x" size="xs" /></button>
        </div>
        <ol class="ios-steps font-mono">
          <li>1. Tap Safari's Share button <span class="share-icon"><BaseIcon name="external-link" size="xs" /></span> at the bottom bar.</li>
          <li>2. Scroll down and tap <strong>"Add to Home Screen"</strong> (+).</li>
          <li>3. Tap <strong>"Add"</strong> in the top-right corner to launch fullscreen.</li>
        </ol>
      </div>
    </div>
  </transition>
</template>

<style scoped>
@import '../../assets/styles/components/pwa-install-banner.css';
</style>
