<script setup lang="ts">
import { reactive, watch, onMounted, onUnmounted } from 'vue'
import type { CreateSLOPayload, UpdateSLOPayload } from '../../api/compute'
import type { SLODefinition } from '../../types/slo'
import BaseIcon from '../ui/BaseIcon.vue'

interface ServiceOption {
  id: string
  name: string
  desc: string
}

const props = defineProps<{
  show: boolean
  realServices: ServiceOption[]
  actionInProgress?: boolean
  editingDefinition?: SLODefinition | null
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'create', payload: CreateSLOPayload): void
  (e: 'save', payload: { id: string; payload: UpdateSLOPayload }): void
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
  if (!props.editingDefinition) {
    const svc = newSvc === 'custom' ? (newSLO.customServiceName || 'my_service') : newSvc
    newSLO.query = getPromQLTemplate(svc, newInd)
  }
})

watch(
  [() => props.editingDefinition, () => props.show],
  ([def, isShown]) => {
    if (!isShown) return
    if (def) {
      const match = props.realServices.find(s => s.id === def.service)
      newSLO.selectedService = match && match.id !== 'custom' ? match.id : 'custom'
      newSLO.customServiceName = match && match.id !== 'custom' ? '' : def.service
      newSLO.indicator_type = def.indicator_type || 'availability'
      newSLO.target = def.target > 1 ? def.target : Number((def.target * 100).toFixed(2))
      newSLO.window = def.window || '30d'
      newSLO.query = def.query || getPromQLTemplate(def.service, def.indicator_type)
      newSLO.alert_threshold = def.alert_threshold || 1.5
    } else {
      newSLO.selectedService = props.realServices[0]?.id || 'custom'
      newSLO.customServiceName = ''
      newSLO.indicator_type = 'availability'
      newSLO.target = 99.90
      newSLO.window = '30d'
      newSLO.query = getPromQLTemplate('my_service', 'availability')
      newSLO.alert_threshold = 1.5
    }
  },
  { immediate: true }
)

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.show) {
    emit('update:show', false)
  }
}
onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))

function setPresetTarget(val: number) {
  newSLO.target = Number(val.toFixed(2))
}

function handleSubmit() {
  const targetService = newSLO.selectedService === 'custom' 
    ? newSLO.customServiceName.trim() 
    : newSLO.selectedService

  const sanitizedTarget = Number(Number(newSLO.target).toFixed(3))
  const sanitizedAlertThreshold = Number(Number(newSLO.alert_threshold).toFixed(2))

  if (props.editingDefinition?.id) {
    emit('save', {
      id: props.editingDefinition.id,
      payload: {
        service: targetService,
        indicator_type: newSLO.indicator_type,
        target: sanitizedTarget,
        window: newSLO.window,
        query: newSLO.query.trim(),
        alert_threshold: sanitizedAlertThreshold,
      }
    })
  } else {
    emit('create', {
      service: targetService,
      indicator_type: newSLO.indicator_type,
      target: sanitizedTarget,
      window: newSLO.window,
      query: newSLO.query.trim(),
      alert_threshold: sanitizedAlertThreshold,
    })
  }
}
</script>

<template>
  <div v-if="show" class="modal-overlay animate-fade-in" @click.self="emit('update:show', false)">
    <div class="modal-card glass-panel animate-scale-up">
      <div class="modal-header">
        <div class="modal-title-group">
          <BaseIcon name="target" size="sm" class="modal-icon" />
          <h2 class="modal-title">{{ editingDefinition ? 'Edit SLO Target Definition' : '+ Create SLO Target Definition' }}</h2>
        </div>
        <button class="modal-close-btn" aria-label="Close modal" @click="emit('update:show', false)"><BaseIcon name="x" size="xs" /></button>
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
              <button type="button" class="btn-preset" @click="setPresetTarget(99.00)">99.0%</button>
              <button type="button" class="btn-preset" @click="setPresetTarget(99.50)">99.5%</button>
              <button type="button" class="btn-preset" @click="setPresetTarget(99.90)">99.9%</button>
              <button type="button" class="btn-preset" @click="setPresetTarget(99.95)">99.95%</button>
              <button type="button" class="btn-preset" @click="setPresetTarget(99.99)">99.99%</button>
            </div>
          </div>
          <input 
            v-model.number="newSLO.target" 
            type="number" 
            step="0.01" 
            min="0" 
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
            <BaseIcon :name="actionInProgress ? 'clock' : 'zap'" size="xs" />
            <span>{{ actionInProgress ? (editingDefinition ? 'Saving...' : 'Creating...') : (editingDefinition ? 'Save Changes' : 'Save & Arm SLO Target') }}</span>
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/slo-drawers.css';
</style>
