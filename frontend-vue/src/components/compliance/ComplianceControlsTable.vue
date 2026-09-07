<template>
  <div class="violations-section">
    <div class="filter-bar glass-panel">
      <div class="filter-group">
        <span class="filter-label">Severity:</span>
        <button
          v-for="sev in severityFilters"
          :key="sev.key"
          class="filter-pill"
          :class="[sev.badgeClass, { 'filter-active': activeSeverity === sev.key }]"
          @click="emit('filter-severity', sev.key)"
        >
          <span>{{ sev.label }} ({{ sev.count }})</span>
        </button>
      </div>

      <div class="filter-actions-group">
        <div v-if="selectedFrameworkName" class="active-filter-badge">
          <span>Framework: {{ selectedFrameworkName }}</span>
          <button class="clear-btn" title="Clear filter" @click="emit('clear-framework')">✕</button>
        </div>

        <button class="btn btn-secondary btn-sm export-btn" title="Export Remediation Playbook" @click="emit('export')">
          <span>📥 Export Playbook</span>
        </button>
      </div>
    </div>

    <DataTable
      :columns="columns"
      :data="tableData"
      :loading="loading"
      :error="error"
      searchable
      search-placeholder="Filter by policy, namespace, cluster, or resource..."
      empty-message="No compliance violations found. All security posture controls are satisfied."
    >
      <template #cell-severity="{ row }">
        <StatusBadge :status="row.severity" :label="row.severity.toUpperCase()" size="sm" />
      </template>

      <template #cell-resource="{ row }">
        <div class="resource-cell">
          <span class="resource-name font-mono">{{ row.resource }}</span>
          <span class="resource-ns font-mono text-muted">{{ row.namespace || 'default' }} ({{ row.cluster || 'primary' }})</span>
        </div>
      </template>

      <template #cell-policy="{ row }">
        <div class="policy-cell">
          <span class="policy-name">{{ row.policy }}</span>
          <span class="policy-msg text-muted">{{ row.message }}</span>
        </div>
      </template>

      <template #cell-framework_id="{ row }">
        <span class="fw-tag font-mono">{{ formatFrameworkTag(row.framework_id) }}</span>
      </template>

      <template #cell-detected_at="{ row }">
        <span class="font-mono text-muted detected-time">{{ formatDate(row.detected_at) }}</span>
      </template>

      <template #cell-actions="{ row }">
        <div class="control-actions">
          <button class="btn btn-secondary btn-xs btn-inspect" title="Inspect Control" @click="emit('inspect', row)">
            <span>🔍 Inspect</span>
          </button>
          <button class="btn btn-primary btn-xs btn-remediate" title="Remediate Control" @click="emit('remediate', row)">
            <span>⚡ Remediate</span>
          </button>
        </div>
      </template>
    </DataTable>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import DataTable, { type Column } from '../ui/DataTable.vue'
import StatusBadge from '../ui/StatusBadge.vue'
import type { ComplianceViolation } from '../../api/governance'
import type { ComplianceControlItem, SeverityFilter } from '../../composables/useCompliance'

const props = defineProps<{
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

const tableData = computed<ComplianceControlItem[]>(() => props.violations as ComplianceControlItem[])

const columns: Column<ComplianceControlItem>[] = [
  { key: 'severity', label: 'Severity', width: '110px', sortable: true },
  { key: 'resource', label: 'Resource & Namespace', width: '220px', sortable: true },
  { key: 'policy', label: 'Policy & Finding' },
  { key: 'framework_id', label: 'Framework', width: '140px', sortable: true },
  { key: 'detected_at', label: 'Detected', width: '130px', sortable: true },
  { key: 'actions', label: 'Actions', width: '200px', align: 'right' },
]
</script>
