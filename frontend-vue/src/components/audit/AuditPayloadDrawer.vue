<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import type { AuditLogEntry } from '../../api/governance'
import StatusBadge from '../ui/StatusBadge.vue'

const props = defineProps<{
  event: AuditLogEntry | null
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const copied = ref(false)

const formattedJson = computed(() => {
  if (!props.event) return ''
  const fullMetadata = {
    event_id: props.event.id,
    timestamp: props.event.timestamp,
    actor: props.event.actor,
    action: props.event.action,
    action_type: props.event.action_type,
    target_resource: props.event.target_resource,
    target_type: props.event.target_type,
    ip_address: props.event.ip_address,
    user_agent: props.event.user_agent,
    status: props.event.status,
    severity: props.event.severity,
    payload: props.event.payload || {},
    details: props.event.details || {},
  }
  return JSON.stringify(fullMetadata, null, 2)
})

async function copyJson() {
  if (!formattedJson.value) return
  try {
    await navigator.clipboard.writeText(formattedJson.value)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch {
    // Clipboard permission fallback
  }
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.open) {
    emit('close')
  }
}

watch(
  () => props.open,
  isOpen => {
    if (isOpen) {
      copied.value = false
    }
  }
)

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})

function formatDate(d?: string): string {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleString()
  } catch {
    return d
  }
}
</script>

<template>
  <div
    v-if="open && event"
    class="drawer-backdrop"
    role="dialog"
    aria-modal="true"
    aria-label="Audit Event Payload Inspector"
    @click.self="emit('close')"
  >
    <div class="drawer-panel">
      <!-- Drawer Header -->
      <div class="drawer-header">
        <div class="drawer-title-group">
          <div style="display: flex; align-items: center; gap: 8px;">
            <span class="drawer-title font-mono">{{ event.action }}</span>
            <StatusBadge :status="event.status === 'success' ? 'active' : event.status === 'denied' ? 'danger' : 'warning'" :label="event.status.toUpperCase()" size="sm" />
          </div>
          <span class="drawer-subtitle font-mono">Event ID: #{{ event.id }}</span>
        </div>

        <div class="drawer-header-actions">
          <button class="btn btn-secondary btn-sm" type="button" @click="copyJson">
            <span>{{ copied ? '✅ Copied!' : '📋 Copy JSON' }}</span>
          </button>
          <button class="drawer-close-btn" type="button" aria-label="Close Inspector" @click="emit('close')">
            ✕
          </button>
        </div>
      </div>

      <!-- Drawer Body -->
      <div class="drawer-body">
        <!-- Quick Metadata Grid -->
        <div class="drawer-meta-grid">
          <div class="meta-item">
            <span class="meta-label">Actor</span>
            <span class="meta-val font-mono">👤 {{ event.actor }}</span>
          </div>

          <div class="meta-item">
            <span class="meta-label">Action Type</span>
            <span class="meta-val font-mono">{{ event.action_type.toUpperCase() }}</span>
          </div>

          <div class="meta-item">
            <span class="meta-label">Target Resource</span>
            <span class="meta-val font-mono" style="color: var(--accent-sky);">{{ event.target_resource }}</span>
          </div>

          <div class="meta-item">
            <span class="meta-label">Target Type</span>
            <span class="meta-val font-mono">{{ event.target_type }}</span>
          </div>

          <div class="meta-item">
            <span class="meta-label">Client IP</span>
            <span class="meta-val font-mono">{{ event.ip_address }}</span>
          </div>

          <div class="meta-item">
            <span class="meta-label">Detected Timestamp</span>
            <span class="meta-val font-mono">{{ formatDate(event.timestamp) }}</span>
          </div>
        </div>

        <!-- Full JSON Inspector -->
        <div class="payload-inspector-container">
          <div class="payload-inspector-header">
            <span class="payload-label">Raw Audit Event & Context Payload (JSON)</span>
            <span class="font-mono text-muted" style="font-size: 11px;">RFC 8259</span>
          </div>
          <pre class="payload-pre"><code>{{ formattedJson }}</code></pre>
        </div>
      </div>

      <!-- Drawer Footer -->
      <div class="drawer-footer">
        <button class="btn btn-secondary" type="button" @click="emit('close')">Close Inspector</button>
      </div>
    </div>
  </div>
</template>
