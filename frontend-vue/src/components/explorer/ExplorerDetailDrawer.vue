<script setup lang="ts">
import { ref, computed } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import StatusBadge from '../ui/StatusBadge.vue'
import EventsTimeline from '../k8s/EventsTimeline.vue'
import { jsonToYaml } from '../../utils/yaml'
import type { K8sResource, ResourceKind } from '../../api/k8s'
import { getResourceAge, getResourceStatus } from '../../composables/useK8sExplorer'

const props = defineProps<{
  show: boolean
  resource: K8sResource | null
  cluster: string
  selectedKind: ResourceKind
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'edit-yaml', resource: K8sResource): void
  (e: 'scale', resource: K8sResource): void
  (e: 'restart', resource: K8sResource): void
}>()

const activeTab = ref<'overview' | 'events' | 'yaml'>('overview')
const yamlCopied = ref(false)

const resourceYaml = computed(() => {
  return props.resource ? jsonToYaml(props.resource) : ''
})

async function copyYaml() {
  if (!resourceYaml.value) return
  await navigator.clipboard.writeText(resourceYaml.value)
  yamlCopied.value = true
  setTimeout(() => {
    yamlCopied.value = false
  }, 2000)
}

const isWorkload = computed(() => {
  const k = props.resource?.kind?.toLowerCase() || ''
  return k === 'deployment' || k === 'statefulset' || k === 'daemonset'
})
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="drawer"
    :title="resource ? `${resource.kind}: ${resource.metadata?.name}` : 'Resource Details'"
    :subtitle="resource ? `Namespace: ${resource.metadata?.namespace || 'cluster-scoped'} • Age: ${getResourceAge(resource)}` : ''"
    max-width="720px"
    @close="$emit('close')"
  >
    <div v-if="resource" class="detail-drawer-body">
      <!-- Tabs Navigation -->
      <div class="drawer-tabs font-mono">
        <button type="button" class="drawer-tab-btn" :class="{ 'is-active': activeTab === 'overview' }" @click="activeTab = 'overview'">
          📋 Overview
        </button>
        <button type="button" class="drawer-tab-btn" :class="{ 'is-active': activeTab === 'events' }" @click="activeTab = 'events'">
          ⚡ Events
        </button>
        <button type="button" class="drawer-tab-btn" :class="{ 'is-active': activeTab === 'yaml' }" @click="activeTab = 'yaml'">
          📄 YAML
        </button>
      </div>

      <!-- Tab: Overview -->
      <div v-if="activeTab === 'overview'" class="tab-content overview-tab">
        <div class="overview-header-card glass-panel">
          <div class="meta-row">
            <span class="meta-label">Status</span>
            <StatusBadge :status="getResourceStatus(resource)" size="sm" />
          </div>
          <div class="meta-row">
            <span class="meta-label">UID</span>
            <span class="meta-val font-mono">{{ resource.metadata?.uid || 'N/A' }}</span>
          </div>
          <div class="meta-row">
            <span class="meta-label">Created</span>
            <span class="meta-val font-mono">{{ resource.metadata?.creationTimestamp || 'N/A' }}</span>
          </div>
          <div v-if="resource.metadata?.labels" class="meta-row labels-row">
            <span class="meta-label">Labels</span>
            <div class="chips-wrap font-mono">
              <span v-for="(v, k) in resource.metadata.labels" :key="k" class="kv-chip">
                {{ k }}: {{ v }}
              </span>
            </div>
          </div>
        </div>

        <!-- Kind Specific Metadata -->
        <div class="overview-section">
          <h4 class="section-title font-mono">Specification & State</h4>
          <pre class="json-spec-box font-mono">{{ JSON.stringify(resource.spec || resource.status || {}, null, 2) }}</pre>
        </div>
      </div>

      <!-- Tab: Events -->
      <div v-else-if="activeTab === 'events'" class="tab-content events-tab">
        <EventsTimeline
          :cluster="cluster"
          :namespace="resource.metadata?.namespace"
          :resource-name="resource.metadata?.name"
          :resource-kind="resource.kind"
        />
      </div>

      <!-- Tab: YAML -->
      <div v-else-if="activeTab === 'yaml'" class="tab-content yaml-tab">
        <div class="yaml-toolbar font-mono">
          <button type="button" class="btn btn-secondary btn-xs" @click="copyYaml">
            {{ yamlCopied ? '✅ Copied!' : '📋 Copy YAML' }}
          </button>
          <button type="button" class="btn btn-primary btn-xs" @click="$emit('edit-yaml', resource)">
            ✏️ Edit Manifest
          </button>
        </div>
        <pre class="yaml-code font-mono">{{ resourceYaml }}</pre>
      </div>
    </div>

    <template #footer>
      <div class="drawer-footer-actions">
        <button v-if="isWorkload" type="button" class="btn btn-secondary btn-xs font-mono" @click="resource && $emit('scale', resource)">
          ⚖️ Scale
        </button>
        <button v-if="isWorkload" type="button" class="btn btn-secondary btn-xs font-mono" @click="resource && $emit('restart', resource)">
          🔄 Restart
        </button>
        <button type="button" class="btn btn-secondary btn-xs font-mono" @click="$emit('close')">
          Close
        </button>
      </div>
    </template>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/views/explorer.css';
</style>
