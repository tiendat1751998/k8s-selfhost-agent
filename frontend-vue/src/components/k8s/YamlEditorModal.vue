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
.yaml-editor-container {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.editor-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: rgba(11, 15, 25, 0.6);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12px;
}

.file-type-badge {
  padding: 2px 8px;
  background: rgba(56, 189, 248, 0.15);
  border: 1px solid rgba(56, 189, 248, 0.3);
  border-radius: 6px;
  color: #38bdf8;
  font-weight: 700;
  font-size: 11px;
}

.target-info {
  color: var(--text-secondary);
}

.target-info strong {
  color: var(--text-primary);
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.editor-banner {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 14px;
  border-radius: 10px;
  font-size: 12px;
  line-height: 1.4;
}

.banner-error {
  background: rgba(244, 63, 94, 0.15);
  border: 1px solid rgba(244, 63, 94, 0.35);
  color: #fb7185;
}

.banner-success {
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.35);
  color: #34d399;
}

.banner-icon {
  font-size: 14px;
  margin-top: 1px;
}

.banner-content {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
}

.banner-close {
  background: none;
  border: none;
  color: inherit;
  cursor: pointer;
  font-size: 12px;
}

.editor-body {
  border: 1px solid var(--border-subtle);
  border-radius: 12px;
  overflow: hidden;
  background: #030712;
}

.yaml-textarea {
  width: 100%;
  height: 480px;
  padding: 16px;
  background: transparent;
  color: #e0f2fe;
  border: none;
  outline: none;
  resize: vertical;
  font-size: 12.5px;
  line-height: 1.6;
  tab-size: 2;
  white-space: pre;
}

.yaml-textarea::placeholder {
  color: #475569;
}

.font-mono {
  font-family: var(--font-mono);
}

.btn-xs {
  padding: 4px 8px;
  font-size: 11px;
}

.btn-copied {
  background: rgba(16, 185, 129, 0.15);
  border-color: rgba(16, 185, 129, 0.3);
  color: #34d399;
}

@media (max-width: 640px) {
  .editor-toolbar {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .toolbar-left,
  .toolbar-right {
    width: 100%;
    justify-content: space-between;
  }

  .yaml-textarea {
    height: 350px;
    font-size: 11px;
    padding: 10px;
  }
}
</style>
