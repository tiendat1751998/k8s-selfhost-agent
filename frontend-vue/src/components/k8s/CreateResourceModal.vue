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
  { label: 'Deployment (Workload)', value: 'deployments', icon: '🚀' },
  { label: 'Service (Networking)', value: 'services', icon: '🔌' },
  { label: 'Ingress (Routing)', value: 'ingresses', icon: '🌐' },
  { label: 'ConfigMap (Configuration)', value: 'configmaps', icon: '🗺️' },
  { label: 'Secret (Credentials)', value: 'secrets', icon: '🔒' },
  { label: 'PersistentVolumeClaim (Storage)', value: 'persistentvolumeclaims', icon: '💾' },
  { label: 'Job (Batch)', value: 'jobs', icon: '⚡' },
  { label: 'CronJob (Scheduled)', value: 'cronjobs', icon: '⏰' },
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

const ingressForm = reactive({
  name: '',
  host: 'example.local',
  path: '/',
  pathType: 'Prefix',
  serviceName: '',
  servicePort: 80,
  tlsSecret: '',
})

const pvcForm = reactive({
  name: '',
  storageClassName: '',
  accessMode: 'ReadWriteOnce',
  capacity: '10Gi',
})

const jobForm = reactive({
  name: '',
  image: 'busybox:latest',
  command: 'echo "Job completed successfully"',
  backoffLimit: 4,
  restartPolicy: 'OnFailure',
})

const cronJobForm = reactive({
  name: '',
  schedule: '*/5 * * * *',
  image: 'curlimages/curl:latest',
  command: 'curl -s https://httpbin.org/get',
  concurrencyPolicy: 'Allow',
  suspend: false,
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

function addConfigMapRow() { configMapForm.entries.push({ key: '', value: '' }) }
function removeConfigMapRow(index: number) { configMapForm.entries.splice(index, 1) }

function addSecretRow() { secretForm.entries.push({ key: '', value: '', masked: true }) }
function removeSecretRow(index: number) { secretForm.entries.splice(index, 1) }

function addEnvVarRow() { deploymentForm.envVars.push({ key: '', value: '' }) }
function removeEnvVarRow(index: number) { deploymentForm.envVars.splice(index, 1) }

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
          labels: { 'app.kubernetes.io/managed-by': 'k8sselfhost' },
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
          labels: { 'app.kubernetes.io/managed-by': 'k8sselfhost' },
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
          labels: { app: appName, 'app.kubernetes.io/managed-by': 'k8sselfhost' },
        },
        spec: {
          replicas: deploymentForm.replicas || 1,
          selector: { matchLabels: { app: appName } },
          template: {
            metadata: { labels: { app: appName } },
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
          labels: { 'app.kubernetes.io/managed-by': 'k8sselfhost' },
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

    case 'ingresses': {
      const ingName = ingressForm.name.trim() || 'my-ingress'
      const targetSvc = ingressForm.serviceName.trim() || (ingName + '-service')
      const specObj: Record<string, unknown> = {
        rules: [
          {
            host: ingressForm.host.trim() || undefined,
            http: {
              paths: [
                {
                  path: ingressForm.path.trim() || '/',
                  pathType: ingressForm.pathType || 'Prefix',
                  backend: {
                    service: {
                      name: targetSvc,
                      port: { number: Number(ingressForm.servicePort) || 80 },
                    },
                  },
                },
              ],
            },
          },
        ],
      }
      if (ingressForm.tlsSecret.trim()) {
        specObj.tls = [
          {
            hosts: ingressForm.host.trim() ? [ingressForm.host.trim()] : [],
            secretName: ingressForm.tlsSecret.trim(),
          },
        ]
      }

      return {
        apiVersion: 'networking.k8s.io/v1',
        kind: 'Ingress',
        metadata: {
          name: ingName,
          namespace: ns,
          labels: { 'app.kubernetes.io/managed-by': 'k8sselfhost' },
        },
        spec: specObj,
      }
    }

    case 'persistentvolumeclaims': {
      const pvcName = pvcForm.name.trim() || 'my-pvc'
      const pvcSpec: Record<string, unknown> = {
        accessModes: [pvcForm.accessMode || 'ReadWriteOnce'],
        resources: {
          requests: { storage: pvcForm.capacity.trim() || '10Gi' },
        },
      }
      if (pvcForm.storageClassName.trim()) {
        pvcSpec.storageClassName = pvcForm.storageClassName.trim()
      }

      return {
        apiVersion: 'v1',
        kind: 'PersistentVolumeClaim',
        metadata: {
          name: pvcName,
          namespace: ns,
          labels: { 'app.kubernetes.io/managed-by': 'k8sselfhost' },
        },
        spec: pvcSpec,
      }
    }

    case 'jobs': {
      const jobName = jobForm.name.trim() || 'my-job'
      const cmdParts = jobForm.command.trim() ? ['/bin/sh', '-c', jobForm.command.trim()] : undefined

      return {
        apiVersion: 'batch/v1',
        kind: 'Job',
        metadata: {
          name: jobName,
          namespace: ns,
          labels: { 'app.kubernetes.io/managed-by': 'k8sselfhost' },
        },
        spec: {
          backoffLimit: Number(jobForm.backoffLimit) || 4,
          template: {
            spec: {
              restartPolicy: jobForm.restartPolicy || 'OnFailure',
              containers: [
                {
                  name: jobName,
                  image: jobForm.image.trim() || 'busybox:latest',
                  command: cmdParts,
                },
              ],
            },
          },
        },
      }
    }

    case 'cronjobs': {
      const cjName = cronJobForm.name.trim() || 'my-cronjob'
      const cmdParts = cronJobForm.command.trim() ? ['/bin/sh', '-c', cronJobForm.command.trim()] : undefined

      return {
        apiVersion: 'batch/v1',
        kind: 'CronJob',
        metadata: {
          name: cjName,
          namespace: ns,
          labels: { 'app.kubernetes.io/managed-by': 'k8sselfhost' },
        },
        spec: {
          schedule: cronJobForm.schedule.trim() || '*/5 * * * *',
          concurrencyPolicy: cronJobForm.concurrencyPolicy || 'Allow',
          suspend: cronJobForm.suspend,
          jobTemplate: {
            spec: {
              template: {
                spec: {
                  restartPolicy: 'OnFailure',
                  containers: [
                    {
                      name: cjName,
                      image: cronJobForm.image.trim() || 'curlimages/curl:latest',
                      command: cmdParts,
                    },
                  ],
                },
              },
            },
          },
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
    case 'ingresses':
      return Boolean(ingressForm.name.trim() && ingressForm.serviceName.trim())
    case 'persistentvolumeclaims':
      return Boolean(pvcForm.name.trim() && pvcForm.capacity.trim())
    case 'jobs':
      return Boolean(jobForm.name.trim() && jobForm.image.trim())
    case 'cronjobs':
      return Boolean(cronJobForm.name.trim() && cronJobForm.schedule.trim() && cronJobForm.image.trim())
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
    :subtitle="'Cluster: ' + cluster + ' • Namespace: ' + selectedNamespace"
    max-width="720px"
    @close="handleClose"
  >
    <div class="create-resource-body">
      <!-- Top selectors -->
      <div class="selectors-row">
        <div class="form-group flex-1">
          <label class="form-label">Resource Kind</label>
          <select v-model="selectedKind" class="input-glass select-full font-mono">
            <option v-for="k in kindOptions" :key="k.value" :value="k.value">
              {{ k.icon }} {{ k.label }}
            </option>
          </select>
        </div>

        <div class="form-group flex-1">
          <label class="form-label">Target Namespace</label>
          <select v-model="selectedNamespace" class="input-glass select-full font-mono">
            <option v-for="ns in namespaceList" :key="ns" :value="ns">{{ ns }}</option>
            <option v-if="!namespaceList.includes(selectedNamespace) && selectedNamespace" :value="selectedNamespace">
              {{ selectedNamespace }}
            </option>
          </select>
        </div>
      </div>

      <!-- Error Notification -->
      <div v-if="errorMessage" class="error-banner animate-fade-in">
        <span class="error-icon">⚠️</span>
        <div class="error-text">
          <strong>Creation Failed:</strong>
          <span>{{ errorMessage }}</span>
        </div>
        <button class="error-close" @click="errorMessage = null">✕</button>
      </div>

      <!-- ConfigMap Form -->
      <div v-if="selectedKind === 'configmaps'" class="kind-form">
        <div class="form-group">
          <label class="form-label required">ConfigMap Name</label>
          <input 
            v-model="configMapForm.name"
            type="text" 
            class="input-glass font-mono" 
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
              ✕
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
              class="input-glass font-mono" 
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
                {{ entry.masked ? '👁️' : '🔒' }}
              </button>
            </div>
            <button 
              type="button" 
              class="btn-icon-danger"
              :disabled="secretForm.entries.length <= 1"
              @click="removeSecretRow(idx)"
              title="Delete row"
            >
              ✕
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
              class="input-glass font-mono" 
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
              class="input-glass font-mono" 
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
              ✕
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
              class="input-glass font-mono" 
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
              :placeholder="'Value (default: ' + (serviceForm.name || 'app-name') + ')'" 
            />
          </div>
        </div>
      </div>

      <!-- Ingress Form -->
      <div v-else-if="selectedKind === 'ingresses'" class="kind-form">
        <div class="form-row-2">
          <div class="form-group flex-1">
            <label class="form-label required">Ingress Name</label>
            <input 
              v-model="ingressForm.name"
              type="text" 
              class="input-glass font-mono" 
              placeholder="e.g. app-ingress" 
              required 
            />
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Host Domain</label>
            <input 
              v-model="ingressForm.host"
              type="text" 
              class="input-glass font-mono" 
              placeholder="e.g. app.example.com" 
            />
          </div>
        </div>

        <div class="form-row-2">
          <div class="form-group flex-1">
            <label class="form-label">Path</label>
            <input 
              v-model="ingressForm.path"
              type="text" 
              class="input-glass font-mono" 
              placeholder="/" 
            />
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Path Type</label>
            <select v-model="ingressForm.pathType" class="input-glass select-full font-mono">
              <option value="Prefix">Prefix</option>
              <option value="Exact">Exact</option>
              <option value="ImplementationSpecific">ImplementationSpecific</option>
            </select>
          </div>
        </div>

        <div class="form-row-2">
          <div class="form-group flex-2">
            <label class="form-label required">Target Service Name</label>
            <input 
              v-model="ingressForm.serviceName"
              type="text" 
              class="input-glass font-mono" 
              placeholder="e.g. web-service" 
              required 
            />
          </div>
          <div class="form-group flex-1">
            <label class="form-label required">Target Port</label>
            <input 
              v-model.number="ingressForm.servicePort"
              type="number" 
              class="input-glass font-mono" 
              placeholder="80" 
              required 
            />
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">TLS Secret (Optional for HTTPS)</label>
          <input 
            v-model="ingressForm.tlsSecret"
            type="text" 
            class="input-glass font-mono" 
            placeholder="e.g. tls-app-secret" 
          />
        </div>
      </div>

      <!-- PVC Form -->
      <div v-else-if="selectedKind === 'persistentvolumeclaims'" class="kind-form">
        <div class="form-row-2">
          <div class="form-group flex-2">
            <label class="form-label required">Claim Name</label>
            <input 
              v-model="pvcForm.name"
              type="text" 
              class="input-glass font-mono" 
              placeholder="e.g. data-volume-pvc" 
              required 
            />
          </div>
          <div class="form-group flex-1">
            <label class="form-label required">Capacity (Size)</label>
            <input 
              v-model="pvcForm.capacity"
              type="text" 
              class="input-glass font-mono" 
              placeholder="10Gi" 
              required 
            />
          </div>
        </div>

        <div class="form-row-2">
          <div class="form-group flex-1">
            <label class="form-label">Access Mode</label>
            <select v-model="pvcForm.accessMode" class="input-glass select-full font-mono">
              <option value="ReadWriteOnce">ReadWriteOnce (RWO)</option>
              <option value="ReadWriteMany">ReadWriteMany (RWX)</option>
              <option value="ReadOnlyMany">ReadOnlyMany (ROX)</option>
            </select>
          </div>
          <div class="form-group flex-1">
            <label class="form-label">StorageClass (Optional)</label>
            <input 
              v-model="pvcForm.storageClassName"
              type="text" 
              class="input-glass font-mono" 
              placeholder="e.g. standard or local-path" 
            />
          </div>
        </div>
      </div>

      <!-- Job Form -->
      <div v-else-if="selectedKind === 'jobs'" class="kind-form">
        <div class="form-row-2">
          <div class="form-group flex-2">
            <label class="form-label required">Job Name</label>
            <input 
              v-model="jobForm.name"
              type="text" 
              class="input-glass font-mono" 
              placeholder="e.g. database-migration-job" 
              required 
            />
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Backoff Limit</label>
            <input 
              v-model.number="jobForm.backoffLimit"
              type="number" 
              min="0"
              max="20"
              class="input-glass font-mono" 
            />
          </div>
        </div>

        <div class="form-row-2">
          <div class="form-group flex-2">
            <label class="form-label required">Container Image</label>
            <input 
              v-model="jobForm.image"
              type="text" 
              class="input-glass font-mono" 
              placeholder="e.g. busybox:latest or alpine" 
              required 
            />
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Restart Policy</label>
            <select v-model="jobForm.restartPolicy" class="input-glass select-full font-mono">
              <option value="OnFailure">OnFailure</option>
              <option value="Never">Never</option>
            </select>
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Shell Command / Script</label>
          <input 
            v-model="jobForm.command"
            type="text" 
            class="input-glass font-mono" 
            placeholder="echo 'Done'" 
          />
        </div>
      </div>

      <!-- CronJob Form -->
      <div v-else-if="selectedKind === 'cronjobs'" class="kind-form">
        <div class="form-row-2">
          <div class="form-group flex-2">
            <label class="form-label required">CronJob Name</label>
            <input 
              v-model="cronJobForm.name"
              type="text" 
              class="input-glass font-mono" 
              placeholder="e.g. nightly-backup-cron" 
              required 
            />
          </div>
          <div class="form-group flex-1">
            <label class="form-label required">Cron Schedule</label>
            <input 
              v-model="cronJobForm.schedule"
              type="text" 
              class="input-glass font-mono" 
              placeholder="*/5 * * * *" 
              required 
            />
          </div>
        </div>

        <div class="form-row-2">
          <div class="form-group flex-2">
            <label class="form-label required">Container Image</label>
            <input 
              v-model="cronJobForm.image"
              type="text" 
              class="input-glass font-mono" 
              placeholder="e.g. curlimages/curl:latest" 
              required 
            />
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Concurrency Policy</label>
            <select v-model="cronJobForm.concurrencyPolicy" class="input-glass select-full font-mono">
              <option value="Allow">Allow</option>
              <option value="Forbid">Forbid</option>
              <option value="Replace">Replace</option>
            </select>
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Shell Command / Script</label>
          <input 
            v-model="cronJobForm.command"
            type="text" 
            class="input-glass font-mono" 
            placeholder="curl -s https://api.service/task" 
          />
        </div>
      </div>

      <!-- Preview Collapsible -->
      <div class="preview-accordion">
        <button 
          type="button" 
          class="preview-toggle-btn"
          @click="showPreview = !showPreview"
        >
          <span>{{ showPreview ? '▼ Hide Generated Manifest' : '▶ Show Live YAML Preview' }}</span>
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
        <span>{{ submitting ? '⏳ Creating...' : '✨ Create Resource' }}</span>
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

.flex-1 {
  flex: 1;
}

.flex-2 {
  flex: 2;
}

.select-full {
  width: 100%;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-label {
  font-size: 0.8rem;
  font-weight: 600;
  color: #94a3b8;
  letter-spacing: 0.02em;
}

.form-label.required::after {
  content: ' *';
  color: #f43f5e;
}

.kind-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.key-value-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
  background: rgba(15, 23, 42, 0.4);
  padding: 12px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.05);
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
  cursor: pointer;
  font-size: 0.85rem;
  opacity: 0.7;
  padding: 2px;
}

.btn-mask-toggle:hover {
  opacity: 1;
}

.btn-icon-danger {
  background: rgba(244, 63, 94, 0.15);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fb7185;
  border-radius: 6px;
  width: 32px;
  height: 32px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  font-size: 0.8rem;
  transition: all 0.15s ease;
}

.btn-icon-danger:hover:not(:disabled) {
  background: rgba(244, 63, 94, 0.3);
  color: #fff;
}

.btn-icon-danger:disabled {
  opacity: 0.3;
  cursor: not-allowed;
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
  color: #00e5ff;
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
  text-align: left;
  padding: 4px 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.preview-toggle-btn:hover {
  text-decoration: underline;
}

.manifest-preview {
  padding: 14px;
  border-radius: 8px;
  background: rgba(10, 15, 30, 0.85);
  border: 1px solid rgba(0, 229, 255, 0.2);
  max-height: 220px;
  overflow-y: auto;
}

.manifest-preview pre {
  margin: 0;
  font-size: 0.78rem;
  line-height: 1.4;
  color: #38bdf8;
  white-space: pre-wrap;
  word-break: break-word;
}

.error-banner {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border-radius: 8px;
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.35);
  color: #fb7185;
}

.error-text {
  display: flex;
  flex-direction: column;
  font-size: 0.82rem;
  flex: 1;
}

.error-close {
  background: none;
  border: none;
  color: #fb7185;
  cursor: pointer;
  font-size: 0.9rem;
}

@media (max-width: 640px) {
  .selectors-row,
  .form-row-2,
  .kv-row {
    flex-direction: column;
  }
}
</style>