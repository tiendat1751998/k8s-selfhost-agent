<script setup lang="ts">
import { ref } from 'vue'
import type { LogEntry } from '../../stores/logStore'
import BaseIcon from '../ui/BaseIcon.vue'
import LogTargetItem from './LogTargetItem.vue'
import {
  useLogTargets,
  type LogTarget,
  type TargetNodeItem,
  type TargetServiceItem,
  type TargetWorkloadItem,
} from '../../composables/useLogTargets'

export type { LogTarget, TargetNodeItem, TargetServiceItem, TargetWorkloadItem }

const props = withDefaults(
  defineProps<{
    modelValue: LogTarget
    logs: LogEntry[]
    cluster?: string
  }>(),
  { cluster: '' }
)
const emit = defineEmits<{
  (e: 'update:modelValue', target: LogTarget): void
  (e: 'select', target: LogTarget): void
}>()

const workloadsExpanded = ref(true)
const appsExpanded = ref(true)
const systemdExpanded = ref(true)
const nodesExpanded = ref(true)

const {
  targetSearch,
  filteredK8sWorkloads,
  filteredAppServices,
  filteredSystemdServices,
  filteredHostNodes,
  getNodeCount,
  getServiceCount,
  selectTarget,
} = useLogTargets(props, emit)
</script>

<template>
  <aside class="log-target-tree glass-panel" aria-label="Log Stream Target Hierarchy">
    <div class="tree-header">
      <div class="tree-title">
        <span class="tree-icon"><BaseIcon name="layers" size="sm" /></span>
        <span>Log Targets</span>
      </div>
      <span class="tree-badge font-mono">{{ logs.length }} logs</span>
    </div>

    <!-- Quick Filter Search Input -->
    <div class="tree-search-bar">
      <span class="tree-search-ico" aria-hidden="true"><BaseIcon name="search" size="sm" /></span>
      <input
        v-model="targetSearch"
        type="text"
        placeholder="Filter targets..."
        class="tree-search-input font-mono"
        aria-label="Filter targets"
      />
      <button
        v-if="targetSearch"
        type="button"
        class="tree-search-clear"
        aria-label="Clear target filter"
        @click="targetSearch = ''"
      ><BaseIcon name="x" size="xs" /></button>
    </div>

    <div class="tree-content">
      <div
        v-if="filteredK8sWorkloads.length === 0 && filteredAppServices.length === 0 && filteredSystemdServices.length === 0 && filteredHostNodes.length === 0"
        class="tree-empty-state font-mono"
        style="padding: 24px 16px; text-align: center; color: var(--text-muted); font-size: 12px; display: flex; flex-direction: column; align-items: center; gap: 8px;"
      >
        <BaseIcon name="layers" size="md" />
        <span>No log targets discovered</span>
      </div>

      <!-- Section 0: Kubernetes Workloads -->
      <div v-if="filteredK8sWorkloads.length > 0" class="tree-section">
        <div class="section-toggle" @click="workloadsExpanded = !workloadsExpanded">
          <span class="section-caret" :class="{ 'caret-down': workloadsExpanded }">&#9656;</span>
          <BaseIcon name="layers" size="xs" />
          <span class="section-label">Kubernetes Workloads</span>
          <span class="section-count font-mono">{{ filteredK8sWorkloads.length }}</span>
        </div>
        <div v-show="workloadsExpanded" class="section-items">
          <LogTargetItem
            v-for="wl in filteredK8sWorkloads"
            :key="wl.id"
            :id="wl.id"
            :name="wl.name"
            :sub="wl.kind + (wl.namespace ? ' · ' + wl.namespace : '')"
            :icon="wl.icon"
            :count="getServiceCount(wl.id)"
            :active="modelValue.type === 'service' && modelValue.id === wl.id"
            @click="selectTarget({ type: 'service', id: wl.id, name: wl.name, icon: wl.icon, namespace: wl.namespace })"
          />
        </div>
      </div>

      <!-- Section 1: Running Apps (Containers) -->
      <div class="tree-section">
        <div class="section-toggle" @click="appsExpanded = !appsExpanded">
          <span class="section-caret" :class="{ 'caret-down': appsExpanded }">&#9656;</span>
          <BaseIcon name="box" size="xs" />
          <span class="section-label">Running Apps (Containers)</span>
          <span class="section-count font-mono">{{ filteredAppServices.length }}</span>
        </div>
        <div v-show="appsExpanded" class="section-items">
          <LogTargetItem
            v-for="svc in filteredAppServices"
            :key="svc.id"
            :id="svc.id"
            :name="svc.name"
            :sub="svc.type"
            :icon="svc.icon || 'box'"
            :count="getServiceCount(svc.id)"
            :active="modelValue.type === 'service' && modelValue.id === svc.id"
            @click="selectTarget({ type: 'service', id: svc.id, name: svc.name, icon: svc.icon })"
          />
        </div>
      </div>

      <!-- Section 2: System Services (SystemD) -->
      <div class="tree-section">
        <div class="section-toggle" @click="systemdExpanded = !systemdExpanded">
          <span class="section-caret" :class="{ 'caret-down': systemdExpanded }">&#9656;</span>
          <BaseIcon name="cpu" size="xs" />
          <span class="section-label">System Services (SystemD)</span>
          <span class="section-count font-mono">{{ filteredSystemdServices.length }}</span>
        </div>
        <div v-show="systemdExpanded" class="section-items">
          <LogTargetItem
            v-for="svc in filteredSystemdServices"
            :key="svc.id"
            :id="svc.id"
            :name="svc.name"
            :sub="svc.type"
            :icon="svc.icon || 'cpu'"
            :count="getServiceCount(svc.id)"
            :active="modelValue.type === 'service' && modelValue.id === svc.id"
            @click="selectTarget({ type: 'service', id: svc.id, name: svc.name, icon: svc.icon })"
          />
        </div>
      </div>

      <!-- Section 3: Node Systems (Log Node) -->
      <div class="tree-section">
        <div class="section-toggle" @click="nodesExpanded = !nodesExpanded">
          <span class="section-caret" :class="{ 'caret-down': nodesExpanded }">&#9656;</span>
          <BaseIcon name="server" size="xs" />
          <span class="section-label">Node Systems (Log Node)</span>
          <span class="section-count font-mono">{{ filteredHostNodes.length }}</span>
        </div>
        <div v-show="nodesExpanded" class="section-items">
          <LogTargetItem
            v-for="node in filteredHostNodes"
            :key="node.id"
            :id="node.id"
            :name="node.name"
            :sub="node.role"
            :icon="node.icon || 'server'"
            :count="getNodeCount(node.id)"
            :active="modelValue.type === 'node' && modelValue.id === node.id"
            @click="selectTarget({ type: 'node', id: node.id, name: node.name, icon: 'server' })"
          />
        </div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
@import '../../assets/styles/views/logstream.css';
</style>
