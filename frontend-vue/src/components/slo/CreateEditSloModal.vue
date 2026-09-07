<script setup lang="ts">
import type { SLOFormState, ServiceOption } from '../../composables/useSLOMonitor'

const props = defineProps<{
  show: boolean
  isEditing: boolean
  form: SLOFormState
  realServices: ServiceOption[]
  actionInProgress: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'save'): void
}>()
</script>

<template>
  <div v-if="show" class="modal-overlay animate-fade-in" @click.self="emit('close')">
    <div class="modal-card glass-panel animate-scale-up">
      <div class="modal-header">
        <div class="modal-title-group">
          <span class="modal-icon">🎯</span>
          <h2 class="modal-title">{{ isEditing ? '✏️ Edit SLO Target Definition' : '+ Create SLO Target Definition' }}</h2>
        </div>
        <button class="modal-close-btn" @click="emit('close')">✕</button>
      </div>

      <form class="modal-body" @submit.prevent="emit('save')">
        <!-- Service Dropdown -->
        <div class="form-group">
          <label class="form-label">Target Service / Workload:</label>
          <select v-model="form.selectedService" class="input-glass">
            <option v-for="s in realServices" :key="s.id" :value="s.id">
              {{ s.name }} ({{ s.desc }})
            </option>
          </select>
        </div>

        <!-- Custom service input if chosen -->
        <div v-if="form.selectedService === 'custom'" class="form-group">
          <label class="form-label">Custom Workload Name:</label>
          <input 
            v-model="form.customServiceName" 
            type="text" 
            required 
            class="input-glass" 
            placeholder="e.g. payment_processor" 
          />
        </div>

        <!-- Indicator Type -->
        <div class="form-group">
          <label class="form-label">Service Level Indicator (SLI) Type:</label>
          <select v-model="form.indicator_type" class="input-glass">
            <option value="availability">Availability (% Successful Requests / Transactions)</option>
            <option value="latency">Latency / SLA (% Requests Under P99 Latency Target)</option>
            <option value="error_rate">Error Rate Inversion (% HTTP Non-5xx Traffic)</option>
            <option value="cache_hit_rate">Cache Hit Rate (% Memory Keyspace Hits)</option>
          </select>
        </div>

        <!-- Target Objective % & Presets -->
        <div class="form-group">
          <div class="label-with-presets">
            <label class="form-label">Target Objective % (Compliance Goal):</label>
            <div class="preset-buttons">
              <button type="button" class="btn-preset" @click="form.target = 99.00">99.0%</button>
              <button type="button" class="btn-preset" @click="form.target = 99.50">99.5%</button>
              <button type="button" class="btn-preset" @click="form.target = 99.90">99.9%</button>
              <button type="button" class="btn-preset" @click="form.target = 99.95">99.95%</button>
              <button type="button" class="btn-preset" @click="form.target = 99.99">99.99%</button>
            </div>
          </div>
          <input 
            v-model.number="form.target" 
            type="number" 
            step="0.01" 
            min="50" 
            max="100" 
            required 
            class="input-glass font-mono" 
            placeholder="99.90" 
          />
        </div>

        <!-- Rolling Window & Alert Threshold in two columns -->
        <div class="form-row-2">
          <div class="form-group">
            <label class="form-label">Rolling Compliance Window:</label>
            <select v-model="form.window" class="input-glass">
              <option value="7d">7 Days (Fast feedback)</option>
              <option value="14d">14 Days (Bi-weekly sprint)</option>
              <option value="30d">30 Days (Standard monthly SRE)</option>
              <option value="90d">90 Days (Quarterly SLA)</option>
            </select>
          </div>

          <div class="form-group">
            <label class="form-label">Burn Alert Multiplier:</label>
            <select v-model.number="form.alert_threshold" class="input-glass">
              <option :value="1.2">1.2x (Strict nominal)</option>
              <option :value="1.5">1.5x (Elevated alert)</option>
              <option :value="2.0">2.0x (Double burn speed)</option>
              <option :value="3.0">3.0x (Severe fast burn)</option>
              <option :value="5.0">5.0x (Critical fast burn)</option>
            </select>
          </div>
        </div>

        <!-- PromQL SLI Query Expression -->
        <div class="form-group">
          <label class="form-label">PromQL SLI Metric Query Expression:</label>
          <textarea 
            v-model="form.query" 
            rows="3" 
            required 
            class="input-glass font-mono query-textarea"
            placeholder="sum(rate(http_requests_total{status=~'2..|3..'}[5m])) / sum(rate(http_requests_total[5m])) * 100"
          ></textarea>
          <span class="field-hint">PromQL expression evaluated continuously against Prometheus telemetry metrics.</span>
        </div>

        <!-- Modal Footer -->
        <div class="modal-footer">
          <button type="button" class="btn btn-secondary" @click="emit('close')">
            Cancel
          </button>
          <button type="submit" class="btn btn-primary" :disabled="actionInProgress">
            <span>{{ actionInProgress ? '⏳ Saving...' : isEditing ? '💾 Update SLO Target' : '✨ Save & Arm SLO Target' }}</span>
          </button>
        </div>
      </form>
    </div>
  </div>
</template>
