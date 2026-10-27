<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'

defineProps<{
  searchQuery: string
  autoScroll: boolean
  loading: boolean
  isK8s: boolean
  availablePods: string[]
  selectedPod: string
  availableContainers: string[]
  selectedContainer: string
}>()

const emit = defineEmits<{
  (e: 'update:searchQuery', val: string): void
  (e: 'update:autoScroll', val: boolean): void
  (e: 'update:selectedPod', val: string): void
  (e: 'update:selectedContainer', val: string): void
  (e: 'refresh'): void
  (e: 'copy'): void
  (e: 'fullStream'): void
}>()
</script>

<template>
  <div class="logs-control-deck glass-panel">
    <div class="logs-search-wrapper">
      <BaseIcon name="search" size="xs" class="logs-search-ico" />
      <input
        :value="searchQuery"
        type="text"
        placeholder="Filter logs by keyword, error, timestamp..."
        class="input-glass logs-search-input font-mono"
        @input="emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
      />
      <button
        v-if="searchQuery"
        type="button"
        class="logs-search-clear"
        aria-label="Clear search"
        @click="emit('update:searchQuery', '')"
      >
        <BaseIcon name="x" size="xs" />
      </button>
    </div>

    <!-- Pod Selector (Kubernetes Workloads) -->
    <div v-if="isK8s && availablePods.length > 0" class="logs-selector-group">
      <select
        :value="selectedPod"
        class="input-glass logs-select font-mono"
        aria-label="Select Pod"
        @change="emit('update:selectedPod', ($event.target as HTMLSelectElement).value)"
      >
        <option value="">All Pods (Aggregated)</option>
        <option v-for="pod in availablePods" :key="pod" :value="pod">
          {{ pod }}
        </option>
      </select>
    </div>

    <!-- Container Selector (Multi-container workloads) -->
    <div v-if="availableContainers.length > 1" class="logs-selector-group">
      <select
        :value="selectedContainer"
        class="input-glass logs-select font-mono"
        aria-label="Select Container"
        @change="emit('update:selectedContainer', ($event.target as HTMLSelectElement).value)"
      >
        <option value="">All Containers</option>
        <option v-for="c in availableContainers" :key="c" :value="c">
          {{ c }}
        </option>
      </select>
    </div>

    <div class="logs-deck-actions">
      <label class="logs-auto-scroll-label font-mono">
        <input
          :checked="autoScroll"
          type="checkbox"
          class="toggle-cb"
          @change="emit('update:autoScroll', ($event.target as HTMLInputElement).checked)"
        />
        <span>Auto-Scroll</span>
      </label>

      <button
        type="button"
        class="btn btn-secondary btn-xs"
        :disabled="loading"
        title="Refresh logs from container daemon"
        @click="emit('refresh')"
      >
        <BaseIcon name="refresh" size="xs" :class="{ 'spin-icon': loading }" />
        <span>{{ loading ? 'Fetching...' : 'Refresh' }}</span>
      </button>

      <button
        type="button"
        class="btn btn-secondary btn-xs"
        title="Copy log buffer to clipboard"
        @click="emit('copy')"
      >
        <BaseIcon name="copy" size="xs" />
        <span>Copy</span>
      </button>

      <button
        type="button"
        class="btn btn-secondary btn-xs btn-open-logs"
        title="Open full interactive live tail in Log Stream View"
        @click="emit('fullStream')"
      >
        <BaseIcon name="external-link" size="xs" />
        <span>Full Stream</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.logs-selector-group {
  display: flex;
  align-items: center;
}

.logs-select {
  height: 28px;
  font-size: 11px;
  padding: 2px 8px;
  background: var(--color-surface-inset);
  border: 1px solid var(--color-border-subtle);
  border-radius: 6px;
  color: var(--color-body);
  max-width: 170px;
  cursor: pointer;
}

.logs-select:focus {
  outline: none;
  border-color: var(--color-primary);
}

@media (max-width: 640px) {
  .logs-control-deck {
    flex-wrap: wrap;
    gap: 8px;
  }
  .logs-search-wrapper {
    min-width: 100%;
  }
  .logs-selector-group {
    flex: 1 1 auto;
  }
  .logs-select {
    width: 100%;
    max-width: none;
  }
  .logs-deck-actions {
    width: 100%;
    justify-content: flex-end;
    flex-wrap: wrap;
    gap: 6px;
  }
}
</style>
