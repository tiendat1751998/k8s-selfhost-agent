<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  searchQuery: string
  selectedLevel: string
  selectedNamespace: string
  selectedPod: string
  availableNamespaces: string[]
  isPaused: boolean
  autoScroll: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:searchQuery', val: string): void
  (e: 'update:selectedLevel', val: string): void
  (e: 'update:selectedNamespace', val: string): void
  (e: 'update:selectedPod', val: string): void
  (e: 'update:autoScroll', val: boolean): void
  (e: 'namespaceChange'): void
  (e: 'togglePause'): void
  (e: 'clearBuffer'): void
  (e: 'exportLogs'): void
}>()

const levels = [
  { label: 'ALL', value: '' },
  { label: 'ERROR', value: 'ERROR', class: 'pill-error' },
  { label: 'WARN', value: 'WARN', class: 'pill-warn' },
  { label: 'INFO', value: 'INFO', class: 'pill-info' },
  { label: 'DEBUG', value: 'DEBUG', class: 'pill-debug' },
]

const isRegexDetected = computed(() => {
  if (!props.searchQuery) return false
  return /[[\]{}()*+?^$\\.|]/.test(props.searchQuery)
})

function onSearchInput(e: Event) {
  emit('update:searchQuery', (e.target as HTMLInputElement).value)
}

function onPodInput(e: Event) {
  emit('update:selectedPod', (e.target as HTMLInputElement).value)
}

function onNamespaceSelect(e: Event) {
  emit('update:selectedNamespace', (e.target as HTMLSelectElement).value)
  emit('namespaceChange')
}

function selectLevel(levelValue: string) {
  emit('update:selectedLevel', levelValue)
}
</script>

<template>
  <div class="control-deck glass-panel" role="toolbar" aria-label="Log Stream Filtering and Controls">
    <div class="deck-row">
      <!-- Search Input with Regex indicator -->
      <div class="search-input-wrapper">
        <span class="search-icon" aria-hidden="true">🔍</span>
        <input
          :value="searchQuery"
          type="text"
          placeholder="Search regex, error codes, trace IDs, panic, OOM..."
          class="input-glass search-input font-mono"
          aria-label="Search logs"
          @input="onSearchInput"
        />
        <span
          v-if="isRegexDetected"
          class="badge badge-info text-cyan font-mono"
          style="position: absolute; right: 32px; font-size: 9px; padding: 1px 4px;"
          title="Regex query detected"
        >
          REGEX
        </span>
        <button
          v-if="searchQuery"
          class="search-clear"
          type="button"
          aria-label="Clear search"
          @click="emit('update:searchQuery', '')"
        >
          ✕
        </button>
      </div>

      <!-- Level Selector Pills -->
      <div class="level-pills" role="radiogroup" aria-label="Filter by log level">
        <button
          v-for="lvl in levels"
          :key="lvl.label"
          type="button"
          class="level-pill"
          :class="[lvl.class, { active: selectedLevel === lvl.value }]"
          :aria-checked="selectedLevel === lvl.value"
          role="radio"
          @click="selectLevel(lvl.value)"
        >
          {{ lvl.label }}
        </button>
      </div>

      <!-- Namespace Selector -->
      <div class="deck-select-group">
        <label class="deck-label" for="log-ns-select">Namespace:</label>
        <select
          id="log-ns-select"
          :value="selectedNamespace"
          class="input-glass deck-select"
          @change="onNamespaceSelect"
        >
          <option value="">ALL NAMESPACES</option>
          <option v-for="ns in availableNamespaces" :key="ns" :value="ns">
            {{ ns }}
          </option>
        </select>
      </div>

      <!-- Pod / Container Selector -->
      <div class="deck-select-group">
        <label class="deck-label" for="log-pod-input">Pod / Container:</label>
        <input
          id="log-pod-input"
          :value="selectedPod"
          type="text"
          placeholder="Filter pod..."
          class="input-glass search-input font-mono"
          style="width: 140px; height: 36px; padding: 6px 10px;"
          @input="onPodInput"
        />
      </div>

      <!-- Auto-scroll lock toggle -->
      <label class="auto-scroll-label">
        <input
          :checked="autoScroll"
          type="checkbox"
          class="toggle-cb"
          @change="emit('update:autoScroll', ($event.target as HTMLInputElement).checked)"
        />
        <span>Auto-Scroll</span>
      </label>

      <!-- Action Buttons -->
      <div class="header-actions" style="margin-left: auto;">
        <button
          type="button"
          class="btn"
          :class="isPaused ? 'btn-primary' : 'btn-secondary'"
          @click="emit('togglePause')"
        >
          <span>{{ isPaused ? '▶ Resume Live Tail' : '⏸ Pause Live Stream' }}</span>
        </button>
        <button
          type="button"
          class="btn btn-secondary"
          title="Clear current log buffer"
          @click="emit('clearBuffer')"
        >
          <span>🧹 Clear</span>
        </button>
        <button
          type="button"
          class="btn btn-secondary"
          title="Export current logs as raw text file"
          @click="emit('exportLogs')"
        >
          <span>📥 Export</span>
        </button>
      </div>
    </div>
  </div>
</template>
