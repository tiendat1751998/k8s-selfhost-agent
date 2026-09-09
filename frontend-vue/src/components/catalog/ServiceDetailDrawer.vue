<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { ServiceEntry } from '../../api/catalog'
import type { ServiceDependency } from '../../composables/useServiceCatalog'

defineProps<{
  show: boolean
  service: ServiceEntry | null
  dependencies: ServiceDependency[]
  copiedKey: string | null
  getTypeIcon: (type: string) => string
  getTypeBadgeClass: (type: string) => string
  getLifecycleBadgeClass: (lifecycle: string) => string
  getLifecycleDotClass: (lifecycle: string) => string
  formatDate: (iso: string) => string
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'edit', service: ServiceEntry): void
  (e: 'delete', service: ServiceEntry): void
  (e: 'deploy', service: ServiceEntry): void
  (e: 'copy', text: string, label: string): void
}>()
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="drawer"
    :title="`Service: ${service?.name || ''}`"
    :subtitle="`Type: ${service?.type || 'service'} · Lifecycle: ${service?.lifecycle || 'development'}`"
    max-width="640px"
    @update:show="emit('update:show', $event)"
  >
    <div v-if="service" class="drawer-inner-content">
      <!-- Service Hero Banner -->
      <div class="service-hero glass-panel">
        <div class="hero-top">
          <div class="hero-brand">
            <span class="hero-icon">{{ getTypeIcon(service.type) }}</span>
            <div>
              <h3 class="hero-title">{{ service.name }}</h3>
              <span class="hero-id font-mono text-muted">ID: {{ service.id }}</span>
            </div>
          </div>
          <div class="hero-badges">
            <span class="type-badge" :class="getTypeBadgeClass(service.type)"><span>{{ service.type }}</span></span>
            <span class="lifecycle-badge" :class="getLifecycleBadgeClass(service.lifecycle)">
              <span class="lifecycle-dot" :class="getLifecycleDotClass(service.lifecycle)"></span>
              <span>{{ service.lifecycle }}</span>
            </span>
          </div>
        </div>
        <p v-if="service.description" class="hero-desc">{{ service.description }}</p>
        <p v-else class="hero-desc-empty text-muted">No description provided for this catalog entry.</p>
      </div>

      <!-- Health & Topology Dependencies -->
      <div class="info-card glass-panel">
        <div class="card-title-row">
          <span class="card-title">Health & Topology Dependencies</span>
          <span class="font-mono text-muted font-small">{{ dependencies.length }} linked</span>
        </div>
        <div v-if="dependencies.length > 0" class="dependency-list">
          <div v-for="dep in dependencies" :key="dep.id" class="dep-item font-mono text-xs">
            <div class="dep-info">
              <span class="dep-badge" :class="`dep-${dep.status}`">{{ dep.status }}</span>
              <span class="font-bold text-white">{{ dep.name }}</span>
              <span class="text-muted">({{ dep.relation }})</span>
            </div>
            <span v-if="dep.latencyMs" class="text-cyan text-xs">{{ dep.latencyMs }}ms latency</span>
          </div>
        </div>
        <p v-else class="text-muted font-mono font-small">No upstream or downstream service dependencies declared.</p>
      </div>

      <!-- Ownership & Timestamps -->
      <div class="info-card glass-panel">
        <div class="card-title">Ownership & Timestamps</div>
        <div class="meta-grid">
          <div class="meta-item"><span class="meta-label">Owner Team</span><span class="meta-val font-mono text-cyan">{{ service.owner_team || 'Unassigned' }}</span></div>
          <div class="meta-item"><span class="meta-label">Owner Email</span><span class="meta-val font-mono">{{ service.owner_email || 'None' }}</span></div>
          <div class="meta-item"><span class="meta-label">Tenant ID</span><span class="meta-val font-mono text-muted">{{ service.tenant_id || 'default-tenant' }}</span></div>
          <div class="meta-item"><span class="meta-label">Created At</span><span class="meta-val font-mono text-muted">{{ formatDate(service.created_at) }}</span></div>
          <div class="meta-item"><span class="meta-label">Last Updated</span><span class="meta-val font-mono text-muted">{{ formatDate(service.updated_at) }}</span></div>
        </div>
      </div>

      <!-- Quick Links & Resources -->
      <div class="info-card glass-panel">
        <div class="card-title">Repository & Documentation Links</div>
        <div class="links-list">
          <div class="link-item">
            <div class="link-left">
              <span class="link-icon">🐙</span>
              <div class="link-text">
                <span class="link-title">Source Repository</span>
                <a v-if="service.repo_url" :href="service.repo_url" target="_blank" rel="noopener noreferrer" class="link-url font-mono">{{ service.repo_url }} ↗</a>
                <span v-else class="text-muted font-mono">No repository URL attached</span>
              </div>
            </div>
            <button v-if="service.repo_url" type="button" class="btn btn-secondary btn-xs" @click="emit('copy', service.repo_url, 'repo')">
              <span>{{ copiedKey === 'repo' ? '✓ Copied' : 'Copy' }}</span>
            </button>
          </div>
          <div class="link-item">
            <div class="link-left">
              <span class="link-icon">📖</span>
              <div class="link-text">
                <span class="link-title">Documentation & API Spec</span>
                <a v-if="service.docs_url" :href="service.docs_url" target="_blank" rel="noopener noreferrer" class="link-url font-mono">{{ service.docs_url }} ↗</a>
                <span v-else class="text-muted font-mono">No documentation URL attached</span>
              </div>
            </div>
            <button v-if="service.docs_url" type="button" class="btn btn-secondary btn-xs" @click="emit('copy', service.docs_url, 'docs')">
              <span>{{ copiedKey === 'docs' ? '✓ Copied' : 'Copy' }}</span>
            </button>
          </div>
        </div>
      </div>

      <!-- Discovery Tags -->
      <div class="info-card glass-panel">
        <div class="card-title">Discovery Tags</div>
        <div v-if="service.tags && service.tags.length > 0" class="tags-wrap">
          <span v-for="(tag, idx) in service.tags" :key="idx" class="tag-chip font-mono">#{{ tag }}</span>
        </div>
        <p v-else class="text-muted font-mono font-small">No tags attached to this service.</p>
      </div>

      <!-- Annotations Section -->
      <div class="info-card glass-panel">
        <div class="card-title-row">
          <span class="card-title">Kubernetes & Infrastructure Annotations</span>
          <span class="font-mono text-muted font-small">{{ Object.keys(service.annotations || {}).length }} keys</span>
        </div>
        <div v-if="service.annotations && Object.keys(service.annotations).length > 0" class="annotations-table-wrap">
          <table class="kv-table">
            <thead><tr><th>Annotation Key</th><th>Value</th></tr></thead>
            <tbody>
              <tr v-for="(val, key) in service.annotations" :key="key">
                <td class="key-cell font-mono text-cyan">{{ key }}</td>
                <td class="val-cell font-mono">{{ val }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="text-muted font-mono font-small">No metadata annotations defined (e.g. k8s.io/namespace, k8s.io/deployment).</p>
      </div>
    </div>

    <template #footer="{ close }">
      <button type="button" class="btn btn-secondary" @click="close">Close</button>
      <button v-if="service" type="button" class="btn btn-secondary btn-action-labeled" @click="emit('deploy', service)"><span>📦 Deploy Service</span></button>
      <button v-if="service" type="button" class="btn btn-danger-crimson btn-action-labeled" @click="emit('delete', service)"><span>🗑 Delete Service</span></button>
      <button v-if="service" type="button" class="btn btn-primary btn-action-labeled" @click="emit('edit', service)"><span>✏️ Edit Service</span></button>
    </template>
  </ModalDrawer>
</template>
