<template>
  <div v-if="violation" class="drawer-backdrop" role="dialog" aria-modal="true" @click.self="emit('close')">
    <div class="drawer-panel" style="max-width: 540px;">
      <div class="drawer-header">
        <div class="drawer-title-group">
          <span class="drawer-title font-bold">
            {{ mode === 'remediate' ? 'Remediate Control' : 'Inspect Finding' }}
          </span>
          <span class="drawer-subtitle font-mono">{{ violation.policy }}</span>
        </div>
        <button class="drawer-close-btn" type="button" aria-label="Close" @click="emit('close')">✕</button>
      </div>

      <div class="drawer-body">
        <div class="drawer-meta-grid">
          <div class="meta-item">
            <span class="meta-label">Severity</span>
            <span
              class="meta-val font-bold font-mono"
              :style="{ color: violation.severity === 'critical' ? 'var(--color-status-offline)' : 'var(--color-status-warning)' }"
            >
              {{ violation.severity.toUpperCase() }}
            </span>
          </div>
          <div class="meta-item">
            <span class="meta-label">Framework</span>
            <span class="meta-val font-mono">{{ formatFrameworkTag(violation.framework_id) }}</span>
          </div>
          <div class="meta-item">
            <span class="meta-label">Target Resource</span>
            <span class="meta-val font-mono" style="color: var(--color-endpoint-syntax);">{{ violation.resource }}</span>
          </div>
          <div class="meta-item">
            <span class="meta-label">Namespace</span>
            <span class="meta-val font-mono">{{ violation.namespace || 'default' }} ({{ violation.cluster || 'primary' }})</span>
          </div>
        </div>

        <div class="payload-inspector-container">
          <span class="payload-label font-bold">Policy Deviation Message</span>
          <pre class="payload-pre"><code>{{ violation.message }}</code></pre>
        </div>
      </div>

      <div class="drawer-footer">
        <button class="btn btn-secondary btn-sm" type="button" @click="emit('close')">Close</button>
        <button
          v-if="mode === 'remediate'"
          class="btn btn-primary btn-sm"
          type="button"
          @click="emit('remediate', violation)"
        >
          <BaseIcon name="zap" size="xs" /> Apply Remediation
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'
import type { ComplianceViolation } from '../../api/governance'

defineProps<{
  violation: ComplianceViolation | null
  mode: 'inspect' | 'remediate' | null
  formatFrameworkTag: (tag: string) => string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'remediate', violation: ComplianceViolation): void
}>()
</script>
