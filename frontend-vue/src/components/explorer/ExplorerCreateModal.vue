<script setup lang="ts">
import { ref, watch } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import { k8sApi, type ResourceKind, type K8sNamespace } from '../../api/k8s'
import { manifestTemplates, type TemplateKey } from './templates'

const props = withDefaults(
  defineProps<{
    show: boolean
    cluster: string
    namespaces?: (K8sNamespace | string)[]
    defaultNamespace?: string
    defaultKind?: ResourceKind
  }>(),
  {
    namespaces: () => [],
    defaultNamespace: 'default',
    defaultKind: 'deployments',
  }
)

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'applied', result: unknown): void
}>()

const selectedTemplate = ref<TemplateKey>('deployment')
const yamlContent = ref('')
const applying = ref(false)
const applyError = ref<string | null>(null)

function selectTemplate(key: TemplateKey) {
  selectedTemplate.value = key
  const tpl = manifestTemplates.find((t) => t.key === key)
  if (tpl) {
    yamlContent.value = tpl.yaml(props.defaultNamespace || 'default')
  }
}

watch(
  () => props.show,
  (isOpen) => {
    if (isOpen && !yamlContent.value) {
      selectTemplate('deployment')
      applyError.value = null
    }
  },
  { immediate: true }
)

async function handleApply() {
  if (!yamlContent.value.trim()) {
    applyError.value = 'YAML manifest content cannot be empty'
    return
  }
  applying.value = true
  applyError.value = null
  try {
    const res = await k8sApi.applyYAML(props.cluster, yamlContent.value)
    emit('applied', res)
    emit('close')
  } catch (err) {
    applyError.value = err instanceof Error ? err.message : 'Failed to apply manifest'
  } finally {
    applying.value = false
  }
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="Create Kubernetes Resource"
    :subtitle="`Cluster: ${cluster} • Apply YAML Manifest`"
    max-width="720px"
    @close="$emit('close')"
  >
    <div class="create-modal-body">
      <div class="template-chips-section">
        <label class="form-label">Choose Manifest Template</label>
        <div class="template-chips">
          <button
            v-for="t in manifestTemplates"
            :key="t.key"
            type="button"
            class="template-chip font-mono"
            :class="{ 'is-active': selectedTemplate === t.key }"
            @click="selectTemplate(t.key)"
          >
            <span>{{ t.icon }}</span>
            <span>{{ t.label }}</span>
          </button>
        </div>
      </div>

      <div v-if="applyError" class="apply-error-banner font-mono">
        <span>⚠️ {{ applyError }}</span>
      </div>

      <div class="yaml-editor-wrap">
        <div class="editor-header font-mono">
          <span>manifest.yaml</span>
          <button type="button" class="btn-clear font-mono" @click="yamlContent = ''">Clear</button>
        </div>
        <textarea
          v-model="yamlContent"
          class="yaml-textarea font-mono"
          rows="14"
          placeholder="Paste or write Kubernetes YAML manifest here..."
          spellcheck="false"
        ></textarea>
      </div>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="applying" @click="$emit('close')">
        Cancel
      </button>
      <button
        type="button"
        class="btn btn-primary"
        :disabled="applying || !yamlContent.trim()"
        @click="handleApply"
      >
        <span>{{ applying ? 'Applying...' : '🚀 Apply Manifest' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/views/explorer.css';
</style>
