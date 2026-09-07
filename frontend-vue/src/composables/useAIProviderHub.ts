import { ref, computed, onMounted } from 'vue'
import {
  aiApi,
  type AIProvider,
  type CreateProviderPayload,
  type TestPromptResult
} from '../api/management'

export interface ProviderQuota {
  usedTokens: number
  maxTokens: number
  costSpentUsd: number
  rpmLimit: number
  tpmLimit: number
  costPer1kPrompt: number
  costPer1kCompletion: number
  promptTokens: number
  completionTokens: number
}

export interface LatencyDataPoint {
  timestamp: string
  latencyMs: number
}

export interface LatencyTelemetry {
  currentMs: number
  p95Ms: number
  p99Ms: number
  errorRate: number
  history: LatencyDataPoint[]
}

export interface FallbackRoute {
  providerName: string
  fallbackProviderName: string
  timeoutMs: number
  circuitThresholdFailures: number
  active: boolean
  mode: 'auto' | 'manual' | 'round-robin'
}

export interface ProviderPreset {
  id: string
  name: string
  type: 'ollama' | 'openai' | 'vllm'
  defaultModel: string
  defaultEndpoint: string
  badge: string
}

export const AI_PROVIDER_PRESETS: ProviderPreset[] = [
  {
    id: 'openai',
    name: 'OpenAI GPT-4o Matrix',
    type: 'openai',
    defaultModel: 'gpt-4o',
    defaultEndpoint: 'https://api.openai.com/v1',
    badge: 'Cloud Enterprise',
  },
  {
    id: 'anthropic',
    name: 'Anthropic Claude 3.5',
    type: 'openai',
    defaultModel: 'claude-3-5-sonnet-20241022',
    defaultEndpoint: 'https://api.anthropic.com/v1',
    badge: 'Reasoning SRE',
  },
  {
    id: 'gemini',
    name: 'Google Gemini 1.5 Flash',
    type: 'openai',
    defaultModel: 'gemini-1.5-flash',
    defaultEndpoint: 'https://generativelanguage.googleapis.com/v1beta/openai',
    badge: 'Low Latency',
  },
  {
    id: 'ollama',
    name: 'Local Ollama Mesh',
    type: 'ollama',
    defaultModel: 'llama3.3:70b',
    defaultEndpoint: 'http://ollama-service.ai-core.svc:11434',
    badge: 'On-Prem Baremetal',
  },
  {
    id: 'localllm',
    name: 'vLLM High-Throughput Node',
    type: 'vllm',
    defaultModel: 'qwen2.5-coder-32b',
    defaultEndpoint: 'http://vllm-service.ai-core.svc:8000/v1',
    badge: 'Air-Gapped GPU',
  },
]

export function useAIProviderHub() {
  // State
  const loading = ref(false)
  const error = ref<string | null>(null)
  const providers = ref<AIProvider[]>([])
  const activeSection = ref<'providers' | 'table' | 'mobile' | 'console' | 'matrix'>('providers')

  // Health Probing State
  const probingName = ref<string | null>(null)
  const healthResults = ref<Record<string, { status: string; latency?: string; error?: string }>>({})

  // Telemetry, Quotas & Routing State
  const quotas = ref<Record<string, ProviderQuota>>({})
  const telemetries = ref<Record<string, LatencyTelemetry>>({})
  const fallbackRoutes = ref<Record<string, FallbackRoute>>({})

  // Interactive Prompt Console State
  const testProvider = ref<string>('default')
  const testSystemPrompt = ref<string>('You are an expert Kubernetes Site Reliability Engineer and DevSecOps co-pilot.')
  const testUserPrompt = ref<string>('Analyze an OOMKilled Pod in namespace "production-east" and generate remediation steps.')
  const isRunningPrompt = ref(false)
  const promptResult = ref<TestPromptResult | null>(null)
  const promptError = ref<string | null>(null)

  // Add Provider Modal State
  const showAddModal = ref(false)
  const isSubmittingProvider = ref(false)
  const newProvider = ref<CreateProviderPayload>({
    name: '',
    type: 'ollama',
    endpoint: 'http://ollama-service.ai-core.svc:11434',
    model: 'llama3.3:70b',
    api_key: '',
    default: false,
  })

  // Metrics Drawer State
  const showMetricsDrawer = ref(false)
  const selectedMetricsProvider = ref<AIProvider | null>(null)

  // Routing Modal State
  const showRoutingModal = ref(false)
  const selectedRoutingProvider = ref<AIProvider | null>(null)

  function initProviderState(p: AIProvider) {
    if (!quotas.value[p.name]) {
      quotas.value[p.name] = {
        usedTokens: 0,
        maxTokens: 1000000,
        costSpentUsd: 0,
        rpmLimit: 500,
        tpmLimit: 100000,
        costPer1kPrompt: p.type === 'ollama' ? 0 : 0.005,
        costPer1kCompletion: p.type === 'ollama' ? 0 : 0.015,
        promptTokens: 0,
        completionTokens: 0,
      }
    }
    if (!telemetries.value[p.name]) {
      const baseMs = parseInt(p.latency || '0') || 0
      telemetries.value[p.name] = {
        currentMs: baseMs,
        p95Ms: baseMs > 0 ? Math.round(baseMs * 1.4) : 0,
        p99Ms: baseMs > 0 ? Math.round(baseMs * 2.1) : 0,
        errorRate: 0,
        history: [],
      }
    }
    if (!fallbackRoutes.value[p.name]) {
      fallbackRoutes.value[p.name] = {
        providerName: p.name,
        fallbackProviderName: 'Local Ollama Mesh',
        timeoutMs: 2500,
        circuitThresholdFailures: 3,
        active: true,
        mode: 'auto',
      }
    }
  }

  async function loadProviders() {
    loading.value = true
    error.value = null
    try {
      const list = await aiApi.getProviders()
      providers.value = (list || []).map(p => ({
        ...p,
        status: p.status || 'ready',
        latency: p.latency || '--',
      }))
      providers.value.forEach(initProviderState)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load AI providers'
      error.value = msg
      providers.value = []
    } finally {
      loading.value = false
    }
  }

  function isHealthy(p: { name: string; status?: string }) {
    const probe = healthResults.value[p.name]
    const s = (probe?.status || p.status || '').toLowerCase()
    return s === 'healthy' || s === 'ready' || s === 'active' || s === 'ok' || s === ''
  }

  const healthyProvidersCount = computed(() => providers.value.filter(isHealthy).length)
  const distinctModelsCount = computed(() => new Set(providers.value.map(p => p.model).filter(Boolean)).size)

  const avgLatency = computed(() => {
    const latencies: number[] = []
    for (const p of providers.value) {
      const probe = healthResults.value[p.name]
      if (probe?.latency) {
        const parsed = parseInt(probe.latency)
        if (!isNaN(parsed)) latencies.push(parsed)
      } else if (p.latency) {
        const parsed = parseInt(p.latency)
        if (!isNaN(parsed)) latencies.push(parsed)
      }
    }
    if (latencies.length === 0) return '--'
    const avg = Math.round(latencies.reduce((a, b) => a + b, 0) / latencies.length)
    return `${avg}ms`
  })

  const failoverResilience = computed(() => {
    if (providers.value.length === 0) return '—'
    return `${Math.round((healthyProvidersCount.value / providers.value.length) * 100)}%`
  })

  async function triggerHealthCheck(name: string) {
    probingName.value = name
    const startTime = performance.now()
    try {
      const res = await aiApi.healthCheckProvider(name)
      const elapsed = Math.round(performance.now() - startTime)
      healthResults.value[name] = {
        status: res.status,
        latency: `${elapsed}ms`,
        error: res.error,
      }
      const p = providers.value.find(item => item.name === name)
      if (p) p.status = res.status

      if (telemetries.value[name]) {
        telemetries.value[name].currentMs = elapsed
        telemetries.value[name].history.push({
          timestamp: 'just now',
          latencyMs: elapsed,
        })
        if (telemetries.value[name].history.length > 20) {
          telemetries.value[name].history.shift()
        }
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Health check probe failed'
      healthResults.value[name] = { status: 'error', error: msg }
      const p = providers.value.find(item => item.name === name)
      if (p) p.status = 'error'
    } finally {
      probingName.value = null
    }
  }

  async function handleRunPrompt() {
    if (!testUserPrompt.value.trim()) return
    isRunningPrompt.value = true
    promptResult.value = null
    promptError.value = null

    try {
      const res = await aiApi.testPrompt({
        provider: testProvider.value === 'default' ? undefined : testProvider.value,
        prompt: testUserPrompt.value,
        system: testSystemPrompt.value,
      })
      promptResult.value = res

      const target = testProvider.value === 'default' ? providers.value[0]?.name : testProvider.value
      if (target && quotas.value[target] && res) {
        quotas.value[target].usedTokens += (res.prompt_tokens + res.response_tokens)
        quotas.value[target].promptTokens += res.prompt_tokens
        quotas.value[target].completionTokens += res.response_tokens
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'AI diagnostic prompt execution failed'
      promptError.value = msg
    } finally {
      isRunningPrompt.value = false
    }
  }

  function selectPromptTemplate(template: string) {
    switch (template) {
      case 'rca':
        testUserPrompt.value = 'Analyze an OOMKilled Pod in namespace "production-east" and generate remediation steps.'
        break
      case 'netpol':
        testUserPrompt.value = 'Generate an air-gapped Kubernetes NetworkPolicy restricting egress to port 5432 PostgreSQL only.'
        break
      case 'cve':
        testUserPrompt.value = 'Explain CVE-2024-21626 (runc container escape) and provide verification commands for worker nodes.'
        break
      case 'hpa':
        testUserPrompt.value = 'Construct a KEDA ScaledObject triggering on RabbitMQ queue length exceeding 500 messages.'
        break
    }
  }

  function copyPromptOutput() {
    if (promptResult.value?.content && typeof navigator !== 'undefined') {
      navigator.clipboard.writeText(promptResult.value.content)
    }
  }

  async function handleCreateProvider(customPayload?: CreateProviderPayload) {
    const payload = customPayload || newProvider.value
    if (!payload.name || !payload.endpoint || !payload.model) return
    isSubmittingProvider.value = true
    try {
      const created = await aiApi.addProvider(payload)
      if (created) {
        providers.value.push(created)
        initProviderState(created)
      } else {
        await loadProviders()
      }
      showAddModal.value = false
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to register provider'
      error.value = msg
    } finally {
      isSubmittingProvider.value = false
    }
  }

  async function deleteProvider(name: string) {
    try {
      await aiApi.deleteProvider(name)
    } catch {
      // best-effort cleanup
    }
    providers.value = providers.value.filter(p => p.name !== name)
    delete quotas.value[name]
    delete telemetries.value[name]
    delete fallbackRoutes.value[name]
  }

  function openMetricsDrawer(provider: AIProvider) {
    selectedMetricsProvider.value = provider
    showMetricsDrawer.value = true
  }

  function closeMetricsDrawer() {
    showMetricsDrawer.value = false
    selectedMetricsProvider.value = null
  }

  function openRoutingModal(provider: AIProvider) {
    selectedRoutingProvider.value = provider
    showRoutingModal.value = true
  }

  function closeRoutingModal() {
    showRoutingModal.value = false
    selectedRoutingProvider.value = null
  }

  function updateFallbackRoute(providerName: string, partial: Partial<FallbackRoute>) {
    if (!fallbackRoutes.value[providerName]) {
      fallbackRoutes.value[providerName] = {
        providerName,
        fallbackProviderName: 'Local Ollama Mesh',
        timeoutMs: 2500,
        circuitThresholdFailures: 3,
        active: true,
        mode: 'auto',
      }
    }
    Object.assign(fallbackRoutes.value[providerName], partial)
  }

  function applyPreset(presetId: string) {
    const preset = AI_PROVIDER_PRESETS.find(p => p.id === presetId)
    if (!preset) return
    newProvider.value = {
      name: `${preset.id}-gateway-${Date.now().toString(36)}`,
      type: preset.type,
      endpoint: preset.defaultEndpoint,
      model: preset.defaultModel,
      api_key: '',
      default: false,
    }
  }

  onMounted(() => {
    loadProviders()
  })

  return {
    // Core state
    loading,
    error,
    providers,
    activeSection,
    probingName,
    healthResults,
    quotas,
    telemetries,
    fallbackRoutes,
    // Metrics
    healthyProvidersCount,
    distinctModelsCount,
    avgLatency,
    failoverResilience,
    // Methods
    isHealthy,
    loadProviders,
    triggerHealthCheck,
    deleteProvider,
    // Console
    testProvider,
    testSystemPrompt,
    testUserPrompt,
    isRunningPrompt,
    promptResult,
    promptError,
    handleRunPrompt,
    selectPromptTemplate,
    copyPromptOutput,
    // Modals & Drawers
    showAddModal,
    isSubmittingProvider,
    newProvider,
    handleCreateProvider,
    applyPreset,
    showMetricsDrawer,
    selectedMetricsProvider,
    openMetricsDrawer,
    closeMetricsDrawer,
    showRoutingModal,
    selectedRoutingProvider,
    openRoutingModal,
    closeRoutingModal,
    updateFallbackRoute,
  }
}
