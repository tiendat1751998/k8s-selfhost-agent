import { ref, reactive, computed, watch, onMounted, onUnmounted } from 'vue'
import {
  ecosystemApi,
  type DetectedTool,
  type EcosystemSummary,
  type CreateToolRequest
} from '../api/ecosystem'

export interface ConnectorPreset {
  id: string
  name: string
  category: string
  icon: string
  defaultEndpoint: string
  description: string
  authType: 'token' | 'basic' | 'oauth' | 'mtls'
}

export interface HealthProbeResult {
  latencyMs: number
  statusCode: number
  status: 'healthy' | 'degraded' | 'unreachable'
  timestamp: string
  dnsResolveMs: number
  tlsExpiryDays?: number
  message: string
}

export interface WebhookSyncRecord {
  id: string
  toolId: string
  event: string
  status: 'success' | 'failed' | 'pending'
  timestamp: string
  durationMs: number
  payloadSize: string
  details: string
}

export interface ToolErrorLog {
  id: string
  timestamp: string
  level: 'error' | 'warn' | 'info'
  message: string
  source: string
}

export interface ConnectFormData extends CreateToolRequest {
  apiToken?: string
  tlsVerify?: boolean
  syncInterval?: string
}

export const PRESET_CONNECTORS: ConnectorPreset[] = [
  { id: 'argocd', name: 'ArgoCD', category: 'gitops', icon: 'layers', defaultEndpoint: '', description: 'GitOps CD for Kubernetes', authType: 'token' },
  { id: 'prometheus', name: 'Prometheus', category: 'monitoring', icon: 'activity', defaultEndpoint: 'http://prometheus-k8s.monitoring.svc:9090', description: 'Monitoring & alerting toolkit', authType: 'basic' },
  { id: 'vault', name: 'HashiCorp Vault', category: 'secrets', icon: 'lock', defaultEndpoint: '', description: 'Secrets & encryption manager', authType: 'token' },
  { id: 'crossplane', name: 'Crossplane', category: 'compute', icon: 'zap', defaultEndpoint: 'https://crossplane.system.svc', description: 'Universal control plane', authType: 'token' },
  { id: 'cilium', name: 'Cilium', category: 'mesh', icon: 'shield', defaultEndpoint: 'http://cilium-agent.kube-system.svc:9879', description: 'eBPF networking & security', authType: 'mtls' },
  { id: 'istio', name: 'Istio Service Mesh', category: 'mesh', icon: 'globe', defaultEndpoint: 'http://istiod.istio-system.svc:15014', description: 'Service mesh traffic control', authType: 'mtls' }
]

export const ECOSYSTEM_CATEGORIES = [
  { key: 'all', label: 'All Categories', icon: 'globe' },
  { key: 'compute', label: 'Compute & Containers', icon: 'box' },
  { key: 'database', label: 'Databases & Storage', icon: 'database' },
  { key: 'messaging', label: 'Messaging & Streaming', icon: 'zap' },
  { key: 'mesh', label: 'Ingress & Mesh', icon: 'globe' },
  { key: 'gitops', label: 'GitOps & CI/CD', icon: 'layers' },
  { key: 'security', label: 'Security & Posture', icon: 'shield' },
  { key: 'monitoring', label: 'Monitoring & Telemetry', icon: 'activity' },
  { key: 'secrets', label: 'Secrets & KMS', icon: 'key' },
  { key: 'policy', label: 'Policy & Guardrails', icon: 'file-text' }
]

export function useEcosystem() {
  const loading = ref(false)
  const scanning = ref(false)
  const saving = ref(false)
  const deleting = ref<string | null>(null)
  const syncing = ref<string | null>(null)
  const error = ref<string | null>(null)
  const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)
  const VIEW_MODE_STORAGE_KEY = 'ecosystem_view_mode'
  const getInitialViewMode = (): 'grid' | 'table' => {
    if (typeof window === 'undefined') return 'table'
    try {
      const stored = localStorage.getItem(VIEW_MODE_STORAGE_KEY)
      if (stored === 'grid' || stored === 'table') return stored
    } catch {
      // Storage access blocked or unavailable
    }
    return 'table'
  }
  const viewMode = ref<'grid' | 'table'>(getInitialViewMode())

  watch(viewMode, (newMode) => {
    if (typeof window !== 'undefined') {
      try {
        localStorage.setItem(VIEW_MODE_STORAGE_KEY, newMode)
      } catch {
        // Storage access blocked or unavailable
      }
    }
  })

  let toastTimer: ReturnType<typeof setTimeout> | null = null
  let autoRefreshTimer: ReturnType<typeof setInterval> | null = null

  const tools = ref<DetectedTool[]>([])
  const summary = ref<EcosystemSummary>({ total: 0, healthy: 0, degraded: 0, by_category: {} })
  const activeCategory = ref('all')
  const searchQuery = ref('')
  const activeStatus = ref('all')
  const selectedStatus = activeStatus

  const showConnectModal = ref(false)
  const connectForm = reactive<ConnectFormData>({
    name: '', category: 'gitops', endpoint: '', version: '', status: 'detected', health: 'healthy', apiToken: '', tlsVerify: true, syncInterval: '5m'
  })

  const showHealthDrawer = ref(false)
  const selectedToolForHealth = ref<DetectedTool | null>(null)
  const healthProbeResult = ref<HealthProbeResult | null>(null)
  const isProbing = ref(false)
  const webhookHistory = ref<WebhookSyncRecord[]>([])
  const errorLogs = ref<ToolErrorLog[]>([])

  function showToast(text: string, type: 'success' | 'error' = 'success') {
    if (toastTimer) clearTimeout(toastTimer)
    toastMessage.value = { text, type }
    toastTimer = setTimeout(() => { toastMessage.value = null }, 4000)
  }

  function getToolIcon(tool: DetectedTool): string {
    const name = tool.name.toLowerCase()
    if (name.includes('docker')) return 'box'
    if (name.includes('postgres')) return 'database'
    if (name.includes('redis')) return 'database'
    if (name.includes('nats')) return 'zap'
    if (name.includes('traefik')) return 'globe'
    if (name.includes('drone')) return 'layers'
    if (name.includes('argo')) return 'layers'
    if (name.includes('trivy')) return 'shield'
    if (name.includes('grafana')) return 'activity'
    if (name.includes('vault')) return 'key'
    if (name.includes('prometheus')) return 'activity'
    if (name.includes('crossplane')) return 'zap'
    if (name.includes('cilium')) return 'shield'
    if (name.includes('kyverno')) return 'file-text'
    if (name.includes('istio')) return 'globe'
    if (name.includes('cert-manager') || name.includes('cert')) return 'lock'

    const catMap: Record<string, string> = {
      compute: 'box', database: 'database', messaging: 'zap', gitops: 'layers',
      security: 'shield', monitoring: 'activity', secrets: 'key', policy: 'file-text', mesh: 'globe', certificates: 'lock'
    }
    return catMap[tool.category.toLowerCase()] || 'box'
  }

  function formatRelativeTime(dateStr: string): string {
    if (!dateStr) return 'Never'
    const date = new Date(dateStr)
    if (isNaN(date.getTime())) return 'Never'
    const diffSec = Math.floor((Date.now() - date.getTime()) / 1000)
    if (diffSec < 60) return 'Just now'
    if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`
    if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h ago`
    return `${Math.floor(diffSec / 86400)}d ago`
  }

  async function loadData() {
    loading.value = true
    error.value = null
    try {
      const [toolList, sum] = await Promise.all([
        ecosystemApi.getTools(),
        ecosystemApi.getSummary().catch(() => null)
      ])
      tools.value = Array.isArray(toolList) ? toolList : []

      const byCategory: Record<string, number> = {}
      let healthyCount = 0
      let degradedCount = 0
      for (const t of tools.value) {
        byCategory[t.category] = (byCategory[t.category] || 0) + 1
        if (t.health === 'healthy') healthyCount++
        else if (t.health === 'degraded' || t.status === 'unreachable') degradedCount++
      }
      summary.value = sum && sum.total > 0 ? sum : {
        total: tools.value.length, healthy: healthyCount, degraded: degradedCount, by_category: byCategory
      }
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to load ecosystem tools'
      tools.value = []
    } finally {
      loading.value = false
    }
  }

  async function handleScan() {
    scanning.value = true
    try {
      const [updatedTools, updatedSummary] = await Promise.all([
        ecosystemApi.triggerScan(),
        ecosystemApi.getSummary().catch(() => null)
      ])
      tools.value = updatedTools || []
      if (updatedSummary) summary.value = updatedSummary
      showToast('Ecosystem scan completed successfully!')
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to run ecosystem scan', 'error')
    } finally {
      scanning.value = false
    }
  }

  function openConnectModal(preset?: ConnectorPreset | DetectedTool) {
    if (preset && 'description' in preset) {
      Object.assign(connectForm, { name: preset.name, category: preset.category, endpoint: preset.defaultEndpoint, version: 'v1.0.0', status: 'detected', health: 'healthy', apiToken: '' })
    } else if (preset && 'tenant_id' in preset) {
      Object.assign(connectForm, { name: preset.name, category: preset.category, endpoint: preset.endpoint || '', version: preset.version || '', status: preset.status || 'detected', health: preset.health || 'healthy', apiToken: '' })
    } else {
      Object.assign(connectForm, { name: '', category: 'gitops', endpoint: '', version: '', status: 'detected', health: 'healthy', apiToken: '' })
    }
    showConnectModal.value = true
  }

  function closeConnectModal() { showConnectModal.value = false }

  async function handleCreateTool() {
    if (!connectForm.name.trim() || !connectForm.category.trim()) {
      showToast('Name and Category are required', 'error')
      return
    }
    saving.value = true
    try {
      await ecosystemApi.createTool({
        name: connectForm.name.trim(), category: connectForm.category.toLowerCase().trim(),
        endpoint: connectForm.endpoint?.trim() || '', version: connectForm.version?.trim() || '',
        status: connectForm.status || 'detected', health: connectForm.health || 'healthy',
        metadata: { tlsVerify: String(connectForm.tlsVerify ?? true), syncInterval: connectForm.syncInterval || '5m' }
      })
      showToast(`Tool "${connectForm.name}" registered successfully!`)
      closeConnectModal()
      await loadData()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to register tool', 'error')
    } finally {
      saving.value = false
    }
  }

  async function handleDeleteTool(tool: DetectedTool) {
    if (!confirm(`Are you sure you want to remove / disconnect "${tool.name}"?`)) return
    deleting.value = tool.id
    try {
      await ecosystemApi.deleteTool(tool.id)
      showToast(`Tool "${tool.name}" disconnected`)
      if (selectedToolForHealth.value?.id === tool.id) closeHealthDrawer()
      await loadData()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to delete tool', 'error')
    } finally {
      deleting.value = null
    }
  }

  async function handleSyncWebhook(tool: DetectedTool) {
    syncing.value = tool.id
    try {
      await ecosystemApi.syncWebhook(tool.id)
      showToast(`Webhook sync dispatched for ${tool.name}`)
      if (selectedToolForHealth.value?.id === tool.id) {
        webhookHistory.value.unshift({
          id: `wh-${Date.now()}`, toolId: tool.id, event: 'manual_sync_trigger', status: 'success',
          timestamp: new Date().toISOString(), durationMs: 0,
          payloadSize: '--', details: 'Dispatched real-time status probe to ingress endpoint.'
        })
      }
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Webhook sync failed', 'error')
    } finally {
      syncing.value = null
    }
  }

  function openHealthDrawer(tool: DetectedTool) {
    selectedToolForHealth.value = tool
    showHealthDrawer.value = true
    runHealthProbe(tool)
  }

  function closeHealthDrawer() {
    showHealthDrawer.value = false
    selectedToolForHealth.value = null
    healthProbeResult.value = null
  }

  async function runHealthProbe(tool: DetectedTool) {
    isProbing.value = true
    webhookHistory.value = []
    errorLogs.value = []
    const startTime = performance.now()
    try {
      let isOk = tool.health === 'healthy' || tool.status === 'detected'
      let statusCode = isOk ? 200 : (tool.status === 'unreachable' ? 503 : 429)
      let message = isOk ? 'Service operational — Probes healthy.' : 'Degraded response: integration probe non-200.'

      if (tool.endpoint && (tool.endpoint.startsWith('http://') || tool.endpoint.startsWith('https://'))) {
        try {
          const controller = new AbortController()
          const timer = setTimeout(() => controller.abort(), 3000)
          await fetch(tool.endpoint, { method: 'HEAD', mode: 'no-cors', signal: controller.signal })
          clearTimeout(timer)
          isOk = true
          statusCode = 200
          message = `Integration endpoint reachable: ${tool.endpoint}`
        } catch (probeErr: unknown) {
          isOk = false
          statusCode = 503
          message = probeErr instanceof Error ? probeErr.message : 'Probe connection failed'
        }
      }

      const latency = Math.round(performance.now() - startTime)
      healthProbeResult.value = {
        latencyMs: isOk ? latency : 0,
        statusCode,
        status: isOk ? 'healthy' : 'degraded',
        timestamp: new Date().toISOString(),
        dnsResolveMs: 0,
        tlsExpiryDays: 0,
        message
      }
    } catch {
      healthProbeResult.value = {
        latencyMs: 0,
        statusCode: 504,
        status: 'unreachable',
        timestamp: new Date().toISOString(),
        dnsResolveMs: 0,
        message: 'Gateway Timeout: Failed to reach integration endpoint.'
      }
    } finally {
      isProbing.value = false
    }
  }

  const filteredTools = computed(() => {
    return tools.value.filter(tool => {
      if (activeCategory.value !== 'all' && tool.category.toLowerCase() !== activeCategory.value) return false
      if (activeStatus.value === 'healthy' && tool.health !== 'healthy') return false
      if (activeStatus.value === 'degraded' && tool.health !== 'degraded' && tool.status !== 'unreachable') return false
      if (activeStatus.value === 'not_configured' && tool.status !== 'not_configured') return false
      if (searchQuery.value.trim()) {
        const q = searchQuery.value.toLowerCase()
        return tool.name.toLowerCase().includes(q) || tool.category.toLowerCase().includes(q) ||
          (tool.endpoint && tool.endpoint.toLowerCase().includes(q)) || (tool.version && tool.version.toLowerCase().includes(q))
      }
      return true
    })
  })

  onMounted(() => {
    loadData()
    autoRefreshTimer = setInterval(() => loadData(), 5 * 60 * 1000)
  })

  onUnmounted(() => {
    if (autoRefreshTimer) clearInterval(autoRefreshTimer)
    if (toastTimer) clearTimeout(toastTimer)
  })

  return {
    loading, scanning, saving, deleting, syncing, error, toastMessage, viewMode,
    tools, summary, activeCategory, searchQuery, activeStatus, selectedStatus,
    categories: ECOSYSTEM_CATEGORIES, presets: PRESET_CONNECTORS,
    showConnectModal, connectForm, showHealthDrawer, selectedToolForHealth,
    healthProbeResult, isProbing, webhookHistory, errorLogs, filteredTools,
    showToast, getToolIcon, formatRelativeTime, loadData, handleScan,
    openConnectModal, closeConnectModal, handleCreateTool, handleDeleteTool,
    handleSyncWebhook, openHealthDrawer, closeHealthDrawer, runHealthProbe
  }
}
