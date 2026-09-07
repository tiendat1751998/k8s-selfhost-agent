<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import type { Incident } from '../../api/compute'

defineProps<{
  incidents: Incident[]
  selectedIncident: Incident | null
  loading?: boolean
  actionLoading?: string | null
}>()

const emit = defineEmits<{
  (e: 'select', incident: Incident): void
  (e: 'triage', incident: Incident): void
  (e: 'rca', incident: Incident): void
  (e: 'mitigate', incident: Incident): void
  (e: 'resolve', incident: Incident): void
}>()

function formatTime(d?: string) {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  } catch {
    return d
  }
}
</script>

<template>
  <div class="incidents-table-container">
    <div v-if="loading" class="table-loading-state font-mono">
      <span>⏳ Querying live incident telemetry...</span>
    </div>

    <div v-else-if="incidents.length === 0" class="empty-list">
      <div class="empty-icon">🛡</div>
      <div class="empty-title">No Incidents Detected</div>
      <p class="empty-desc">
        Cluster telemetry is nominal. Inject a test anomaly scenario to evaluate autonomous AI diagnostics and GitOps remediation.
      </p>
    </div>

    <div v-else class="table-responsive">
      <table class="incidents-table">
        <thead>
          <tr>
            <th class="table-th">Target / Workload</th>
            <th class="table-th">Anomaly Type</th>
            <th class="table-th">Severity</th>
            <th class="table-th">Status</th>
            <th class="table-th">Detected</th>
            <th class="table-th text-right">Triage Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="inc in incidents"
            :key="inc.id"
            class="table-row"
            :class="{ 'row-active': selectedIncident?.id === inc.id, ['border-sev-' + inc.severity]: true }"
            tabindex="0"
            role="button"
            @click="emit('select', inc)"
            @keydown.enter="emit('select', inc)"
          >
            <td class="table-td">
              <div class="target-cell">
                <span class="target-pod-text">{{ inc.pod_name }}</span>
                <span class="target-sub-text font-mono">{{ inc.cluster_name }} / {{ inc.namespace }}</span>
              </div>
            </td>
            <td class="table-td">
              <span class="type-badge font-mono">{{ inc.type }}</span>
            </td>
            <td class="table-td">
              <StatusBadge :status="inc.severity" size="sm" />
            </td>
            <td class="table-td">
              <StatusBadge :status="inc.status" size="sm" />
            </td>
            <td class="table-td font-mono text-muted text-sm">
              {{ formatTime(inc.created_at) }}
            </td>
            <td class="table-td text-right">
              <div class="btn-action-group" @click.stop>
                <button
                  type="button"
                  class="btn-action btn-action-triage"
                  title="Open incident triage inspector"
                  @click="emit('triage', inc)"
                >
                  <span>🔍 Triage</span>
                </button>
                <button
                  type="button"
                  class="btn-action btn-action-rca"
                  title="View chronological RCA reasoning timeline"
                  @click="emit('rca', inc)"
                >
                  <span>🔬 RCA Timeline</span>
                </button>
                <button
                  type="button"
                  class="btn-action btn-action-mitigate"
                  :disabled="actionLoading === 'mitigate-' + inc.id || inc.status === 'resolved'"
                  title="Initiate autonomous mitigation pipeline"
                  @click="emit('mitigate', inc)"
                >
                  <span>{{ actionLoading === 'mitigate-' + inc.id ? '⏳ Mitigating' : '🎯 Mitigate' }}</span>
                </button>
                <button
                  type="button"
                  class="btn-action btn-action-resolve"
                  :disabled="actionLoading === 'resolve-' + inc.id || inc.status === 'resolved'"
                  title="Immediately resolve and clear anomaly"
                  @click="emit('resolve', inc)"
                >
                  <span>{{ actionLoading === 'resolve-' + inc.id ? '⏳' : '🕑 Resolve' }}</span>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
