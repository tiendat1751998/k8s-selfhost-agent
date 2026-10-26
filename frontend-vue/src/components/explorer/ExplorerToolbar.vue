<script setup lang="ts">
import { computed } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import ActionDropdown, { type ActionItem } from '../ui/ActionDropdown.vue'
import type { ResourceKind } from '../../api/k8s'

defineProps<{
  currentKindLabel: string
  totalInKind: number
  loading: boolean
  showMobileSearch: boolean
  searchQuery: string
  selectedCluster: string
  selectedNamespace: string
  selectedKind: ResourceKind
  allKindItems: { label: string; kind: ResourceKind }[]
}>()

const emit = defineEmits<{
  (e: 'update:showMobileSearch', val: boolean): void
  (e: 'update:searchQuery', val: string): void
  (e: 'selectKind', kind: ResourceKind): void
  (e: 'refresh'): void
  (e: 'applyYaml'): void
  (e: 'createResource'): void
  (e: 'importCluster'): void
  (e: 'newNamespace'): void
}>()

// Primary Kinds for Linear Segmented Control
const primaryKinds: { label: string; kind: ResourceKind }[] = [
  { label: 'Pods', kind: 'pods' },
  { label: 'Deployments', kind: 'deployments' },
  { label: 'StatefulSets', kind: 'statefulsets' },
  { label: 'DaemonSets', kind: 'daemonsets' },
  { label: 'ConfigMaps', kind: 'configmaps' },
  { label: 'Services', kind: 'services' },
]

// Secondary Kinds in Dropdown
const secondaryKinds: { label: string; kind: ResourceKind; icon: string }[] = [
  { label: 'Jobs', kind: 'jobs', icon: 'clock' },
  { label: 'CronJobs', kind: 'cronjobs', icon: 'clock' },
  { label: 'Secrets', kind: 'secrets', icon: 'shield' },
  { label: 'Ingresses', kind: 'ingresses', icon: 'globe' },
  { label: 'Nodes', kind: 'nodes', icon: 'server' },
  { label: 'Events', kind: 'events', icon: 'activity' },
  { label: 'PersistentVolumeClaims', kind: 'persistentvolumeclaims', icon: 'database' },
  { label: 'HorizontalPodAutoscalers', kind: 'horizontalpodautoscalers', icon: 'layers' },
  { label: 'NetworkPolicies', kind: 'networkpolicies', icon: 'shield' },
]

const moreActions = computed<ActionItem[]>(() => [
  ...secondaryKinds.map((k) => ({
    id: `kind:${k.kind}`,
    label: k.label,
    icon: k.icon,
  })),
  { id: 'sep-actions', label: '', separator: true },
  { id: 'refresh', label: 'Refresh', icon: 'refresh' },
  { id: 'import-cluster', label: 'Import Cluster', icon: 'cloud' },
  { id: 'new-namespace', label: 'New Namespace', icon: 'plus' },
])

function handleMoreActionSelect(actionId: string) {
  if (actionId.startsWith('kind:')) {
    const kind = actionId.replace('kind:', '') as ResourceKind
    emit('selectKind', kind)
  } else if (actionId === 'refresh') {
    emit('refresh')
  } else if (actionId === 'import-cluster') {
    emit('importCluster')
  } else if (actionId === 'new-namespace') {
    emit('newNamespace')
  }
}
</script>

<template>
  <div class="explorer-toolbar-wrapper">
    <!-- Mobile 44px Command Bar (< 640px) -->
    <div class="explorer-mobile-command-bar">
      <div class="mobile-command-brand">
        <span class="mobile-command-title font-mono"><BaseIcon name="anchor" size="xs" /> {{ currentKindLabel }}</span>
        <span class="mobile-count-badge font-mono">({{ totalInKind }})</span>
      </div>
      <div class="mobile-command-actions">
        <button
          type="button"
          class="btn-mobile-cmd"
          title="Toggle Search"
          aria-label="Search"
          @click="emit('update:showMobileSearch', !showMobileSearch)"
        >
          <BaseIcon name="search" size="xs" />
        </button>
        <button
          type="button"
          class="btn-mobile-cmd"
          title="Refresh"
          aria-label="Refresh"
          :disabled="loading"
          @click="emit('refresh')"
        >
          <BaseIcon :name="loading ? 'clock' : 'refresh'" size="xs" />
        </button>
        <button
          type="button"
          class="btn-mobile-cmd"
          title="Apply YAML"
          aria-label="Apply YAML"
          @click="emit('applyYaml')"
        >
          <BaseIcon name="file-text" size="xs" />
        </button>
        <button
          type="button"
          class="btn-mobile-cmd"
          title="Create Resource"
          aria-label="Create Resource"
          @click="emit('createResource')"
        >
          <BaseIcon name="plus" size="xs" />
        </button>
      </div>
    </div>

    <!-- Mobile Search Strip (toggled by search) -->
    <div v-if="showMobileSearch" class="mobile-search-strip">
      <BaseIcon name="search" size="xs" class="search-icon" />
      <input
        :value="searchQuery"
        type="text"
        placeholder="Filter by name, namespace, status..."
        class="input-glass mobile-search-input font-mono"
        @input="emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
      />
    </div>

    <!-- Mobile Micro-Telemetry (20px) -->
    <div class="mobile-micro-telemetry font-mono">
      <BaseIcon name="globe" size="xs" /> {{ selectedCluster }} · <BaseIcon name="folder" size="xs" /> {{ selectedNamespace }} · <BaseIcon name="box" size="xs" /> {{ totalInKind }} {{ currentKindLabel }}
    </div>

    <!-- Mobile Horizontal Kind Scroller -->
    <div class="mobile-kind-scroller">
      <button
        v-for="item in allKindItems"
        :key="item.kind"
        type="button"
        class="mobile-kind-pill"
        :class="{ 'is-active': selectedKind === item.kind }"
        @click="emit('selectKind', item.kind)"
      >
        <span class="pill-label font-mono">{{ item.label }}</span>
      </button>
    </div>

    <!-- Sleek 1-line Enterprise Command Bar (~42px) -->
    <div class="explorer-command-bar glass-panel desktop-only" role="toolbar" aria-label="Kubernetes Explorer Command Bar">
      <!-- Left: Search input -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          :value="searchQuery"
          type="text"
          placeholder="Search resources in namespace..."
          class="toolbar-search-input font-mono"
          aria-label="Search resources in namespace"
          @input="emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
        />
        <button
          v-if="searchQuery"
          type="button"
          class="clear-input-btn"
          aria-label="Clear search"
          @click="emit('update:searchQuery', '')"
        >
          <BaseIcon name="x" size="xs" />
        </button>
      </div>

      <!-- Center: Linear-style segmented control -->
      <div class="linear-segmented-group" role="tablist" aria-label="Resource Types">
        <button
          v-for="item in primaryKinds"
          :key="item.kind"
          type="button"
          role="tab"
          :aria-selected="selectedKind === item.kind"
          class="linear-segment-btn font-mono"
          :class="{ active: selectedKind === item.kind }"
          @click="emit('selectKind', item.kind)"
        >
          <span>{{ item.label }}</span>
        </button>
        <!-- Secondary Active Kind Indicator -->
        <div
          v-if="!primaryKinds.some(k => k.kind === selectedKind)"
          class="linear-segment-btn active font-mono secondary-active-segment"
        >
          <span>{{ currentKindLabel }}</span>
        </div>
      </div>

      <!-- Right: Actions group -->
      <div class="toolbar-actions-group">
        <button
          type="button"
          class="toolbar-btn btn-secondary btn-apply-yaml"
          style="white-space: nowrap; font-size: 11px; padding: 4px 8px;"
          title="Apply YAML Manifest"
          @click="emit('applyYaml')"
        >
          <BaseIcon name="file-text" size="xs" />
          <span>Apply YAML</span>
        </button>

        <button
          type="button"
          class="toolbar-btn btn-primary"
          title="Create Resource"
          @click="emit('createResource')"
        >
          <BaseIcon name="plus" size="xs" />
          <span>Create Resource</span>
        </button>

        <ActionDropdown
          :items="moreActions"
          size="sm"
          trigger-title="More actions"
          @select="handleMoreActionSelect"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/explorer.css';
@import '../../assets/styles/components/explorer-toolbar.css';
</style>

