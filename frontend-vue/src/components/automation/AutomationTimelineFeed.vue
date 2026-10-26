<script setup lang="ts">
import { ref, computed } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import type { AutomationExecution, AutomationRule } from '../../api/governance'
import { formatRelativeTime } from '../../composables/useAutomationPipeline'

const props = defineProps<{
  executions: AutomationExecution[]
  rules: AutomationRule[]
  loading: boolean
  triggeringId: string | null
  formatDate: (date: string) => string
  getExecutionDuration: (exec: AutomationExecution) => string
}>()

const emit = defineEmits<{
  (e: 'inspect', execution: AutomationExecution): void
  (e: 'test-fire-rule', rule: AutomationRule): void
  (e: 'refresh'): void
}>()

const expandedIds = ref<Set<string>>(new Set())

function toggleExpand(id: string) {
  if (expandedIds.value.has(id)) {
    expandedIds.value.delete(id)
  } else {
    expandedIds.value.add(id)
  }
}

const firstActiveRule = computed(() => {
  return props.rules.find(r => r.enabled) || props.rules[0] || null
})
</script>

<template>
  <aside class="timeline-feed-panel glass-panel" aria-label="Self-Healing Live Execution Timeline">
    <header class="timeline-header">
      <div class="timeline-title-wrap">
        <h3 class="timeline-heading">Self-Healing Execution Timeline</h3>
        <span class="timeline-subheading">Live cluster remediation audit stream</span>
      </div>
      <button
        class="btn-icon-cmd btn-refresh-feed"
        :disabled="loading"
        title="Sync Execution Feed"
        aria-label="Refresh feed"
        @click="emit('refresh')"
      >
        <BaseIcon name="refresh" size="xs" :class="{ 'spin-anim': loading }" />
      </button>
    </header>

    <!-- Loading Stream -->
    <div v-if="loading && executions.length === 0" class="timeline-loading" role="status">
      <div v-for="i in 4" :key="i" class="timeline-skeleton-item"></div>
    </div>

    <!-- Actionable Empty State -->
    <div v-else-if="executions.length === 0" class="timeline-empty-card font-mono">
      <div class="empty-feed-icon"><BaseIcon name="shield" size="lg" /></div>
      <h4 class="empty-feed-title">Zero Active Incidents</h4>
      <p class="empty-feed-desc">
        No self-healing rules have fired in the current telemetry window. Cluster state is healthy.
      </p>
      <div v-if="firstActiveRule" class="empty-cta-wrap">
        <button
          class="btn btn-primary btn-sm btn-test-active"
          :disabled="triggeringId === firstActiveRule.id"
          :title="`Test-fire active rule: ${firstActiveRule.name}`"
          @click="emit('test-fire-rule', firstActiveRule)"
        >
          <BaseIcon :name="triggeringId === firstActiveRule.id ? 'clock' : 'zap'" size="xs" :class="{ 'spin-anim': triggeringId === firstActiveRule.id }" />
          <span>Test-Fire Active Rule</span>
        </button>
        <span class="cta-rule-preview text-muted">Runs: {{ firstActiveRule.name }}</span>
      </div>
    </div>

    <!-- Chronological Execution Stream -->
    <div v-else class="timeline-stream-list" role="feed" aria-busy="false">
      <div
        v-for="exec in executions"
        :key="exec.id"
        class="timeline-event-card"
        :class="'result-' + (exec.result || 'unknown')"
      >
        <div class="event-gutter">
          <span
            class="status-indicator-dot"
            :class="exec.result === 'success' ? 'dot-success' : 'dot-failed'"
            :title="`Status: ${exec.result}`"
          ></span>
          <span class="gutter-connector"></span>
        </div>

        <div class="event-main">
          <div class="event-meta-row font-mono">
            <span class="rule-name-tag font-semibold" :title="exec.rule_name || exec.rule_id">
              {{ exec.rule_name || `Rule #${exec.rule_id.slice(0, 8)}` }}
            </span>
            <span class="event-time" :title="formatDate(exec.created_at)">
              {{ formatRelativeTime(exec.created_at) }}
            </span>
          </div>

          <div class="event-flow-row font-mono">
            <span class="trigger-label text-amber">{{ exec.trigger_event }}</span>
            <span class="flow-arrow">→</span>
            <span class="action-label text-cyan">{{ exec.action_taken }}</span>
          </div>

          <div class="event-footer-row font-mono">
            <span class="duration-tag">{{ getExecutionDuration(exec) }}</span>
            <div class="footer-actions">
              <button
                v-if="exec.error_detail"
                class="btn-text-action text-rose"
                @click="toggleExpand(exec.id)"
              >
                {{ expandedIds.has(exec.id) ? 'Hide Error' : 'View Error' }}
              </button>
              <button class="btn-text-action" @click="emit('inspect', exec)">
                Logs
              </button>
            </div>
          </div>

          <!-- Expandable Error Detail -->
          <div v-if="exec.error_detail && expandedIds.has(exec.id)" class="error-detail-box font-mono animate-fade-in">
            <span class="error-detail-title">Diagnostic Payload:</span>
            <pre class="error-detail-text">{{ exec.error_detail }}</pre>
          </div>
        </div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.timeline-feed-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  border-radius: 12px;
  background: var(--bg-card);
  border: 1px solid var(--color-border-subtle);
  height: fit-content;
}

.timeline-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.timeline-heading { font-size: 13.5px; font-weight: 700; color: var(--text-primary); margin: 0; }
.timeline-subheading { font-size: 11px; color: var(--text-muted); display: block; }
.btn-refresh-feed { width: 28px; height: 28px; min-width: 28px; min-height: 28px; }

.timeline-stream-list { display: flex; flex-direction: column; gap: 0; max-height: 680px; overflow-y: auto; padding-right: 2px; }
.timeline-event-card { display: flex; gap: 10px; position: relative; padding: 10px 0; }
.event-gutter { display: flex; flex-direction: column; align-items: center; width: 14px; flex-shrink: 0; }
.status-indicator-dot { width: 8px; height: 8px; border-radius: 50%; margin-top: 4px; flex-shrink: 0; }
.dot-success { background: var(--color-status-online, #34d399); box-shadow: 0 0 6px rgba(52, 211, 153, 0.4); }
.dot-failed { background: var(--color-status-offline, #f43f5e); box-shadow: 0 0 6px rgba(244, 63, 94, 0.4); }
.gutter-connector { flex: 1; width: 1px; background: var(--color-border-subtle); margin-top: 4px; }
.timeline-event-card:last-child .gutter-connector { display: none; }

.event-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
  background: var(--color-surface-inset);
  padding: 8px 10px;
  border-radius: 8px;
  border: 1px solid var(--color-border-subtle);
}

.event-meta-row { display: flex; align-items: center; justify-content: space-between; gap: 6px; }
.rule-name-tag { font-size: 12px; color: var(--text-primary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.event-time { font-size: 10.5px; color: var(--text-muted); flex-shrink: 0; }
.event-flow-row { display: flex; align-items: center; gap: 6px; font-size: 11px; flex-wrap: wrap; }
.trigger-label { color: var(--color-status-warning, #fbbf24); }
.flow-arrow { color: var(--text-muted); }
.action-label { color: var(--accent-cyan, #06b6d4); font-weight: 600; }
.event-footer-row { display: flex; align-items: center; justify-content: space-between; font-size: 10px; color: var(--text-muted); margin-top: 2px; }
.duration-tag { background: var(--bg-card); padding: 1px 5px; border-radius: 4px; border: 1px solid var(--color-border-subtle); }
.footer-actions { display: flex; align-items: center; gap: 8px; }
.btn-text-action { background: none; border: none; font-size: 10.5px; font-weight: 600; color: var(--accent-cyan, #06b6d4); cursor: pointer; padding: 0; }
.btn-text-action:hover { text-decoration: underline; }

.error-detail-box {
  margin-top: 6px;
  padding: 6px 8px;
  border-radius: 6px;
  background: var(--color-status-offline-bg, rgba(244, 63, 94, 0.1));
  border: 1px solid var(--color-status-offline-border, rgba(244, 63, 94, 0.25));
}
.error-detail-title { font-size: 10px; font-weight: 700; color: var(--color-status-offline, #f43f5e); display: block; margin-bottom: 2px; }
.error-detail-text { margin: 0; font-size: 10px; color: var(--text-primary); white-space: pre-wrap; word-break: break-all; }

/* Empty state */
.timeline-empty-card {
  padding: 24px 16px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}
.empty-feed-icon { color: var(--color-status-online, #34d399); opacity: 0.9; }
.empty-feed-title { font-size: 13px; font-weight: 700; color: var(--text-primary); margin: 0; }
.empty-feed-desc { font-size: 11px; color: var(--text-muted); margin: 0; line-height: 1.4; }
.empty-cta-wrap { display: flex; flex-direction: column; align-items: center; gap: 4px; margin-top: 6px; width: 100%; }
.btn-test-active { width: 100%; justify-content: center; }
.cta-rule-preview { font-size: 10px; max-width: 240px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

/* Loading skeleton */
.timeline-loading { display: flex; flex-direction: column; gap: 8px; }
.timeline-skeleton-item { height: 60px; border-radius: 8px; background: var(--color-surface-inset); animation: pulse 1.5s ease-in-out infinite; }
@keyframes pulse { 0%, 100% { opacity: 0.6; } 50% { opacity: 0.3; } }
</style>
