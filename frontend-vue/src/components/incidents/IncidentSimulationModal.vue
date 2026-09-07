<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import ModalDrawer from '../ui/ModalDrawer.vue'

export interface SimulationScenario {
  key: string
  icon: string
  title: string
  subtitle: string
  workload: string
  namespace: string
  cluster: string
  type: string
  severity: 'critical' | 'high' | 'medium'
  description: string
  badgeText: string
}

defineProps<{
  show: boolean
  actionLoading: string | null
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'simulate', scenario: SimulationScenario): void
}>()

const simulationScenarios: SimulationScenario[] = [
  {
    key: 'oom',
    icon: '🔥',
    title: 'Pod OOMKilled (Exit Code 137)',
    subtitle: 'JVM Heap Memory Exhaustion on checkout-api',
    workload: 'checkout-api-7b9c6f8d-4x2kl',
    namespace: 'ecommerce',
    cluster: 'prod-us-east-1',
    type: 'OOMKilled',
    severity: 'critical',
    description: 'cgroup memory limit reached (512Mi). Kubernetes Linux kernel OOM killer terminated container with exit code 137.',
    badgeText: 'JVM Heap Exhaustion'
  },
  {
    key: 'node_down',
    icon: '🚨',
    title: 'Server Node Down (NodeNotReady)',
    subtitle: 'Infrastructure Host masterdb Unreachable',
    workload: 'masterdb',
    namespace: 'kube-system',
    cluster: 'prod-eu-west-1',
    type: 'NodeNotReady',
    severity: 'critical',
    description: 'Host node heartbeat lease failed. Kubelet stopped posting status (NodeStatusUnknown), causing node eviction.',
    badgeText: 'Host Node Down'
  },
  {
    key: 'crashloop',
    icon: '⚠️',
    title: 'CrashLoopBackOff',
    subtitle: 'PostgreSQL Connection Refused on payment-gateway',
    workload: 'payment-gateway-5f8d9b-w9z7x',
    namespace: 'payments',
    cluster: 'prod-us-east-1',
    type: 'CrashLoopBackOff',
    severity: 'high',
    description: 'PostgreSQL Connection Refused on payment-gateway (dial tcp 10.96.12.44:5432). Repeated container exits triggered CrashLoopBackOff.',
    badgeText: 'DB Connection Refused'
  }
]
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="⚡ Incident Simulation & Telemetry Injection (Demo Mode)"
    subtitle="Inject synthetic Kubernetes cluster anomalies to test Autonomous RCA and GitOps remediation"
    max-width="640px"
    @update:show="emit('update:show', $event)"
  >
    <div class="simulation-modal-body">
      <div class="debug-demo-banner">
        <span class="badge badge-amber font-mono">⚠️ DEMO MODE</span>
        <span class="debug-banner-text">Synthetic cluster failure scenarios for demonstration and testing</span>
      </div>
      <p class="simulation-guide-text">
        Select a quick-start failure scenario below to trigger synthetic cluster metrics, container lifecycle events, and autonomous root-cause reasoning.
      </p>

      <div class="simulation-cards-grid">
        <div
          v-for="scenario in simulationScenarios"
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
              class="btn-slate-primary sim-action-btn"
              :disabled="actionLoading === `sim-${scenario.key}`"
              @click.stop="emit('simulate', scenario)"
            >
              <span>{{ actionLoading === `sim-${scenario.key}` ? '⏳ Injecting...' : '⚡ Inject Scenario' }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <template #footer="{ close }">
      <button class="btn-slate" @click="close">Cancel</button>
    </template>
  </ModalDrawer>
</template>
