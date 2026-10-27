<script setup lang="ts">
import { ref, computed } from 'vue'
import type { DeploymentApp } from '../../api/compute'
import { computeReplicaCells, groupDeploymentsByNamespace, type NamespaceGroup } from '../../composables/useDeployments'
import BaseIcon from '../ui/BaseIcon.vue'
import ActionDropdown, { type ActionItem } from '../ui/ActionDropdown.vue'
import { formatContainerName, formatImageName } from '../../utils/dockerFormat'
import '../../assets/styles/views/deployments-topology.css'

interface Props {
  namespaceGroups?: NamespaceGroup[]
  workloads?: DeploymentApp[]
  loading?: boolean
  error?: string | null
  actionLoading?: string | null
  totalWorkloads?: number
  totalPodReplicas?: number
  readyReplicas?: number
  healthyCount?: number
  attentionCount?: number
}

const props = withDefaults(defineProps<Props>(), {
  namespaceGroups: () => [],
  workloads: undefined,
  loading: false,
  error: null,
  actionLoading: null,
  totalWorkloads: 0,
  totalPodReplicas: 0,
  readyReplicas: 0,
  healthyCount: 0,
  attentionCount: 0,
})

const emit = defineEmits<{
  (e: 'inspect', app: DeploymentApp): void
  (e: 'logs', app: DeploymentApp): void
  (e: 'scale', app: DeploymentApp): void
  (e: 'strategy', app: DeploymentApp): void
  (e: 'restart', app: DeploymentApp): void
  (e: 'delete', app: DeploymentApp): void
}>()

const collapsedNamespaces = ref<Record<string, boolean>>({})

const effectiveGroups = computed<NamespaceGroup[]>(() => {
  if (props.workloads !== undefined) {
    return groupDeploymentsByNamespace(props.workloads)
  }
  return props.namespaceGroups
})

function toggleNamespace(ns: string) {
  collapsedNamespaces.value[ns] = !collapsedNamespaces.value[ns]
}

function isDaemon(app: DeploymentApp): boolean {
  return app.type === 'daemon' || app.type === 'host-daemon'
}

function getCardActions(app: DeploymentApp): ActionItem[] {
  if (isDaemon(app)) {
    return [
      { id: 'inspect', label: 'Inspect Details', icon: 'search' },
      {
        id: 'restart',
        label: props.actionLoading === app.name ? 'Restarting...' : 'Restart Service',
        icon: 'refresh',
        disabled: props.actionLoading === app.name
      },
    ]
  }
  return [
    { id: 'inspect', label: 'Inspect Details', icon: 'search' },
    { id: 'strategy', label: 'Rollout Strategy', icon: 'git-branch' },
    {
      id: 'restart',
      label: props.actionLoading === app.name ? 'Restarting...' : 'Restart Pods',
      icon: 'refresh',
      disabled: props.actionLoading === app.name
    },
    { id: 'sep', label: '', separator: true },
    {
      id: 'delete',
      label: 'Delete Workload',
      icon: 'trash',
      variant: 'danger',
      disabled: props.actionLoading === app.name
    },
  ]
}

function handleCardAction(actionId: string, app: DeploymentApp) {
  if (actionId === 'inspect') emit('inspect', app)
  else if (actionId === 'strategy') emit('strategy', app)
  else if (actionId === 'restart') emit('restart', app)
  else if (actionId === 'delete') emit('delete', app)
}

function getAppStatusClass(app: DeploymentApp): string {
  if (app.replicas === 0) return 'status-scaled-zero'
  if (app.status === 'healthy') return 'status-healthy'
  return 'status-degraded'
}

function getAppStatusLabel(app: DeploymentApp): string {
  if (app.replicas === 0) return 'SCALED 0'
  if (app.status === 'healthy') return 'HEALTHY'
  if (app.status === 'degraded') return 'DEGRADED'
  return (app.status || 'UNKNOWN').toUpperCase()
}
</script>

<template>
  <div class="topology-wrapper animate-fade-in">
    <!-- Workload Fleet Posture Strip -->
    <div class="topology-posture-strip">
      <div class="posture-metric-group">
        <div class="posture-item">
          <span class="posture-label">Namespaces Active</span>
          <span class="posture-value font-mono">{{ effectiveGroups.length }}</span>
        </div>
        <div class="posture-item">
          <span class="posture-label">Total Workloads</span>
          <span class="posture-value font-mono">{{ totalWorkloads || (workloads ? workloads.length : 0) }}</span>
        </div>
        <div class="posture-item">
          <span class="posture-label">Running Pod Replicas</span>
          <span class="posture-value font-mono">{{ readyReplicas }} / {{ totalPodReplicas }}</span>
        </div>
      </div>

      <div class="posture-status-pills font-mono">
        <span class="posture-pill posture-pill-healthy" title="Healthy Workloads">
          <BaseIcon name="check-circle" size="xs" />
          {{ healthyCount }} Healthy
        </span>
        <span v-if="attentionCount > 0" class="posture-pill posture-pill-attention" title="Workloads needing attention">
          <BaseIcon name="alert-triangle" size="xs" />
          {{ attentionCount }} Attention
        </span>
      </div>
    </div>

    <!-- Empty State -->
    <div v-if="effectiveGroups.length === 0" class="topology-empty-state">
      <BaseIcon name="box" size="lg" />
      <div class="empty-state-title">0 Workloads Found</div>
      <div class="empty-state-desc">No active workloads match the selected namespace, kind, or search query.</div>
    </div>

    <!-- Namespace Topology Lanes -->
    <div v-else class="topology-lanes-container">
      <section
        v-for="group in effectiveGroups"
        :key="group.namespace"
        class="namespace-lane"
        :aria-label="`Namespace ${group.namespace}`"
      >
        <!-- Collapsible Namespace Header Bar -->
        <div
          class="namespace-lane-header"
          role="button"
          tabindex="0"
          :aria-expanded="!collapsedNamespaces[group.namespace]"
          @click="toggleNamespace(group.namespace)"
          @keydown.enter="toggleNamespace(group.namespace)"
          @keydown.space.prevent="toggleNamespace(group.namespace)"
        >
          <div class="lane-header-left">
            <span class="namespace-badge font-mono">
              <BaseIcon name="box" size="xs" />
              <span class="ns-prefix">ns /</span>
              <span class="ns-name">{{ group.namespace }}</span>
            </span>
            <span class="cluster-tag font-mono">cluster: {{ group.cluster }}</span>
            <span class="workload-count-badge font-mono">({{ group.workloads.length }} workloads)</span>
          </div>

          <div class="lane-header-right">
            <span class="lane-replica-summary font-mono">{{ group.totalReplicas }} pods ready</span>
            <span
              class="lane-health-dot"
              :class="group.overallHealth === 'healthy' ? 'dot-healthy' : group.overallHealth === 'down' ? 'dot-down' : 'dot-degraded'"
              :title="`Health: ${group.overallHealth}`"
            />
            <BaseIcon
              name="chevron-down"
              size="xs"
              class="chevron-icon"
              :class="{ 'is-collapsed': collapsedNamespaces[group.namespace] }"
            />
          </div>
        </div>

        <!-- Workload Topology Cards Grid -->
        <div v-if="!collapsedNamespaces[group.namespace]" class="namespace-cards-grid">
          <div
            v-for="app in group.workloads"
            :key="app.name"
            class="workload-topology-card"
          >
            <!-- Card Header -->
            <div class="card-top-row">
              <div class="workload-name-wrap">
                <span
                  class="workload-title font-mono"
                  :title="app.name"
                  @click="emit('inspect', app)"
                >
                  {{ formatContainerName(app.name).serviceName }}
                </span>
              </div>

              <div class="card-pills-wrap font-mono">
                <span class="pill-runtime">{{ app.strategy || app.type || 'Rolling' }}</span>
                <span v-if="app.port > 0" class="pill-port">:{{ app.port }} / {{ app.netType || 'ingress' }}</span>
                <span class="pill-status" :class="getAppStatusClass(app)">{{ getAppStatusLabel(app) }}</span>
              </div>
            </div>

            <!-- Container Image Spec -->
            <div class="card-image-row font-mono" :title="app.image">
              <BaseIcon name="layers" size="xs" class="image-icon" />
              <span class="image-text">{{ formatImageName(app.image).display }}</span>
            </div>

            <!-- Visual Pod Replica Matrix (ArgoCD / K9s style) -->
            <div class="replica-matrix-row">
              <div class="replica-matrix" :title="computeReplicaCells(app).label">
                <span
                  v-for="b in computeReplicaCells(app).blocks"
                  :key="b.index"
                  class="replica-block"
                  :class="['state-' + b.state, { 'state-scaled-0': b.isZero }]"
                />
                <span v-if="computeReplicaCells(app).overflowCount > 0" class="replica-overflow-badge font-mono">
                  +{{ computeReplicaCells(app).overflowCount }}
                </span>
              </div>
              <span class="replica-label font-mono">{{ computeReplicaCells(app).label }}</span>
            </div>

            <!-- Resource Footprint & Clean Actions Footer -->
            <div class="card-footer-row">
              <div class="resource-badges font-mono">
                <span v-if="app.cpu" class="res-badge" :title="`CPU Request/Limit: ${app.cpu}`">
                  CPU: {{ app.cpu }}{{ (app as any).cpu_limit || app.cpuLimit ? ' / ' + ((app as any).cpu_limit || app.cpuLimit) : '' }}
                </span>
                <span v-if="app.memory" class="res-badge" :title="`Memory Request/Limit: ${app.memory}`">
                  MEM: {{ app.memory }}{{ (app as any).memory_limit || app.memoryLimit ? ' / ' + ((app as any).memory_limit || app.memoryLimit) : '' }}
                </span>
              </div>

              <div class="card-actions-group">
                <button
                  type="button"
                  class="btn btn-secondary btn-xs"
                  title="Inspect Container Logs"
                  @click="emit('logs', app)"
                >
                  <BaseIcon name="file-text" size="xs" />
                  <span>Logs</span>
                </button>
                <button
                  v-if="!isDaemon(app)"
                  type="button"
                  class="btn btn-secondary btn-xs"
                  title="Scale Replicas"
                  @click="emit('scale', app)"
                >
                  <BaseIcon name="zap" size="xs" />
                  <span>Scale</span>
                </button>
                <span
                  v-else
                  class="badge-systemd-indicator font-mono text-muted text-xs"
                  title="Managed via Linux systemd on host"
                >
                  systemd
                </span>
                <ActionDropdown
                  :items="getCardActions(app)"
                  size="xs"
                  trigger-title="Workload Operations"
                  @select="handleCardAction($event, app)"
                />
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>
