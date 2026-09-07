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
.header-banner {
  padding: 18px 22px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
  background: linear-gradient(135deg, rgba(16, 24, 40, 0.85) 0%, rgba(11, 15, 25, 0.9) 100%);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-top: 1px solid rgba(6, 182, 212, 0.3);
}

.header-left {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.view-tag {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 0.7rem;
  font-weight: 700;
  color: #06b6d4;
  letter-spacing: 0.08em;
}

.tag-title {
  text-transform: uppercase;
}

.status-live-chip {
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.35);
  color: #34d399;
  font-size: 0.65rem;
  padding: 1px 6px;
  border-radius: 4px;
  font-family: var(--font-mono);
  font-weight: 700;
}

.title-with-icon {
  display: flex;
  align-items: center;
  gap: 10px;
}

.view-title {
  font-size: 1.65rem;
  font-weight: 800;
  color: #fff;
  margin: 0;
  letter-spacing: -0.02em;
}

.breadcrumbs {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.75rem;
  flex-wrap: wrap;
}

.crumb-pill {
  padding: 2px 8px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 6px;
  color: #94a3b8;
}

.crumb-pill.active-kind {
  background: rgba(6, 182, 212, 0.12);
  border-color: rgba(6, 182, 212, 0.35);
  color: #38bdf8;
  font-weight: 600;
}

.crumb-sep {
  color: #475569;
  font-weight: 700;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.btn-header {
  height: 38px;
  padding: 0 14px;
  font-size: 0.82rem;
  font-weight: 600;
  border-radius: 8px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  transition: all 0.15s cubic-bezier(0.4, 0, 0.2, 1);
  white-space: nowrap;
}

.btn-yaml {
  border-color: rgba(99, 102, 241, 0.3);
}

.btn-yaml:hover {
  border-color: #6366f1;
  box-shadow: 0 0 14px rgba(99, 102, 241, 0.3);
}

.btn-create {
  background: linear-gradient(135deg, #06b6d4 0%, #3b82f6 100%);
  color: #fff;
  box-shadow: 0 4px 15px rgba(6, 182, 212, 0.3);
}

.btn-create:hover {
  filter: brightness(1.1);
  transform: translateY(-1px);
  box-shadow: 0 6px 20px rgba(6, 182, 212, 0.45);
}

@media (max-width: 768px) {
  .header-banner {
    padding: 14px;
  }
  .view-title {
    font-size: 1.35rem;
  }
  .header-actions {
    width: 100%;
    justify-content: flex-start;
  }
  .btn-header {
    flex: 1;
    min-height: 42px;
  }
}
</style>
