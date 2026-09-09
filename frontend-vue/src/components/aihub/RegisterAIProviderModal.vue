<script setup lang="ts">
import { ref, watch } from 'vue'
import type { CreateProviderPayload } from '../../api/management'
import { AI_PROVIDER_PRESETS } from '../../composables/useAIProviderHub'
import ModalDrawer from '../ui/ModalDrawer.vue'

const props = defineProps<{
  show: boolean
  isSubmitting: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'submit', payload: CreateProviderPayload): void
}>()

const formData = ref<CreateProviderPayload>({
  name: '',
  type: 'ollama',
  endpoint: 'http://ollama-service.ai-core.svc:11434',
  model: 'llama3.3:70b',
  api_key: '',
  default: false,
})

watch(
  () => props.show,
  (val) => {
    if (val && !formData.value.name) {
      applyPreset('ollama')
    }
  }
)

function applyPreset(id: string) {
  const preset = AI_PROVIDER_PRESETS.find(p => p.id === id)
  if (!preset) return
  formData.value = {
    name: `${preset.id}-gateway-${Date.now().toString(36)}`,
    type: preset.type,
    endpoint: preset.defaultEndpoint,
    model: preset.defaultModel,
    api_key: '',
    default: false,
  }
}

function handleSubmit() {
  if (!formData.value.name || !formData.value.endpoint || !formData.value.model) return
  emit('submit', { ...formData.value })
}
</script>

<template>
  <ModalDrawer
    :show="show"
    title="Register LLM Provider Gateway"
    subtitle="Configure an on-premise Ollama or external API provider with circuit breaking."
    @update:show="emit('update:show', $event)"
  >
    <form @submit.prevent="handleSubmit" class="form-layout">
      <!-- Presets quick select -->
      <div class="form-group">
        <label>Quick Configuration Presets</label>
        <div class="preset-selector">
          <button
            v-for="p in AI_PROVIDER_PRESETS"
            :key="p.id"
            type="button"
            class="preset-chip"
            @click="applyPreset(p.id)"
          >
            {{ p.badge }}: {{ p.name }}
          </button>
        </div>
      </div>

      <div class="form-group">
        <label>Provider Instance Name</label>
        <input 
          v-model="formData.name" 
          type="text" 
          placeholder="e.g. onprem-llama3-worker-1" 
          class="input-glass" 
          required 
        />
      </div>

      <div class="form-group">
        <label>Provider Architecture Type</label>
        <select v-model="formData.type" class="input-glass">
          <option value="ollama">Ollama (Native Local On-Premise)</option>
          <option value="vllm">vLLM (High-Throughput GPU Server)</option>
          <option value="openai">OpenAI / Compatible REST API</option>
        </select>
      </div>

      <div class="form-group">
        <label>Service Endpoint URL</label>
        <input 
          v-model="formData.endpoint" 
          type="url" 
          placeholder="http://ollama.ai-core.svc:11434" 
          class="input-glass" 
          required 
        />
      </div>

      <div class="form-group">
        <label>Target Model Identifier</label>
        <input 
          v-model="formData.model" 
          type="text" 
          placeholder="e.g. llama3.3:70b or gpt-4o" 
          class="input-glass" 
          required 
        />
      </div>

      <div v-if="formData.type !== 'ollama'" class="form-group">
        <label>API Key / Bearer Secret (Optional)</label>
        <input 
          v-model="formData.api_key" 
          type="password" 
          placeholder="sk-..." 
          class="input-glass" 
        />
      </div>

      <div class="form-group" style="flex-direction: row; align-items: center; gap: 8px; margin-top: 4px;">
        <input 
          id="is-default-cb"
          v-model="formData.default" 
          type="checkbox" 
          style="cursor: pointer;"
        />
        <label for="is-default-cb" style="margin: 0; cursor: pointer;">Set as Cluster Default LLM Gateway</label>
      </div>
    </form>

    <template #footer="{ close }">
      <button class="btn btn-secondary" type="button" @click="close">Cancel</button>
      <button 
        class="btn btn-primary" 
        type="button"
        :disabled="isSubmitting || !formData.name || !formData.endpoint || !formData.model" 
        @click="handleSubmit"
      >
        <span>{{ isSubmitting ? 'Connecting...' : 'Arm Provider Circuit' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>
