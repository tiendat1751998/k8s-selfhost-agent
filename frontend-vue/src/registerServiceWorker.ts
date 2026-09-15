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
export const isInstallable = ref<boolean>(false)
export const isInstalled = ref<boolean>(false)
export const deferredPrompt = ref<BeforeInstallPromptEvent | null>(null)
export const hasUpdate = ref<boolean>(false)

export function setDeferredPrompt(event: BeforeInstallPromptEvent | null): void {
  deferredPrompt.value = event
  isInstallable.value = event !== null
}

if (typeof window !== 'undefined') {
  // Detect if app is running in standalone mode (desktop or mobile)
  const isStandalone = window.matchMedia('(display-mode: standalone)').matches ||
    (window.navigator as unknown as { standalone?: boolean }).standalone === true

  if (isStandalone) {
    isInstalled.value = true
  }

  // Intercept Chrome/Edge/Android PWA install prompt across all environments
  window.addEventListener('beforeinstallprompt', (e: Event) => {
    e.preventDefault()
    setDeferredPrompt(e as BeforeInstallPromptEvent)
  })

  // Listen for successful PWA installation
  window.addEventListener('appinstalled', () => {
    isInstalled.value = true
    isInstallable.value = false
    deferredPrompt.value = null
  })

  // Expose inspection & debug helpers on window for development and automated testing
  ;(window as unknown as { __k8s_pwa?: unknown }).__k8s_pwa = {
    isInstallable,
    isInstalled,
    deferredPrompt,
    hasUpdate,
    setDeferredPrompt,
    promptInstall: () => usePwaInstall().promptInstall()
  }
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
    setDeferredPrompt,
    promptInstall
  }
}

export function registerSW(): void {
  if (typeof window === 'undefined' || !('serviceWorker' in navigator)) {
    return
  }

  // Allow service worker registration in development mode when enable_pwa_dev is set to true
  const enableDevSw = typeof localStorage !== 'undefined' && localStorage.getItem('enable_pwa_dev') === 'true'

  if (import.meta.env.DEV && !enableDevSw) {
    // In development mode (unless enable_pwa_dev is explicitly enabled),
    // purge any active service worker and cache to ensure clean Vite HMR
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

  // Production or Dev with enable_pwa_dev: register service worker
  const performRegistration = () => {
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
  }

  if (document.readyState === 'complete') {
    performRegistration()
  } else {
    window.addEventListener('load', performRegistration)
  }
}
