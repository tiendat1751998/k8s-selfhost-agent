<script setup lang="ts">
import { ref, watch } from 'vue'
import type { DeploymentApp } from '../../api/compute'
import ModalDrawer from '../ui/ModalDrawer.vue'
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

function switchTab(tab: 'overview' | 'strategy' | 'network' | 'logs' | 'env' | 'yaml') {
  activeTab.value = tab
}

function generateYamlManifest(app: DeploymentApp): string {
  if (app.type === 'swarm' || app.type === 'docker') {
    return `version: "3.8"
services:
  ${app.name}:
    image: ${app.image}
    deploy:
      replicas: ${app.replicas}
      resources:
        limits:
          cpus: "${app.cpu}"
          memory: ${app.memory}
    ports:
      - "${app.port}:${app.port}"
    environment:
      - APP_ENV=${app.env}`
  }

  const isCanary = app.strategy === 'Canary'
  const isBlueGreen = app.strategy === 'BlueGreen'

  return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${app.name}
  namespace: ${app.namespace || 'default'}
  labels:
    app.kubernetes.io/name: ${app.name}
    app.kubernetes.io/team: ${app.team || 'platform'}
    app.kubernetes.io/env: ${app.env || 'production'}
    app.kubernetes.io/strategy: ${app.strategy || 'RollingUpdate'}
spec:
  replicas: ${app.replicas}
  strategy:
    type: ${app.strategy === 'Recreate' ? 'Recreate' : 'RollingUpdate'}
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 0
  selector:
    matchLabels:
      app: ${app.name}
  template:
    metadata:
      labels:
        app: ${app.name}
        track: ${isCanary ? 'stable' : isBlueGreen ? (app.blueGreenActive || 'blue') : 'main'}
    spec:
      containers:
      - name: ${app.name}
        image: ${app.image}
        ports:
        - containerPort: ${app.port}
          name: http
        resources:
          requests:
            cpu: "${app.cpu || '250m'}"
            memory: "${app.memory || '512Mi'}"
          limits:
            cpu: "${app.cpu || '500m'}"
            memory: "${app.memory || '1Gi'}"
        readinessProbe:
          httpGet:
            path: ${app.readinessProbe || '/healthz'}
            port: ${app.port}
          initialDelaySeconds: 5
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: ${app.name}
  namespace: ${app.namespace || 'default'}
spec:
  type: ${app.netType || 'ClusterIP'}
  selector:
    app: ${app.name}
  ports:
  - port: ${app.port}
    targetPort: ${app.port}
    name: http`
}

function copyToClipboard(text: string) {
  if (typeof navigator !== 'undefined' && navigator.clipboard) {
    navigator.clipboard.writeText(text).then(() => {
      emit('toast', 'YAML manifest copied to clipboard!', 'info')
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
          📜 Container Logs
        </button>
        <button
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
          YAML Manifest
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
            <span>📜 Inspect Logs</span>
          </button>
          <button type="button" class="btn btn-secondary btn-sm" @click="emit('openScale', app)">
            <span>⚡ Scale Replicas</span>
          </button>
          <button type="button" class="btn btn-secondary btn-sm" @click="emit('restart', app)">
            <span>🔄 Rolling Restart</span>
          </button>
          <button type="button" class="btn btn-secondary btn-sm btn-remove" @click="emit('delete', app)">
            <span>🗑️ Delete Workload</span>
          </button>
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
        <div class="glass-panel p-4 mb-4">
          <h4 class="strat-title mb-2">Active Strategy: {{ app.strategy || 'RollingUpdate' }}</h4>
          <p class="strat-desc">Revision #{{ app.revision || 1 }} deployed on cluster {{ app.target }}.</p>
        </div>
        <button type="button" class="btn btn-primary btn-sm w-full" @click="emit('openStrategy', app)">
          <span>Configure Canary & Blue-Green Traffic Controls ➔</span>
        </button>
      </div>

      <!-- Tab 4: Networking -->
      <div v-else-if="activeTab === 'network'" class="insp-panel animate-fade-in">
        <div class="spec-grid font-mono">
          <div class="spec-row">
            <span class="spec-label">Service Type</span>
            <span class="spec-val">{{ app.netType || 'ClusterIP' }}</span>
          </div>
          <div class="spec-row">
            <span class="spec-label">Internal Endpoint</span>
            <span class="spec-val text-cyan">{{ app.name }}.{{ app.namespace || 'default' }}.svc.cluster.local:{{ app.port }}</span>
          </div>
          <div class="spec-row">
            <span class="spec-label">Ingress Hostname</span>
            <span class="spec-val text-emerald">{{ app.ingressHost || 'Not exposed via Ingress' }}</span>
          </div>
          <div class="spec-row">
            <span class="spec-label">Readiness Probe</span>
            <span class="spec-val">{{ app.readinessProbe || '/healthz' }}</span>
          </div>
        </div>
      </div>

      <!-- Tab 5: YAML Manifest -->
      <div v-else-if="activeTab === 'yaml'" class="insp-panel animate-fade-in">
        <div class="yaml-actions-bar">
          <span class="yaml-title font-mono">Live Kubernetes Spec</span>
          <button type="button" class="btn btn-secondary btn-xs" @click="copyToClipboard(generateYamlManifest(app))">
            <span>📋 Copy Manifest</span>
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