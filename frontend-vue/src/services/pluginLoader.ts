import { reactive } from 'vue'
import { pluginsApi, type Plugin } from '../api/plugins'

export type PluginRuntimeStatus = 'idle' | 'loading' | 'active' | 'error'

export interface LoadedPluginRuntime {
  id: string
  name: string
  status: PluginRuntimeStatus
  error?: string
  loadedAt?: string
  exports?: any
}

export interface PluginContext {
  plugin: Plugin
  config: Record<string, string>
  notify: (msg: string, type?: 'info' | 'success' | 'warning' | 'error') => void
  version: string
}

class PluginLoaderService {
  public state = reactive<{
    loadedPlugins: Record<string, LoadedPluginRuntime>
    initialized: boolean
  }>({
    loadedPlugins: {},
    initialized: false,
  })

  // Global registry for Headlamp-compatible plugin registrations
  initGlobalHook() {
    if (typeof window !== 'undefined' && !(window as any).__K8S_PLUGIN_SYSTEM__) {
      (window as any).__K8S_PLUGIN_SYSTEM__ = {
        registeredPlugins: new Map<string, any>(),
        registerPlugin: (name: string, pluginDef: any) => {
          (window as any).__K8S_PLUGIN_SYSTEM__.registeredPlugins.set(name, pluginDef)
        },
      }
    }
  }

  async loadAllEnabled(): Promise<void> {
    this.initGlobalHook()
    try {
      const plugins = await pluginsApi.list(true)
      for (const p of plugins) {
        if (p.enabled && p.entry_point) {
          await this.loadPlugin(p)
        }
      }
      this.state.initialized = true
    } catch (err: any) {
      console.warn('[PluginLoader] Failed to load enabled plugins:', err)
    }
  }

  async loadPlugin(plugin: Plugin): Promise<boolean> {
    this.initGlobalHook()

    this.state.loadedPlugins[plugin.id] = {
      id: plugin.id,
      name: plugin.name,
      status: 'loading',
    }

    if (!plugin.entry_point || !plugin.entry_point.trim()) {
      this.state.loadedPlugins[plugin.id] = {
        id: plugin.id,
        name: plugin.name,
        status: 'active',
        loadedAt: new Date().toISOString(),
      }
      return true
    }

    if (typeof document === 'undefined') {
      return true
    }

    const safeUrl = plugin.entry_point.trim()
    if (!this.isValidPluginUrl(safeUrl)) {
      this.state.loadedPlugins[plugin.id] = {
        id: plugin.id,
        name: plugin.name,
        status: 'error',
        error: `Untrusted plugin entry point protocol or URI: ${safeUrl}`,
      }
      return false
    }

    try {
      const existingIframe = document.getElementById(`plugin-sandbox-${plugin.id}`)
      if (existingIframe) {
        existingIframe.remove()
      }
      const existingScript = document.getElementById(`plugin-bundle-${plugin.id}`)
      if (existingScript) {
        existingScript.remove()
      }

      await new Promise<void>((resolve, reject) => {
        const iframe = document.createElement('iframe')
        iframe.id = `plugin-sandbox-${plugin.id}`
        // Strict sandbox: allow-scripts only without same-origin privileges to isolate tokens and DOM
        iframe.setAttribute('sandbox', 'allow-scripts')
        iframe.style.display = 'none'

        // Strict Content-Security-Policy (CSP) inside iframe
        const escapedUrl = safeUrl.replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
        iframe.srcdoc = `<!DOCTYPE html>
<html>
<head>
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'unsafe-inline' 'unsafe-eval' https: http:; connect-src https: http:;">
</head>
<body>
  <script>
    window.addEventListener('error', function(e) {
      window.parent.postMessage({ type: 'plugin-error', id: '${plugin.id}', message: e.message }, '*');
    });
  </script>
  <script src="${escapedUrl}" onload="window.parent.postMessage({ type: 'plugin-loaded', id: '${plugin.id}' }, '*')"></script>
</body>
</html>`

        let timer: ReturnType<typeof setTimeout> | null = null

        const cleanup = () => {
          if (typeof window !== 'undefined') {
            window.removeEventListener('message', handleMessage)
          }
          if (timer) clearTimeout(timer)
        }

        const handleMessage = (event: MessageEvent) => {
          if (!event.data || typeof event.data !== 'object') return
          if (event.data.id !== plugin.id) return
          if (event.data.type === 'plugin-loaded') {
            cleanup()
            resolve()
          } else if (event.data.type === 'plugin-error') {
            cleanup()
            reject(new Error(event.data.message || 'Plugin execution failed inside sandbox'))
          }
        }

        if (typeof window !== 'undefined') {
          window.addEventListener('message', handleMessage)
        }

        timer = setTimeout(() => {
          cleanup()
          resolve()
        }, 5000)

        document.body.appendChild(iframe)
      })

      this.state.loadedPlugins[plugin.id] = {
        id: plugin.id,
        name: plugin.name,
        status: 'active',
        loadedAt: new Date().toISOString(),
      }
      return true
    } catch (err: any) {
      this.state.loadedPlugins[plugin.id] = {
        id: plugin.id,
        name: plugin.name,
        status: 'error',
        error: err.message || 'Failed to load script bundle',
      }
      return false
    }
  }

  private isValidPluginUrl(url: string): boolean {
    if (!url || typeof url !== 'string') return false
    const trimmed = url.trim()
    const lower = trimmed.toLowerCase()
    if (lower.startsWith('javascript:') || lower.startsWith('data:') || lower.startsWith('vbscript:')) {
      return false
    }
    if (trimmed.startsWith('/') || trimmed.startsWith('./')) return true
    try {
      const base = typeof window !== 'undefined' ? window.location.origin : 'http://localhost'
      const parsed = new URL(trimmed, base)
      return parsed.protocol === 'http:' || parsed.protocol === 'https:'
    } catch {
      return false
    }
  }

  unloadPlugin(id: string): void {
    if (typeof document !== 'undefined') {
      const existingIframe = document.getElementById(`plugin-sandbox-${id}`)
      if (existingIframe) {
        existingIframe.remove()
      }
      const existingScript = document.getElementById(`plugin-bundle-${id}`)
      if (existingScript) {
        existingScript.remove()
      }
    }
    delete this.state.loadedPlugins[id]
  }

  getRuntimeStatus(id: string): LoadedPluginRuntime | undefined {
    return this.state.loadedPlugins[id]
  }
}

export const pluginLoader = new PluginLoaderService()
