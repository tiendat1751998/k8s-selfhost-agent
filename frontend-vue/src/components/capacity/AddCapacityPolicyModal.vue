<script setup lang="ts">
import { reactive } from 'vue'
import type { CapacityPolicy } from '../../composables/useCapacityForecast'

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'save', policy: Omit<CapacityPolicy, 'id' | 'createdAt'>): void
}>()

const form = reactive({
  name: 'Production Auto-Scaling Policy',
  cluster: 'k8s-prod-primary',
  cpuThresholdPercent: 80,
  ramThresholdPercent: 85,
  headroomBufferPercent: 20,
  targetBinPackingPercent: 75,
  actionType: 'scale_up' as 'scale_up' | 'alert_only' | 'pod_rebalance',
  enabled: true,
})

function handleSubmit() {
  emit('save', { ...form })
  emit('close')
}
</script>

<template>
  <div class="modal-overlay" @click.self="emit('close')">
    <div class="modal-card glass-panel animate-fade-in">
      <div class="modal-header">
        <div class="modal-title-group">
          <span class="badge badge-cyan">AUTONOMOUS GOVERNANCE</span>
          <h3 class="modal-title">Define Capacity & Headroom Policy</h3>
        </div>
        <button type="button" class="modal-close" @click="emit('close')">✕</button>
      </div>

      <form class="modal-body" @submit.prevent="handleSubmit">
        <div class="form-group">
          <label class="form-label">Policy Name:</label>
          <input
            v-model="form.name"
            type="text"
            required
            class="input-glass"
            placeholder="e.g. Core Worker Scale Buffer"
          />
        </div>

        <div class="form-group-row">
          <div class="form-group" style="flex: 1;">
            <label class="form-label">Cluster Scope:</label>
            <input
              v-model="form.cluster"
              type="text"
              required
              class="input-glass font-mono"
              placeholder="k8s-prod-primary"
            />
          </div>

          <div class="form-group" style="flex: 1;">
            <label class="form-label">Automated Action:</label>
            <select v-model="form.actionType" class="input-glass">
              <option value="scale_up">Scale Node Pool (+1 Node)</option>
              <option value="pod_rebalance">Trigger Pod Eviction / Rebalance</option>
              <option value="alert_only">Telemetry Alert Only</option>
            </select>
          </div>
        </div>

        <div class="form-group-row">
          <div class="form-group" style="flex: 1;">
            <label class="form-label">CPU Saturation Threshold (%):</label>
            <input
              v-model.number="form.cpuThresholdPercent"
              type="number"
              min="40"
              max="95"
              required
              class="input-glass font-mono"
            />
          </div>

          <div class="form-group" style="flex: 1;">
            <label class="form-label">RAM Saturation Threshold (%):</label>
            <input
              v-model.number="form.ramThresholdPercent"
              type="number"
              min="40"
              max="95"
              required
              class="input-glass font-mono"
            />
          </div>
        </div>

        <div class="form-group-row">
          <div class="form-group" style="flex: 1;">
            <label class="form-label">Headroom Safety Buffer (%):</label>
            <input
              v-model.number="form.headroomBufferPercent"
              type="number"
              min="5"
              max="50"
              required
              class="input-glass font-mono"
            />
          </div>

          <div class="form-group" style="flex: 1;">
            <label class="form-label">Target Bin-Packing (%):</label>
            <input
              v-model.number="form.targetBinPackingPercent"
              type="number"
              min="50"
              max="95"
              required
              class="input-glass font-mono"
            />
          </div>
        </div>

        <div class="modal-footer" style="padding: 16px 0 0 0; background: transparent; border-top: none;">
          <button type="button" class="btn btn-secondary" @click="emit('close')">Cancel</button>
          <button type="submit" class="btn btn-primary">
            <span>Enforce Capacity Policy</span>
          </button>
        </div>
      </form>
    </div>
  </div>
</template>