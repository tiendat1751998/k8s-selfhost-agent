<template>
  <div v-if="runbooks.length === 0" class="console-empty-state">
    <BaseIcon name="book-open" size="xl" class="console-empty-icon" />
    <h3 class="console-empty-title">No Operational Runbooks Found</h3>
    <p class="console-empty-desc">Create your first incident response or disaster recovery playbook to guide on-call SREs through step-by-step triage.</p>
    <button type="button" class="btn btn-primary" @click="$emit('create')">
      <BaseIcon name="plus" size="xs" />
      <span>Create First Runbook</span>
    </button>
  </div>

  <div v-else class="runbook-playbook-console">
    <!-- Left Pane: SOP Navigator -->
    <nav class="console-navigator" aria-label="SOP Playbooks Navigation">
      <div class="navigator-header">
        <span class="navigator-title font-mono"><BaseIcon name="list" size="xs" /> SOP Navigator</span>
        <span class="navigator-count font-mono">{{ runbooks.length }}</span>
      </div>
      <div class="navigator-list" role="list">
        <button
          v-for="rb in runbooks"
          :key="rb.id"
          type="button"
          class="navigator-item"
          :class="{ 'is-active': selectedRunbook?.id === rb.id }"
          :aria-current="selectedRunbook?.id === rb.id ? 'true' : undefined"
          @click="$emit('select', rb)"
        >
          <div class="navigator-item-top font-mono">
            <span class="nav-cat-pill"><BaseIcon :name="getCategoryIcon(rb.category)" size="xs" /> {{ rb.category }}</span>
            <span class="nav-steps-badge">{{ rb.steps_count || 3 }} steps</span>
          </div>
          <h4 class="navigator-item-title">{{ rb.title }}</h4>
          <div class="navigator-item-meta font-mono">
            <span>{{ rb.author || 'Platform SRE' }}</span>
            <span>{{ rb.last_used_at ? formatDate(rb.last_used_at) : 'Not used' }}</span>
          </div>
        </button>
      </div>
    </nav>

    <!-- Right Pane: Interactive Playbook Workspace -->
    <main v-if="selectedRunbook" class="console-workspace" aria-label="Active Playbook Workspace">
      <!-- Workspace Header -->
      <div class="workspace-header">
        <div class="workspace-header-info">
          <div class="workspace-badges-row">
            <span class="workspace-cat-badge font-mono">
              <BaseIcon :name="getCategoryIcon(selectedRunbook.category)" size="xs" />
              {{ selectedRunbook.category.toUpperCase() }}
            </span>
            <span class="text-muted font-mono" style="font-size: 11px;">Author: <strong class="text-secondary">{{ selectedRunbook.author || 'Platform SRE' }}</strong></span>
            <span v-if="selectedRunbook.last_used_at" class="text-muted font-mono" style="font-size: 11px;">Last run: {{ formatDate(selectedRunbook.last_used_at) }}</span>
          </div>
          <h2 class="workspace-title">{{ selectedRunbook.title }}</h2>
          <div v-if="selectedRunbook.tags && selectedRunbook.tags.length > 0" class="workspace-tags">
            <span v-for="tag in selectedRunbook.tags" :key="tag" class="workspace-tag-pill font-mono">#{{ tag }}</span>
          </div>
        </div>

        <!-- Primary Action Group (Max 2 inline + kebab) -->
        <div class="workspace-actions">
          <button
            type="button"
            class="btn btn-primary"
            :disabled="executingId === selectedRunbook.id"
            title="Execute entire playbook"
            @click="$emit('execute', selectedRunbook)"
          >
            <BaseIcon :name="executingId === selectedRunbook.id ? 'clock' : 'zap'" size="xs" />
            <span>{{ executingId === selectedRunbook.id ? 'Running...' : 'Execute Playbook' }}</span>
          </button>
          <button
            type="button"
            class="btn btn-secondary"
            title="Edit playbook metadata and steps"
            @click="$emit('edit', selectedRunbook)"
          >
            <BaseIcon name="sliders" size="xs" />
            <span>Edit</span>
          </button>

          <!-- Kebab Menu for Secondary Actions -->
          <div class="btn-kebab-menu">
            <button
              type="button"
              class="btn btn-secondary"
              style="padding: 0 8px; min-width: 32px;"
              aria-label="More actions"
              @click="showKebab = !showKebab"
            >
              <span>⋯</span>
            </button>
            <div v-if="showKebab" class="kebab-popover" @mouseleave="showKebab = false">
              <button
                type="button"
                class="kebab-item is-danger"
                @click="showKebab = false; $emit('delete', selectedRunbook.id)"
              >
                <BaseIcon name="trash" size="xs" />
                <span>Delete Runbook</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Interactive Step Checklist -->
      <section class="workspace-steps-container" aria-label="Procedure Steps">
        <!-- Progress HUD -->
        <div class="execution-progress">
          <div class="progress-header">
            <span>SOP Procedure Progress</span>
            <span class="font-mono">{{ completedSteps.size }} / {{ steps.length }} Steps ({{ progressPercent }}%)</span>
          </div>
          <div class="progress-bar-bg" role="progressbar" :aria-valuenow="progressPercent" aria-valuemin="0" aria-valuemax="100">
            <div class="progress-bar-fill" :style="{ width: `${progressPercent}%` }"></div>
          </div>
        </div>

        <!-- Step Cards -->
        <div class="steps-list">
          <article
            v-for="(step, idx) in steps"
            :key="step.id || idx"
            class="step-card"
            :class="{ 'step-done': completedSteps.has(idx), 'step-running': step.status === 'running' }"
          >
            <div class="step-header" @click="$emit('toggleStep', idx)">
              <div
                class="step-checkbox"
                :class="{ 'step-checkbox-active': completedSteps.has(idx) }"
                role="checkbox"
                :aria-checked="completedSteps.has(idx)"
                tabindex="0"
                @keydown.enter.prevent="$emit('toggleStep', idx)"
                @keydown.space.prevent="$emit('toggleStep', idx)"
              >
                <BaseIcon v-if="completedSteps.has(idx)" name="check" size="xs" />
                <span v-else>{{ idx + 1 }}</span>
              </div>
              <span class="step-title" :class="{ 'text-strikethrough': completedSteps.has(idx) }">
                {{ step.title }}
              </span>
              <button
                v-if="step.command"
                type="button"
                class="btn btn-xs btn-primary font-mono"
                title="Execute single command"
                :disabled="step.status === 'running'"
                @click.stop="$emit('executeStep', step, idx)"
              >
                <BaseIcon :name="step.status === 'running' ? 'clock' : 'zap'" size="xs" />
                <span>{{ step.status === 'running' ? 'Running...' : 'Run Step' }}</span>
              </button>
            </div>

            <div class="step-body">
              <p v-if="step.content" class="step-desc">{{ step.content }}</p>
              <div v-if="step.command" class="command-box font-mono">
                <pre class="command-pre">{{ step.command }}</pre>
                <button
                  type="button"
                  class="copy-btn"
                  title="Copy command to clipboard"
                  @click.stop="$emit('copyCommand', step.command)"
                >
                  <BaseIcon name="copy" size="xs" /> Copy
                </button>
              </div>
            </div>
          </article>
        </div>

        <!-- Step Actions -->
        <div class="steps-action-bar">
          <button
            type="button"
            class="btn btn-secondary font-mono"
            style="font-size: 11px;"
            :disabled="completedSteps.size === steps.length || steps.length === 0"
            @click="$emit('markAllComplete')"
          >
            <BaseIcon name="check-circle" size="xs" />
            <span>Mark All Steps Complete</span>
          </button>
        </div>
      </section>

      <!-- Streaming Terminal Logs -->
      <section v-if="logs.length > 0" class="log-terminal-box" aria-label="Execution Telemetry">
        <div class="log-terminal-header font-mono">
          <span>LIVE EXECUTION TELEMETRY &amp; STREAMING LOGS</span>
          <span class="text-muted">{{ logs.length }} events</span>
        </div>
        <div class="log-terminal-body font-mono">
          <div v-for="(log, lIdx) in logs" :key="lIdx" class="log-terminal-line">
            <span v-if="log.includes('[ERROR]')" class="text-rose">{{ log }}</span>
            <span v-else-if="log.includes('[SUCCESS]') || log.includes('[OK]')" class="text-emerald">{{ log }}</span>
            <span v-else-if="log.includes('[EXEC]') || log.includes('[CMD]')" class="text-cyan">{{ log }}</span>
            <span v-else class="text-secondary">{{ log }}</span>
          </div>
        </div>
      </section>
    </main>

    <div v-else class="console-workspace text-center p-8">
      <p class="text-muted font-mono">Select a runbook from the SOP Navigator to view procedure steps.</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { Runbook } from '../../api/governance'
import type { RunbookStep } from '../../composables/useRunbooks'
import BaseIcon from '../ui/BaseIcon.vue'
import '../../assets/styles/views/runbooks-console.css'

const props = defineProps<{
  runbooks: Runbook[]
  selectedRunbook: Runbook | null
  steps: RunbookStep[]
  completedSteps: Set<number>
  logs: string[]
  executingId: string | null
  getCategoryIcon: (cat: string) => string
  formatDate: (d?: string) => string
}>()

defineEmits<{
  (e: 'select', rb: Runbook): void
  (e: 'execute', rb: Runbook): void
  (e: 'executeStep', step: RunbookStep, idx: number): void
  (e: 'toggleStep', idx: number): void
  (e: 'markAllComplete'): void
  (e: 'copyCommand', cmd?: string): void
  (e: 'edit', rb: Runbook): void
  (e: 'delete', id: string): void
  (e: 'create'): void
}>()

const showKebab = ref(false)

const progressPercent = computed(() => {
  if (!props.steps || props.steps.length === 0) return 0
  return Math.round((props.completedSteps.size / props.steps.length) * 100)
})
</script>
