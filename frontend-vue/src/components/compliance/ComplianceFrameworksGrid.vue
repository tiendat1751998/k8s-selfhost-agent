<template>
  <div class="compliance-frameworks-strip glass-panel font-mono">
    <div class="framework-pills" role="tablist" aria-label="Compliance Framework Filter">
      <button
        type="button"
        class="fw-pill"
        :class="{ active: !selectedFrameworkId && selectedStandard === 'ALL' }"
        role="tab"
        :aria-selected="!selectedFrameworkId && selectedStandard === 'ALL'"
        @click="$emit('select-standard', 'ALL'); $emit('select-framework', '')"
      >
        <span>ALL</span>
        <span class="pill-metric">({{ totalControlsCount || totalChecksSum }})</span>
      </button>

      <button
        v-for="fw in frameworks"
        :key="fw.id"
        type="button"
        class="fw-pill"
        :class="{ active: selectedFrameworkId === fw.id }"
        role="tab"
        :aria-selected="selectedFrameworkId === fw.id"
        @click="$emit('select-framework', fw.id)"
      >
        <span>{{ fw.name }}</span>
        <span class="pill-metric" :class="getProgressColorClass(fw.score)">({{ fw.score.toFixed(0) }}%)</span>
      </button>
    </div>

    <div class="fw-actions-group desktop-only">
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="loading"
        title="Refresh Posture"
        @click="$emit('refresh')"
      >
        <BaseIcon :name="loading ? 'clock' : 'refresh'" size="xs" />
        <span>{{ loading ? 'Syncing...' : 'Refresh Posture' }}</span>
      </button>
      <button
        type="button"
        class="btn btn-primary btn-sm"
        :disabled="loading"
        title="Run Audit Scan"
        @click="$emit('run-scan')"
      >
        <BaseIcon name="play" size="xs" />
        <span>Run Audit Scan</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { ComplianceStandard } from '../../composables/useCompliance'
import type { ComplianceFramework } from '../../api/governance'
import BaseIcon from '../ui/BaseIcon.vue'

const props = defineProps<{
  frameworks: ComplianceFramework[]
  selectedFrameworkId: string
  selectedStandard: ComplianceStandard
  totalControlsCount?: number
  loading: boolean
  getProgressColorClass: (score: number) => string
  formatDate: (d: string) => string
}>()

defineEmits<{
  (e: 'select-framework', id: string): void
  (e: 'select-standard', standard: ComplianceStandard): void
  (e: 'run-scan'): void
  (e: 'refresh'): void
}>()

const totalChecksSum = computed(() => {
  return props.frameworks.reduce((sum, f) => sum + (f.total_checks || 0), 0)
})
</script>
