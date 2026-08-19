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
          console.log(`[PluginLoader] Plugin "${name}" registered successfully.`, pluginDef)
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

    try {
      // Create script tag or dynamic import
      const existingScript = document.getElementById(`plugin-bundle-${plugin.id}`)
      if (existingScript) {
        existingScript.remove()
      }

      await new Promise<void>((resolve, reject) => {
        const script = document.createElement('script')
        script.id = `plugin-bundle-${plugin.id}`
        script.src = plugin.entry_point
        script.async = true
        script.crossOrigin = 'anonymous'

        script.onload = () => {
          resolve()
        }

        script.onerror = () => {
          reject(new Error(`Failed to fetch bundle from ${plugin.entry_point}`))
        }

        document.head.appendChild(script)
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

  unloadPlugin(id: string): void {
    const existingScript = document.getElementById(`plugin-bundle-${id}`)
    if (existingScript) {
      existingScript.remove()
    }
    delete this.state.loadedPlugins[id]
  }

  getRuntimeStatus(id: string): LoadedPluginRuntime | undefined {
    return this.state.loadedPlugins[id]
  }
}

export const pluginLoader = new PluginLoaderService()
