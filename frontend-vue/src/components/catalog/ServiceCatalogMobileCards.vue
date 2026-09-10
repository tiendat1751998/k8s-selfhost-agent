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

function handleApiClick(service: ServiceEntry) {
  if (service.docs_url) {
    window.open(service.docs_url, '_blank', 'noopener,noreferrer')
  } else {
    emit('open-detail', service)
  }
}

const emit = defineEmits<{
  (e: 'open-detail', service: ServiceEntry): void
  (e: 'deploy', service: ServiceEntry): void
  (e: 'config', service: ServiceEntry): void
  (e: 'delete', service: ServiceEntry): void
}>()
</script>

<template>
  <div class="catalog-mobile-cards-wrap">
    <div v-if="loading && services.length === 0" class="catalog-mobile-stream">
      <div v-for="i in 5" :key="i" class="mobile-card-item animate-pulse">
        <div class="mobile-card-left">
          <div class="mobile-type-icon skeleton-icon"></div>
          <div class="mobile-card-meta">
            <div class="skeleton-text w-24"></div>
            <div class="skeleton-text-sm w-16"></div>
          </div>
        </div>
      </div>
    </div>

    <div v-else-if="services.length === 0" class="empty-state-box glass-panel text-center text-muted">
      <p class="font-mono text-xs">📦 No registered catalog services found. Tap ➕ Register to add a service.</p>
    </div>

    <div v-else class="catalog-mobile-stream">
      <div
        v-for="service in services"
        :key="service.id"
        class="mobile-card-item"
        @click="emit('open-detail', service)"
      >
        <div class="mobile-card-left">
          <div class="mobile-type-icon">
            {{ getTypeIcon(service.type) }}
          </div>
          <div class="mobile-card-meta">
            <div class="mobile-card-title font-mono">
              {{ service.name }}
            </div>
            <div class="mobile-card-sub font-mono">
              <span class="lifecycle-dot" :class="getLifecycleDotClass(service.lifecycle)"></span>
              <span>{{ service.type }}</span>
              <span>·</span>
              <span class="text-cyan">{{ service.owner_team || 'Unassigned' }}</span>
            </div>
            <div v-if="service.description" class="mobile-card-desc">
              {{ service.description }}
            </div>
          </div>
        </div>

        <div class="mobile-card-right" @click.stop>
          <button
            type="button"
            class="btn btn-secondary btn-xs btn-action-compact"
            title="Details"
            aria-label="View Details"
            @click="emit('open-detail', service)"
          >
            <span>🔍</span>
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-xs btn-action-compact"
            title="APIs & Docs"
            aria-label="APIs & Documentation"
            @click="handleApiClick(service)"
          >
            <span>⚡</span>
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-xs btn-action-compact"
            title="Edit Service"
            aria-label="Edit Service"
            @click="emit('config', service)"
          >
            <span>✏️</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
