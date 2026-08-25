/**
 * K8sControl Service Worker - Offline Shell & Cyber Caching Engine
 * Version: k8scontrol-shell-v1
 */

const CACHE_NAME = 'k8scontrol-shell-v1'
const PRECACHE_ASSETS = [
  '/',
  '/index.html',
  '/favicon.svg',
  '/icons.svg'
]

// 1. Install Phase: Skip waiting and cache core SPA shell
self.addEventListener('install', (event) => {
  self.skipWaiting()
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      return cache.addAll(PRECACHE_ASSETS)
    }).catch((err) => {
      console.warn('[SW] Pre-caching core assets encountered error:', err)
    })
  )
})

// 2. Activate Phase: Claim clients immediately and purge previous cache versions
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((cacheNames) => {
      return Promise.all(
        cacheNames.map((cacheName) => {
          if (cacheName !== CACHE_NAME) {
            return caches.delete(cacheName)
          }
        })
      )
    }).then(() => self.clients.claim())
  )
})

// 3. Fetch Phase: Dynamic strategy based on request type
self.addEventListener('fetch', (event) => {
  const { request } = event

  // Only handle GET requests
  if (request.method !== 'GET') {
    return
  }

  const url = new URL(request.url)

  // A. API & WebSocket routes: Network-first with graceful fallback
  if (url.pathname.startsWith('/api') || url.pathname.startsWith('/ws')) {
    event.respondWith(
      fetch(request).catch((err) => {
        console.warn('[SW] API network request failed (Offline):', err)
        return new Response(
          JSON.stringify({ 
            error: 'Network unavailable. Running in offline mode.', 
            offline: true 
          }), 
          {
            status: 503,
            headers: { 'Content-Type': 'application/json' }
          }
        )
      })
    )
    return
  }

  // B. SPA Navigation: Network-first, fallback to cached /index.html shell
  if (request.mode === 'navigate') {
    event.respondWith(
      fetch(request)
        .then((response) => {
          if (response && response.status === 200) {
            const responseClone = response.clone()
            caches.open(CACHE_NAME).then((cache) => {
              cache.put(request, responseClone).catch((err) => {
                console.warn('[SW] Cache put navigation failed:', err)
              })
            }).catch((err) => {
              console.warn('[SW] Open cache navigation failed:', err)
            })
          }
          return response
        })
        .catch(() => {
          return caches.match('/index.html').then((cachedIndex) => {
            if (cachedIndex) return cachedIndex
            return caches.match('/')
          })
        })
    )
    return
  }

  // C. Static Assets: Stale-While-Revalidate caching pattern
  const isStaticAsset = /\.(js|css|svg|png|jpg|jpeg|gif|webp|woff|woff2|ttf|eot|ico)$/i.test(url.pathname)
  if (isStaticAsset) {
    event.respondWith(
      caches.open(CACHE_NAME).then((cache) => {
        return cache.match(request).then((cachedResponse) => {
          const fetchPromise = fetch(request)
            .then((networkResponse) => {
              if (networkResponse && networkResponse.status === 200) {
                cache.put(request, networkResponse.clone()).catch((err) => {
                  console.warn('[SW] Static cache put failed:', err)
                })
              }
              return networkResponse
            })
            .catch((err) => {
              console.warn('[SW] Static asset fetch failed:', err)
              return cachedResponse
            })

          return cachedResponse || fetchPromise
        })
      })
    )
    return
  }

  // D. Default Strategy: Cache with network fallback
  event.respondWith(
    caches.match(request).then((cachedResponse) => {
      if (cachedResponse) {
        return cachedResponse
      }
      return fetch(request).then((networkResponse) => {
        if (networkResponse && networkResponse.status === 200 && networkResponse.type === 'basic') {
          const responseClone = networkResponse.clone()
          caches.open(CACHE_NAME).then((cache) => {
            cache.put(request, responseClone).catch((err) => {
              console.warn('[SW] Default cache put failed:', err)
            })
          }).catch((err) => {
            console.warn('[SW] Open cache failed:', err)
          })
        }
        return networkResponse
      })
    })
  )
})
