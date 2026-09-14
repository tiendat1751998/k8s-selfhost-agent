<template>
  <ComplianceControlsTable
    :violations="violations"
    :loading="loading"
    :error="error"
    :active-severity="activeSeverity"
    :severity-filters="severityFilters"
    :selected-framework-name="selectedFrameworkName"
    :format-framework-tag="formatFrameworkTag"
    :format-date="formatDate"
    @filter-severity="emit('filter-severity', $event)"
    @clear-framework="emit('clear-framework')"
    @inspect="emit('inspect', $event)"
    @remediate="emit('remediate', $event)"
    @export="emit('export')"
  />
</template>

<script setup lang="ts">
import ComplianceControlsTable from './ComplianceControlsTable.vue'
import type { ComplianceViolation } from '../../api/governance'
import type { SeverityFilter } from '../../composables/useCompliance'

defineProps<{
  violations: ComplianceViolation[]
  loading: boolean
  error: string | null
  activeSeverity: SeverityFilter
  severityFilters: Array<{ key: SeverityFilter; label: string; count: number; badgeClass: string }>
  selectedFrameworkName?: string
  formatFrameworkTag: (tag: string) => string
  formatDate: (d: string) => string
}>()

const emit = defineEmits<{
  (e: 'filter-severity', sev: SeverityFilter): void
  (e: 'clear-framework'): void
  (e: 'inspect', violation: ComplianceViolation): void
  (e: 'remediate', violation: ComplianceViolation): void
  (e: 'export'): void
}>()
</script>
