<script setup lang="ts">
import type { AlertRule } from '../../api/management'
import BaseIcon from '../ui/BaseIcon.vue'

defineProps<{ rules: AlertRule[]; loading?: boolean }>()
const emit = defineEmits<{
  (e: 'edit', rule: AlertRule): void
  (e: 'delete', id: string): void
  (e: 'toggle', rule: AlertRule): void
}>()
</script>

<template>
  <div class="alert-rules-mobile-stream mobile-only">
    <div v-if="rules.length === 0" class="mobile-empty-rules glass-panel">
      <BaseIcon name="sliders" size="sm" />
      <span>No alert rules defined. Create a rule to monitor cluster metrics.</span>
    </div>

    <div v-else v-for="rule in rules" :key="rule.ID" class="mobile-rule-card glass-panel animate-fade-in">
      <div class="mobile-rule-header">
        <div class="rule-title-group">
          <span class="rule-title font-bold" style="color: var(--text-primary);">{{ rule.Name }}</span>
          <span class="badge font-mono font-bold" :class="rule.Severity === 'critical' ? 'badge-rose' : rule.Severity === 'high' ? 'badge-amber' : 'badge-cyan'" style="font-size: 10px;">
            {{ String(rule.Severity || 'INFO').toUpperCase() }}
          </span>
        </div>
        <button type="button" class="btn-rule-state" :class="rule.Enabled ? 'state-active' : 'state-disabled'" @click="emit('toggle', rule)">
          {{ rule.Enabled ? 'ARMED' : 'PAUSED' }}
        </button>
      </div>

      <div class="mobile-rule-body font-mono text-xs">
        <div class="metric-row truncate"><span class="text-muted">Metric: </span><span class="text-cyan">{{ rule.MetricName }}</span></div>
        <div class="condition-row"><span class="text-muted">Condition: </span><span class="text-amber font-bold">{{ rule.Condition }} {{ rule.Threshold }} ({{ rule.DurationSeconds }}s)</span></div>
        <div v-if="rule.Description" class="desc-row text-muted font-sans truncate" style="font-size: 11px;">{{ rule.Description }}</div>
      </div>

      <div class="mobile-rule-actions">
        <button type="button" class="btn-card-action" title="Edit Rule" @click="emit('edit', rule)">
          <BaseIcon name="edit" size="xs" /><span>Edit</span>
        </button>
        <button type="button" class="btn-card-action btn-card-delete" title="Delete Rule" @click="emit('delete', String(rule.ID))">
          <BaseIcon name="trash" size="xs" /><span>Delete</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.alert-rules-mobile-stream { display: flex; flex-direction: column; gap: 8px; width: 100%; }
.mobile-empty-rules { padding: 24px 16px; text-align: center; color: var(--text-muted); font-size: 12px; display: flex; align-items: center; justify-content: center; gap: 8px; border-radius: 8px; }
.mobile-rule-card { padding: 10px 12px; border-radius: 8px; background: var(--bg-card); border: 1px solid var(--color-border-subtle); display: flex; flex-direction: column; gap: 8px; }
.mobile-rule-header { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.rule-title-group { display: flex; align-items: center; gap: 6px; min-width: 0; flex: 1; }
.rule-title { font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.btn-rule-state { font-size: 10px; font-weight: 700; font-family: var(--font-mono, monospace); padding: 2px 6px; border-radius: 4px; cursor: pointer; border: 1px solid transparent; line-height: 1.2; }
.btn-rule-state.state-active { background: rgba(16, 185, 129, 0.15); border-color: rgba(16, 185, 129, 0.35); color: #34d399; }
.btn-rule-state.state-disabled { background: var(--color-surface-inset); border-color: var(--color-border-subtle); color: var(--text-muted); }
.mobile-rule-body { display: flex; flex-direction: column; gap: 3px; }
.mobile-rule-actions { display: flex; align-items: center; justify-content: flex-end; gap: 8px; padding-top: 6px; border-top: 1px solid var(--color-border-subtle); }
.btn-card-action { display: inline-flex; align-items: center; gap: 4px; padding: 4px 8px; font-size: 11px; font-weight: 600; border-radius: 6px; background: var(--color-surface-inset); border: 1px solid var(--color-border-subtle); color: var(--text-secondary); cursor: pointer; }
.btn-card-action:hover { background: var(--bg-card-hover); color: var(--text-primary); }
.btn-card-delete { color: var(--color-status-offline, #fb7185); border-color: var(--color-status-offline-border, rgba(244, 63, 94, 0.25)); background: var(--color-status-offline-bg, rgba(244, 63, 94, 0.08)); }
@media (min-width: 1024px) { .mobile-only, .alert-rules-mobile-stream { display: none !important; } }
@media (max-width: 1023.98px) { .alert-rules-mobile-stream { display: flex !important; } }
</style>
