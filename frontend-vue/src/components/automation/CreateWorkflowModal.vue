<script setup lang="ts">
import { reactive, watch } from 'vue'
import type { AutomationRule } from '../../api/governance'
import { TRIGGER_OPTIONS, ACTION_OPTIONS } from '../../composables/useAutomationEngine'

const props = defineProps<{
  show: boolean
  loading: boolean
  ruleToEdit?: AutomationRule | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'save', ruleData: Partial<AutomationRule>): void
}>()

const formData = reactive({
  name: '',
  trigger_type: 'deployment_failure',
  condition: '',
  action_type: 'rollback',
  target: '',
  enabled: true,
})

watch(
  () => props.ruleToEdit,
  (rule) => {
    if (rule) {
      formData.name = rule.name
      formData.trigger_type = rule.trigger_type
      formData.condition = rule.trigger_config?.schedule || rule.trigger_config?.condition || ''
      formData.action_type = rule.action_type
      formData.target = rule.action_config?.target || ''
      formData.enabled = rule.enabled
    } else {
      formData.name = ''
      formData.trigger_type = 'deployment_failure'
      formData.condition = ''
      formData.action_type = 'rollback'
      formData.target = ''
      formData.enabled = true
    }
  },
  { immediate: true }
)

function handleSubmit() {
  const triggerConfig: Record<string, string> = {}
  if (formData.trigger_type === 'cron_schedule') {
    triggerConfig.schedule = formData.condition || '0 */2 * * *'
  } else if (formData.condition) {
    triggerConfig.condition = formData.condition
  }

  const actionConfig: Record<string, string> = {}
  if (formData.target) {
    actionConfig.target = formData.target
  }

  emit('save', {
    name: formData.name,
    trigger_type: formData.trigger_type,
    trigger_config: triggerConfig,
    action_type: formData.action_type,
    action_config: actionConfig,
    enabled: formData.enabled,
  })
}
</script>

<template>
  <div v-if="show" class="modal-overlay" @click.self="emit('close')">
    <div class="modal-card glass-panel animate-fade-in">
      <div class="modal-header">
        <div class="modal-title-group">
          <span class="badge badge-cyan">AUTOMATION PIPELINE</span>
          <h3 class="modal-title">{{ ruleToEdit ? 'Edit Automation Rule' : 'Create Workflow Automation Rule' }}</h3>
        </div>
        <button class="modal-close" @click="emit('close')">✕</button>
      </div>

      <form class="modal-body" @submit.prevent="handleSubmit">
        <div class="form-group">
          <label class="form-label">Rule Name:</label>
          <input
            v-model="formData.name"
            type="text"
            required
            class="input-glass"
            placeholder="e.g. Auto-Rollback on CrashLoopBackOff"
          />
        </div>

        <div class="form-group">
          <label class="form-label">Trigger Event / Schedule:</label>
          <select v-model="formData.trigger_type" class="input-glass">
            <optgroup label="Event-Driven Self Healing">
              <option v-for="t in TRIGGER_OPTIONS.filter(o => o.category === 'event')" :key="t.value" :value="t.value">
                {{ t.icon }} {{ t.label }}
              </option>
            </optgroup>
            <optgroup label="Scheduled & Webhook Triggers">
              <option v-for="t in TRIGGER_OPTIONS.filter(o => o.category !== 'event')" :key="t.value" :value="t.value">
                {{ t.icon }} {{ t.label }}
              </option>
            </optgroup>
          </select>
          <span class="form-hint">
            {{ TRIGGER_OPTIONS.find(o => o.value === formData.trigger_type)?.description }}
          </span>
        </div>

        <div class="form-group">
          <label class="form-label">
            {{ formData.trigger_type === 'cron_schedule' ? 'Cron Expression:' : 'Condition / Threshold:' }}
          </label>
          <input
            v-model="formData.condition"
            type="text"
            class="input-glass"
            :placeholder="formData.trigger_type === 'cron_schedule' ? 'e.g. 0 */4 * * * (Every 4 hours)' : 'e.g. threshold > 85%, p99 > 200ms'"
          />
        </div>

        <div class="form-group">
          <label class="form-label">Automated Remediation Action:</label>
          <select v-model="formData.action_type" class="input-glass">
            <option v-for="a in ACTION_OPTIONS" :key="a.value" :value="a.value">
              {{ a.icon }} {{ a.label }}
            </option>
          </select>
          <span class="form-hint">
            {{ ACTION_OPTIONS.find(o => o.value === formData.action_type)?.description }}
          </span>
        </div>

        <div class="form-group">
          <label class="form-label">Target Workload / Scope (Optional):</label>
          <input
            v-model="formData.target"
            type="text"
            class="input-glass"
            placeholder="e.g. namespace=production, app=checkout-api"
          />
        </div>

        <div class="modal-footer" style="padding: 16px 0 0 0; background: transparent; border-top: none;">
          <button type="button" class="btn btn-secondary" @click="emit('close')">Cancel</button>
          <button type="submit" class="btn btn-primary" :disabled="loading">
            <span>{{ loading ? 'Saving...' : (ruleToEdit ? 'Save Changes' : 'Save & Arm Automation') }}</span>
          </button>
        </div>
      </form>
    </div>
  </div>
</template>
