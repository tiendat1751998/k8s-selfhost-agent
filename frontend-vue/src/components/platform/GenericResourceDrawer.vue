<script setup lang="ts">
import { ref } from 'vue'
import type { GenericPlatformItem } from '../../composables/useGenericPlatform'
import ModalDrawer from '../ui/ModalDrawer.vue'
import StatusBadge from '../ui/StatusBadge.vue'

const props = withDefaults(
  defineProps<{
    isOpen: boolean
    item: GenericPlatformItem | null
    yamlContent?: string
    jsonContent?: string
  }>(),
  {
    yamlContent: '',
    jsonContent: ''
  }
)

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'action', actionId: string, item: GenericPlatformItem): void
}>()

const activeFormat = ref<'yaml' | 'json'>('yaml')
const copied = ref(false)

async function handleCopy() {
  const text = activeFormat.value === 'yaml' ? props.yamlContent : props.jsonContent
  if (!text) return

  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text)
    } else {
      const textArea = document.createElement('textarea')
      textArea.value = text
      textArea.style.position = 'fixed'
      textArea.style.opacity = '0'
      document.body.appendChild(textArea)
      textArea.focus()
      textArea.select()
      document.execCommand('copy')
      document.body.removeChild(textArea)
    }
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch (err: unknown) {
    console.error('Failed to copy to clipboard', err)
  }
}

function handleDownload() {
  const text = activeFormat.value === 'yaml' ? props.yamlContent : props.jsonContent
  if (!text) return

  const ext = activeFormat.value === 'yaml' ? 'yaml' : 'json'
  const mime = activeFormat.value === 'yaml' ? 'text/yaml' : 'application/json'
  const filename = `${props.item?.name || 'platform-resource'}.${ext}`

  const blob = new Blob([text], { type: `${mime};charset=utf-8` })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
</script>

<template>
  <ModalDrawer
    :show="isOpen"
    mode="drawer"
    placement="right"
    max-width="640px"
    :title="item ? item.name : 'Resource Inspector'"
    :subtitle="item ? (item.kind || 'Platform Subsystem Object') : undefined"
    @close="emit('close')"
  >
    <template #header-prefix>
      <span class="drawer-header-icon">🔎</span>
    </template>

    <div v-if="item" class="drawer-inspect-body">
      <!-- Metadata Summary Header -->
      <div class="meta-strip glass-panel">
        <div class="meta-chip">
          <span class="meta-label">STATUS</span>
          <StatusBadge :status="item.status" size="sm" />
        </div>
        <div class="meta-chip">
          <span class="meta-label">TAG</span>
          <span class="badge badge-cyan font-mono">{{ item.tag }}</span>
        </div>
        <div v-if="item.namespace" class="meta-chip">
          <span class="meta-label">NAMESPACE</span>
          <span class="font-mono text-cyan">{{ item.namespace }}</span>
        </div>
        <div v-if="item.id" class="meta-chip">
          <span class="meta-label">ID</span>
          <span class="font-mono text-muted">{{ item.id }}</span>
        </div>
      </div>

      <!-- Telemetry Description -->
      <div class="desc-box">
        <span class="desc-caption">Live Telemetry Details:</span>
        <p class="desc-text">{{ item.detail }}</p>
      </div>

      <!-- Format Switch & Action Bar -->
      <div class="drawer-toolbar">
        <div class="format-switch" role="tablist">
          <button
            type="button"
            role="tab"
            :aria-selected="activeFormat === 'yaml'"
            class="switch-tab"
            :class="{ active: activeFormat === 'yaml' }"
            @click="activeFormat = 'yaml'"
          >
            YAML Spec
          </button>
          <button
            type="button"
            role="tab"
            :aria-selected="activeFormat === 'json'"
            class="switch-tab"
            :class="{ active: activeFormat === 'json' }"
            @click="activeFormat = 'json'"
          >
            JSON Raw
          </button>
        </div>

        <div class="toolbar-actions">
          <button
            type="button"
            class="btn btn-xs btn-secondary action-btn"
            :title="copied ? 'Copied to clipboard' : 'Copy code to clipboard'"
            @click="handleCopy"
          >
            <span>{{ copied ? '✓ Copied' : '📋 Copy' }}</span>
          </button>
          <button
            type="button"
            class="btn btn-xs btn-secondary action-btn"
            title="Download manifest file"
            @click="handleDownload"
          >
            <span>💾 Download</span>
          </button>
        </div>
      </div>

      <!-- Code Inspector Viewer -->
      <div class="code-container">
        <pre class="code-block font-mono"><code>{{ activeFormat === 'yaml' ? yamlContent : jsonContent }}</code></pre>
      </div>
    </div>
  </ModalDrawer>
</template>