<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import BaseIcon from '../ui/BaseIcon.vue'
import StatusBadge from '../ui/StatusBadge.vue'
import type { SimulationScenario, CreateIncidentPayload } from '../../composables/useIncidents'

const props = withDefaults(defineProps<{
  show: boolean
  actionLoading?: string | null
  scenarios?: SimulationScenario[]
  initialTab?: 'simulate' | 'report'
}>(), {
  actionLoading: null,
  scenarios: () => [],
  initialTab: 'report'
})

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'simulate', scenario: SimulationScenario): void
  (e: 'create', payload: CreateIncidentPayload): void
}>()

const activeTab = ref<'simulate' | 'report'>(props.initialTab || (props.scenarios && props.scenarios.length > 0 ? 'simulate' : 'report'))

watch(() => props.show, (val) => {
  if (val) {
    activeTab.value = props.initialTab || (props.scenarios && props.scenarios.length > 0 ? 'simulate' : 'report')
  }
})

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.show) {
    emit('update:show', false)
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})

const form = ref<CreateIncidentPayload>({
  pod_name: '',
  namespace: 'ecommerce',
  cluster_name: 'default',
  type: 'CrashLoopBackOff',
  severity: 'high',
  message: ''
})

function getScenarioIcon(key?: string, icon?: string): string {
  if (key === 'oom' || icon === 'flame') return 'flame'
  if (key === 'node_down' || icon === 'server') return 'server'
  if (key === 'crashloop' || icon === 'alert-triangle') return 'alert-triangle'
  return 'zap'
}

function handleFormSubmit() {
  if (!form.value.pod_name.trim()) return
  emit('create', { ...form.value })
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="Declare Operational Incident"
    subtitle="Log an active Kubernetes workload anomaly for autonomous AI RCA and remediation"
    max-width="640px"
    @update:show="emit('update:show', $event)"
  >
    <div class="create-incident-modal-body" @keydown.esc="emit('update:show', false)">
      <!-- Tabs header if scenarios provided -->
      <div v-if="scenarios && scenarios.length > 0" class="modal-tab-bar">
        <button
          type="button"
          class="modal-tab-btn"
          :class="{ 'tab-btn-active': activeTab === 'simulate' }"
          @click="activeTab = 'simulate'"
        >
          <BaseIcon name="zap" size="xs" /> <span>Simulation Scenarios (Debug / Demo)</span>
        </button>
        <button
          type="button"
          class="modal-tab-btn"
          :class="{ 'tab-btn-active': activeTab === 'report' }"
          @click="activeTab = 'report'"
        >
          <BaseIcon name="edit" size="xs" /> <span>Log Incident Manually</span>
        </button>
      </div>

      <!-- Tab 1: Simulation Scenarios -->
      <div v-if="scenarios && scenarios.length > 0 && activeTab === 'simulate'" class="tab-content-pane animate-fade-in">
        <div class="debug-demo-banner">
          <span class="badge badge-amber font-mono"><BaseIcon name="alert-triangle" size="xs" /> DEBUG / DEMO MODE</span>
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
                <span class="sim-card-icon"><BaseIcon :name="getScenarioIcon(scenario.key, scenario.icon)" size="md" /></span>
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
                <BaseIcon :name="actionLoading === `sim-${scenario.key}` ? 'clock' : 'zap'" size="xs" /> <span>{{ actionLoading === `sim-${scenario.key}` ? 'Injecting...' : 'Inject Scenario' }}</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Tab 2: Manual Incident Form -->
      <div v-else class="tab-content-pane animate-fade-in">
        <form id="create-incident-form" class="modal-form" @submit.prevent="handleFormSubmit">
          <div class="form-row">
            <div class="form-group flex-1">
              <label class="form-label" for="inc-pod-name">Pod / Workload Name</label>
              <input
                id="inc-pod-name"
                v-model="form.pod_name"
                type="text"
                class="input-glass font-mono"
                placeholder="e.g. order-service-7f89bc"
                required
              />
            </div>
            <div class="form-group flex-1">
              <label class="form-label" for="inc-namespace">Namespace</label>
              <input
                id="inc-namespace"
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
              <label class="form-label" for="inc-cluster">Cluster</label>
              <input
                id="inc-cluster"
                v-model="form.cluster_name"
                type="text"
                class="input-glass font-mono"
                placeholder="e.g. default"
                required
              />
            </div>
            <div class="form-group flex-1">
              <label class="form-label" for="inc-type">Anomaly Type</label>
              <input
                id="inc-type"
                v-model="form.type"
                type="text"
                class="input-glass font-mono"
                placeholder="e.g. CrashLoopBackOff, OOMKilled"
                required
              />
            </div>
          </div>

          <div class="form-group">
            <label class="form-label" for="inc-severity">Severity Level</label>
            <select id="inc-severity" v-model="form.severity" class="input-glass">
              <option value="critical">Critical (P1 Outage)</option>
              <option value="high">High (P2 Degradation)</option>
              <option value="medium">Medium (P3 Warning)</option>
              <option value="low">Low (P4 Info)</option>
            </select>
          </div>

          <div class="form-group">
            <label class="form-label" for="inc-message">Incident Symptoms / Failure Log</label>
            <textarea
              id="inc-message"
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
        v-if="!scenarios || scenarios.length === 0 || activeTab === 'report'"
        type="submit"
        form="create-incident-form"
        class="btn btn-primary"
        :disabled="actionLoading === 'create-incident' || !form.pod_name.trim()"
      >
        <BaseIcon :name="actionLoading === 'create-incident' ? 'clock' : 'alert-triangle'" size="xs" /> <span>{{ actionLoading === 'create-incident' ? 'Declaring...' : 'Declare Incident' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/views/incidents.css';
</style>

