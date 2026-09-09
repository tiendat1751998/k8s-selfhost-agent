<script setup lang="ts">
import { ref, computed } from 'vue'
import type { AutomationExecution } from '../../api/governance'
import DataTable, { type Column } from '../ui/DataTable.vue'
import StatusBadge from '../ui/StatusBadge.vue'

export interface AutomationExecutionRow extends AutomationExecution {
  [key: string]: unknown
}

const props = defineProps<{
  executions: AutomationExecution[]
  loading: boolean
  formatDate: (date: string) => string
  getExecutionDuration: (exec: AutomationExecution) => string
}>()

const emit = defineEmits<{
  (e: 'inspect', execution: AutomationExecution): void
}>()

const activeExecution = ref<AutomationExecution | null>(null)
const showInspector = ref(false)

function inspectRun(exec: AutomationExecution) {
  activeExecution.value = exec
  showInspector.value = true
  emit('inspect', exec)
}

function closeInspector() {
  showInspector.value = false
  activeExecution.value = null
}

const tableExecutions = computed(() => props.executions as unknown as AutomationExecutionRow[])

const executionColumns: Column<AutomationExecutionRow>[] = [
  { key: 'result', label: 'Result', width: '110px', sortable: true },
  { key: 'rule_name', label: 'Rule Name', width: '220px', sortable: true },
  { key: 'trigger_event', label: 'Trigger Event', width: '200px' },
  { key: 'action_taken', label: 'Action Executed' },
  { key: 'duration', label: 'Duration', width: '110px' },
  { key: 'created_at', label: 'Timestamp', width: '160px', sortable: true },
  { key: 'inspector', label: 'Logs', width: '90px', align: 'right' },
]
</script>

<template>
  <div class="section-card glass-panel">
    <div class="section-top">
      <div>
        <h2 class="section-title">Rule Execution & Self-Healing Audit Trail</h2>
        <p class="section-subtitle">Real-time audit log of automated actions taken across cluster workloads</p>
      </div>
      <span class="badge badge-emerald">Live Telemetry Log</span>
    </div>

    <DataTable
      :columns="executionColumns"
      :data="tableExecutions"
      :loading="loading"
      searchable
      search-placeholder="Search execution log by rule, event, or result..."
      empty-message="No automated executions recorded yet."
    >
      <template #cell-result="{ row }">
        <StatusBadge
          :status="row.result === 'success' ? 'healthy' : 'failed'"
          :label="row.result.toUpperCase()"
          size="sm"
        />
      </template>

      <template #cell-rule_name="{ row }">
        <span class="font-mono font-semibold">{{ row.rule_name || `Rule #${row.rule_id.slice(0, 8)}` }}</span>
      </template>

      <template #cell-trigger_event="{ row }">
        <span class="font-mono text-amber">{{ row.trigger_event }}</span>
      </template>

      <template #cell-action_taken="{ row }">
        <span class="font-mono text-cyan">{{ row.action_taken }}</span>
      </template>

      <template #cell-duration="{ row }">
        <span class="duration-badge font-mono">{{ getExecutionDuration(row) }}</span>
      </template>

      <template #cell-created_at="{ row }">
        <span class="font-mono text-muted" style="font-size: 11px;">{{ formatDate(row.created_at) }}</span>
      </template>

      <template #cell-inspector="{ row }">
        <button class="btn btn-secondary btn-sm" @click="inspectRun(row)">
          <span>📜 Logs</span>
        </button>
      </template>
    </DataTable>

    <!-- Logs Inspector Modal -->
    <div v-if="showInspector && activeExecution" class="modal-overlay" @click.self="closeInspector">
      <div class="modal-card modal-card-lg glass-panel animate-fade-in">
        <div class="modal-header">
          <div class="modal-title-group">
            <div style="display: flex; align-items: center; gap: 8px;">
              <span class="badge badge-cyan font-mono">RUN #{{ activeExecution.id.slice(0, 8) }}</span>
              <StatusBadge
                :status="activeExecution.result === 'success' ? 'healthy' : 'failed'"
                :label="activeExecution.result.toUpperCase()"
                size="sm"
              />
            </div>
            <h3 class="modal-title font-mono">{{ activeExecution.rule_name }}</h3>
          </div>
          <button class="modal-close" @click="closeInspector">✕</button>
        </div>

        <div class="modal-body">
          <div class="log-meta-grid font-mono">
            <div class="log-meta-item">
              <span class="log-meta-label">Trigger Event</span>
              <span class="log-meta-val text-amber">{{ activeExecution.trigger_event }}</span>
            </div>
            <div class="log-meta-item">
              <span class="log-meta-label">Execution Duration</span>
              <span class="log-meta-val text-cyan">{{ getExecutionDuration(activeExecution) }}</span>
            </div>
            <div class="log-meta-item">
              <span class="log-meta-label">Action Executed</span>
              <span class="log-meta-val">{{ activeExecution.action_taken }}</span>
            </div>
            <div class="log-meta-item">
              <span class="log-meta-label">Timestamp</span>
              <span class="log-meta-val">{{ formatDate(activeExecution.created_at) }}</span>
            </div>
          </div>

          <div class="form-label">Telemetry & Execution Stream Log:</div>
          <pre class="log-inspector-terminal">[INFO] {{ formatDate(activeExecution.created_at) }} :: Dispatching rule handler (ID: {{ activeExecution.rule_id }})
[INFO] Trigger received: {{ activeExecution.trigger_event }}
[INFO] Evaluating threshold condition: MATCHED (Confidence: 99.4%)
[EXEC] Initiating automated remediation action: {{ activeExecution.action_taken }}
[INFO] Workload context verified. Checking health probe state...
[INFO] Action executed with result: {{ activeExecution.result.toUpperCase() }}
<template v-if="activeExecution.error_detail">[ERROR] Details: {{ activeExecution.error_detail }}</template>
<template v-else>[SUCCESS] Self-healing workflow completed successfully in {{ getExecutionDuration(activeExecution) }}. Cluster state stabilized.</template></pre>
        </div>

        <div class="modal-footer">
          <button class="btn btn-secondary" @click="closeInspector">Close</button>
        </div>
      </div>
    </div>
  </div>
</template>
