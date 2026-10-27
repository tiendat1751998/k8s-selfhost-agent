<script setup lang="ts">
import { ref } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import BaseIcon from '../ui/BaseIcon.vue'
import StatusBadge from '../ui/StatusBadge.vue'
import type { ComplianceViolation } from '../../api/governance'

const props = defineProps<{
  show: boolean
  violation: ComplianceViolation | null
  summary?: string
  commands?: string[]
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'remediated'): void
}>()

const copiedIdx = ref<number | null>(null)

function copyCommand(cmd: string, idx: number) {
  navigator.clipboard.writeText(cmd)
  copiedIdx.value = idx
  setTimeout(() => {
    if (copiedIdx.value === idx) copiedIdx.value = null
  }, 2000)
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="drawer"
    title="Manual Action Required"
    :subtitle="violation?.resource ? `Remediation instructions for ${violation.resource}` : 'Remediation Runbook'"
    max-width="580px"
    @update:show="(val) => emit('update:show', val)"
  >
    <div class="remediation-drawer-content">
      <!-- Violation Overview Meta -->
      <div v-if="violation" class="violation-meta-box glass-panel">
        <div class="meta-row">
          <span class="meta-lbl font-mono text-muted">Policy Control:</span>
          <span class="meta-val font-mono font-bold">{{ violation.policy }}</span>
        </div>
        <div class="meta-row">
          <span class="meta-lbl font-mono text-muted">Target Resource:</span>
          <span class="meta-val font-mono text-cyan">{{ violation.resource }}</span>
        </div>
        <div class="meta-row">
          <span class="meta-lbl font-mono text-muted">Severity:</span>
          <StatusBadge :status="violation.severity" size="sm" />
        </div>
      </div>

      <!-- Remediation Summary Alert -->
      <div class="remediation-summary-box">
        <div class="summary-header">
          <BaseIcon name="alert-triangle" size="xs" class="text-amber" />
          <h4 class="summary-title font-mono">Remediation Guidance</h4>
        </div>
        <p class="summary-text">{{ summary || 'Execute the following commands to safely remediate this policy violation.' }}</p>
      </div>

      <!-- Actionable Commands List -->
      <div v-if="commands && commands.length > 0" class="commands-section">
        <h5 class="commands-sec-title font-mono text-muted">
          <BaseIcon name="terminal" size="xs" /> Remediate Commands ({{ commands.length }})
        </h5>
        <div class="commands-list">
          <div v-for="(cmd, idx) in commands" :key="idx" class="command-card font-mono">
            <div class="command-meta">
              <span class="cmd-num">Step {{ idx + 1 }}</span>
              <button
                type="button"
                class="copy-btn btn btn-xs btn-secondary"
                :title="copiedIdx === idx ? 'Copied!' : 'Copy command'"
                @click="copyCommand(cmd, idx)"
              >
                <BaseIcon :name="copiedIdx === idx ? 'check' : 'copy'" size="xs" />
                <span>{{ copiedIdx === idx ? 'Copied' : 'Copy' }}</span>
              </button>
            </div>
            <pre class="cmd-pre">{{ cmd }}</pre>
          </div>
        </div>
      </div>

      <div v-else class="empty-commands text-muted font-mono">
        No automated terminal commands specified. Please refer to organization compliance guidelines.
      </div>
    </div>

    <template #footer="{ close }">
      <div class="drawer-footer-actions">
        <button type="button" class="btn btn-secondary" @click="close">Dismiss</button>
        <button
          type="button"
          class="btn btn-primary"
          @click="emit('remediated'); close()"
        >
          <BaseIcon name="check" size="xs" />
          <span>Mark as Remediated</span>
        </button>
      </div>
    </template>
  </ModalDrawer>
</template>

<style scoped>
.remediation-drawer-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 4px 0;
}

.violation-meta-box {
  padding: 12px 14px;
  border-radius: var(--radius-sm, 6px);
  display: flex;
  flex-direction: column;
  gap: 8px;
  border: 1px solid var(--color-border-subtle);
  background: var(--color-surface-inset);
}

.meta-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 12px;
}

.remediation-summary-box {
  padding: 12px 14px;
  border-radius: var(--radius-sm, 6px);
  background: var(--color-warning-subtle, rgba(245, 158, 11, 0.12));
  border: 1px solid var(--color-warning, #f59e0b);
}

.summary-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.summary-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--color-warning, #f59e0b);
  margin: 0;
}

.summary-text {
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-primary);
  margin: 0;
}

.commands-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.commands-sec-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin: 0;
}

.commands-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.command-card {
  background: var(--color-surface-inset);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-sm, 6px);
  overflow: hidden;
}

.command-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  background: var(--color-surface-elevated);
  border-bottom: 1px solid var(--color-border-subtle);
  font-size: 11px;
}

.cmd-num {
  font-weight: 600;
  color: var(--text-muted);
}

.cmd-pre {
  margin: 0;
  padding: 10px 12px;
  font-size: 12px;
  line-height: 1.4;
  color: var(--color-badge-agent-text, var(--color-primary));
  white-space: pre-wrap;
  word-break: break-all;
  overflow-x: auto;
}

.empty-commands {
  font-size: 12px;
  padding: 16px;
  text-align: center;
  background: var(--color-surface-inset);
  border-radius: var(--radius-sm, 6px);
}

.drawer-footer-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  width: 100%;
}
</style>
