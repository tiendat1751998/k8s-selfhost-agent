<script setup lang="ts">
import type { ServiceEntry } from '../../api/catalog'

defineProps<{
  services: ServiceEntry[]
  loading?: boolean
  getTypeIcon: (type: string) => string
  getTypeBadgeClass: (type: string) => string
  getLifecycleBadgeClass: (lifecycle: string) => string
  getLifecycleDotClass: (lifecycle: string) => string
}>()

const emit = defineEmits<{
  (e: 'open-detail', service: ServiceEntry): void
  (e: 'deploy', service: ServiceEntry): void
  (e: 'config', service: ServiceEntry): void
  (e: 'delete', service: ServiceEntry): void
}>()
</script>

<template>
  <div class="section-box glass-panel table-box">
    <div class="box-header">
      <div class="box-header-title">
        <h2 class="box-title">Registered Services & Component Directory (Grid)</h2>
        <p class="box-subtitle">
          Visual card-based overview of microservices, APIs, and shared components.
        </p>
      </div>
      <div class="box-header-meta">
        <span class="badge badge-cyan font-mono">{{ services.length }} Services</span>
      </div>
    </div>

    <div v-if="loading" class="catalog-cards-grid">
      <div v-for="i in 6" :key="i" class="catalog-card animate-pulse">
        <div class="catalog-card-header">
          <div class="card-title-group">
            <span class="type-mini-icon">📦</span>
            <div class="h-4 bg-slate-700 rounded w-28"></div>
          </div>
          <span class="h-4 bg-slate-700 rounded w-16"></span>
        </div>
        <div class="card-desc h-8 bg-slate-800 rounded"></div>
      </div>
    </div>

    <div v-else-if="services.length === 0" class="empty-state-box p-8 text-center text-muted">
      <p class="font-mono text-sm">No services registered matching current filter criteria.</p>
    </div>

    <div v-else class="catalog-cards-grid animate-fade-in">
      <div
        v-for="service in services"
        :key="service.id"
        class="catalog-card"
      >
        <div class="catalog-card-header">
          <div class="card-title-group">
            <span class="type-mini-icon">{{ getTypeIcon(service.type) }}</span>
            <a
              href="javascript:void(0)"
              class="card-title-link font-mono"
              @click="emit('open-detail', service)"
              :title="service.name"
            >
              {{ service.name }}
            </a>
          </div>
          <div class="card-badges">
            <span class="lifecycle-badge" :class="getLifecycleBadgeClass(service.lifecycle)">
              <span class="lifecycle-dot" :class="getLifecycleDotClass(service.lifecycle)"></span>
              <span>{{ service.lifecycle }}</span>
            </span>
          </div>
        </div>

        <p class="card-desc">
          {{ service.description || 'No description provided.' }}
        </p>

        <div class="card-meta-row">
          <span class="type-badge" :class="getTypeBadgeClass(service.type)">
            <span>{{ service.type }}</span>
          </span>
          <span class="font-mono text-cyan">{{ service.owner_team || 'Unassigned' }}</span>
        </div>

        <div v-if="service.tags && service.tags.length > 0" class="tags-pill-wrap">
          <span
            v-for="(tag, idx) in service.tags.slice(0, 3)"
            :key="idx"
            class="tag-pill font-mono"
          >
            #{{ tag }}
          </span>
          <span v-if="service.tags.length > 3" class="tag-overflow font-mono">
            +{{ service.tags.length - 3 }}
          </span>
        </div>

        <div class="catalog-card-footer">
          <button
            type="button"
            class="btn btn-secondary btn-xs btn-action-labeled"
            title="View service details"
            @click="emit('open-detail', service)"
          >
            <span>🔍 Details</span>
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-xs btn-action-labeled"
            title="Deploy service"
            @click="emit('deploy', service)"
          >
            <span>📦 Deploy</span>
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-xs btn-action-labeled"
            title="Configure service"
            @click="emit('config', service)"
          >
            <span>⚙️ Config</span>
          </button>
          <button
            type="button"
            class="btn btn-danger-crimson btn-xs btn-action-labeled"
            title="Delete service"
            @click="emit('delete', service)"
          >
            <span>🗑 Delete</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
