<script setup lang="ts">
import { ref, watch } from 'vue'
import type { DeploymentApp, UpdateResourcesPayload } from '../../api/compute'
import ModalDrawer from '../ui/ModalDrawer.vue'
import ScaleVerticalSection from './ScaleVerticalSection.vue'
import { formatContainerName } from '../../utils/dockerFormat'

interface Props {
  show: boolean
  app: DeploymentApp | null
  actionLoading?: string | null
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'apply', payload: UpdateResourcesPayload): void
}>()

const scaleModalTab = ref<'horizontal' | 'vertical'>('horizontal')
const targetReplicas = ref(1)
const targetMemoryLimit = ref('1GiB')
const targetMemoryReservation = ref('256MiB')
const targetCpuLimit = ref('1')
const targetCpuReservation = ref('250m')

watch(
  () => props.app,
  (app) => {
    if (app) {
      targetReplicas.value = app.replicas !== undefined ? app.replicas : 1
      const mem = (app.memoryLimit || app.memory || '1GiB').trim()
      targetMemoryLimit.value = mem.replace(/\s+/g, '')
      targetMemoryReservation.value = app.memoryReservation ? app.memoryReservation.replace(/\s+/g, '') : '256MiB'
      targetCpuLimit.value = (app.cpuLimit || app.cpu || '1').trim()
      targetCpuReservation.value = app.cpuReservation ? app.cpuReservation.trim() : '250m'
      scaleModalTab.value = 'horizontal'
    }
  },
  { immediate: true }
)

function submitApply() {
  if (!props.app) return
  const payload: UpdateResourcesPayload = {
    type: props.app.type,
    cluster: props.app.target,
    namespace: props.app.namespace,
    name: props.app.rawId || props.app.name,
    replicas: targetReplicas.value,
    memory_limit: targetMemoryLimit.value,
    memory_reservation: targetMemoryReservation.value,
    cpu_limit: targetCpuLimit.value,
    cpu_reservation: targetCpuReservation.value,
  }
  emit('apply', payload)
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    :title="`Scale & Resource Tuning: ${formatContainerName(app?.name).serviceName || app?.name || ''}`"
    subtitle="Adjust horizontal replicas and vertical CPU/RAM hardware allocations in real time"
    max-width="640px"
    @update:show="emit('update:show', $event)"
  >
    <div v-if="app" class="scale-modal-content">
      <!-- Target Scope Banner -->
      <div class="scale-info-row font-mono">
        <div class="scope-item">
          <span class="text-muted">Target Scope:</span>
          <span class="text-cyan font-semibold">{{ app.namespace }}/{{ formatContainerName(app.name).serviceName || app.name }}</span>
        </div>
        <div class="scope-badges">
          <span class="scope-type-badge">{{ app.type }}</span>
          <span class="scope-cluster-badge">{{ app.target || 'default-cluster' }}</span>
        </div>
      </div>

      <!-- Tab Switcher -->
      <div class="scale-tab-switcher">
        <button
          type="button"
          class="scale-tab-btn"
          :class="{ active: scaleModalTab === 'horizontal' }"
          @click="scaleModalTab = 'horizontal'"
        >
          <span class="tab-icon">🌐</span>
          <span class="tab-title">Horizontal Scaling (Replicas)</span>
          <span class="tab-badge font-mono">{{ targetReplicas }} Pods</span>
        </button>
        <button
          type="button"
          class="scale-tab-btn"
          :class="{ active: scaleModalTab === 'vertical' }"
          @click="scaleModalTab = 'vertical'"
        >
          <span class="tab-icon">⚡</span>
          <span class="tab-title">Vertical Resources (CPU & RAM)</span>
          <span class="tab-badge font-mono">{{ targetMemoryLimit }} / {{ targetCpuLimit }}</span>
        </button>
      </div>

      <!-- TAB 1: HORIZONTAL SCALING (REPLICAS) -->
      <div v-if="scaleModalTab === 'horizontal'" class="scale-section-wrap animate-fade-in">
        <div class="section-subhead">
          <span class="subhead-title">Horizontal Pod Replicas</span>
          <span class="subhead-desc text-muted">Dynamically scale instance count across cluster worker nodes</span>
        </div>

        <div class="scale-slider-wrap">
          <div class="replicas-display">
            <span class="replicas-number font-mono">{{ targetReplicas }}</span>
            <span class="replicas-label font-mono">Target Desired Pods</span>
          </div>

          <input
            v-model.number="targetReplicas"
            type="range"
            min="0"
            max="30"
            class="scale-range"
          />

          <div class="range-marks font-mono">
            <span>0 (Suspended)</span>
            <span>10</span>
            <span>20</span>
            <span>30</span>
          </div>

          <!-- Quick Presets -->
          <div class="scale-presets">
            <button
              type="button"
              class="preset-btn"
              :class="{ active: targetReplicas === 1 }"
              @click="targetReplicas = 1"
            >1 Pod</button>
            <button
              type="button"
              class="preset-btn"
              :class="{ active: targetReplicas === 3 }"
              @click="targetReplicas = 3"
            >3 HA</button>
            <button
              type="button"
              class="preset-btn"
              :class="{ active: targetReplicas === 5 }"
              @click="targetReplicas = 5"
            >5 Pods</button>
            <button
              type="button"
              class="preset-btn"
              :class="{ active: targetReplicas === 10 }"
              @click="targetReplicas = 10"
            >10 Pods</button>
          </div>
        </div>
      </div>

      <!-- TAB 2: VERTICAL RESOURCE TUNING (CPU & RAM) -->
      <ScaleVerticalSection
        v-if="scaleModalTab === 'vertical'"
        v-model:memory-limit="targetMemoryLimit"
        v-model:memory-reservation="targetMemoryReservation"
        v-model:cpu-limit="targetCpuLimit"
        v-model:cpu-reservation="targetCpuReservation"
      />

      <!-- Configuration Summary Footer Bar -->
      <div class="scale-summary-bar font-mono">
        <div class="summary-chip">
          <span class="chip-label">Replicas:</span>
          <span class="chip-val text-cyan">{{ targetReplicas }} Pods</span>
        </div>
        <div class="summary-chip">
          <span class="chip-label">RAM (Max / Res):</span>
          <span class="chip-val text-emerald">{{ targetMemoryLimit }} / {{ targetMemoryReservation }}</span>
        </div>
        <div class="summary-chip">
          <span class="chip-label">CPU (Max / Res):</span>
          <span class="chip-val text-amber">{{ targetCpuLimit }} / {{ targetCpuReservation }}</span>
        </div>
      </div>
    </div>

    <template #footer="{ close }">
      <button type="button" class="btn btn-secondary" @click="close">Cancel</button>
      <button
        type="button"
        class="btn btn-primary"
        :disabled="actionLoading === 'resources' || actionLoading === 'scale'"
        @click="submitApply"
      >
        <span v-if="actionLoading === 'resources' || actionLoading === 'scale'" class="spinner-inline"></span>
        <span>{{ (actionLoading === 'resources' || actionLoading === 'scale') ? 'Applying Resource Configuration...' : 'Apply Resource Configuration ➔' }}</span>
      </button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/components/scale-modal.css';
</style>