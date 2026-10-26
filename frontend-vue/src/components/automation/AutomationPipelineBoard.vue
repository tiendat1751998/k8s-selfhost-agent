<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'
import ActionDropdown, { type ActionItem } from '../ui/ActionDropdown.vue'
import type { AutomationRule } from '../../api/governance'
import {
  extractPipelineStages,
  formatRelativeTime,
} from '../../composables/useAutomationPipeline'

defineProps<{
  rules: AutomationRule[]
  loading: boolean
  error?: string | null
  togglingId: string | null
  triggeringId: string | null
  deletingId: string | null
}>()

const emit = defineEmits<{
  (e: 'toggle', id: string, nextState: boolean): void
  (e: 'trigger', rule: AutomationRule): void
  (e: 'edit', rule: AutomationRule): void
  (e: 'delete', id: string): void
  (e: 'create'): void
}>()

const menuActions: ActionItem[] = [
  { id: 'delete', label: 'Delete Rule', icon: 'trash', variant: 'danger', isDestructive: true },
]

function handleMenuSelect(actionId: string, rule: AutomationRule) {
  if (actionId === 'delete') {
    emit('delete', rule.id)
  }
}
</script>

<template>
  <div class="pipeline-board-container">
    <div class="board-header">
      <div class="board-title-group">
        <h2 class="board-heading">Event-Driven Automation Pipelines</h2>
        <span class="board-subheading">Autonomous self-healing triggers reacting to real-time cluster telemetry</span>
      </div>
      <span class="pipeline-count-pill font-mono">{{ rules.length }} Configured</span>
    </div>

    <!-- Loading Skeleton State -->
    <div v-if="loading && rules.length === 0" class="pipeline-loading-skeleton" role="status">
      <div v-for="i in 3" :key="i" class="skeleton-pipeline-card"></div>
    </div>

    <!-- Empty State -->
    <div v-else-if="rules.length === 0" class="pipeline-empty-card glass-panel font-mono">
      <div class="empty-icon-wrap"><BaseIcon name="git-commit" size="lg" /></div>
      <h3 class="empty-title">No Automation Pipelines Configured</h3>
      <p class="empty-description">Create an event pipeline to automatically remediate CrashLoops, high load, or node pressure.</p>
      <button class="btn btn-primary btn-sm" @click="emit('create')">
        <BaseIcon name="plus" size="xs" />
        <span>Create First Pipeline</span>
      </button>
    </div>

    <!-- Active Pipeline Cards List -->
    <div v-else class="pipeline-cards-list">
      <article
        v-for="rule in rules"
        :key="rule.id"
        class="pipeline-card glass-panel"
        :class="{ 'rule-disabled': !rule.enabled }"
      >
        <!-- Card Header: Switch + Title + Short-Hash + KPIs + Max 2 Inline Actions + [⋯] -->
        <header class="pipeline-card-header">
          <div class="card-identity-group">
            <label class="toggle-switch" :title="rule.enabled ? 'Disarm Rule' : 'Arm Rule'">
              <input
                type="checkbox"
                :checked="rule.enabled"
                :disabled="togglingId === rule.id"
                :aria-label="`Toggle ${rule.name}`"
                @change="emit('toggle', rule.id, !rule.enabled)"
              />
              <span class="slider"></span>
            </label>

            <div class="identity-text">
              <div class="name-hash-row">
                <span class="rule-title" :title="rule.name">{{ rule.name }}</span>
                <span class="rule-hash font-mono" :title="rule.id">#{{ rule.id.slice(0, 8) }}</span>
              </div>
              <div class="telemetry-meta font-mono">
                <span class="meta-runs text-emerald">{{ rule.executions || 0 }} runs</span>
                <span class="meta-sep">·</span>
                <span class="meta-last">Last: {{ formatRelativeTime(rule.last_triggered) }}</span>
              </div>
            </div>
          </div>

          <!-- Actions Group: Max 2 Inline (Test Fire, Edit) + [⋯] Menu (Delete) -->
          <div class="card-actions-group">
            <button
              class="btn-icon-cmd btn-test-fire"
              :disabled="triggeringId === rule.id"
              :title="triggeringId === rule.id ? 'Test firing pipeline...' : 'Test Fire Pipeline'"
              :aria-label="`Test Fire ${rule.name}`"
              @click="emit('trigger', rule)"
            >
              <BaseIcon :name="triggeringId === rule.id ? 'clock' : 'zap'" size="xs" :class="{ 'spin-anim': triggeringId === rule.id }" />
              <span class="btn-action-label desktop-only">Test Fire</span>
            </button>

            <button
              class="btn-icon-cmd btn-edit-rule"
              title="Edit Rule Configuration"
              :aria-label="`Edit ${rule.name}`"
              @click="emit('edit', rule)"
            >
              <BaseIcon name="sliders" size="xs" />
              <span class="btn-action-label desktop-only">Edit</span>
            </button>

            <ActionDropdown
              :items="menuActions"
              size="xs"
              trigger-title="Rule options"
              :aria-label="`More options for ${rule.name}`"
              @select="handleMenuSelect($event, rule)"
            />
          </div>
        </header>

        <!-- Visual 3-Stage Connected Flow Strip: [WHEN] ──▶ [IF] ──▶ [THEN] -->
        <section class="pipeline-flow-strip" :aria-label="`Pipeline stages for ${rule.name}`">
          <!-- Stage 1: WHEN Trigger -->
          <div class="stage-node stage-when">
            <div class="stage-badge font-mono">
              <BaseIcon :name="extractPipelineStages(rule).when.icon" size="xs" />
              <span>WHEN</span>
            </div>
            <div class="stage-body font-mono">
              <span class="stage-subtext font-semibold" :title="extractPipelineStages(rule).when.subtext">
                {{ extractPipelineStages(rule).when.subtext }}
              </span>
              <span class="stage-chip">{{ extractPipelineStages(rule).when.chip }}</span>
            </div>
          </div>

          <div class="stage-connector" aria-hidden="true"><span class="connector-symbol">→</span></div>

          <!-- Stage 2: IF Gate / Condition -->
          <div class="stage-node stage-if">
            <div class="stage-badge font-mono">
              <BaseIcon :name="extractPipelineStages(rule).ifGate.icon" size="xs" />
              <span>IF</span>
            </div>
            <div class="stage-body font-mono">
              <span class="stage-subtext" :title="extractPipelineStages(rule).ifGate.subtext">
                {{ extractPipelineStages(rule).ifGate.subtext }}
              </span>
              <span class="stage-chip">{{ extractPipelineStages(rule).ifGate.chip }}</span>
            </div>
          </div>

          <div class="stage-connector" aria-hidden="true"><span class="connector-symbol">→</span></div>

          <!-- Stage 3: THEN Automated Remediation Action -->
          <div class="stage-node stage-then">
            <div class="stage-badge font-mono">
              <BaseIcon :name="extractPipelineStages(rule).thenAction.icon" size="xs" />
              <span>THEN</span>
            </div>
            <div class="stage-body font-mono">
              <span class="stage-subtext font-semibold text-cyan" :title="extractPipelineStages(rule).thenAction.subtext">
                {{ extractPipelineStages(rule).thenAction.subtext }}
              </span>
              <span class="stage-chip">{{ extractPipelineStages(rule).thenAction.chip }}</span>
            </div>
          </div>
        </section>
      </article>
    </div>
  </div>
</template>

<style scoped>
.pipeline-board-container { display: flex; flex-direction: column; gap: 14px; width: 100%; }
.board-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.board-heading { font-size: 15px; font-weight: 700; color: var(--text-primary); margin: 0; }
.board-subheading { font-size: 12px; color: var(--text-muted); display: block; }
.pipeline-count-pill { font-size: 11px; padding: 3px 8px; border-radius: 6px; background: var(--color-surface-inset); border: 1px solid var(--color-border-subtle); color: var(--text-secondary); }
.pipeline-cards-list { display: flex; flex-direction: column; gap: 12px; }
.pipeline-card { padding: 14px 16px; border-radius: 10px; background: var(--bg-card); border: 1px solid var(--color-border-subtle); display: flex; flex-direction: column; gap: 12px; transition: border-color 0.15s ease; }
.pipeline-card:hover { border-color: var(--color-border-strong); }
.pipeline-card.rule-disabled { opacity: 0.65; }
.pipeline-card-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.card-identity-group { display: flex; align-items: center; gap: 10px; min-width: 0; flex: 1; }
.name-hash-row { display: flex; align-items: center; gap: 8px; min-width: 0; }
.rule-title { font-size: 13.5px; font-weight: 700; color: var(--text-primary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.rule-hash { font-size: 11px; color: var(--text-muted); padding: 1px 5px; background: var(--color-surface-inset); border-radius: 4px; }
.telemetry-meta { display: flex; align-items: center; gap: 6px; font-size: 11px; margin-top: 2px; }
.meta-runs { font-weight: 600; }
.meta-sep, .meta-last { color: var(--text-muted); }
.card-actions-group { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }
.btn-test-fire, .btn-edit-rule { width: auto; padding: 0 8px; gap: 5px; }
.btn-action-label { font-size: 11.5px; font-weight: 600; }
.pipeline-flow-strip { display: flex; align-items: stretch; gap: 6px; background: var(--color-surface-inset); border: 1px solid var(--color-border-subtle); border-radius: 8px; padding: 8px 10px; overflow-x: auto; }
.stage-node { flex: 1; min-width: 140px; display: flex; flex-direction: column; gap: 4px; }
.stage-badge { display: inline-flex; align-items: center; gap: 4px; font-size: 10px; font-weight: 700; letter-spacing: 0.05em; color: var(--text-muted); }
.stage-when .stage-badge { color: var(--color-status-warning, #fbbf24); }
.stage-if .stage-badge { color: var(--accent-cyan, #06b6d4); }
.stage-then .stage-badge { color: var(--color-status-online, #34d399); }
.stage-body { display: flex; flex-direction: column; gap: 3px; font-size: 11px; }
.stage-subtext { color: var(--text-primary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-size: 11.5px; }
.stage-chip { display: inline-block; font-size: 10px; color: var(--text-muted); background: var(--bg-card); border: 1px solid var(--color-border-subtle); padding: 1px 5px; border-radius: 4px; width: fit-content; max-width: 100%; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.stage-connector { display: flex; align-items: center; justify-content: center; padding: 0 4px; color: var(--text-muted); font-weight: 700; user-select: none; }
.connector-symbol { font-size: 14px; opacity: 0.6; }
.pipeline-empty-card { padding: 32px 20px; text-align: center; border-radius: 10px; background: var(--bg-card); border: 1px dashed var(--color-border-subtle); display: flex; flex-direction: column; align-items: center; gap: 10px; }
.empty-icon-wrap { color: var(--accent-cyan, #06b6d4); opacity: 0.8; }
.empty-title { font-size: 14px; font-weight: 700; color: var(--text-primary); margin: 0; }
.empty-description { font-size: 12px; color: var(--text-muted); max-width: 380px; margin: 0; }
.pipeline-loading-skeleton { display: flex; flex-direction: column; gap: 12px; }
.skeleton-pipeline-card { height: 110px; border-radius: 10px; background: var(--color-surface-inset); animation: pulse 1.5s ease-in-out infinite; }
@keyframes pulse { 0%, 100% { opacity: 0.6; } 50% { opacity: 0.3; } }
@media (max-width: 767.98px) {
  .pipeline-flow-strip { flex-direction: column; gap: 8px; }
  .stage-connector { transform: rotate(90deg); padding: 2px 0; }
}
</style>
