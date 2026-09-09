<script setup lang="ts">
import { ref } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { SwarmStackDeployPayload } from '../../composables/useDockerSwarm'

const props = defineProps<{
  show: boolean
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'deploy', payload: SwarmStackDeployPayload): void
  (e: 'close'): void
}>()

const stackName = ref('')
const composeYaml = ref(`version: '3.8'
services:
  web:
    image: nginx:alpine
    ports:
      - "80:80"
    deploy:
      replicas: 3
      restart_policy:
        condition: on-failure
      update_config:
        parallelism: 1
        delay: 10s
`)
const envVars = ref('NODE_ENV=production\nPORT=80')
const pruneDeadServices = ref(true)

const templates: Record<string, string> = {
  'Nginx Mesh': `version: '3.8'
services:
  web:
    image: nginx:alpine
    ports:
      - "80:80"
    deploy:
      replicas: 3
`,
  'Redis Cluster': `version: '3.8'
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    deploy:
      replicas: 2
`,
  'Observability Stack': `version: '3.8'
services:
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    deploy:
      replicas: 1
`,
}

function loadTemplate(name: string) {
  if (templates[name]) {
    composeYaml.value = templates[name]
    if (!stackName.value) {
      stackName.value = name.toLowerCase().replace(/\s+/g, '-')
    }
  }
}

function handleDeploy() {
  if (!stackName.value.trim()) return

  const envObj: Record<string, string> = {}
  envVars.value.split('\n').forEach(line => {
    const [k, ...v] = line.split('=')
    if (k && v.length) {
      envObj[k.trim()] = v.join('=').trim()
    }
  })

  emit('deploy', {
    name: stackName.value.trim(),
    compose_yaml: composeYaml.value,
    env_vars: envObj,
    prune: pruneDeadServices.value,
  })
}
</script>

<template>
  <ModalDrawer
    :show="props.show"
    mode="modal"
    title="Deploy Swarm Stack"
    subtitle="Deploy multi-service Docker Compose stacks into overlay mesh network"
    max-width="640px"
    @update:show="emit('update:show', $event)"
    @close="emit('close')"
  >
    <div class="deploy-modal-body">
      <div class="form-group">
        <label class="form-label" for="stack-name">STACK NAME</label>
        <input
          id="stack-name"
          v-model="stackName"
          type="text"
          class="form-input font-mono"
          placeholder="e.g. production-observability"
          required
        />
      </div>

      <div class="form-group">
        <div style="display: flex; justify-content: space-between; align-items: center;">
          <label class="form-label" for="compose-yaml">DOCKER COMPOSE SPECIFICATION (YAML)</label>
          <div class="template-quick-buttons">
            <span
              v-for="tplName in Object.keys(templates)"
              :key="tplName"
              class="template-chip"
              @click="loadTemplate(tplName)"
            >
              + {{ tplName }}
            </span>
          </div>
        </div>
        <textarea
          id="compose-yaml"
          v-model="composeYaml"
          class="form-textarea"
          spellcheck="false"
        ></textarea>
      </div>

      <div class="form-group">
        <label class="form-label" for="env-vars">ENVIRONMENT VARIABLES (KEY=VALUE)</label>
        <textarea
          id="env-vars"
          v-model="envVars"
          class="form-input font-mono"
          style="min-height: 70px; resize: vertical;"
          placeholder="KEY=VALUE"
        ></textarea>
      </div>

      <div style="display: flex; align-items: center; gap: 8px; font-size: 12px; color: var(--text-secondary);">
        <input id="prune-toggle" v-model="pruneDeadServices" type="checkbox" />
        <label for="prune-toggle" style="cursor: pointer;">Prune obsolete services no longer referenced in stack spec</label>
      </div>
    </div>

    <template #footer="{ close }">
      <div class="drawer-footer-actions">
        <button class="btn btn-secondary" @click="close">Cancel</button>
        <button
          class="btn btn-primary"
          :disabled="props.loading || !stackName.trim()"
          @click="handleDeploy"
        >
          <span>{{ props.loading ? '⏳ Deploying Stack...' : '🚀 Deploy Stack' }}</span>
        </button>
      </div>
    </template>
  </ModalDrawer>
</template>
