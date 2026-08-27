import { ref } from 'vue'

export interface BeforeInstallPromptEvent extends Event {
  readonly platforms: string[]
  readonly userChoice: Promise<{
    outcome: 'accepted' | 'dismissed'
    platform: string
  }>
  prompt(): Promise<void>
}

// Global reactive states shared across all components
const isInstallable = ref<boolean>(false)
const isInstalled = ref<boolean>(false)
const deferredPrompt = ref<BeforeInstallPromptEvent | null>(null)
const hasUpdate = ref<boolean>(false)

if (typeof window !== 'undefined') {
  // Detect if app is running in standalone mode (desktop or mobile)
  const isStandalone = window.matchMedia('(display-mode: standalone)').matches ||
    (window.navigator as unknown as { standalone?: boolean }).standalone === true

  if (isStandalone) {
    isInstalled.value = true
  }

  // Intercept Chrome/Edge/Android PWA install prompt
  window.addEventListener('beforeinstallprompt', (e: Event) => {
    e.preventDefault()
    deferredPrompt.value = e as BeforeInstallPromptEvent
    isInstallable.value = true
  })

  // Listen for successful PWA installation
  window.addEventListener('appinstalled', () => {
    isInstalled.value = true
    isInstallable.value = false
    deferredPrompt.value = null
  })
}

export function usePwaInstall() {
  async function promptInstall(): Promise<boolean> {
    if (!deferredPrompt.value) {
      return false
    }

    try {
      await deferredPrompt.value.prompt()
      const choiceResult = await deferredPrompt.value.userChoice
      if (choiceResult.outcome === 'accepted') {
        isInstallable.value = false
        deferredPrompt.value = null
        return true
      }
      return false
    } catch (err) {
      console.warn('[PWA] prompt install execution failed:', err)
      return false
    }
  }

  return {
    isInstallable,
    isInstalled,
    deferredPrompt,
    hasUpdate,
    promptInstall
  }
}

export function registerSW(): void {
  if (typeof window === 'undefined' || !('serviceWorker' in navigator)) {
    return
  }

  if (import.meta.env.DEV) {
    // In development mode, purge any active service worker and cache to ensure clean Vite HMR
    navigator.serviceWorker.getRegistrations().then((registrations) => {
      for (const registration of registrations) {
        registration.unregister()
      }
    })
    if ('caches' in window) {
      caches.keys().then((keys) => {
        for (const key of keys) {
          caches.delete(key)
        }
      })
    }
    return
  }

  // Production registration
  window.addEventListener('load', () => {
    navigator.serviceWorker
      .register('/sw.js')
      .then((reg) => {
        reg.addEventListener('updatefound', () => {
          const installingWorker = reg.installing
          if (installingWorker) {
            installingWorker.addEventListener('statechange', () => {
              if (installingWorker.state === 'installed' && navigator.serviceWorker.controller) {
                hasUpdate.value = true
              }
            })
          }
        })
      })
      .catch((err) => {
        console.warn('[SW] Service worker registration failed:', err)
      })
  })
}
