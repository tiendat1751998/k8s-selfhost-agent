<script setup lang="ts">
import MetricCard from '../components/ui/MetricCard.vue'
import AIProvidersGrid from '../components/aihub/AIProvidersGrid.vue'
import AIProvidersTable from '../components/aihub/AIProvidersTable.vue'
import AIProvidersMobileCards from '../components/aihub/AIProvidersMobileCards.vue'
import AIPromptConsole from '../components/aihub/AIPromptConsole.vue'
import RegisterAIProviderModal from '../components/aihub/RegisterAIProviderModal.vue'
import AIProviderMetricsDrawer from '../components/aihub/AIProviderMetricsDrawer.vue'
import { useAIProviderHub } from '../composables/useAIProviderHub'
import '../assets/styles/views/aihub.css'

const {
  error,
  providers,
  activeSection,
  probingName,
  healthResults,
  quotas,
  telemetries,
  fallbackRoutes,
  healthyProvidersCount,
  distinctModelsCount,
  avgLatency,
  failoverResilience,
  triggerHealthCheck,
  deleteProvider,
  testProvider,
  testSystemPrompt,
  testUserPrompt,
  isRunningPrompt,
  promptResult,
  promptError,
  handleRunPrompt,
  selectPromptTemplate,
  copyPromptOutput,
  showAddModal,
  isSubmittingProvider,
  handleCreateProvider,
  showMetricsDrawer,
  selectedMetricsProvider,
  openMetricsDrawer,
  openRoutingModal,
} = useAIProviderHub()
</script>

<template>
  <div class="ai-hub-page">
    <!-- Header -->
    <div class="page-header desktop-header desktop-only">
      <div class="header-titles">
        <div class="header-badge">
          <span class="badge badge-cyan">{{ providers.length }} LLM {{ providers.length === 1 ? 'Gateway' : 'Gateways' }}</span>
          <span class="badge badge-emerald">Circuit Breaker Active</span>
          <span class="badge badge-violet">Air-Gapped + Cloud Failover</span>
        </div>
        <h1 class="page-title">AI Provider Hub & Multi-Model Matrix</h1>
        <p class="page-desc">
          High-performance LLM gateway managing enterprise models across OpenAI, Anthropic, Gemini, DeepSeek, and Ollama with autonomous circuit breakers and live latency probing.
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

    <!-- Mobile 40px Command Bar (<640px) -->
    <div class="aihub-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">🤖 AI Hub ({{ providers.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          title="Add Provider"
          aria-label="Add Provider"
          @click="showAddModal = true"
        >
          <span>➕</span>
        </button>
        <button
          class="btn-icon-cmd"
          title="Toggle Console"
          aria-label="Toggle Console"
          @click="activeSection = activeSection === 'console' ? 'matrix' : 'console'"
        >
          <span>⚡</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<640px) -->
    <div class="aihub-micro-telemetry mobile-only font-mono" role="status" aria-label="AI Hub Micro Telemetry">
      <span class="tel-item tel-gw">🤖 {{ providers.length }} gw</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-models">🧠 {{ distinctModelsCount }} models</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-lat">⚡ {{ avgLatency }}</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-failover">🛡️ {{ failoverResilience }}</span>
    </div>

    <!-- Error Banner -->
    <div v-if="error" class="status-banner banner-error animate-fade-in">
      <span class="banner-icon">⚠️</span>
      <span class="banner-text">{{ error }}</span>
      <button class="banner-close" @click="error = null">✕</button>
    </div>

    <!-- Summary Metrics -->
    <div class="metrics-grid desktop-metrics desktop-only">
      <MetricCard 
        title="Active Gateways" 
        :value="providers.length" 
        :trend="providers.length > 0 ? 'Circuit Breakers Armed' : 'No active gateways'" 
        :trendType="providers.length > 0 ? 'positive' : 'neutral'" 
      />
      <MetricCard 
        title="Configured Models" 
        :value="distinctModelsCount" 
        :trend="distinctModelsCount > 0 ? `${providers.length} Endpoints Active` : 'No models registered'" 
        trendType="neutral" 
      />
      <MetricCard 
        title="Avg Gateway Latency" 
        :value="avgLatency" 
        :trend="avgLatency !== '—' ? 'Live Telemetry Probe' : 'No latency probes yet'" 
        :trendType="avgLatency !== '—' ? 'positive' : 'neutral'" 
      />
      <MetricCard 
        title="Failover Resilience" 
        :value="failoverResilience" 
        :trend="failoverResilience !== '—' ? `${healthyProvidersCount}/${providers.length} Endpoints Healthy` : 'No active gateways'" 
        :trendType="failoverResilience !== '—' ? 'positive' : 'neutral'" 
      />
    </div>

    <!-- Navigation Tabs -->
    <div class="view-tabs-bar glass-panel">
      <div class="view-tabs">
        <button 
          class="vtab-btn" 
          :class="{ active: activeSection === 'providers' || activeSection === 'matrix' }" 
          @click="activeSection = 'providers'"
        >
          <span>🔌 Grid Matrix</span>
          <span class="vtab-count">{{ providers.length }}</span>
        </button>
        <button 
          class="vtab-btn" 
          :class="{ active: activeSection === 'table' }" 
          @click="activeSection = 'table'"
        >
          <span>📋 Routes Table</span>
        </button>
        <button 
          class="vtab-btn" 
          :class="{ active: activeSection === 'mobile' }" 
          @click="activeSection = 'mobile'"
        >
          <span>📱 Mobile Stream</span>
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

    <!-- VIEW 1: GRID MATRIX -->
    <AIProvidersGrid
      v-if="activeSection === 'providers' || activeSection === 'matrix'"
      :providers="providers"
      :probingName="probingName"
      :healthResults="healthResults"
      :quotas="quotas"
      @probe="triggerHealthCheck"
      @testInConsole="(name) => { testProvider = name; activeSection = 'console'; }"
      @remove="deleteProvider"
      @openMetrics="openMetricsDrawer"
      @register="showAddModal = true"
    />

    <!-- VIEW 2: ROUTES TABLE -->
    <AIProvidersTable
      v-else-if="activeSection === 'table'"
      :providers="providers"
      :probingName="probingName"
      :healthResults="healthResults"
      :quotas="quotas"
      :telemetries="telemetries"
      :fallbackRoutes="fallbackRoutes"
      @probe="triggerHealthCheck"
      @testInConsole="(name) => { testProvider = name; activeSection = 'console'; }"
      @openRouting="openRoutingModal"
      @openMetrics="openMetricsDrawer"
      @remove="deleteProvider"
      @register="showAddModal = true"
    />

    <!-- VIEW 3: MOBILE STREAM -->
    <AIProvidersMobileCards
      v-else-if="activeSection === 'mobile'"
      :providers="providers"
      :probingName="probingName"
      :healthResults="healthResults"
      :quotas="quotas"
      @probe="triggerHealthCheck"
      @testInConsole="(name) => { testProvider = name; activeSection = 'console'; }"
      @openMetrics="openMetricsDrawer"
      @remove="deleteProvider"
      @register="showAddModal = true"
    />

    <!-- VIEW 4: INTERACTIVE PROMPT CONSOLE -->
    <AIPromptConsole
      v-else-if="activeSection === 'console'"
      :providers="providers"
      :modelValueProvider="testProvider"
      :systemPrompt="testSystemPrompt"
      :userPrompt="testUserPrompt"
      :isRunning="isRunningPrompt"
      :promptResult="promptResult"
      :promptError="promptError"
      @update:modelValueProvider="testProvider = $event"
      @update:systemPrompt="testSystemPrompt = $event"
      @update:userPrompt="testUserPrompt = $event"
      @runPrompt="handleRunPrompt"
      @selectTemplate="selectPromptTemplate"
      @copyOutput="copyPromptOutput"
    />

    <!-- MODAL: REGISTER PROVIDER -->
    <RegisterAIProviderModal
      v-model:show="showAddModal"
      :isSubmitting="isSubmittingProvider"
      @submit="handleCreateProvider"
    />

    <!-- DRAWER: METRICS & QUOTA -->
    <AIProviderMetricsDrawer
      v-model:show="showMetricsDrawer"
      :provider="selectedMetricsProvider"
      :quota="selectedMetricsProvider ? quotas[selectedMetricsProvider.name] : undefined"
      :telemetry="selectedMetricsProvider ? telemetries[selectedMetricsProvider.name] : undefined"
      :fallbackRoute="selectedMetricsProvider ? fallbackRoutes[selectedMetricsProvider.name] : undefined"
    />
  </div>
</template>
