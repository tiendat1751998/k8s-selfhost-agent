<template>
  <div class="compliance-mobile-stream">
    <div v-if="violations.length === 0" class="empty-mobile-box glass-panel">
      <span>🛡️ No compliance violations found.</span>
    </div>

    <div
      v-for="v in violations"
      :key="v.id"
      class="mobile-card-row glass-panel"
      :class="`border-${v.severity}`"
    >
      <!-- Severity Indicator Pill -->
      <div class="mobile-card-left">
        <StatusBadge :status="v.severity" :label="v.severity.slice(0, 4).toUpperCase()" size="sm" />
      </div>

      <!-- Core Content (Capped text, single line with ellipsis, zero horizontal overflow) -->
      <div class="mobile-card-center">
        <div class="mobile-policy-title" :title="v.policy">{{ v.policy }}</div>
        <div class="mobile-meta font-mono text-muted">
          <span class="mobile-resource" :title="v.resource">{{ v.resource }}</span>
          <span class="mobile-ns">({{ v.namespace || 'default' }})</span>
        </div>
      </div>

      <!-- Quick Action Buttons -->
      <div class="mobile-card-right">
        <button class="btn btn-secondary btn-icon-sm" title="Inspect" @click="$emit('inspect', v)">
          🔍
        </button>
        <button class="btn btn-primary btn-icon-sm" title="Remediate" @click="$emit('remediate', v)">
          ⚡
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import type { ComplianceViolation } from '../../api/governance'

defineProps<{
  violations: ComplianceViolation[]
}>()

defineEmits<{
  (e: 'inspect', violation: ComplianceViolation): void
  (e: 'remediate', violation: ComplianceViolation): void
}>()
</script>
