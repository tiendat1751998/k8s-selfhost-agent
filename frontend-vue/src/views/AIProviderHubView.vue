<script setup lang="ts">
import AIProvidersGrid from '../components/aihub/AIProvidersGrid.vue'
import AIProvidersTable from '../components/aihub/AIProvidersTable.vue'
import AIProvidersMobileCards from '../components/aihub/AIProvidersMobileCards.vue'
import AIPromptConsole from '../components/aihub/AIPromptConsole.vue'
import RegisterAIProviderModal from '../components/aihub/RegisterAIProviderModal.vue'
import AIProviderMetricsDrawer from '../components/aihub/AIProviderMetricsDrawer.vue'
import { useAIProviderHub } from '../composables/useAIProviderHub'
import '../assets/styles/views/aihub.css'
import '../assets/styles/components/aihub-drawers.css'

const {
  error,
  providers,
  activeSection,
  probingName,
  healthResults,
  quotas,
  telemetries,
  fallbackRoutes,
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
    <!-- Mobile 40-44px Command Bar (<768px) -->
    <div class="aihub-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold"><BaseIcon name="bot" size="sm" /> AI Hub ({{ providers.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          title="Add Provider"
          aria-label="Add Provider"
          @click="showAddModal = true"
        >
          <BaseIcon name="plus" size="xs" />
        </button>
        <button
          class="btn-icon-cmd"
          title="Toggle Console"
          aria-label="Toggle Console"
          @click="activeSection = activeSection === 'console' ? 'matrix' : 'console'"
        >
          <BaseIcon name="sparkles" size="xs" />
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px) -->
    <div class="aihub-micro-telemetry mobile-only font-mono" role="status" aria-label="AI Hub Micro Telemetry">
      <span class="tel-item tel-gw"><BaseIcon name="bot" size="xs" /> {{ providers.length }} gw</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-models"><BaseIcon name="cpu" size="xs" /> {{ distinctModelsCount }} models</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-lat"><BaseIcon name="zap" size="xs" /> {{ avgLatency }}</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-failover"><BaseIcon name="shield" size="xs" /> {{ failoverResilience }}</span>
    </div>

    <!-- Error Banner -->
    <div v-if="error" class="status-banner banner-error animate-fade-in">
      <BaseIcon name="alert-triangle" size="sm" class="banner-icon" />
      <span class="banner-text">{{ error }}</span>
      <button class="banner-close" @click="error = null" aria-label="Close"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Navigation Tabs & Toolbar Actions -->
    <div class="view-tabs-bar glass-panel">
      <div class="view-tabs" role="tablist" aria-label="AI Hub Views">
        <button 
          class="vtab-btn" 
          :class="{ active: activeSection === 'providers' || activeSection === 'matrix' }" 
          role="tab"
          :aria-selected="activeSection === 'providers' || activeSection === 'matrix'"
          @click="activeSection = 'providers'"
        >
          <BaseIcon name="grid" size="xs" /> <span>Grid Matrix</span>
          <span class="vtab-count">{{ providers.length }}</span>
        </button>
        <button 
          class="vtab-btn" 
          :class="{ active: activeSection === 'table' }" 
          role="tab"
          :aria-selected="activeSection === 'table'"
          @click="activeSection = 'table'"
        >
          <BaseIcon name="file-text" size="xs" /> <span>Routes Table</span>
        </button>
        <button 
          class="vtab-btn" 
          :class="{ active: activeSection === 'mobile' }" 
          role="tab"
          :aria-selected="activeSection === 'mobile'"
          @click="activeSection = 'mobile'"
        >
          <BaseIcon name="box" size="xs" /> <span>Mobile Stream</span>
        </button>
        <button 
          class="vtab-btn" 
          :class="{ active: activeSection === 'console' }" 
          role="tab"
          :aria-selected="activeSection === 'console'"
          @click="activeSection = 'console'"
        >
          <BaseIcon name="sparkles" size="xs" /> <span>Interactive Prompt Console</span>
        </button>
      </div>

      <div class="view-tabs-actions desktop-only">
        <button class="btn btn-secondary btn-sm" @click="activeSection = 'console'">
          <BaseIcon name="sparkles" size="xs" /> <span>Prompt Console</span>
        </button>
        <button class="btn btn-primary btn-sm" @click="showAddModal = true">
          <span>+ Add Custom Provider</span>
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
