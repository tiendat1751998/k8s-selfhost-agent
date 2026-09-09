<script setup lang="ts">
import type { Cluster } from '../../api/compute'
import type { K8sNamespace } from '../../api/k8s'

defineProps<{
  selectedCluster: string
  clusters: Cluster[]
  selectedNamespace: string
  namespaces: K8sNamespace[]
  currentKindLabel: string
  totalInKind: number
  loading: boolean
}>()

const emit = defineEmits<{
  (e: 'update:selectedCluster', val: string): void
  (e: 'update:selectedNamespace', val: string): void
  (e: 'refresh'): void
  (e: 'apply-yaml'): void
  (e: 'create-resource'): void
  (e: 'import-cluster'): void
  (e: 'new-namespace'): void
}>()
</script>

<template>
  <div class="view-header glass-panel header-banner">
    <div class="header-left">
      <div class="view-tag">
        <span class="pulse-dot pulse-dot-cyan"></span>
        <span class="tag-title">KUBERNETES CONTROL PLANE</span>
        <span class="status-live-chip">LIVE</span>
      </div>
      <div class="title-with-icon">
        <h1 class="view-title font-sans">{{ currentKindLabel }}</h1>
      </div>
      <div class="breadcrumbs font-mono">
        <span class="crumb-pill crumb-cluster">🌐 {{ selectedCluster }}</span>
        <span class="crumb-sep">›</span>
        <span class="crumb-pill crumb-ns">📁 {{ selectedNamespace === 'all' ? 'All Namespaces' : selectedNamespace }}</span>
        <span class="crumb-sep">›</span>
        <span class="crumb-pill crumb-kind active-kind">{{ currentKindLabel }} ({{ totalInKind }})</span>
      </div>
    </div>

    <div class="header-actions">
      <button 
        type="button" 
        class="btn btn-secondary btn-header" 
        :disabled="loading" 
        title="Refresh resource telemetry"
        @click="emit('refresh')"
      >
        <span class="btn-emoji">{{ loading ? '⏳' : '🔄' }}</span>
        <span class="btn-label">{{ loading ? 'Syncing...' : 'Refresh' }}</span>
      </button>
      <button 
        type="button" 
        class="btn btn-secondary btn-header btn-yaml" 
        title="Apply raw Kubernetes manifest"
        @click="emit('apply-yaml')"
      >
        <span class="btn-emoji">📄</span>
        <span class="btn-label">Apply YAML</span>
      </button>
      <button 
        type="button" 
        class="btn btn-primary btn-header btn-create" 
        title="Create a new Kubernetes resource"
        @click="emit('create-resource')"
      >
        <span class="btn-emoji">✨</span>
        <span class="btn-label">+ Create {{ currentKindLabel.slice(0, -1) || 'Resource' }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/explorer.css';
</style>
