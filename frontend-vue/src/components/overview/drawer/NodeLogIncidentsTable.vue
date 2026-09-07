<script setup lang="ts">
interface Props {
  incidents: any[]
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'select-incident', inc: any): void
}>()
</script>

<template>
  <div class="incidents-table-wrapper mt-4">
    <h5 class="inc-subheading">🚨 Automated Anomaly & Incident Snapshots</h5>
    <table class="incidents-mini-table">
      <thead>
        <tr>
          <th>Type & Severity</th>
          <th>Message & Root Cause</th>
          <th>Pre-Crash State</th>
          <th>Status</th>
          <th>Action</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="inc in incidents" :key="inc.id" class="inc-row">
          <td>
            <div class="inc-type-cell">
              <span class="badge" :class="inc.severity === 'critical' ? 'badge-rose' : inc.severity === 'high' ? 'badge-amber' : 'badge-indigo'">
                {{ inc.severity?.toUpperCase() || 'HIGH' }}
              </span>
              <strong class="inc-type-name font-mono">{{ inc.type }}</strong>
            </div>
          </td>
          <td class="col-inc-msg">
            <div class="inc-msg-text">{{ inc.message }}</div>
            <div class="inc-meta-tags font-mono" v-if="inc.raw_data">
              <span v-if="inc.raw_data.top_processes" class="inc-tag">Has Process Dump</span>
              <span v-if="inc.raw_data.container_count" class="inc-tag">{{ inc.raw_data.container_count }} containers</span>
            </div>
          </td>
          <td class="font-mono text-slate">
            <div v-if="inc.raw_data?.pre_incident_cpu">CPU: {{ inc.raw_data.pre_incident_cpu }}</div>
            <div v-if="inc.raw_data?.pre_incident_memory">RAM: {{ inc.raw_data.pre_incident_memory }}</div>
            <span v-if="!inc.raw_data?.pre_incident_cpu && !inc.raw_data?.pre_incident_memory" class="text-muted">—</span>
          </td>
          <td>
            <span class="badge" :class="inc.status === 'resolved' ? 'badge-emerald' : inc.status === 'remediating' ? 'badge-cyan' : 'badge-amber'">
              {{ (inc.status || 'open').toUpperCase() }}
            </span>
          </td>
          <td>
            <button class="btn btn-xs btn-secondary font-mono" @click="emit('select-incident', inc)">
              🔍 Logs
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
@import '../../../assets/styles/components/node-log-terminal.css';
</style>
