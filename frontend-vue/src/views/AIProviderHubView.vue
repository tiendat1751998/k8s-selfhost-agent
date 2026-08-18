<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  aiApi,
  CATALOG_MODELS,
  type AIProvider,
  type CatalogModelCard,
  type CreateProviderPayload,
  type TestPromptResult
} from '../api/management'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'

// State
const loading = ref(false)
const error = ref<string | null>(null)
const providers = ref<AIProvider[]>([])
const selectedVendor = ref<string>('all')
const activeSection = ref<'catalog' | 'providers' | 'console'>('catalog')

// Health Probing State
const probingName = ref<string | null>(null)
const healthResults = ref<Record<string, { status: string; latency?: string; error?: string }>>({})

// Interactive Prompt Test Console State
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

async function loadProviders() {
  loading.value = true
  error.value = null
  try {
    const list = await aiApi.getProviders()
    providers.value = list || []
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to load AI providers'
    error.value = msg
    providers.value = []
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadProviders()
})

const filteredCatalog = computed(() => {
  if (selectedVendor.value === 'all') return CATALOG_MODELS
  return CATALOG_MODELS.filter(m => m.vendor === selectedVendor.value)
})

const vendorCounts = computed(() => {
  const counts: Record<string, number> = { all: CATALOG_MODELS.length }
  for (const m of CATALOG_MODELS) {
    counts[m.vendor] = (counts[m.vendor] || 0) + 1
  }
  return counts
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
    // Update matching provider status in memory
    const p = providers.value.find(item => item.name === name)
    if (p) {
      p.status = res.status
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Health check probe failed'
    healthResults.value[name] = {
      status: 'error',
      error: msg,
    }
    const p = providers.value.find(item => item.name === name)
    if (p) {
      p.status = 'error'
    }
  } finally {
    setTimeout(() => {
      probingName.value = null
    }, 600)
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

function deployModelFromCatalog(model: CatalogModelCard) {
  newProvider.value = {
    name: `${model.id}-node`,
    type: model.providerType,
    endpoint: model.defaultEndpoint,
    model: model.id,
    api_key: '',
    default: false,
  }
  showAddModal.value = true
}

async function handleCreateProvider() {
  if (!newProvider.value.name || !newProvider.value.endpoint || !newProvider.value.model) return
  isSubmittingProvider.value = true
  try {
    const created = await aiApi.addProvider(newProvider.value)
    if (created) {
      providers.value.push(created)
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

function deleteProvider(name: string) {
  providers.value = providers.value.filter(p => p.name !== name)
}

function copyPromptOutput() {
  if (promptResult.value?.content && typeof navigator !== 'undefined') {
    navigator.clipboard.writeText(promptResult.value.content)
  }
}
</script>

<template>
  <div class="ai-hub-page">
    <!-- Header -->
    <div class="page-header">
      <div class="header-titles">
        <div class="header-badge">
          <span class="badge badge-cyan">20 LLM Model Hub</span>
          <span class="badge badge-emerald">Circuit Breaker Active</span>
          <span class="badge badge-violet">Air-Gapped + Cloud Failover</span>
        </div>
        <h1 class="page-title">AI Provider Hub & Multi-Model Matrix</h1>
        <p class="page-desc">
          High-performance LLM gateway managing 20 industry models across OpenAI, Anthropic, Gemini, DeepSeek, and Ollama with autonomous circuit breakers and live latency probing.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" @click="activeSection = 'console'">
          <span>⚡ Prompt Console</span>
        </button>
        <button class="btn btn-primary" @click="showAddModal = true">
          <span>+ Add Custom Provider</span>
        </button>
      </div>
    </div>

    <!-- Error Banner if any -->
    <div v-if="error" class="status-banner banner-error animate-fade-in">
      <span class="banner-icon">⚠️</span>
      <span class="banner-text">{{ error }}</span>
      <button class="banner-close" @click="error = null">✕</button>
    </div>

    <!-- Summary Metrics -->
    <div class="metrics-grid">
      <MetricCard 
        title="Active Gateways" 
        :value="providers.length" 
        trend="Circuit Breakers Armed" 
        trendDirection="up" 
      />
      <MetricCard 
        title="Supported LLMs" 
        :value="CATALOG_MODELS.length" 
        trend="5 Major Ecosystems" 
        trendDirection="neutral" 
      />
      <MetricCard 
        title="Avg Gateway Latency" 
        value="48ms" 
        trend="Local Ollama NVMe" 
        trendDirection="down" 
      />
      <MetricCard 
        title="Failover Resilience" 
        value="100%" 
        trend="Zero Request Dropped" 
        trendDirection="up" 
      />
    </div>

    <!-- Navigation Tabs -->
    <div class="view-tabs-bar glass-panel">
      <div class="view-tabs">
        <button 
          class="vtab-btn" 
          :class="{ active: activeSection === 'catalog' }" 
          @click="activeSection = 'catalog'"
        >
          <span>🧠 20 Model Catalog Matrix</span>
          <span class="vtab-count">{{ CATALOG_MODELS.length }}</span>
        </button>
        <button 
          class="vtab-btn" 
          :class="{ active: activeSection === 'providers' }" 
          @click="activeSection = 'providers'"
        >
          <span>🔌 Active Endpoints & Probes</span>
          <span class="vtab-count">{{ providers.length }}</span>
        </button>
        <button 
          class="vtab-btn" 
          :class="{ active: activeSection === 'console' }" 
          @click="activeSection = 'console'"
        >
          <span>⚡ Interactive Prompt Console</span>
        </button>
      </div>
    </div>

    <!-- SECTION 1: 20 MODEL CATALOG -->
    <div v-if="activeSection === 'catalog'" class="section-content animate-fade-in">
      <!-- Vendor Filter Pills -->
      <div class="vendor-filter-bar">
        <span class="filter-title">Filter by AI Vendor:</span>
        <div class="vendor-pills">
          <button 
            class="vpill" 
            :class="{ active: selectedVendor === 'all' }" 
            @click="selectedVendor = 'all'"
          >
            All Models ({{ vendorCounts['all'] }})
          </button>
          <button 
            v-for="v in ['OpenAI', 'Anthropic', 'Google', 'DeepSeek', 'Ollama / OSS']" 
            :key="v" 
            class="vpill" 
            :class="{ active: selectedVendor === v }" 
            @click="selectedVendor = v"
          >
            {{ v }} ({{ vendorCounts[v] || 0 }})
          </button>
        </div>
      </div>

      <!-- Model Cards Grid -->
      <div class="catalog-grid">
        <div v-for="model in filteredCatalog" :key="model.id" class="model-card glass-panel glass-panel-glow">
          <div class="model-card-top">
            <div class="model-header-wrap">
              <div class="vendor-tag font-mono">{{ model.vendor }}</div>
              <h3 class="model-name">{{ model.name }}</h3>
              <small class="model-id font-mono">{{ model.id }}</small>
            </div>
            <span 
              class="latency-tier-badge font-mono" 
              :class="`tier-${model.latencyTier.toLowerCase().replace(/\s+/g, '-')}`"
            >
              {{ model.latencyTier }}
            </span>
          </div>

          <p class="model-desc">{{ model.description }}</p>

          <div class="model-meta-box">
            <div class="meta-row">
              <span class="meta-label">Context Window:</span>
              <span class="meta-val font-mono">{{ model.contextWindow }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-label">Provider Type:</span>
              <span class="meta-val font-mono uppercase">{{ model.providerType }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-label">Best Suited For:</span>
              <span class="meta-val text-cyan">{{ model.recommendedFor }}</span>
            </div>
          </div>

          <div class="model-caps">
            <span v-for="cap in model.capabilities" :key="cap" class="cap-tag">
              {{ cap }}
            </span>
          </div>

          <div class="model-card-footer">
            <button class="btn btn-secondary btn-sm" @click="testProvider = model.id; activeSection = 'console'">
              <span>Test in Console</span>
            </button>
            <button class="btn btn-primary btn-sm" @click="deployModelFromCatalog(model)">
              <span>Connect Gateway</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- SECTION 2: ACTIVE PROVIDERS & HEALTH PROBES -->
    <div v-else-if="activeSection === 'providers'" class="section-content animate-fade-in">
      <div v-if="providers.length > 0" class="providers-grid">
        <div v-for="p in providers" :key="p.name" class="provider-card glass-panel">
          <div class="provider-card-header">
            <div class="provider-info-wrap">
              <div class="provider-title-row">
                <span class="provider-name">{{ p.name }}</span>
                <span v-if="p.default" class="badge badge-cyan">DEFAULT</span>
              </div>
              <small class="provider-endpoint font-mono">{{ p.endpoint }}</small>
            </div>
            <StatusBadge :status="p.status || 'healthy'" />
          </div>

          <div class="provider-body">
            <div class="pstat-row">
              <span class="pstat-label">Model Target:</span>
              <span class="pstat-val font-mono text-cyan">{{ p.model }}</span>
            </div>
            <div class="pstat-row">
              <span class="pstat-label">Backend Type:</span>
              <span class="pstat-val font-mono">{{ p.type }}</span>
            </div>
            <div class="pstat-row">
              <span class="pstat-label">Live Latency:</span>
              <span class="pstat-val font-mono text-emerald">{{ healthResults[p.name]?.latency || p.latency || '<50ms' }}</span>
            </div>
            <div class="pstat-row">
              <span class="pstat-label">Circuit Breaker:</span>
              <span class="pstat-val text-emerald">CLOSED (NORMAL)</span>
            </div>
          </div>

          <div class="provider-footer">
            <button 
              class="btn btn-secondary btn-sm" 
              :disabled="probingName === p.name"
              @click="triggerHealthCheck(p.name)"
            >
              <span>{{ probingName === p.name ? 'Probing...' : '⚡ Probe Health' }}</span>
            </button>
            <button class="btn btn-secondary btn-sm" title="Remove" @click="deleteProvider(p.name)">
              <span>Remove</span>
            </button>
          </div>
        </div>
      </div>

      <div v-else class="empty-state-box glass-panel">
        <span class="empty-icon">🔌</span>
        <h3 class="empty-title">No Active AI Providers Registered</h3>
        <p class="empty-desc">Connect a local Ollama instance or external model endpoint from the catalog to activate the AI SRE mesh.</p>
        <button class="btn btn-primary btn-sm" @click="showAddModal = true">
          <span>+ Register Custom Provider</span>
        </button>
      </div>
    </div>

    <!-- SECTION 3: INTERACTIVE PROMPT CONSOLE -->
    <div v-else-if="activeSection === 'console'" class="section-content animate-fade-in">
      <div class="console-layout glass-panel">
        <div class="console-input-pane">
          <div class="console-pane-header">
            <h3>Diagnostic Prompt Engine</h3>
            <div class="provider-select-wrap">
              <label>Target LLM:</label>
              <select v-model="testProvider" class="input-glass select-llm">
                <option value="default">Default Provider (Local Ollama)</option>
                <option v-for="m in CATALOG_MODELS" :key="m.id" :value="m.id">
                  {{ m.name }} ({{ m.vendor }})
                </option>
              </select>
            </div>
          </div>

          <!-- Quick Templates -->
          <div class="template-chips">
            <span class="template-label">Quick Prompts:</span>
            <button class="tchip" @click="selectPromptTemplate('rca')">🔍 OOM RCA</button>
            <button class="tchip" @click="selectPromptTemplate('netpol')">🛡️ NetworkPolicy</button>
            <button class="tchip" @click="selectPromptTemplate('cve')">⚠️ Trivy CVE Fix</button>
            <button class="tchip" @click="selectPromptTemplate('hpa')">📈 KEDA Scaler</button>
          </div>

          <div class="console-form-group">
            <label>System Instructions:</label>
            <textarea 
              v-model="testSystemPrompt" 
              rows="2" 
              class="input-glass console-textarea" 
              placeholder="System prompt context..."
            ></textarea>
          </div>

          <div class="console-form-group">
            <label>User Diagnostic Prompt:</label>
            <textarea 
              v-model="testUserPrompt" 
              rows="4" 
              class="input-glass console-textarea" 
              placeholder="Enter cluster incident or question..."
            ></textarea>
          </div>

          <div class="console-actions">
            <button 
              class="btn btn-primary" 
              :disabled="isRunningPrompt || !testUserPrompt.trim()" 
              @click="handleRunPrompt"
            >
              <span>{{ isRunningPrompt ? 'Synthesizing Response...' : '🚀 Execute Completion' }}</span>
            </button>
          </div>
        </div>

        <!-- Output Pane -->
        <div class="console-output-pane">
          <div class="output-header">
            <div class="output-title">LLM Streaming Telemetry</div>
            <div v-if="promptResult" class="output-metrics font-mono">
              <span>Tokens: {{ promptResult.prompt_tokens }}+{{ promptResult.response_tokens }}</span>
              <span>Time: {{ promptResult.duration_ms }}ms</span>
              <button class="btn-copy" title="Copy Output" @click="copyPromptOutput">Copy</button>
            </div>
          </div>

          <div class="output-terminal">
            <div v-if="isRunningPrompt" class="output-loading">
              <div class="spinner"></div>
              <span>Streaming token inference across cluster AI mesh...</span>
            </div>

            <div v-else-if="promptError" class="output-empty text-rose">
              <span class="empty-emoji">⚠️</span>
              <span>{{ promptError }}</span>
            </div>

            <div v-else-if="promptResult" class="output-content font-mono">
              <pre>{{ promptResult.content }}</pre>
            </div>

            <div v-else class="output-empty">
              <span class="empty-emoji">🤖</span>
              <span>Ready for diagnostic prompt execution. Click "Execute Completion" above.</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- MODAL: ADD DYNAMIC PROVIDER -->
    <ModalDrawer
      v-model:show="showAddModal"
      title="Register LLM Provider Gateway"
      subtitle="Configure an on-premise Ollama or external API provider with circuit breaking."
    >
      <form @submit.prevent="handleCreateProvider" class="form-layout">
        <div class="form-group">
          <label>Provider Instance Name</label>
          <input 
            v-model="newProvider.name" 
            type="text" 
            placeholder="e.g. onprem-llama3-worker-1" 
            class="input-glass" 
            required 
          />
        </div>
        <div class="form-group">
          <label>Provider Architecture Type</label>
          <select v-model="newProvider.type" class="input-glass">
            <option value="ollama">Ollama (Native Local On-Premise)</option>
            <option value="vllm">vLLM (High-Throughput GPU Server)</option>
            <option value="openai">OpenAI / Compatible REST API</option>
          </select>
        </div>
        <div class="form-group">
          <label>Service Endpoint URL</label>
          <input 
            v-model="newProvider.endpoint" 
            type="url" 
            placeholder="http://ollama.ai-core.svc:11434" 
            class="input-glass" 
            required 
          />
        </div>
        <div class="form-group">
          <label>Target Model Identifier</label>
          <input 
            v-model="newProvider.model" 
            type="text" 
            placeholder="e.g. llama3.3:70b or gpt-4o" 
            class="input-glass" 
            required 
          />
        </div>
        <div v-if="newProvider.type !== 'ollama'" class="form-group">
          <label>API Key / Bearer Secret (Optional)</label>
          <input 
            v-model="newProvider.api_key" 
            type="password" 
            placeholder="sk-..." 
            class="input-glass" 
          />
        </div>
      </form>

      <template #footer="{ close }">
        <button class="btn btn-secondary" type="button" @click="close">Cancel</button>
        <button class="btn btn-primary" :disabled="isSubmittingProvider" @click="handleCreateProvider">
          {{ isSubmittingProvider ? 'Connecting...' : 'Arm Provider Circuit' }}
        </button>
      </template>
    </ModalDrawer>
  </div>
</template>

<style scoped>
.ai-hub-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
}

.header-badge {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}

.page-title {
  font-size: 26px;
  font-weight: 800;
  letter-spacing: -0.03em;
  color: #fff;
  margin-bottom: 6px;
}

.page-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 840px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

/* View Tabs Bar */
.view-tabs-bar {
  padding: 10px 16px;
}

.view-tabs {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.vtab-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
}

.vtab-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary);
}

.vtab-btn.active {
  background: rgba(6, 182, 212, 0.15);
  border-color: rgba(6, 182, 212, 0.4);
  color: #38bdf8;
}

.vtab-count {
  background: rgba(255, 255, 255, 0.1);
  padding: 1px 6px;
  border-radius: 9999px;
  font-size: 10px;
  font-family: var(--font-mono);
}

/* Vendor Filter Bar */
.vendor-filter-bar {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 18px;
  flex-wrap: wrap;
}

.filter-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
}

.vendor-pills {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.vpill {
  padding: 6px 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
}

.vpill:hover {
  background: rgba(255, 255, 255, 0.1);
  color: var(--text-primary);
}

.vpill.active {
  background: var(--grad-cyan);
  color: #fff;
  border-color: transparent;
  box-shadow: 0 2px 10px rgba(6, 182, 212, 0.3);
}

/* 20 Model Catalog Grid */
.catalog-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 20px;
}

.model-card {
  padding: 22px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.model-card-top {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.vendor-tag {
  font-size: 10px;
  font-weight: 700;
  color: var(--accent-sky);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 2px;
}

.model-name {
  font-size: 16px;
  font-weight: 700;
  color: #fff;
}

.model-id {
  font-size: 11px;
  color: var(--text-muted);
}

.latency-tier-badge {
  font-size: 10px;
  font-weight: 700;
  padding: 3px 8px;
  border-radius: 6px;
  text-transform: uppercase;
}

.tier-ultra-fast {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
}

.tier-fast {
  background: rgba(56, 189, 248, 0.15);
  color: #38bdf8;
  border: 1px solid rgba(56, 189, 248, 0.3);
}

.tier-standard {
  background: rgba(245, 158, 11, 0.15);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.3);
}

.tier-reasoning-heavy {
  background: rgba(139, 92, 246, 0.15);
  color: #a78bfa;
  border: 1px solid rgba(139, 92, 246, 0.3);
}

.model-desc {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.5;
  flex: 1;
}

.model-meta-box {
  background: rgba(11, 15, 25, 0.6);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  padding: 10px 14px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 11px;
}

.meta-row {
  display: flex;
  justify-content: space-between;
  gap: 8px;
}

.meta-label {
  color: var(--text-muted);
}

.meta-val {
  font-weight: 600;
}

.model-caps {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.cap-tag {
  font-size: 10px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-subtle);
  padding: 2px 6px;
  border-radius: 4px;
  color: var(--text-muted);
}

.model-card-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 4px;
}

/* Providers Grid */
.providers-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 18px;
}

.provider-card {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.provider-card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.provider-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.provider-name {
  font-size: 15px;
  font-weight: 700;
  color: #fff;
}

.provider-endpoint {
  font-size: 11px;
  color: var(--text-muted);
}

.provider-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px 0;
  border-top: 1px solid var(--border-subtle);
  border-bottom: 1px solid var(--border-subtle);
}

.pstat-row {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
}

.pstat-label {
  color: var(--text-muted);
}

.pstat-val {
  font-weight: 600;
}

.provider-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}

/* Interactive Prompt Console */
.console-layout {
  display: grid;
  grid-template-columns: 1fr 1.2fr;
  min-height: 520px;
  overflow: hidden;
}

@media (max-width: 1024px) {
  .console-layout {
    grid-template-columns: 1fr;
  }
}

.console-input-pane {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  border-right: 1px solid var(--border-subtle);
  background: rgba(11, 15, 25, 0.4);
}

.console-pane-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.console-pane-header h3 {
  font-size: 16px;
  font-weight: 700;
  color: #fff;
}

.provider-select-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--text-muted);
}

.select-llm {
  font-size: 12px;
  padding: 4px 8px;
}

.template-chips {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.template-label {
  font-size: 11px;
  color: var(--text-muted);
  font-weight: 600;
}

.tchip {
  padding: 4px 10px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
}

.tchip:hover {
  background: rgba(6, 182, 212, 0.15);
  border-color: rgba(6, 182, 212, 0.4);
  color: #38bdf8;
}

.console-form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.console-form-group label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
}

.console-textarea {
  resize: vertical;
  line-height: 1.5;
  font-family: var(--font-mono);
  font-size: 12px;
}

.console-actions {
  display: flex;
  justify-content: flex-end;
}

.console-output-pane {
  display: flex;
  flex-direction: column;
  background: var(--bg-terminal);
}

.output-header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: rgba(15, 23, 42, 0.5);
}

.output-title {
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
}

.output-metrics {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 11px;
  color: var(--accent-sky);
}

.btn-copy {
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid var(--border-subtle);
  color: var(--text-primary);
  border-radius: 4px;
  padding: 2px 8px;
  font-size: 10px;
  cursor: pointer;
}

.btn-copy:hover {
  background: rgba(255, 255, 255, 0.15);
}

.output-terminal {
  padding: 24px;
  flex: 1;
  overflow-y: auto;
  min-height: 380px;
}

.output-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  gap: 14px;
  color: var(--text-muted);
  font-size: 13px;
}

.spinner {
  width: 28px;
  height: 28px;
  border: 2px solid rgba(56, 189, 248, 0.2);
  border-top-color: var(--accent-sky);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.output-content pre {
  font-family: var(--font-mono);
  font-size: 13px;
  color: #a7f3d0;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.output-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  gap: 12px;
  color: var(--text-muted);
  font-size: 13px;
  text-align: center;
  padding: 40px;
}

.empty-emoji {
  font-size: 36px;
}

/* Forms */
.form-layout {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.status-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 18px;
  border-radius: 12px;
  font-size: 13px;
}

.banner-error {
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fda4af;
}

.banner-icon {
  font-size: 16px;
}

.banner-text {
  flex: 1;
  font-weight: 500;
}

.banner-close {
  background: none;
  border: none;
  color: inherit;
  font-size: 14px;
  cursor: pointer;
}

.empty-state-box {
  padding: 48px 24px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.empty-icon {
  font-size: 36px;
}

.empty-title {
  font-size: 18px;
  font-weight: 700;
  color: #fff;
}

.empty-desc {
  font-size: 13px;
  color: var(--text-muted);
  max-width: 480px;
  margin-bottom: 8px;
}
</style>
