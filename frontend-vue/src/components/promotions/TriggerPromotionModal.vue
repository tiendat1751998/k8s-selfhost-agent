<script setup lang="ts">
import { ref } from 'vue'
import type { CreatePromotionPayload } from '../../api/compute'
import type { RunningServiceOption } from '../../composables/usePromotions'
import ModalDrawer from '../ui/ModalDrawer.vue'

const comboboxRef = ref<HTMLElement | null>(null)
void comboboxRef


defineProps<{
  show: boolean
  runningServices: RunningServiceOption[]
  loadingServices: boolean
  actionLoading?: string | null
  newPromotion: CreatePromotionPayload
  serviceSearchQuery: string
  isServiceDropdownOpen: boolean
  filteredServices: RunningServiceOption[]
  selectedService?: RunningServiceOption
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'update:serviceSearchQuery', val: string): void
  (e: 'update:isServiceDropdownOpen', val: boolean): void
  (e: 'select-service', svc: RunningServiceOption): void
  (e: 'clear-service-search'): void
  (e: 'search-input'): void
  (e: 'search-focus'): void
  (e: 'submit'): void
}>()
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="Request Environment Promotion"
    subtitle="Submit a workload version promotion request for gatekeeper review"
    max-width="560px"
    @update:show="val => emit('update:show', val)"
    @close="emit('update:show', false)"
  >
    <div class="modal-form">
      <div class="form-group">
        <div class="form-label-row">
          <label class="form-label">Service Name</label>
          <span v-if="runningServices.length > 0" class="service-count-pill font-mono">
            {{ runningServices.length }} Workload{{ runningServices.length === 1 ? '' : 's' }} Available
          </span>
        </div>

        <div v-if="loadingServices" class="input-loading-hint font-mono">
          <span>⏳ Fetching running services & host placement topology...</span>
        </div>

        <div v-else ref="comboboxRef" class="combobox-wrapper">
          <div class="combobox-input-box">
            <input
              :value="serviceSearchQuery"
              type="text"
              class="input-glass combobox-input font-mono"
              placeholder="🔍 Type to search service name, image, or host server..."
              autocomplete="off"
              @focus="emit('search-focus')"
              @input="(e: Event) => {
                emit('update:serviceSearchQuery', (e.target as HTMLInputElement).value)
                emit('search-input')
              }"
              @keydown.escape="emit('update:isServiceDropdownOpen', false)"
            />
            <button
              v-if="serviceSearchQuery"
              type="button"
              class="combobox-btn combobox-clear"
              title="Clear selection"
              @click.stop="emit('clear-service-search')"
            >
              ✕
            </button>
            <button
              type="button"
              class="combobox-btn combobox-toggle"
              title="Toggle service list"
              @click.stop="emit('update:isServiceDropdownOpen', !isServiceDropdownOpen)"
            >
              <span class="dropdown-chevron" :class="{ 'chevron-open': isServiceDropdownOpen }">▾</span>
            </button>
          </div>

          <div v-if="isServiceDropdownOpen" class="combobox-dropdown glass-panel">
            <div v-if="filteredServices.length === 0" class="combobox-empty font-mono">
              <span>⚠️ No workloads matching "{{ serviceSearchQuery }}" found</span>
            </div>

            <div
              v-for="svc in filteredServices"
              :key="svc.id + '-' + svc.name"
              class="combobox-option"
              :class="{ 'is-selected': svc.name === newPromotion.service }"
              @click="emit('select-service', svc)"
            >
              <div class="option-line-1">
                <span class="option-name font-mono">{{ svc.name }}</span>
                <div class="option-tags">
                  <span
                    class="workload-type-badge font-mono"
                    :class="svc.type === 'swarm' ? 'badge-swarm' : (svc.type === 'k8s' ? 'badge-k8s' : 'badge-docker')"
                  >
                    {{ svc.type === 'swarm' ? 'Swarm' : (svc.type === 'k8s' ? 'K8s' : 'Container') }}
                  </span>
                  <span
                    class="option-status-pill font-mono"
                    :class="svc.status === 'running' || (svc.replicas ?? 0) > 0 ? 'status-active' : 'status-inactive'"
                  >
                    <span class="status-dot"></span>
                    <span>{{ svc.replicas ?? 1 }} Rep</span>
                  </span>
                </div>
              </div>

              <div class="option-line-2 font-mono">
                <span class="option-meta-part">
                  <span class="meta-label">🖥️ Server:</span>
                  <span class="meta-val text-host">{{ svc.host }}</span>
                </span>
                <span class="meta-bar">|</span>
                <span class="option-meta-part">
                  <span class="meta-label">📦 Image:</span>
                  <span class="meta-val text-cyan">{{ svc.image }}</span>
                </span>
                <span class="meta-bar">|</span>
                <span class="option-meta-part">
                  <span class="meta-label">🔌 Ports:</span>
                  <span class="meta-val text-amber">{{ svc.ports && svc.ports.length > 0 ? svc.ports.join(', ') : 'None' }}</span>
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="selectedService" class="placement-preview-card glass-panel">
        <div class="placement-header">
          <div class="placement-title font-mono">
            <span class="placement-icon">🌐</span>
            <span>WORKLOAD TOPOLOGY & PLACEMENT</span>
          </div>
          <span class="active-badge font-mono" :class="selectedService.type === 'swarm' ? 'badge-swarm' : (selectedService.type === 'k8s' ? 'badge-k8s' : 'badge-docker')">
            {{ selectedService.type === 'swarm' ? 'Docker Swarm Service' : (selectedService.type === 'k8s' ? 'Kubernetes Workload' : 'Standalone Container') }}
          </span>
        </div>

        <div class="placement-details-grid font-mono">
          <div class="placement-grid-row">
            <span class="placement-row-key">🖥️ Host Server:</span>
            <span class="placement-row-val text-host font-bold">{{ selectedService.host }}</span>
          </div>

          <div class="placement-grid-row">
            <span class="placement-row-key">📦 Active Running Image:</span>
            <span class="placement-row-val text-cyan font-bold">{{ selectedService.image }}</span>
          </div>

          <div class="placement-grid-row">
            <span class="placement-row-key">⚙️ Replicas & Ports:</span>
            <span class="placement-row-val">
              <span class="text-emerald font-bold">{{ selectedService.replicas ?? 1 }} Replica(s)</span>
              <span class="placement-sep">•</span>
              <span class="text-muted">Ports:</span>
              <span class="text-amber">{{ selectedService.ports && selectedService.ports.length > 0 ? selectedService.ports.join(', ') : 'None' }}</span>
            </span>
          </div>
        </div>
      </div>

      <div class="form-group">
        <label class="form-label">Target Release Version / Image Tag</label>
        <input
          v-model="newPromotion.version"
          type="text"
          :placeholder="selectedService ? `e.g. ${selectedService.image}` : 'e.g. redis:8-alpine, nginx:1.27-alpine'"
          class="input-glass font-mono"
        />
        <span class="form-hint">Specify the target Docker image tag or release version to promote</span>
      </div>

      <div class="form-row">
        <div class="form-group flex-1">
          <label class="form-label">Source Stage</label>
          <select v-model="newPromotion.from_env" class="input-glass">
            <option value="dev">DEV (Development)</option>
            <option value="qa">QA (Automated Tests)</option>
            <option value="staging">STAGING (Pre-Prod)</option>
          </select>
        </div>

        <div class="form-group flex-1">
          <label class="form-label">Destination Stage</label>
          <select v-model="newPromotion.to_env" class="input-glass">
            <option value="qa">QA (Automated Tests)</option>
            <option value="staging">STAGING (Pre-Prod)</option>
            <option value="production">PRODUCTION (Live Users)</option>
          </select>
        </div>
      </div>

      <div class="form-group">
        <label class="form-label">Requester ID / Name</label>
        <input v-model="newPromotion.requester" type="text" class="input-glass font-mono" />
      </div>
    </div>

    <template #footer="{ close }">
      <button class="btn btn-secondary" @click="close">Cancel</button>
      <button
        class="btn btn-primary"
        :disabled="actionLoading === 'create' || !newPromotion.service || !newPromotion.version"
        @click="emit('submit')"
      >
        <span>{{ actionLoading === 'create' ? 'Submitting...' : 'Submit Promotion Request ➔' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>
