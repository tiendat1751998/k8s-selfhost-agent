<script setup lang="ts">
import type { BackupPolicy } from '../../api/governance'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  policies: BackupPolicy[]
  triggeringPolicyId?: string | null
}>()

const emit = defineEmits<{
  (e: 'create'): void
  (e: 'trigger', policyId: string): void
}>()

function getDbIcon(type: string): string {
  const t = (type || '').toLowerCase()
  if (t.includes('postgres')) return '🐘'
  if (t.includes('mysql')) return '🐬'
  if (t.includes('maria')) return '🦭'
  if (t.includes('mongo')) return '🍃'
  if (t.includes('redis')) return '⚡'
  if (t.includes('nats')) return '📬'
  return '📦'
}
</script>

<template>
  <div class="schedules-view-wrapper animate-fade-in">
    <div v-if="policies.length > 0" class="policies-grid">
      <div
        v-for="policy in policies"
        :key="policy.id"
        class="policy-card glass-panel"
        :class="{ 'glass-panel-glow': policy.enabled }"
      >
        <div class="policy-card-top">
          <div class="policy-icon-box">{{ getDbIcon(policy.db_type) }}</div>
          <div class="policy-meta">
            <h3 class="policy-name">{{ policy.name }}</h3>
            <span class="policy-sub font-mono text-muted">
              {{ policy.db_type.toUpperCase() }} @ {{ policy.db_host }}:{{ policy.db_port }}
            </span>
          </div>
          <StatusBadge
            :status="policy.enabled ? 'active' : 'idle'"
            :label="policy.enabled ? 'ARMED' : 'PAUSED'"
            size="sm"
          />
        </div>

        <div class="policy-body">
          <div class="policy-stat-row">
            <span class="policy-k">Target Database:</span>
            <span class="policy-v font-mono text-cyan">{{ policy.db_name }}</span>
          </div>
          <div class="policy-stat-row">
            <span class="policy-k">Cron Schedule:</span>
            <span class="policy-v font-mono">{{ policy.schedule }}</span>
          </div>
          <div class="policy-stat-row">
            <span class="policy-k">Retention Count:</span>
            <span class="policy-v font-mono">{{ policy.retention_count }} snapshots</span>
          </div>
          <div class="policy-stat-row">
            <span class="policy-k">Backup Type:</span>
            <span class="policy-v font-mono text-emerald">{{ (policy.backup_type || 'full').toUpperCase() }}</span>
          </div>
        </div>

        <div class="policy-actions">
          <button
            class="btn btn-primary btn-sm"
            :disabled="triggeringPolicyId === policy.id"
            @click="emit('trigger', policy.id)"
          >
            <span>{{ triggeringPolicyId === policy.id ? '⚡ Dispatching...' : '⚡ Backup Now' }}</span>
          </button>
        </div>
      </div>
    </div>

    <div v-else class="empty-state-box glass-panel">
      <span class="empty-icon">📋</span>
      <h3 class="empty-title">No Backup Policies Configured</h3>
      <p class="empty-desc">Create your first automated database policy to protect workloads across clusters.</p>
      <button class="btn btn-primary" @click="emit('create')">
        <span>+ Create Backup Policy</span>
      </button>
    </div>
  </div>
</template>
