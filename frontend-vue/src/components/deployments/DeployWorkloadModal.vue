<script setup lang="ts">
import { ref, watch } from 'vue'
import type { DeploymentApp, DeploymentTemplate } from '../../api/compute'
import ModalDrawer from '../ui/ModalDrawer.vue'
import DeployWorkloadIdentityStep from './DeployWorkloadIdentityStep.vue'
import DeployWorkloadStrategyStep from './DeployWorkloadStrategyStep.vue'
import DeployWorkloadNetworkStep from './DeployWorkloadNetworkStep.vue'
import DeployWorkloadResourcesStep from './DeployWorkloadResourcesStep.vue'

export interface NewAppForm extends DeploymentApp {
  servicePort: number
  cpuLimit?: string
  memLimit?: string
  envList: { key: string; value: string }[]
}

interface Props {
  show: boolean
  actionLoading?: string | null
  template?: DeploymentTemplate | null
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'create', payload: DeploymentApp): void
  (e: 'toast', msg: string, type?: 'success' | 'error' | 'info'): void
}>()

const wizardActiveTab = ref<'general' | 'strategy' | 'network' | 'resources'>('general')
const aiPrompt = ref('')
const aiGenerating = ref(false)

function createInitialForm(): NewAppForm {
  return {
    name: '', team: 'platform-engineering', env: 'production', image: '', target: 'prod-us-east-1',
    namespace: 'default', type: 'kubernetes', replicas: 3, status: 'healthy', cpu: '250m', memory: '512Mi',
    port: 80, servicePort: 80, netType: 'ClusterIP', strategy: 'RollingUpdate', canaryWeight: 10,
    canaryVersion: '', blueGreenActive: 'blue', blueVersion: '', greenVersion: '', ingressHost: '',
    tlsEnabled: false, storageClass: 'standard', storageSize: '10Gi', mountPath: '/data',
    livenessProbe: '/healthz', readinessProbe: '/ready',
    envList: [{ key: 'NODE_ENV', value: 'production' }, { key: 'LOG_LEVEL', value: 'info' }]
  }
}

const newApp = ref<NewAppForm>(createInitialForm())

watch(
  () => props.show,
  (isOpen) => {
    if (isOpen) {
      if (props.template) {
        applyTemplate(props.template)
      }
    } else {
      wizardActiveTab.value = 'general'
    }
  }
)

watch(
  () => props.template,
  (tmpl) => {
    if (tmpl) {
      applyTemplate(tmpl)
    }
  }
)

function applyTemplate(tmpl: DeploymentTemplate) {
  newApp.value.name = tmpl.name.toLowerCase().replace(/[^a-z0-9-]/g, '-')
  newApp.value.image = `${tmpl.name.split(' ')[0].toLowerCase()}:${tmpl.version}`
  newApp.value.cpu = tmpl.cpu
  newApp.value.memory = tmpl.mem
  newApp.value.port = tmpl.ports
  newApp.value.servicePort = tmpl.ports
  newApp.value.strategy = tmpl.strategy || 'RollingUpdate'
}

async function generateWithAI() {
  if (!aiPrompt.value.trim()) {
    emit('toast', 'Please type a deployment specification prompt first', 'error')
    return
  }

  aiGenerating.value = true
  try {
    const p = aiPrompt.value.toLowerCase()

    let parsedName = 'microservice-api'
    let parsedImage = 'nginx:alpine'
    let parsedReplicas = 3
    let parsedPort = 80
    let parsedStrategy = 'RollingUpdate'
    let parsedCanaryWeight = 20

    const imgMatch = aiPrompt.value.match(/(?:image|deploy|service)\s+([a-zA-Z0-9_\-\.\/]+)(?::([a-zA-Z0-9_\-\.]+))?/i)
    if (imgMatch) {
      parsedImage = imgMatch[1] + (imgMatch[2] ? `:${imgMatch[2]}` : ':latest')
      parsedName = imgMatch[1].split('/').pop()?.split(':')[0] || 'app'
    } else if (p.includes('redis')) {
      parsedName = 'redis-store'
      parsedImage = 'redis:7.2-alpine'
      parsedPort = 6379
    } else if (p.includes('postgres') || p.includes('database')) {
      parsedName = 'postgres-db'
      parsedImage = 'postgres:16-alpine'
      parsedPort = 5432
    } else if (p.includes('node') || p.includes('express')) {
      parsedName = 'node-api'
      parsedImage = 'node:20-alpine'
      parsedPort = 3000
    } else if (p.includes('python') || p.includes('fastapi')) {
      parsedName = 'fastapi-gateway'
      parsedImage = 'python:3.11-slim'
      parsedPort = 8000
    }

    const repMatch = p.match(/(\d+)\s*(?:replicas|pods|instances)/)
    if (repMatch) parsedReplicas = parseInt(repMatch[1], 10)

    const portMatch = p.match(/port\s*(\d+)/)
    if (portMatch) parsedPort = parseInt(portMatch[1], 10)

    if (p.includes('canary')) {
      parsedStrategy = 'Canary'
      const weightMatch = p.match(/(\d+)\s*%/)
      if (weightMatch) parsedCanaryWeight = parseInt(weightMatch[1], 10)
    } else if (p.includes('blue') || p.includes('green') || p.includes('bluegreen')) {
      parsedStrategy = 'BlueGreen'
    }

    newApp.value.name = parsedName
    newApp.value.image = parsedImage
    newApp.value.replicas = parsedReplicas
    newApp.value.port = parsedPort
    newApp.value.servicePort = parsedPort
    newApp.value.strategy = parsedStrategy
    newApp.value.canaryWeight = parsedCanaryWeight

    if (p.includes('ingress')) {
      newApp.value.netType = 'Ingress'
      newApp.value.ingressHost = `${parsedName}.corp.internal`
    }

    emit('toast', 'AI synthesized deployment configuration populated!', 'success')
  } catch {
    emit('toast', 'AI generation fallback applied', 'info')
  } finally {
    aiGenerating.value = false
  }
}

function submitCreate() {
  if (!newApp.value.name || !newApp.value.image) {
    emit('toast', 'Workload Name and Container Image are required', 'error')
    return
  }

  const envObj: Record<string, string> = {}
  for (const item of newApp.value.envList) {
    if (item.key.trim()) envObj[item.key.trim()] = item.value
  }

  const payload: DeploymentApp = {
    name: newApp.value.name.trim().toLowerCase(),
    team: newApp.value.team,
    env: newApp.value.env,
    image: newApp.value.image.trim(),
    target: newApp.value.target,
    namespace: newApp.value.namespace.trim() || 'default',
    type: newApp.value.type,
    replicas: newApp.value.replicas,
    status: 'healthy',
    cpu: newApp.value.cpu,
    memory: newApp.value.memory,
    port: newApp.value.port,
    netType: newApp.value.netType,
    strategy: newApp.value.strategy,
    canaryWeight: newApp.value.strategy === 'Canary' ? newApp.value.canaryWeight : 0,
    canaryVersion: newApp.value.strategy === 'Canary' ? (newApp.value.canaryVersion || newApp.value.image) : undefined,
    blueGreenActive: newApp.value.strategy === 'BlueGreen' ? 'blue' : undefined,
    blueVersion: newApp.value.strategy === 'BlueGreen' ? newApp.value.image : undefined,
    greenVersion: newApp.value.strategy === 'BlueGreen' ? (newApp.value.greenVersion || `${newApp.value.image}-next`) : undefined,
    ingressHost: newApp.value.ingressHost,
    envVars: envObj,
    revision: 1,
    availableReplicas: newApp.value.replicas,
    updatedReplicas: newApp.value.replicas,
    readyReplicas: newApp.value.replicas,
  }

  emit('create', payload)
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="Deploy Application Workload"
    subtitle="Configure target runtime, rollout strategy, container resources, and networking"
    max-width="680px"
    @update:show="emit('update:show', $event)"
  >
    <div class="wizard-modal-content">
      <!-- AI Assisted Prompt Bar -->
      <div class="ai-prompt-bar glass-panel">
        <div class="ai-prompt-input-wrap">
          <span class="ai-icon">🪄</span>
          <input
            v-model="aiPrompt"
            type="text"
            placeholder="AI Prompt: 'Deploy payment-api with 3 replicas, nginx:alpine, 20% canary, and ingress pay.corp.io'..."
            class="input-glass ai-input"
            @keydown.enter="generateWithAI"
          />
          <button
            type="button"
            class="btn btn-primary btn-xs"
            :disabled="aiGenerating || !aiPrompt.trim()"
            @click="generateWithAI"
          >
            <span>{{ aiGenerating ? 'Thinking...' : '▶ Synthesize' }}</span>
          </button>
        </div>
      </div>

      <!-- Wizard Steps Navigation -->
      <div class="wizard-nav font-mono">
        <button
          type="button"
          class="wiz-nav-btn"
          :class="{ 'wiz-nav-active': wizardActiveTab === 'general' }"
          @click="wizardActiveTab = 'general'"
        >
          1. Identity & Target
        </button>
        <button
          type="button"
          class="wiz-nav-btn"
          :class="{ 'wiz-nav-active': wizardActiveTab === 'strategy' }"
          @click="wizardActiveTab = 'strategy'"
        >
          2. Rollout Strategy
        </button>
        <button
          type="button"
          class="wiz-nav-btn"
          :class="{ 'wiz-nav-active': wizardActiveTab === 'network' }"
          @click="wizardActiveTab = 'network'"
        >
          3. Networking
        </button>
        <button
          type="button"
          class="wiz-nav-btn"
          :class="{ 'wiz-nav-active': wizardActiveTab === 'resources' }"
          @click="wizardActiveTab = 'resources'"
        >
          4. Compute & Env
        </button>
      </div>

      <!-- Tab 1: General & Identity -->
      <DeployWorkloadIdentityStep
        v-if="wizardActiveTab === 'general'"
        :form="newApp"
      />

      <!-- Tab 2: Strategy -->
      <DeployWorkloadStrategyStep
        v-else-if="wizardActiveTab === 'strategy'"
        :form="newApp"
      />

      <!-- Tab 3: Networking -->
      <DeployWorkloadNetworkStep
        v-else-if="wizardActiveTab === 'network'"
        :form="newApp"
      />

      <!-- Tab 4: Resources & Environment -->
      <DeployWorkloadResourcesStep
        v-else-if="wizardActiveTab === 'resources'"
        :form="newApp"
      />
    </div>

    <template #footer="{ close }">
      <button type="button" class="btn btn-secondary" @click="close">Cancel</button>
      <button
        type="button"
        class="btn btn-primary"
        :disabled="actionLoading === 'create'"
        @click="submitCreate"
      >
        <span>{{ actionLoading === 'create' ? 'Deploying Fleet...' : 'Deploy Workload ➔' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/components/deploy-workload-modal.css';
</style>