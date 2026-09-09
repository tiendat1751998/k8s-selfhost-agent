<script setup lang="ts">
import { ref } from 'vue'
import type { Report } from '../../api/management'
import type { ReportSchedule } from '../../composables/useReports'
import ModalDrawer from '../ui/ModalDrawer.vue'

defineProps<{
  show: boolean
  isSubmitting?: boolean
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  save: [schedule: Omit<ReportSchedule, 'id' | 'createdAt'>]
  close: []
}>()

const title = ref('Weekly Executive Infrastructure & FinOps Digest')
const type = ref<Report['type']>('compliance')
const format = ref<Report['format']>('pdf')
const cadencePreset = ref('weekly')
const customCron = ref('0 8 * * 1')
const recipientsInput = ref('ciso@enterprise.io, platform-leads@enterprise.io')
const clusterScope = ref('all-clusters')

function handleClose() {
  emit('close')
  emit('update:show', false)
}

function handleSave() {
  if (!title.value.trim()) return

  let cron = '0 8 * * 1'
  let cronLabel = 'Weekly (Every Monday @ 08:00 UTC)'

  if (cadencePreset.value === 'daily') {
    cron = '0 6 * * *'
    cronLabel = 'Daily Digest (@ 06:00 UTC)'
  } else if (cadencePreset.value === 'monthly') {
    cron = '0 0 1 * *'
    cronLabel = 'Monthly on 1st (@ 00:00 UTC)'
  } else if (cadencePreset.value === 'custom') {
    cron = customCron.value.trim() || '0 8 * * 1'
    cronLabel = `Custom Cron (${cron})`
  }

  const recipients = recipientsInput.value
    .split(',')
    .map(email => email.trim())
    .filter(Boolean)

  emit('save', {
    title: title.value.trim(),
    type: type.value,
    format: format.value,
    cron,
    cronLabel,
    recipients,
    clusterScope: clusterScope.value,
    enabled: true
  })

  handleClose()
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="Automate Scheduled Report Cadence"
    subtitle="Configure autonomous cron execution, executive digest generation, and email dispatch."
    @close="handleClose"
  >
    <form @submit.prevent="handleSave" class="form-layout">
      <div class="form-group">
        <label>Schedule Name & Executive Subject</label>
        <input 
          v-model="title" 
          type="text" 
          placeholder="e.g. Weekly Executive Infrastructure & FinOps Digest" 
          class="input-glass" 
          required 
        />
      </div>

      <div class="form-row">
        <div class="form-group">
          <label>Report Template / Category</label>
          <select v-model="type" class="input-glass">
            <option value="compliance">Compliance (CIS Benchmark & SOC2)</option>
            <option value="cost">FinOps & Cost Optimization</option>
            <option value="operational">Operational & Cluster Topology</option>
            <option value="security">DevSecOps & Vulnerability Digest</option>
            <option value="incident">Incident & Post-Mortem Log</option>
          </select>
        </div>

        <div class="form-group">
          <label>Export Format</label>
          <select v-model="format" class="input-glass">
            <option value="pdf">PDF (Executive Signed Document)</option>
            <option value="csv">CSV (Raw Telemetry Export)</option>
            <option value="excel">Excel (Data Pivot Sheet)</option>
          </select>
        </div>
      </div>

      <div class="form-row">
        <div class="form-group">
          <label>Automated Cadence</label>
          <select v-model="cadencePreset" class="input-glass">
            <option value="weekly">Weekly (Every Monday @ 08:00 UTC)</option>
            <option value="daily">Daily Digest (@ 06:00 UTC)</option>
            <option value="monthly">Monthly on 1st (@ 00:00 UTC)</option>
            <option value="custom">Custom Cron Expression</option>
          </select>
        </div>

        <div class="form-group" v-if="cadencePreset === 'custom'">
          <label>Cron Expression (5-Field)</label>
          <input 
            v-model="customCron" 
            type="text" 
            placeholder="e.g. 0 8 * * 1" 
            class="input-glass font-mono" 
          />
        </div>

        <div class="form-group" v-else>
          <label>Cluster Scope</label>
          <select v-model="clusterScope" class="input-glass">
            <option value="all-clusters">All Clusters (Global Mesh)</option>
            <option value="prod-us-east-1">prod-us-east-1 (Primary)</option>
            <option value="prod-eu-west-1">prod-eu-west-1 (Secondary)</option>
          </select>
        </div>
      </div>

      <div class="form-group">
        <label>Recipient Email Addresses (Comma Separated)</label>
        <input 
          v-model="recipientsInput" 
          type="text" 
          placeholder="ciso@enterprise.io, sre-lead@enterprise.io" 
          class="input-glass" 
          required 
        />
        <span class="help-text">Automated dispatch via zero-trust SMTP TLS gateway.</span>
      </div>
    </form>

    <template #footer="{ close }">
      <button class="btn btn-secondary" type="button" @click="close">Cancel</button>
      <button class="btn btn-primary" :disabled="isSubmitting" @click="handleSave">
        {{ isSubmitting ? 'Saving Cadence...' : 'Save & Arm Schedule' }}
      </button>
    </template>
  </ModalDrawer>
</template>
