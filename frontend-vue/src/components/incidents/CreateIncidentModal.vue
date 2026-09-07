<script setup lang="ts">
import { ref } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import StatusBadge from '../ui/StatusBadge.vue'
import type { SimulationScenario, CreateIncidentPayload } from '../../composables/useIncidents'

defineProps<{
  show: boolean
  actionLoading?: string | null
  scenarios: SimulationScenario[]
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'simulate', scenario: SimulationScenario): void
  (e: 'create', payload: CreateIncidentPayload): void
}>()

const activeTab = ref<'simulate' | 'report'>('simulate')

const form = ref<CreateIncidentPayload>({
  pod_name: '',
  namespace: 'ecommerce',
  cluster_name: 'prod-us-east-1',
  type: 'CrashLoopBackOff',
  severity: 'high',
  message: ''
})

function handleFormSubmit() {
  if (!form.value.pod_name.trim()) return
  emit('create', { ...form.value })
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="⚡ Incident Center: Simulation & Manual Incident Reporting"
    subtitle="Inject synthetic Kubernetes cluster anomalies or log an ad-hoc operational incident"
    max-width="680px"
    @update:show="emit('update:show', $event)"
  >
    <div class="create-incident-modal-body">
      <!-- Tabs header -->
      <div class="modal-tab-bar">
        <button
          type="button"
          class="modal-tab-btn"
          :class="{ 'tab-btn-active': activeTab === 'simulate' }"
          @click="activeTab = 'simulate'"
        >
          <span>⚡ Simulation Scenarios (Debug / Demo)</span>
        </button>
        <button
          type="button"
          class="modal-tab-btn"
          :class="{ 'tab-btn-active': activeTab === 'report' }"
          @click="activeTab = 'report'"
        >
          <span>📝 Log Incident Manually</span>
        </button>
      </div>


      <!-- Tab 1: Simulation Scenarios -->
      <div v-if="activeTab === 'simulate'" class="tab-content-pane animate-fade-in">
        <div class="debug-demo-banner">
          <span class="badge badge-amber font-mono">⚠️ DEBUG / DEMO MODE</span>
          <span class="debug-banner-text">Synthetic cluster failure scenarios for demonstration and autonomous diagnostics</span>
        </div>
        <p class="simulation-guide-text">
          Select a quick-start failure scenario below to trigger synthetic cluster metrics, container lifecycle events, and autonomous root-cause reasoning.
        </p>

        <div class="simulation-cards-grid">
          <div
            v-for="scenario in scenarios"
            :key="scenario.key"
            class="sim-card glass-panel"
            :class="[`sim-card-sev-${scenario.severity}`]"
            @click="emit('simulate', scenario)"
          >
            <div class="sim-card-header">
              <div class="sim-card-icon-wrap">
                <span class="sim-card-icon">{{ scenario.icon }}</span>
                <div class="sim-card-titles">
                  <h4 class="sim-card-title">{{ scenario.title }}</h4>
                  <span class="sim-card-subtitle font-mono">{{ scenario.subtitle }}</span>
                </div>
              </div>
              <StatusBadge :status="scenario.severity" size="sm" />
            </div>


            <p class="sim-card-desc">{{ scenario.description }}</p>


            <div class="sim-card-footer">
              <div class="sim-tags font-mono">
                <span class="sim-tag">{{ scenario.cluster }}</span>
                <span class="sim-tag">ns/{{ scenario.namespace }}</span>
                <span class="sim-tag badge-tag">{{ scenario.badgeText }}</span>
              </div>


              <button
                type="button"
                class="btn btn-sm btn-primary sim-action-btn"
                :disabled="actionLoading === `sim-${scenario.key}`"
                @click.stop="emit('simulate', scenario)"
              >
                <span>{{ actionLoading === `sim-${scenario.key}` ? '⏳ Injecting...' : '⚡ Inject Scenario' }}</span>
              </button>
            </div>
          </div>
        </div>
      </div>


      <!-- Tab 2: Manual Incident Form -->
      <div v-else class="tab-content-pane animate-fade-in">
        <form class="modal-form" @submit.prevent="handleFormSubmit">
          <div class="form-row">
            <div class="form-group flex-1">
              <label class="form-label">Pod / Workload Name</label>
              <input
                v-model="form.pod_name"
                type="text"
                class="input-glass font-mono"
                placeholder="e.g. order-service-7f89bc"
                required
              />
            </div>
            <div class="form-group flex-1">
              <label class="form-label">Namespace</label>
              <input
                v-model="form.namespace"
                type="text"
                class="input-glass font-mono"
                placeholder="e.g. ecommerce"
                required
              />
            </div>
          </div>


          <div class="form-row">
            <div class="form-group flex-1">
              <label class="form-label">Cluster</label>
              <input
                v-model="form.cluster_name"
                type="text"
                class="input-glass font-mono"
                placeholder="e.g. prod-us-east-1"
                required
              />
            </div>
            <div class="form-group flex-1">
              <label class="form-label">Anomaly Type</label>
              <input
                v-model="form.type"
                type="text"
                class="input-glass font-mono"
                placeholder="e.g. CrashLoopBackOff, OOMKilled"
                required
              />
            </div>
          </div>


          <div class="form-group">
            <label class="form-label">Severity Level</label>
            <select v-model="form.severity" class="input-glass">
              <option value="critical">Critical (P1 Outage)</option>
              <option value="high">High (P2 Degradation)</option>
              <option value="medium">Medium (P3 Warning)</option>
              <option value="low">Low (P4 Info)</option>
            </select>
          </div>


          <div class="form-group">
            <label class="form-label">Incident Description / Failure Log</label>
            <textarea
              v-model="form.message"
              rows="3"
              class="input-glass font-mono"
              placeholder="Describe error symptoms, failure messages, or kernel alerts..."
            ></textarea>
          </div>
        </form>
      </div>
    </div>


    <template #footer="{ close }">
      <button type="button" class="btn btn-secondary" @click="close">Cancel</button>
      <button
        v-if="activeTab === 'report'"
        type="button"
        class="btn btn-primary"
        :disabled="actionLoading === 'create-incident' || !form.pod_name.trim()"
        @click="handleFormSubmit"
      >
        <span>{{ actionLoading === 'create-incident' ? '⏳ Submitting...' : '📄 File Incident Report' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>
