<script setup lang="ts">
import { ref, reactive, watch, computed } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import { k8sApi, type ResourceKind, type K8sResource, type K8sNamespace } from '../../api/k8s'
import { jsonToYaml } from '../../utils/yaml'

const props = withDefaults(
  defineProps<{
    show: boolean
    cluster: string
    namespaces: (K8sNamespace | string)[]
    defaultNamespace?: string
    defaultKind?: ResourceKind
  }>(),
  {
    defaultNamespace: 'default',
    defaultKind: 'deployments',
  }
)

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update:show', value: boolean): void
  (e: 'created', resource: K8sResource): void
}>()

// Kind options
const kindOptions: { label: string; value: ResourceKind; icon: string }[] = [
  { label: 'Deployment (Workload)', value: 'deployments', icon: '??' },
  { label: 'Service (Networking)', value: 'services', icon: '??' },
  { label: 'ConfigMap (Configuration)', value: 'configmaps', icon: '??' },
  { label: 'Secret (Credentials)', value: 'secrets', icon: '??' },
]

const selectedKind = ref<ResourceKind>('deployments')
const selectedNamespace = ref('default')
const showPreview = ref(false)
const submitting = ref(false)
const errorMessage = ref<string | null>(null)

// Form states
const configMapForm = reactive({
  name: '',
  entries: [{ key: 'APP_CONFIG', value: 'enabled' }],
})

const secretForm = reactive({
  name: '',
  type: 'Opaque',
  entries: [{ key: 'API_SECRET', value: '', masked: true }],
})

const deploymentForm = reactive({
  name: '',
  image: 'nginx:alpine',
  replicas: 1,
  port: 80,
  envVars: [{ key: 'PORT', value: '80' }],
})

const serviceForm = reactive({
  name: '',
  type: 'ClusterIP',
  port: 80,
  targetPort: 80,
  selectorKey: 'app',
  selectorValue: '',
})

const namespaceList = computed<string[]>(() => {
  return props.namespaces.map(ns => typeof ns === 'string' ? ns : ns.name).filter(Boolean)
})

watch(
  () => props.show,
  (isOpen) => {
    if (isOpen) {
      errorMessage.value = null
      selectedNamespace.value = props.defaultNamespace || (namespaceList.value[0] || 'default')
      if (props.defaultKind) {
        selectedKind.value = props.defaultKind
      }
    }
  },
  { immediate: true }
)

// Dynamic Key-Value row helpers
function addConfigMapRow() {
  configMapForm.entries.push({ key: '', value: '' })
}
function removeConfigMapRow(index: number) {
  configMapForm.entries.splice(index, 1)
}

function addSecretRow() {
  secretForm.entries.push({ key: '', value: '', masked: true })
}
function removeSecretRow(index: number) {
  secretForm.entries.splice(index, 1)
}

function addEnvVarRow() {
  deploymentForm.envVars.push({ key: '', value: '' })
}
function removeEnvVarRow(index: number) {
  deploymentForm.envVars.splice(index, 1)
}

// Generate the manifest payload
const manifestPayload = computed<Record<string, unknown>>(() => {
  const ns = selectedNamespace.value || 'default'

  switch (selectedKind.value) {
    case 'configmaps': {
      const dataObj: Record<string, string> = {}
      for (const e of configMapForm.entries) {
        if (e.key.trim()) {
          dataObj[e.key.trim()] = e.value
        }
      }
      return {
        apiVersion: 'v1',
        kind: 'ConfigMap',
        metadata: {
          name: configMapForm.name.trim() || 'my-configmap',
          namespace: ns,
          labels: {
            'app.kubernetes.io/managed-by': 'k8sselfhost',
          },
        },
        data: dataObj,
      }
    }

    case 'secrets': {
      const dataObj: Record<string, string> = {}
      for (const e of secretForm.entries) {
        if (e.key.trim()) {
          try {
            dataObj[e.key.trim()] = btoa(e.value)
          } catch {
            dataObj[e.key.trim()] = e.value
          }
        }
      }
      return {
        apiVersion: 'v1',
        kind: 'Secret',
        type: secretForm.type,
        metadata: {
          name: secretForm.name.trim() || 'my-secret',
          namespace: ns,
          labels: {
            'app.kubernetes.io/managed-by': 'k8sselfhost',
          },
        },
        data: dataObj,
      }
    }

    case 'deployments': {
      const appName = deploymentForm.name.trim() || 'my-deployment'
      const envList = deploymentForm.envVars
        .filter(e => e.key.trim())
        .map(e => ({ name: e.key.trim(), value: e.value }))

      return {
        apiVersion: 'apps/v1',
        kind: 'Deployment',
        metadata: {
          name: appName,
          namespace: ns,
          labels: {
            app: appName,
            'app.kubernetes.io/managed-by': 'k8sselfhost',
          },
        },
        spec: {
          replicas: deploymentForm.replicas || 1,
          selector: {
            matchLabels: {
              app: appName,
            },
          },
          template: {
            metadata: {
              labels: {
                app: appName,
              },
            },
            spec: {
              containers: [
                {
                  name: appName,
                  image: deploymentForm.image.trim() || 'nginx:alpine',
                  ports: deploymentForm.port ? [{ containerPort: Number(deploymentForm.port) }] : [],
                  env: envList,
                },
              ],
            },
          },
        },
      }
    }

    case 'services': {
      const svcName = serviceForm.name.trim() || 'my-service'
      const selectorObj: Record<string, string> = {}
      if (serviceForm.selectorKey.trim()) {
        selectorObj[serviceForm.selectorKey.trim()] = serviceForm.selectorValue.trim() || svcName
      }

      return {
        apiVersion: 'v1',
        kind: 'Service',
        metadata: {
          name: svcName,
          namespace: ns,
          labels: {
            'app.kubernetes.io/managed-by': 'k8sselfhost',
          },
        },
        spec: {
          type: serviceForm.type,
          selector: selectorObj,
          ports: [
            {
              name: 'http',
              port: Number(serviceForm.port) || 80,
              targetPort: Number(serviceForm.targetPort) || Number(serviceForm.port) || 80,
              protocol: 'TCP',
            },
          ],
        },
      }
    }

    default:
      return {
        apiVersion: 'v1',
        kind: selectedKind.value,
        metadata: { name: 'resource', namespace: ns },
      }
  }
})

const isFormValid = computed(() => {
  switch (selectedKind.value) {
    case 'configmaps':
      return Boolean(configMapForm.name.trim())
    case 'secrets':
      return Boolean(secretForm.name.trim())
    case 'deployments':
      return Boolean(deploymentForm.name.trim() && deploymentForm.image.trim())
    case 'services':
      return Boolean(serviceForm.name.trim() && serviceForm.port)
    default:
      return true
  }
})

function handleClose() {
  emit('close')
  emit('update:show', false)
}

async function handleSubmit() {
  if (!isFormValid.value) {
    errorMessage.value = 'Please fill in all required fields.'
    return
  }

  submitting.value = true
  errorMessage.value = null

  try {
    const created = await k8sApi.createResource(
      props.cluster,
      selectedKind.value,
      manifestPayload.value,
      selectedNamespace.value
    )
    emit('created', created)
    handleClose()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to create Kubernetes resource'
    errorMessage.value = msg
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="Create Kubernetes Resource"
    :subtitle="`Cluster: ${cluster} ? Namespace: ${selectedNamespace}`"
    max-width="720px"
    @close="handleClose"
  >
    <div class="create-resource-body">
      <!-- Top selectors -->
      <div class="selectors-row">
        <div class="form-group flex-1">
          <label class="form-label">Resource Kind</label>
          <select v-model="selectedKind" class="input-glass select-full">
            <option v-for="k in kindOptions" :key="k.value" :value="k.value">
              {{ k.icon }} {{ k.label }}
            </option>
          </select>
        </div>

        <div class="form-group flex-1">
          <label class="form-label">Target Namespace</label>
          <select v-model="selectedNamespace" class="input-glass select-full">
            <option v-for="ns in namespaceList" :key="ns" :value="ns">{{ ns }}</option>
            <option v-if="!namespaceList.includes(selectedNamespace) && selectedNamespace" :value="selectedNamespace">
              {{ selectedNamespace }}
            </option>
          </select>
        </div>
      </div>

      <!-- Error Notification -->
      <div v-if="errorMessage" class="error-banner animate-fade-in">
        <span class="error-icon">??</span>
        <div class="error-text">
          <strong>Creation Failed:</strong>
          <span>{{ errorMessage }}</span>
        </div>
        <button class="error-close" @click="errorMessage = null">?</button>
      </div>

      <!-- ConfigMap Form -->
      <div v-if="selectedKind === 'configmaps'" class="kind-form">
        <div class="form-group">
          <label class="form-label required">ConfigMap Name</label>
          <input 
            v-model="configMapForm.name"
            type="text" 
            class="input-glass" 
            placeholder="e.g. app-config" 
            required 
          />
        </div>

        <div class="key-value-section">
          <div class="section-title-row">
            <label class="form-label">Data Key-Value Pairs</label>
            <button type="button" class="btn btn-secondary btn-xs" @click="addConfigMapRow">
              <span>+ Add Key</span>
            </button>
          </div>

          <div v-for="(entry, idx) in configMapForm.entries" :key="idx" class="kv-row">
            <input 
              v-model="entry.key" 
              type="text" 
              class="input-glass font-mono flex-1" 
              placeholder="KEY_NAME" 
            />
            <input 
              v-model="entry.value" 
              type="text" 
              class="input-glass font-mono flex-2" 
              placeholder="Value" 
            />
            <button 
              type="button" 
              class="btn-icon-danger"
              :disabled="configMapForm.entries.length <= 1"
              @click="removeConfigMapRow(idx)"
              title="Delete row"
            >
              ?
            </button>
          </div>
        </div>
      </div>

      <!-- Secret Form -->
      <div v-else-if="selectedKind === 'secrets'" class="kind-form">
        <div class="form-row-2">
          <div class="form-group flex-2">
            <label class="form-label required">Secret Name</label>
            <input 
              v-model="secretForm.name"
              type="text" 
              class="input-glass" 
              placeholder="e.g. db-credentials" 
              required 
            />
          </div>

          <div class="form-group flex-1">
            <label class="form-label">Secret Type</label>
            <select v-model="secretForm.type" class="input-glass select-full font-mono">
              <option value="Opaque">Opaque</option>
              <option value="kubernetes.io/tls">kubernetes.io/tls</option>
              <option value="kubernetes.io/dockerconfigjson">dockerconfigjson</option>
              <option value="kubernetes.io/basic-auth">basic-auth</option>
            </select>
          </div>
        </div>

        <div class="key-value-section">
          <div class="section-title-row">
            <label class="form-label">Secret Key-Value Pairs (Base64 Encoded on Submit)</label>
            <button type="button" class="btn btn-secondary btn-xs" @click="addSecretRow">
              <span>+ Add Secret Key</span>
            </button>
          </div>

          <div v-for="(entry, idx) in secretForm.entries" :key="idx" class="kv-row">
            <input 
              v-model="entry.key" 
              type="text" 
              class="input-glass font-mono flex-1" 
              placeholder="SECRET_KEY" 
            />
            <div class="input-masked-wrapper flex-2">
              <input 
                v-model="entry.value" 
                :type="entry.masked ? 'password' : 'text'" 
                class="input-glass font-mono select-full" 
                placeholder="Secret value" 
              />
              <button 
                type="button" 
                class="btn-mask-toggle"
                @click="entry.masked = !entry.masked"
                :title="entry.masked ? 'Show' : 'Hide'"
              >
                {{ entry.masked ? '???' : '??' }}
              </button>
            </div>
            <button 
              type="button" 
              class="btn-icon-danger"
              :disabled="secretForm.entries.length <= 1"
              @click="removeSecretRow(idx)"
              title="Delete row"
            >
              ?
            </button>
          </div>
        </div>
      </div>

      <!-- Deployment Form -->
      <div v-else-if="selectedKind === 'deployments'" class="kind-form">
        <div class="form-row-2">
          <div class="form-group flex-2">
            <label class="form-label required">Deployment Name</label>
            <input 
              v-model="deploymentForm.name"
              type="text" 
              class="input-glass" 
              placeholder="e.g. web-app" 
              required 
            />
          </div>

          <div class="form-group flex-1">
            <label class="form-label required">Replicas</label>
            <input 
              v-model.number="deploymentForm.replicas"
              type="number" 
              min="1"
              max="50"
              class="input-glass" 
              required 
            />
          </div>
        </div>

        <div class="form-row-2">
          <div class="form-group flex-2">
            <label class="form-label required">Container Image</label>
            <input 
              v-model="deploymentForm.image"
              type="text" 
              class="input-glass font-mono" 
              placeholder="e.g. nginx:alpine or redis:7" 
              required 
            />
          </div>

          <div class="form-group flex-1">
            <label class="form-label">Container Port</label>
            <input 
              v-model.number="deploymentForm.port"
              type="number" 
              min="1"
              max="65535"
              class="input-glass font-mono" 
              placeholder="80" 
            />
          </div>
        </div>

        <div class="key-value-section">
          <div class="section-title-row">
            <label class="form-label">Environment Variables</label>
            <button type="button" class="btn btn-secondary btn-xs" @click="addEnvVarRow">
              <span>+ Add Env Var</span>
            </button>
          </div>

          <div v-for="(entry, idx) in deploymentForm.envVars" :key="idx" class="kv-row">
            <input 
              v-model="entry.key" 
              type="text" 
              class="input-glass font-mono flex-1" 
              placeholder="ENV_NAME" 
            />
            <input 
              v-model="entry.value" 
              type="text" 
              class="input-glass font-mono flex-2" 
              placeholder="Value" 
            />
            <button 
              type="button" 
              class="btn-icon-danger"
              @click="removeEnvVarRow(idx)"
              title="Delete env var"
            >
              ?
            </button>
          </div>
        </div>
      </div>

      <!-- Service Form -->
      <div v-else-if="selectedKind === 'services'" class="kind-form">
        <div class="form-row-2">
          <div class="form-group flex-2">
            <label class="form-label required">Service Name</label>
            <input 
              v-model="serviceForm.name"
              type="text" 
              class="input-glass" 
              placeholder="e.g. web-service" 
              required 
            />
          </div>

          <div class="form-group flex-1">
            <label class="form-label">Service Type</label>
            <select v-model="serviceForm.type" class="input-glass select-full font-mono">
              <option value="ClusterIP">ClusterIP</option>
              <option value="NodePort">NodePort</option>
              <option value="LoadBalancer">LoadBalancer</option>
            </select>
          </div>
        </div>

        <div class="form-row-2">
          <div class="form-group flex-1">
            <label class="form-label required">Port</label>
            <input 
              v-model.number="serviceForm.port"
              type="number" 
              class="input-glass font-mono" 
              placeholder="80" 
              required 
            />
          </div>

          <div class="form-group flex-1">
            <label class="form-label">Target Port</label>
            <input 
              v-model.number="serviceForm.targetPort"
              type="number" 
              class="input-glass font-mono" 
              placeholder="8080" 
            />
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Pod Selector (app label)</label>
          <div class="kv-row">
            <input 
              v-model="serviceForm.selectorKey" 
              type="text" 
              class="input-glass font-mono flex-1" 
              placeholder="Key (e.g. app)" 
            />
            <input 
              v-model="serviceForm.selectorValue" 
              type="text" 
              class="input-glass font-mono flex-2" 
              :placeholder="`Value (default: ${serviceForm.name || 'app-name'})`" 
            />
          </div>
        </div>
      </div>

      <!-- Preview Collapsible -->
      <div class="preview-accordion">
        <button 
          type="button" 
          class="preview-toggle-btn"
          @click="showPreview = !showPreview"
        >
          <span>{{ showPreview ? '? Hide Generated Manifest' : '? Show Live YAML Preview' }}</span>
        </button>

        <div v-if="showPreview" class="manifest-preview glass-panel font-mono animate-fade-in">
          <pre>{{ jsonToYaml(manifestPayload) }}</pre>
        </div>
      </div>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="submitting" @click="handleClose">
        Cancel
      </button>
      <button 
        type="button" 
        class="btn btn-primary"
        :disabled="submitting || !isFormValid"
        @click="handleSubmit"
      >
        <span>{{ submitting ? '? Creating...' : '? Create Resource' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
.create-resource-body {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.selectors-row {
  display: flex;
  gap: 14px;
}

.form-row-2 {
  display: flex;
  gap: 14px;
}

.flex-1 { flex: 1; }
.flex-2 { flex: 2; }

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.form-label.required::after {
  content: ' *';
  color: #fb7185;
}

.select-full {
  width: 100%;
}

.kind-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px;
  background: rgba(11, 15, 25, 0.4);
  border: 1px solid var(--border-subtle);
  border-radius: 12px;
}

.key-value-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.section-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.kv-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.input-masked-wrapper {
  position: relative;
  display: flex;
  align-items: center;
}

.btn-mask-toggle {
  position: absolute;
  right: 8px;
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 12px;
}

.btn-icon-danger {
  background: rgba(244, 63, 94, 0.1);
  border: 1px solid rgba(244, 63, 94, 0.2);
  border-radius: 8px;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fb7185;
  cursor: pointer;
  transition: all 0.15s ease;
  flex-shrink: 0;
}

.btn-icon-danger:hover:not(:disabled) {
  background: rgba(244, 63, 94, 0.25);
  border-color: #fb7185;
}

.btn-icon-danger:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}

.error-banner {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 14px;
  background: rgba(244, 63, 94, 0.15);
  border: 1px solid rgba(244, 63, 94, 0.35);
  border-radius: 10px;
  color: #fb7185;
  font-size: 12px;
}

.error-icon {
  font-size: 14px;
}

.error-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
}

.error-close {
  background: none;
  border: none;
  color: inherit;
  cursor: pointer;
}

.preview-accordion {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 4px;
}

.preview-toggle-btn {
  background: none;
  border: none;
  color: var(--accent-sky);
  font-size: 11px;
  font-weight: 600;
  text-align: left;
  cursor: pointer;
  padding: 4px 0;
}

.manifest-preview {
  background: #030712;
  padding: 12px;
  border-radius: 8px;
  border: 1px solid var(--border-subtle);
  max-height: 220px;
  overflow-y: auto;
  font-size: 11px;
  color: #38bdf8;
  white-space: pre-wrap;
}

.font-mono {
  font-family: var(--font-mono);
}

.btn-xs {
  padding: 3px 8px;
  font-size: 11px;
}
</style>
