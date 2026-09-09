<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import VolumeReplicaMatrix from './VolumeReplicaMatrix.vue'
import { storageApi, formatBytes, type DistributedVolume } from '../../api/storage'

const props = defineProps<{
  show: boolean
  volume: DistributedVolume | null
  clusterId: string
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'expanded'): void
  (e: 'snapshot-created'): void
}>()

const currentSizeGb = computed(() => {
  if (!props.volume?.capacity_bytes) return 10
  return Math.max(1, Math.round(props.volume.capacity_bytes / (1024 * 1024 * 1024)))
})

const maxSizeGb = computed(() => Math.max(currentSizeGb.value * 5, 200))
const targetSizeGb = ref(10)
const isExpanding = ref(false)
const expandAlert = ref<{ type: 'success' | 'error'; message: string } | null>(null)

watch(currentSizeGb, (val) => {
  targetSizeGb.value = val
  expandAlert.value = null
}, { immediate: true })

async function handleExpand() {
  if (!props.volume || targetSizeGb.value <= currentSizeGb.value) return
  isExpanding.value = true
  expandAlert.value = null
  try {
    const bytes = targetSizeGb.value * 1024 * 1024 * 1024
    await storageApi.expandVolume(props.clusterId, props.volume.name, props.volume.namespace, bytes)
    expandAlert.value = { type: 'success', message: `Volume expanded to ${targetSizeGb.value} GiB successfully.` }
    emit('expanded')
  } catch (err: unknown) {
    expandAlert.value = { type: 'error', message: err instanceof Error ? err.message : 'Online expansion failed.' }
  } finally {
    isExpanding.value = false
  }
}

const isCreatingSnapshot = ref(false)
const confirmingSnapshot = ref(false)
const snapshotAlert = ref<{ type: 'success' | 'error'; message: string } | null>(null)

async function handleCreateSnapshot() {
  if (!props.volume) return
  confirmingSnapshot.value = false
  isCreatingSnapshot.value = true
  snapshotAlert.value = null
  try {
    const res = await storageApi.createSnapshot(props.clusterId, props.volume.name, props.volume.namespace)
    snapshotAlert.value = { type: 'success', message: `Snapshot created: ${res.snapshot_name || 'Success'}` }
    emit('snapshot-created')
  } catch (err: unknown) {
    snapshotAlert.value = { type: 'error', message: err instanceof Error ? err.message : 'Snapshot creation failed.' }
  } finally {
    isCreatingSnapshot.value = false
  }
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="drawer"
    placement="right"
    max-width="540px"
    :title="volume ? `HA Volume: ${volume.name}` : 'HA Volume Management'"
    :subtitle="volume ? `Namespace: ${volume.namespace} | StorageClass: ${volume.storage_class}` : 'Distributed Storage Controller'"
    @close="emit('update:show', false)"
  >
    <div v-if="volume" class="drawer-content">
      <!-- Volume Metadata Grid -->
      <div class="meta-card font-mono">
        <div class="meta-row"><span class="meta-label">Volume:</span><span class="meta-val text-white">{{ volume.name }}</span></div>
        <div class="meta-row"><span class="meta-label">Namespace:</span><span class="meta-val text-muted">{{ volume.namespace }}</span></div>
        <div class="meta-row"><span class="meta-label">StorageClass:</span><span class="meta-val text-violet">{{ volume.storage_class }}</span></div>
        <div class="meta-row"><span class="meta-label">Capacity:</span><span class="meta-val text-cyan font-bold">{{ volume.capacity_human || formatBytes(volume.capacity_bytes) }}</span></div>
      </div>

      <!-- Distributed Topology Matrix -->
      <VolumeReplicaMatrix :volume="volume" />

      <!-- Online Expansion Slider Section -->
      <div class="action-card">
        <div class="card-head">
          <span class="card-title">Online Volume Expansion</span>
          <span class="growth-tag font-mono">+{{ targetSizeGb - currentSizeGb }} GiB</span>
        </div>
        <div class="slider-metric font-mono">
          <span>Current: <strong class="text-white">{{ currentSizeGb }} GiB</strong></span>
          <span>Target: <strong class="text-cyan">{{ targetSizeGb }} GiB</strong></span>
        </div>
        <input 
          type="range" 
          class="size-slider"
          :min="currentSizeGb" 
          :max="maxSizeGb" 
          step="1"
          v-model.number="targetSizeGb"
          :disabled="isExpanding"
        />
        <div class="slider-bounds font-mono">
          <span>{{ currentSizeGb }} GiB</span>
          <span>{{ maxSizeGb }} GiB</span>
        </div>
        <button 
          type="button" 
          class="btn-action btn-expand" 
          :disabled="targetSizeGb <= currentSizeGb || isExpanding"
          @click="handleExpand"
        >
          {{ isExpanding ? '? Expanding Online...' : '? Expand Online' }}
        </button>
        <p v-if="expandAlert" class="status-alert font-mono" :class="`alert-${expandAlert.type}`">
          {{ expandAlert.message }}
        </p>
      </div>

      <!-- 1-Click Volume Snapshot Section -->
      <div class="action-card">
        <div class="card-head">
          <span class="card-title">1-Click Volume Snapshot</span>
          <span class="card-sub font-mono">Point-in-time state</span>
        </div>
        <div v-if="confirmingSnapshot" class="confirm-prompt font-mono">
          <span>Create online snapshot for <strong>{{ volume.name }}</strong>?</span>
          <div class="confirm-btns">
            <button type="button" class="btn-xs btn-confirm" :disabled="isCreatingSnapshot" @click="handleCreateSnapshot">Confirm</button>
            <button type="button" class="btn-xs btn-cancel" @click="confirmingSnapshot = false">Cancel</button>
          </div>
        </div>
        <button 
          v-else 
          type="button" 
          class="btn-action btn-snapshot" 
          :disabled="isCreatingSnapshot"
          @click="confirmingSnapshot = true"
        >
          {{ isCreatingSnapshot ? '📸 Creating Snapshot...' : '📸 Create Snapshot' }}
        </button>
        <p v-if="snapshotAlert" class="status-alert font-mono" :class="`alert-${snapshotAlert.type}`">
          {{ snapshotAlert.message }}
        </p>
      </div>
    </div>
    <div v-else class="empty-state font-mono">No volume selected</div>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/components/storage.css';
</style>
