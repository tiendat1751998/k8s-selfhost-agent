<script setup lang="ts">
import { ref, watch } from 'vue'
import type { AlertRule, AlertChannel, AlertRuleInput } from '../../api/management'
import ModalDrawer from '../ui/ModalDrawer.vue'

const props = defineProps<{
  show: boolean
  ruleToEdit: AlertRule | null
  channels: AlertChannel[]
  isSubmitting: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'submit', payload: AlertRuleInput): void
}>()

const form = ref({
  name: '',
  description: '',
  metric_name: 'container_cpu_usage_seconds_total',
  condition: '>',
  threshold: 85,
  duration_seconds: 300,
  severity: 'critical' as 'critical' | 'high' | 'medium' | 'low',
  channel_ids: [] as string[]
})

watch(() => props.ruleToEdit, (rule) => {
  if (rule) {
    form.value = {
      name: rule.Name,
      description: rule.Description,
      metric_name: rule.MetricName,
      condition: rule.Condition,
      threshold: rule.Threshold,
      duration_seconds: rule.DurationSeconds,
      severity: (rule.Severity as 'critical' | 'high' | 'medium' | 'low') || 'critical',
      channel_ids: [...(rule.ChannelIDs || [])]
    }
  } else {
    form.value = {
      name: '',
      description: '',
      metric_name: 'container_cpu_usage_seconds_total',
      condition: '>',
      threshold: 85,
      duration_seconds: 300,
      severity: 'critical',
      channel_ids: []
    }
  }
}, { immediate: true })

function toggleChannel(id: string) {
  const idx = form.value.channel_ids.indexOf(id)
  if (idx > -1) {
    form.value.channel_ids.splice(idx, 1)
  } else {
    form.value.channel_ids.push(id)
  }
}

function handleSubmit() {
  if (!form.value.name || !form.value.metric_name) return
  emit('submit', { ...form.value })
}
</script>

<template>
  <ModalDrawer
    :show="show"
    :title="ruleToEdit ? 'Edit Prometheus Alert Rule' : 'Configure Prometheus Alert Rule'"
    subtitle="Define threshold metrics, evaluation durations, and notification routing."
    @update:show="emit('update:show', $event)"
  >
    <form @submit.prevent="handleSubmit" class="form-layout">
      <div class="form-group">
        <label>Alert Rule Name</label>
        <input 
          v-model="form.name" 
          type="text" 
          placeholder="e.g. Node Memory Working Set > 90%" 
          class="input-glass" 
          required 
        />
      </div>

      <div class="form-group">
        <label>Rule Description</label>
        <input 
          v-model="form.description" 
          type="text" 
          placeholder="e.g. Alert triggers when container memory limit threshold reached." 
          class="input-glass" 
        />
      </div>

      <div class="form-row">
        <div class="form-group">
          <label>Prometheus Metric Expression / PromQL</label>
          <input 
            v-model="form.metric_name" 
            type="text" 
            placeholder="e.g. node_cpu_utilization_percent" 
            class="input-glass" 
            required 
          />
        </div>
        <div class="form-group">
          <label>Operator</label>
          <select v-model="form.condition" class="input-glass">
            <option value=">">&gt; (Greater Than)</option>
            <option value="<">&lt; (Less Than)</option>
            <option value=">=">&gt;= (Greater or Equal)</option>
            <option value="==">== (Equal)</option>
          </select>
        </div>
      </div>

      <div class="form-row">
        <div class="form-group">
          <label>Threshold Value</label>
          <input 
            v-model.number="form.threshold" 
            type="number" 
            step="any" 
            class="input-glass" 
            required 
          />
        </div>
        <div class="form-group">
          <label>Duration Window (Seconds)</label>
          <input 
            v-model.number="form.duration_seconds" 
            type="number" 
            placeholder="300" 
            class="input-glass" 
            required 
          />
        </div>
      </div>

      <div class="form-group">
        <label>Severity Level</label>
        <select v-model="form.severity" class="input-glass">
          <option value="critical">Critical (Page SRE On-Call)</option>
          <option value="high">High (Dispatch Slack War Room)</option>
          <option value="medium">Medium (Notification Digest)</option>
          <option value="low">Low (Audit Log Only)</option>
        </select>
      </div>

      <div class="form-group" v-if="channels.length > 0">
        <label>Dispatch Notification Channels</label>
        <div class="channel-select-grid">
          <label 
            v-for="chan in channels" 
            :key="chan.ID" 
            class="channel-pill-label"
          >
            <input 
              type="checkbox" 
              :checked="form.channel_ids.includes(chan.ID)" 
              @change="toggleChannel(chan.ID)"
            />
            <span class="font-bold">{{ chan.Name }}</span>
            <span class="text-xs font-mono uppercase text-cyan">({{ chan.Type }})</span>
          </label>
        </div>
      </div>
    </form>

    <template #footer="{ close }">
      <button class="btn btn-secondary" type="button" @click="close">Cancel</button>
      <button class="btn btn-primary" :disabled="isSubmitting" @click="handleSubmit">
        {{ isSubmitting ? 'Saving...' : (ruleToEdit ? 'Save Changes' : 'Arm Alert Rule') }}
      </button>
    </template>
  </ModalDrawer>
</template>
