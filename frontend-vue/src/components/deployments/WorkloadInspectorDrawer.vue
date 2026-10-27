<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { DeploymentApp } from '../../api/compute'
import ModalDrawer from '../ui/ModalDrawer.vue'
import BaseIcon from '../ui/BaseIcon.vue'
import WorkloadLogsTab from './WorkloadLogsTab.vue'
import { formatContainerName, formatImageName } from '../../utils/dockerFormat'

interface Props {
  show: boolean
  app: DeploymentApp | null
  initialTab?: 'overview' | 'strategy' | 'network' | 'logs' | 'env' | 'yaml'
}

const props = withDefaults(defineProps<Props>(), {
  initialTab: 'overview'
})

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'openScale', app: DeploymentApp): void
  (e: 'openStrategy', app: DeploymentApp): void
  (e: 'restart', app: DeploymentApp): void
  (e: 'delete', app: DeploymentApp): void
  (e: 'toast', msg: string, type?: 'success' | 'error' | 'info'): void
}>()

const activeTab = ref<'overview' | 'strategy' | 'network' | 'logs' | 'env' | 'yaml'>('overview')

const isDaemon = computed(() => {
  return props.app?.type === 'daemon' || props.app?.type === 'host-daemon'
})

watch(
  () => props.show,
  (isOpen) => {
    if (isOpen) {
      activeTab.value = props.initialTab || 'overview'
    }
  }
)

watch(
  () => props.initialTab,
  (tab) => {
    if (tab) {
      activeTab.value = tab
    }
  }
)

import { generateYamlManifest } from './workloadInspectorYaml'

function switchTab(tab: 'overview' | 'strategy' | 'network' | 'logs' | 'env' | 'yaml') {
  activeTab.value = tab
}

function copyToClipboard(text: string) {
  if (typeof navigator !== 'undefined' && navigator.clipboard) {
    navigator.clipboard.writeText(text).then(() => {
      emit('toast', isDaemon.value ? 'Systemd unit copied to clipboard!' : 'YAML manifest copied to clipboard!', 'info')
    })
  }
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="drawer"
    :title="`App Inspector: ${formatContainerName(app?.name).serviceName || ''}`"
    :subtitle="`${app?.namespace || 'default'} · ${app?.type?.toUpperCase()} Workload`"
    max-width="680px"
    @update:show="emit('update:show', $event)"
  >
    <div v-if="app" class="inspector-content">
      <!-- Sub Tabs Nav -->
      <div class="inspector-tabs-nav font-mono">
        <button
          type="button"
          class="insp-tab"
          :class="{ 'insp-tab-active': activeTab === 'overview' }"
          @click="switchTab('overview')"
        >
          Overview & Specs
        </button>
        <button
          type="button"
          class="insp-tab"
          :class="{ 'insp-tab-active': activeTab === 'logs' }"
          @click="switchTab('logs')"
        >
          <BaseIcon name="file-text" size="xs" /> {{ isDaemon ? 'Daemon Logs' : 'Container Logs' }}
        </button>
        <button
          v-if="!isDaemon"
          type="button"
          class="insp-tab"
          :class="{ 'insp-tab-active': activeTab === 'strategy' }"
          @click="switchTab('strategy')"
        >
          Strategy & Traffic
        </button>
        <button
          type="button"
          class="insp-tab"
          :class="{ 'insp-tab-active': activeTab === 'network' }"
          @click="switchTab('network')"
        >
          Networking
        </button>
        <button
          type="button"
          class="insp-tab"
          :class="{ 'insp-tab-active': activeTab === 'yaml' }"
          @click="switchTab('yaml')"
        >
          {{ isDaemon ? 'Systemd Unit' : 'YAML Manifest' }}
        </button>
      </div>

      <!-- Tab 1: Overview & Specs -->
      <div v-if="activeTab === 'overview'" class="insp-panel animate-fade-in">
        <div class="spec-grid font-mono">
          <div class="spec-row">
            <span class="spec-label">Workload Name</span>
            <span class="spec-val text-cyan" :title="app.name">{{ formatContainerName(app.name).serviceName }}</span>
          </div>
          <div v-if="formatContainerName(app.name).isSwarmTask" class="spec-row">
            <span class="spec-label">Full Swarm Task</span>
            <span class="spec-val font-mono">{{ app.name }}</span>
          </div>
          <div class="spec-row">
            <span class="spec-label">Namespace</span>
            <span class="spec-val">{{ app.namespace || 'default' }}</span>
          </div>
          <div class="spec-row">
            <span class="spec-label">Target Cluster</span>
            <span class="spec-val">{{ app.target }}</span>
          </div>
          <div class="spec-row">
            <span class="spec-label">Runtime Engine</span>
            <span class="spec-val">{{ app.type }}</span>
          </div>
          <div class="spec-row">
            <span class="spec-label">Container Image</span>
            <span class="spec-val text-emerald" :title="app.image">{{ formatImageName(app.image).display }}</span>
          </div>
          <div class="spec-row">
            <span class="spec-label">Replicas Running</span>
            <span class="spec-val text-cyan">{{ app.readyReplicas || app.replicas }} / {{ app.replicas }} Pods</span>
          </div>
          <div class="spec-row">
            <span class="spec-label">CPU Request</span>
            <span class="spec-val">{{ app.cpu }}</span>
          </div>
          <div class="spec-row">
            <span class="spec-label">Memory Request</span>
            <span class="spec-val">{{ app.memory }}</span>
          </div>
          <div class="spec-row">
            <span class="spec-label">Service Port</span>
            <span class="spec-val">{{ app.port }} (TCP)</span>
          </div>
          <div class="spec-row">
            <span class="spec-label">Network Sync</span>
            <span class="spec-val">{{ app.netType || 'ClusterIP' }}</span>
          </div>
        </div>

        <div class="inspector-quick-actions">
          <button type="button" class="btn btn-secondary btn-sm btn-logs" @click="switchTab('logs')">
            <span><BaseIcon name="file-text" size="xs" /> Inspect Logs</span>
          </button>
          <button v-if="!isDaemon" type="button" class="btn btn-secondary btn-sm" @click="emit('openScale', app)">
            <span><BaseIcon name="zap" size="xs" /> Scale Replicas</span>
          </button>
          <button type="button" class="btn btn-secondary btn-sm" @click="emit('restart', app)">
            <span><BaseIcon name="refresh" size="xs" /> {{ isDaemon ? 'Restart Service' : 'Rolling Restart' }}</span>
          </button>
          <button v-if="!isDaemon" type="button" class="btn btn-secondary btn-sm btn-remove" @click="emit('delete', app)">
            <span><BaseIcon name="trash" size="xs" /> Delete Workload</span>
          </button>
          <span v-else class="text-xs text-muted font-mono flex items-center gap-1" title="Managed via Linux systemd on host">
            <BaseIcon name="info" size="xs" /> Managed via Linux systemd on host
          </span>
        </div>
      </div>

      <!-- Tab 2: Container Logs -->
      <WorkloadLogsTab
        v-else-if="activeTab === 'logs'"
        :app="app"
        :active="activeTab === 'logs'"
        @toast="(msg, type) => emit('toast', msg, type)"
      />

      <!-- Tab 3: Strategy & Traffic -->
      <div v-else-if="activeTab === 'strategy'" class="insp-panel animate-fade-in">
        <div v-if="isDaemon" class="glass-panel p-4 mb-4 font-mono">
          <h4 class="strat-title mb-2">Host Daemon Unit (systemd)</h4>
          <p class="strat-desc">Managed directly as a Linux systemd service on host node. Dynamic traffic splitting (Canary / Blue-Green) is not applicable to standalone daemon workloads.</p>
        </div>
        <div v-else>
          <div class="glass-panel p-4 mb-4">
            <h4 class="strat-title mb-2">Active Strategy: {{ app.strategy || 'RollingUpdate' }}</h4>
            <p class="strat-desc">Revision #{{ app.revision || 1 }} deployed on cluster {{ app.target }}.</p>
          </div>
          <button type="button" class="btn btn-primary btn-sm w-full" @click="emit('openStrategy', app)">
            <span>Configure Canary & Blue-Green Traffic Controls <BaseIcon name="play" size="xs" /></span>
          </button>
        </div>
      </div>

      <!-- Tab 4: Networking -->
      <div v-else-if="activeTab === 'network'" class="insp-panel animate-fade-in">
        <div class="spec-grid font-mono">
          <div class="spec-row">
            <span class="spec-label">Service Type</span>
            <span class="spec-val">{{ isDaemon ? 'Host Daemon Socket' : (app.netType || 'ClusterIP') }}</span>
          </div>
          <div class="spec-row">
            <span class="spec-label">Internal Endpoint</span>
            <span class="spec-val text-cyan">{{ isDaemon ? `${app.namespace}:${app.port || 9100}` : `${app.name}.${app.namespace || 'default'}.svc.cluster.local:${app.port}` }}</span>
          </div>
          <div class="spec-row">
            <span class="spec-label">Ingress Hostname</span>
            <span class="spec-val text-emerald">{{ app.ingressHost || 'Not exposed via Ingress' }}</span>
          </div>
          <div class="spec-row">
            <span class="spec-label">Readiness Probe</span>
            <span class="spec-val">{{ isDaemon ? 'systemd is-active check' : (app.readinessProbe || '/healthz') }}</span>
          </div>
        </div>
      </div>

      <!-- Tab 5: YAML Manifest / Systemd Unit -->
      <div v-else-if="activeTab === 'yaml'" class="insp-panel animate-fade-in">
        <div class="yaml-actions-bar">
          <span class="yaml-title font-mono">{{ isDaemon ? 'Systemd Unit Spec' : 'Live Kubernetes Spec' }}</span>
          <button type="button" class="btn btn-secondary btn-xs" @click="copyToClipboard(generateYamlManifest(app))">
            <span><BaseIcon name="file-text" size="xs" /> {{ isDaemon ? 'Copy Unit' : 'Copy Manifest' }}</span>
          </button>
        </div>
        <pre class="yaml-viewer font-mono">{{ generateYamlManifest(app) }}</pre>
      </div>
    </div>

    <template #footer="{ close }">
      <button type="button" class="btn btn-secondary" @click="close">Close Inspector</button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/components/workload-inspector.css';
</style>