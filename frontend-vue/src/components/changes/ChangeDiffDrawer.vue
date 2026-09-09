<script setup lang="ts">
import { ref, computed } from 'vue'
import type { TimelineEvent } from '../../composables/useChangesTimeline'

const props = defineProps<{
  show: boolean
  event: TimelineEvent | null
}>()

const emit = defineEmits<{
  'update:show': [val: boolean]
  rollback: [event: TimelineEvent]
}>()

const activeView = ref<'unified' | 'split'>('unified')
const copied = ref(false)

const rawDiff = computed(() => {
  return props.event?.diffPayload?.unifiedDiff || 'No unified diff generated for this state mutation.'
})

const parsedLines = computed(() => {
  const lines = rawDiff.value.split('\n')
  return lines.map((line, idx) => {
    let type = 'normal'
    if (line.startsWith('+') && !line.startsWith('+++')) type = 'addition'
    else if (line.startsWith('-') && !line.startsWith('---')) type = 'deletion'
    else if (line.startsWith('@@')) type = 'header'
    else if (line.startsWith('---') || line.startsWith('+++')) type = 'file'

    return {
      id: idx + 1,
      type,
      content: line
    }
  })
})

function close() {
  emit('update:show', false)
}

function handleCopy() {
  if (typeof navigator !== 'undefined' && navigator.clipboard) {
    navigator.clipboard.writeText(rawDiff.value)
    copied.value = true
    setTimeout(() => { copied.value = false }, 2500)
  }
}
</script>

<template>
  <div v-if="show" class="diff-drawer-backdrop" @click="close">
    <div class="diff-drawer-panel glass-panel" @click.stop>
      <!-- Drawer Header -->
      <div class="diff-drawer-header">
        <div class="dd-title-wrap">
          <div class="dd-badge-row">
            <span class="badge badge-cyan font-mono">SPEC DIFF INSPECTION</span>
            <span v-if="event?.cluster" class="badge badge-emerald font-mono">{{ event.cluster }}</span>
          </div>
          <h2 class="dd-title">{{ event?.title || 'Configuration Diff' }}</h2>
          <p class="dd-subtitle font-mono">{{ event?.resource }} &bull; {{ event?.namespace }}</p>
        </div>

        <button class="dd-close-btn" @click="close">✕</button>
      </div>

      <!-- Drawer Toolbar -->
      <div class="diff-drawer-toolbar">
        <div class="diff-view-switch">
          <button
            class="switch-btn"
            :class="{ active: activeView === 'unified' }"
            @click="activeView = 'unified'"
          >
            Unified View
          </button>
          <button
            class="switch-btn"
            :class="{ active: activeView === 'split' }"
            @click="activeView = 'split'"
          >
            Side-by-Side
          </button>
        </div>

        <div class="toolbar-actions">
          <button class="btn btn-secondary btn-sm" @click="handleCopy">
            <span>{{ copied ? '✅ Copied' : '📋 Copy Diff' }}</span>
          </button>
          <button
            v-if="event?.canRollback"
            class="btn btn-primary btn-sm"
            @click="event && $emit('rollback', event)"
          >
            <span>↺ Revert / Rollback</span>
          </button>
        </div>
      </div>

      <!-- Unified Diff Viewer -->
      <div v-if="activeView === 'unified'" class="diff-code-viewer font-mono">
        <div class="diff-code-header">
          <span>Target Resource Manifest (GitOps vs Live)</span>
          <span class="badge badge-cyan">{{ parsedLines.length }} lines</span>
        </div>
        <div class="diff-lines-container">
          <div
            v-for="l in parsedLines"
            :key="l.id"
            class="diff-line-row"
            :class="`line-${l.type}`"
          >
            <span class="diff-ln">{{ l.id }}</span>
            <span class="diff-marker">
              {{ l.type === 'addition' ? '+' : l.type === 'deletion' ? '-' : ' ' }}
            </span>
            <span class="diff-text">{{ l.content }}</span>
          </div>
        </div>
      </div>

      <!-- Split Diff Viewer -->
      <div v-else class="split-diff-grid font-mono">
        <div class="split-pane">
          <div class="pane-header text-amber">
            <span>BASELINE / LIVE ETCD</span>
            <span>Rev #1</span>
          </div>
          <div class="pane-content">
            <pre class="diff-pre-pane">{{ event?.diffPayload?.actualState || rawDiff }}</pre>
          </div>
        </div>
        <div class="split-pane">
          <div class="pane-header text-emerald">
            <span>PROPOSED / GIT HEAD</span>
            <span>Rev #2</span>
          </div>
          <div class="pane-content">
            <pre class="diff-pre-pane">{{ event?.diffPayload?.expectedState || rawDiff }}</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
