<template>
  <div class="automation-mobile-cards">
    <div v-if="loading" class="stream-status font-mono">
      <span class="spin-icon">⏳</span> Loading executions...
    </div>
    <div v-else-if="executions.length === 0" class="stream-empty glass-panel font-mono">
      <span class="empty-icon">📜</span>
      <p class="empty-text">No automated executions recorded yet.</p>
    </div>
    <div v-else class="cards-list">
      <div
        v-for="item in executions"
        :key="item.id"
        class="auto-card glass-panel"
      >
        <!-- Top: Rule Name & Status Badge -->
        <div class="card-header-row">
          <span class="rule-title font-mono" :title="item.rule_name || item.rule_id">
            {{ item.rule_name || `Rule #${item.rule_id.slice(0, 8)}` }}
          </span>
          <span class="status-pill font-mono" :class="`pill-${getStatusKey(item.result)}`">
            {{ formatStatus(item.result) }}
          </span>
        </div>

        <!-- Mid Row: Trigger & Action -->
        <div class="card-details-grid font-mono">
          <div class="detail-row">
            <span class="detail-k">Trigger:</span>
            <span class="detail-v text-amber">{{ item.trigger_event || 'system_event' }}</span>
          </div>
          <div class="detail-row">
            <span class="detail-k">Action:</span>
            <span class="detail-v text-cyan">{{ item.action_taken || 'remediation' }}</span>
          </div>
          <div class="detail-row">
            <span class="detail-k">Target:</span>
            <span class="detail-v text-violet">{{ getTargetWorkload(item) }}</span>
          </div>
        </div>

        <!-- Bottom Row: Duration & Timestamp -->
        <div class="card-footer-row font-mono">
          <span class="meta-duration text-emerald">⏱️ {{ getDuration(item) }}</span>
          <span class="meta-time text-muted">{{ formatDate(item.created_at) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { AutomationExecution } from '../../api/governance'

defineProps<{
  executions: AutomationExecution[]
  loading?: boolean
}>()

function getStatusKey(result: string): 'success' | 'running' | 'failed' {
  const r = (result || '').toLowerCase()
  if (r.includes('success')) return 'success'
  if (r.includes('running') || r.includes('pending')) return 'running'
  return 'failed'
}

function formatStatus(result: string): string {
  const key = getStatusKey(result)
  if (key === 'success') return 'SUCCESS'
  if (key === 'running') return 'RUNNING'
  return 'FAILED'
}

function getTargetWorkload(item: AutomationExecution): string {
  if (item.error_detail && item.error_detail.includes('workload:')) {
    const match = item.error_detail.match(/workload:\s*([^\s,]+)/)
    if (match) return match[1]
  }
  const match = (item.trigger_event || '').match(/(?:pod|deploy|node)[\/:\s]+([a-zA-Z0-9_-]+)/i)
  if (match) return match[1]
  return 'Cluster Scope'
}

function getDuration(item: AutomationExecution): string {
  if (item.result === 'running') return 'in-flight'
  return '< 1.2s'
}

function formatDate(dateStr?: string): string {
  if (!dateStr) return '-'
  try {
    const d = new Date(dateStr)
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }) + ' ' + (d.getMonth() + 1) + '/' + d.getDate()
  } catch {
    return dateStr
  }
}
</script>

<style scoped>
.automation-mobile-cards { display: flex; flex-direction: column; gap: 8px; width: 100%; }
.stream-status, .stream-empty { padding: 20px 16px; text-align: center; font-size: 12px; color: var(--text-muted); }
.cards-list { display: flex; flex-direction: column; gap: 8px; }
.auto-card {
  padding: 10px 12px; border-radius: 10px; background: rgba(15, 23, 42, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.08); display: flex; flex-direction: column; gap: 6px;
}
.auto-card:hover { border-color: rgba(6, 182, 212, 0.3); }
.card-header-row { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.rule-title { font-size: 13px; font-weight: 700; color: #fff; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }
.status-pill {
  font-size: 9.5px; font-weight: 800; padding: 2px 6px; border-radius: 4px; letter-spacing: 0.04em; flex-shrink: 0;
}
.pill-success { background: rgba(16, 185, 129, 0.15); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.3); }
.pill-running { background: rgba(6, 182, 212, 0.15); color: #38bdf8; border: 1px solid rgba(6, 182, 212, 0.3); }
.pill-failed { background: rgba(244, 63, 94, 0.15); color: #f87171; border: 1px solid rgba(244, 63, 94, 0.3); }

.card-details-grid {
  display: flex; flex-direction: column; gap: 3px; font-size: 11px;
  background: rgba(0, 0, 0, 0.2); padding: 6px 8px; border-radius: 6px;
}
.detail-row { display: flex; align-items: center; gap: 6px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.detail-k { color: var(--text-muted); min-width: 48px; font-size: 10px; }
.detail-v { font-weight: 600; overflow: hidden; text-overflow: ellipsis; }

.card-footer-row { display: flex; align-items: center; justify-content: space-between; font-size: 10.5px; padding-top: 2px; }
.meta-duration { font-weight: 600; }
.meta-time { font-size: 10px; }
</style>