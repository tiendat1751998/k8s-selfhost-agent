<script setup lang="ts">
import { reactive, watch } from 'vue'
import type { CreateSLOPayload } from '../../api/compute'

interface ServiceOption {
  id: string
  name: string
  desc: string
}

const props = defineProps<{
  show: boolean
  realServices: ServiceOption[]
  actionInProgress?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'create', payload: CreateSLOPayload): void
}>()

const newSLO = reactive({
  selectedService: 'custom',
  customServiceName: '',
  indicator_type: 'availability',
  target: 99.90,
  window: '30d',
  query: 'sum(rate(http_requests_total{status=~"2..|3.."}[5m])) / sum(rate(http_requests_total[5m])) * 100',
  alert_threshold: 1.5,
})

function getPromQLTemplate(service: string, indicator: string): string {
  if (indicator === 'latency') {
    return `histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{service="${service}"}[5m])) by (le)) * 1000 < 100`
  }
  if (indicator === 'cache_hit_rate') {
    return 'sum(rate(redis_keyspace_hits_total[5m])) / (sum(rate(redis_keyspace_hits_total[5m])) + sum(rate(redis_keyspace_misses_total[5m]))) * 100'
  }
  if (indicator === 'error_rate') {
    return `(1 - sum(rate(http_requests_total{service="${service}",status=~"5.."}[5m])) / sum(rate(http_requests_total{service="${service}"}[5m]))) * 100`
  }
  return `sum(rate(http_requests_total{service="${service}",status=~"2..|3.."}[5m])) / sum(rate(http_requests_total{service="${service}"}[5m])) * 100`
}

watch([() => newSLO.selectedService, () => newSLO.indicator_type], ([newSvc, newInd]) => {
  const svc = newSvc === 'custom' ? (newSLO.customServiceName || 'my_service') : newSvc
  newSLO.query = getPromQLTemplate(svc, newInd)
})

function handleSubmit() {
  const targetService = newSLO.selectedService === 'custom' 
    ? newSLO.customServiceName.trim() 
    : newSLO.selectedService

  emit('create', {
    service: targetService,
    indicator_type: newSLO.indicator_type,
    target: newSLO.target,
    window: newSLO.window,
    query: newSLO.query.trim(),
    alert_threshold: newSLO.alert_threshold,
  })
}
</script>

<template>
  <div v-if="show" class="modal-overlay animate-fade-in" @click.self="emit('update:show', false)">
    <div class="modal-card glass-panel animate-scale-up">
      <div class="modal-header">
        <div class="modal-title-group">
          <span class="modal-icon">🎯</span>
          <h2 class="modal-title">+ Create SLO Target Definition</h2>
        </div>
        <button class="modal-close-btn" @click="emit('update:show', false)">✕</button>
      </div>

      <form class="modal-body" @submit.prevent="handleSubmit">
        <div class="form-group">
          <label class="form-label">Target Service / Workload:</label>
          <select v-model="newSLO.selectedService" class="input-glass">
            <option v-for="s in realServices" :key="s.id" :value="s.id">
              {{ s.name }} ({{ s.desc }})
            </option>
          </select>
        </div>

        <div v-if="newSLO.selectedService === 'custom'" class="form-group">
          <label class="form-label">Custom Workload Name:</label>
          <input 
            v-model="newSLO.customServiceName" 
            type="text" 
            required 
            class="input-glass font-mono" 
            placeholder="e.g. payment_processor" 
          />
        </div>

        <div class="form-group">
          <label class="form-label">Service Level Indicator (SLI) Type:</label>
          <select v-model="newSLO.indicator_type" class="input-glass">
            <option value="availability">Availability (% Successful Requests / Transactions)</option>
            <option value="latency">Latency / SLA (% Requests Under P99 Latency Target)</option>
            <option value="error_rate">Error Rate Inversion (% HTTP Non-5xx Traffic)</option>
            <option value="cache_hit_rate">Cache Hit Rate (% Memory Keyspace Hits)</option>
          </select>
        </div>

        <div class="form-group">
          <div class="label-with-presets">
            <label class="form-label">Target Objective % (Compliance Goal):</label>
            <div class="preset-buttons">
              <button type="button" class="btn-preset" @click="newSLO.target = 99.00">99.0%</button>
              <button type="button" class="btn-preset" @click="newSLO.target = 99.50">99.5%</button>
              <button type="button" class="btn-preset" @click="newSLO.target = 99.90">99.9%</button>
              <button type="button" class="btn-preset" @click="newSLO.target = 99.95">99.95%</button>
              <button type="button" class="btn-preset" @click="newSLO.target = 99.99">99.99%</button>
            </div>
          </div>
          <input 
            v-model.number="newSLO.target" 
            type="number" 
            step="0.01" 
            min="50" 
            max="100" 
            required 
            class="input-glass font-mono" 
            placeholder="99.90" 
          />
        </div>

        <div class="form-row-2">
          <div class="form-group">
            <label class="form-label">Rolling Compliance Window:</label>
            <select v-model="newSLO.window" class="input-glass">
              <option value="7d">7 Days (Fast feedback)</option>
              <option value="14d">14 Days (Bi-weekly sprint)</option>
              <option value="30d">30 Days (Standard monthly SRE)</option>
              <option value="90d">90 Days (Quarterly SLA)</option>
            </select>
          </div>

          <div class="form-group">
            <label class="form-label">Burn Alert Multiplier:</label>
            <select v-model.number="newSLO.alert_threshold" class="input-glass">
              <option :value="1.2">1.2x (Strict nominal)</option>
              <option :value="1.5">1.5x (Elevated alert)</option>
              <option :value="2.0">2.0x (Double burn speed)</option>
              <option :value="3.0">3.0x (Severe fast burn)</option>
              <option :value="5.0">5.0x (Critical fast burn)</option>
            </select>
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">PromQL SLI Metric Query Expression:</label>
          <textarea 
            v-model="newSLO.query" 
            rows="3" 
            required 
            class="input-glass font-mono query-textarea"
            placeholder="sum(rate(http_requests_total{status=~'2..|3..'}[5m])) / sum(rate(http_requests_total[5m])) * 100"
          ></textarea>
          <span class="field-hint">PromQL expression evaluated continuously against Prometheus telemetry metrics.</span>
        </div>

        <div class="modal-footer">
          <button type="button" class="btn btn-secondary" @click="emit('update:show', false)">
            Cancel
          </button>
          <button type="submit" class="btn btn-primary" :disabled="actionInProgress">
            <span>{{ actionInProgress ? '⏳ Creating...' : '✨ Save & Arm SLO Target' }}</span>
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  backdrop-filter: blur(4px);
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.modal-card {
  width: 100%;
  max-width: 600px;
  background: #161f30;
  border: 1px solid var(--border-medium);
  border-radius: 12px;
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.6);
  overflow: hidden;
}

.modal-header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.modal-title-group {
  display: flex;
  align-items: center;
  gap: 10px;
}

.modal-icon { font-size: 18px; }
.modal-title { font-size: 15px; font-weight: 700; color: #fff; margin: 0; }
.modal-close-btn { background: none; border: none; color: var(--text-muted); cursor: pointer; font-size: 16px; }

.modal-body {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-group { display: flex; flex-direction: column; gap: 6px; }
.form-row-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.form-label { font-size: 11px; font-weight: 700; color: var(--text-muted); text-transform: uppercase; }

.label-with-presets {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.preset-buttons { display: flex; gap: 4px; }
.btn-preset {
  padding: 2px 6px;
  font-size: 10px;
  background: #0b0f19;
  border: 1px solid var(--border-subtle);
  border-radius: 4px;
  color: var(--text-secondary);
  cursor: pointer;
}
.btn-preset:hover { border-color: #3b82f6; color: #fff; }

.query-textarea {
  font-size: 11px;
  line-height: 1.4;
  resize: vertical;
  min-height: 60px;
}

.field-hint { font-size: 11px; color: var(--text-muted); }

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--border-subtle);
}

.font-mono { font-family: var(--font-mono); }
</style>