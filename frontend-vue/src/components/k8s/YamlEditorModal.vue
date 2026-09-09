<script setup lang="ts">
import { ref, watch } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import { k8sApi } from '../../api/k8s'
import { getDefaultYamlTemplate } from '../../utils/yaml'

const props = withDefaults(
  defineProps<{
    show: boolean
    cluster: string
    namespace?: string
    initialYaml?: string
    title?: string
    kind?: string
    resourceName?: string
    mode?: 'create' | 'edit'
  }>(),
  {
    namespace: 'default',
    initialYaml: '',
    title: 'Apply Kubernetes Manifest (YAML)',
    kind: 'Deployment',
    resourceName: 'my-resource',
    mode: 'create'
  }
)

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update:show', value: boolean): void
  (e: 'applied', result: { message: string }): void
}>()

const yamlContent = ref('')
const applying = ref(false)
const errorMessage = ref<string | null>(null)
const successMessage = ref<string | null>(null)
const copied = ref(false)

watch(
  () => props.show,
  (isOpen) => {
    if (isOpen) {
      errorMessage.value = null
      successMessage.value = null
      copied.value = false
      if (props.initialYaml) {
        yamlContent.value = props.initialYaml
      } else {
        yamlContent.value = getDefaultYamlTemplate(props.kind, props.resourceName, props.namespace)
      }
    }
  },
  { immediate: true }
)

function handleClose() {
  emit('close')
  emit('update:show', false)
}

async function handleCopy() {
  try {
    await navigator.clipboard.writeText(yamlContent.value)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch {
    // fallback
  }
}

function handleResetTemplate() {
  yamlContent.value = getDefaultYamlTemplate(props.kind, props.resourceName, props.namespace)
  errorMessage.value = null
}

async function handleApply() {
  if (!yamlContent.value.trim()) {
    errorMessage.value = 'YAML manifest cannot be empty.'
    return
  }

  applying.value = true
  errorMessage.value = null
  successMessage.value = null

  try {
    const res = await k8sApi.applyYAML(props.cluster, yamlContent.value, props.namespace)
    successMessage.value = res.message || 'Manifest applied successfully!'
    emit('applied', res)
    setTimeout(() => {
      handleClose()
    }, 1200)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to apply YAML manifest to cluster'
    errorMessage.value = msg
  } finally {
    applying.value = false
  }
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    :title="title"
    :subtitle="`Cluster: ${cluster} ? Target Namespace: ${namespace || 'default'}`"
    max-width="820px"
    @close="handleClose"
  >
    <div class="yaml-editor-container">
      <!-- Toolbar -->
      <div class="editor-toolbar">
        <div class="toolbar-left">
          <span class="file-type-badge font-mono">YAML</span>
          <span class="target-info">Targeting <strong>{{ cluster }}</strong></span>
        </div>
        <div class="toolbar-right">
          <button 
            type="button" 
            class="btn btn-secondary btn-xs"
            @click="handleResetTemplate"
            title="Reset to default template"
          >
            <span>?? Reset Template</span>
          </button>
          <button 
            type="button" 
            class="btn btn-secondary btn-xs"
            :class="{ 'btn-copied': copied }"
            @click="handleCopy"
            title="Copy YAML to clipboard"
          >
            <span>{{ copied ? '? Copied!' : '?? Copy YAML' }}</span>
          </button>
        </div>
      </div>

      <!-- Error / Success Notification -->
      <div v-if="errorMessage" class="editor-banner banner-error animate-fade-in">
        <span class="banner-icon">??</span>
        <div class="banner-content">
          <strong>Validation / Apply Error:</strong>
          <span>{{ errorMessage }}</span>
        </div>
        <button class="banner-close" @click="errorMessage = null">?</button>
      </div>

      <div v-if="successMessage" class="editor-banner banner-success animate-fade-in">
        <span class="banner-icon">?</span>
        <div class="banner-content">
          <span>{{ successMessage }}</span>
        </div>
      </div>

      <!-- Textarea Editor -->
      <div class="editor-body glass-panel">
        <textarea
          v-model="yamlContent"
          class="yaml-textarea font-mono"
          placeholder="Paste or write your raw Kubernetes YAML manifest here..."
          rows="22"
          spellcheck="false"
        ></textarea>
      </div>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="applying" @click="handleClose">
        Cancel
      </button>
      <button 
        type="button" 
        class="btn btn-primary"
        :disabled="applying || !yamlContent.trim()"
        @click="handleApply"
      >
        <span>{{ applying ? '? Applying to Cluster...' : '?? Apply Manifest' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/components/yaml-editor.css';
</style>
