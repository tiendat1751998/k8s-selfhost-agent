<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import type { EnrichedDriftRecord } from '../../composables/useDriftDetection'

interface Props {
  drift: EnrichedDriftRecord | null
  resolving: boolean
  diffMode: 'split' | 'unified'
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'reconcile', id: string): void
  (e: 'suppress', id: string): void
  (e: 'update:diffMode', mode: 'split' | 'unified'): void
}>()

function formatDriftStatus(s: string): string {
  if (s === 'drifted') return 'DRIFTED'
  if (s === 'in_sync') return 'IN SYNC'
  return (s || 'UNKNOWN').toUpperCase()
}

function formatDate(d: string): string {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleString()
  } catch {
    return d
  }
}

function getDiffLineClass(line: string): string {
  if (line.startsWith('+')) return 'diff-line-add'
  if (line.startsWith('-')) return 'diff-line-del'
  if (line.startsWith('@@')) return 'diff-line-hunk'
  return ''
}
</script>

<template>
  <div v-if="drift" class="modal-overlay" @click.self="emit('close')">
    <div class="modal-card modal-card-wide glass-panel animate-fade-in">
      <!-- Drawer Header -->
      <div class="modal-header">
        <div class="modal-title-group">
          <div class="modal-badge-row">
            <StatusBadge :status="drift.status" :label="formatDriftStatus(drift.status)" />
            <span 
              v-if="drift.status === 'drifted' && !drift.isSuppressed" 
              class="severity-tag" 
              :class="`severity-${drift.severity}`"
            >
              {{ drift.severity.toUpperCase() }}
            </span>
            <span v-if="drift.isSuppressed" class="severity-tag severity-suppressed">
              SUPPRESSED
            </span>
            <span class="font-mono text-muted" style="font-size: 11px;">ID: #{{ drift.id.slice(0, 8) }}</span>
          </div>
          <h3 class="modal-title">{{ drift.resource_kind }}: {{ drift.resource }}</h3>
        </div>

        <div class="modal-header-actions">
          <!-- Diff Mode Switcher -->
          <div class="diff-mode-toggle">
            <button
              class="diff-mode-btn"
              :class="{ active: diffMode === 'split' }"
              @click="emit('update:diffMode', 'split')"
            >
              ◫ Split
            </button>
            <button
              class="diff-mode-btn"
              :class="{ active: diffMode === 'unified' }"
              @click="emit('update:diffMode', 'unified')"
            >
              ☰ Unified
            </button>
          </div>
          <button class="modal-close" aria-label="Close drawer" @click="emit('close')">✕</button>
        </div>
      </div>

      <!-- Drawer Body -->
      <div class="modal-body">
        <!-- Metadata Strip -->
        <div class="meta-strip font-mono">
          <span>Namespace: <strong class="text-cyan">{{ drift.namespace || 'default' }}</strong></span>
          <span>Cluster: <strong>{{ drift.cluster || 'primary' }}</strong></span>
          <span>Mutation: <strong class="text-amber">{{ drift.driftType }}</strong></span>
          <span>Detected: <strong>{{ formatDate(drift.detected_at) }}</strong></span>
        </div>

        <!-- Unified Diff View -->
        <div v-if="diffMode === 'unified'" class="diff-block">
          <div class="diff-block-header">
            <span class="font-mono">UNIFIED CRYPTOGRAPHIC DIFF (GIT MANIFEST ↔ LIVE ETCD RUNTIME)</span>
          </div>
          <div class="diff-pre-container font-mono">
            <template v-if="drift.diff">
              <div 
                v-for="(line, idx) in drift.diff.split('\n')" 
                :key="idx" 
                class="diff-code-line"
                :class="getDiffLineClass(line)"
              >
                <span class="diff-line-number">{{ idx + 1 }}</span>
                <span class="diff-line-text">{{ line }}</span>
              </div>
            </template>
            <div v-else class="diff-code-line text-muted">
              # No mutations detected in unified diff
            </div>
          </div>
        </div>

        <!-- Split Diff View -->
        <div v-else class="diff-split-grid">
          <div class="diff-pane">
            <div class="diff-pane-header text-emerald">
              <span>📄 Expected State (Git Repository Manifest)</span>
            </div>
            <pre class="code-pre font-mono">{{ drift.expected_state || '# No Git Manifest Available' }}</pre>
          </div>

          <div class="diff-pane">
            <div class="diff-pane-header text-amber">
              <span>⚡ Actual State (Live Cluster Runtime)</span>
            </div>
            <pre class="code-pre font-mono">{{ drift.actual_state || '# No Live State Available' }}</pre>
          </div>
        </div>
      </div>

      <!-- Drawer Footer -->
      <div class="modal-footer">
        <div class="drawer-footer-secondary">
          <button 
            class="btn btn-secondary btn-sm"
            @click="emit('suppress', drift.id)"
          >
            <span>{{ drift.isSuppressed ? '🛡️ Remove Suppression' : '👁️ Suppress Alert' }}</span>
          </button>
        </div>
        <div class="drawer-footer-primary">
          <button 
            v-if="drift.status === 'drifted'" 
            class="btn btn-primary"
            :disabled="resolving"
            @click="emit('reconcile', drift.id)"
          >
            <span>{{ resolving ? 'Reconciling...' : '⚡ Apply Git Manifest & Sync to Git' }}</span>
          </button>
          <button class="btn btn-secondary" @click="emit('close')">Close</button>
        </div>
      </div>
    </div>
  </div>
</template>