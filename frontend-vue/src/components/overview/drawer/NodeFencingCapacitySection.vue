<script setup lang="ts">
import { computed } from 'vue'
import BaseIcon from '../../ui/BaseIcon.vue'
import type { CapacityAdmissionPlan, CircuitBreakerStatus } from '../../../api/dr'

interface Props {
  capacityPlan?: CapacityAdmissionPlan
  circuitBreaker?: CircuitBreakerStatus
  isSingleWorkerCluster: boolean
  overrideBreaker: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:overrideBreaker', val: boolean): void
}>()

const overrideModel = computed({
  get: () => props.overrideBreaker,
  set: (val: boolean) => emit('update:overrideBreaker', val)
})

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B'
  const gib = bytes / (1024 * 1024 * 1024)
  if (gib >= 1) return `${gib.toFixed(1)} GiB`
  const mib = bytes / (1024 * 1024)
  return `${mib.toFixed(0)} MiB`
}

function formatCpu(millis: number): string {
  if (!millis || millis <= 0) return '0m'
  if (millis >= 1000) return `${(millis / 1000).toFixed(1)} cores`
  return `${millis}m`
}
</script>

<template>
  <div class="capacity-section font-mono">
    <!-- Fleet Circuit Breaker Tripped Alert -->
    <div v-if="circuitBreaker?.eviction_frozen" class="circuit-breaker-banner animate-fade-in">
      <div class="circuit-breaker-header">
        <span class="breaker-icon">⛔</span>
        <div>
          <div class="breaker-title">
            Evictions Frozen: Cluster Blast-Radius Limit Exceeded
            ({{ (circuitBreaker.unready_ratio * 100).toFixed(0) }}% nodes unready &gt; 15%)
          </div>
          <div class="breaker-reason text-xs">
            {{ circuitBreaker.freeze_reason || 'Circuit Breaker Tripped — Evictions suspended to prevent cascading fleet partition.' }}
          </div>
        </div>
      </div>
      <label class="breaker-override-toggle">
        <input type="checkbox" v-model="overrideModel" />
        <span>Emergency SRE Override: Force evictions despite circuit breaker trip</span>
      </label>
    </div>

    <!-- 1M+1W Master Headroom & Workload Admission Plan -->
    <div v-if="isSingleWorkerCluster && capacityPlan" class="admission-plan-card">
      <div class="admission-plan-title">
        <BaseIcon name="server" size="xs" />
        <span>1M+1W Master Node Capacity Admission (Target: {{ capacityPlan.master_node_name }})</span>
      </div>

      <!-- Headroom Meters -->
      <div class="headroom-meters-grid">
        <div class="headroom-meter">
          <div class="meter-info">
            <span>RAM Headroom</span>
            <span :class="capacityPlan.headroom_ram_percent < 20 ? 'text-rose' : 'text-emerald'">
              {{ capacityPlan.headroom_ram_percent.toFixed(1) }}% ({{ formatBytes(capacityPlan.available_ram_bytes) }} avail)
            </span>
          </div>
          <div class="meter-bar-track">
            <div
              class="meter-bar-fill"
              :class="capacityPlan.headroom_ram_percent < 20 ? 'meter-danger' : 'meter-safe'"
              :style="{ width: `${Math.min(100, Math.max(5, capacityPlan.headroom_ram_percent))}%` }"
            ></div>
          </div>
        </div>

        <div class="headroom-meter">
          <div class="meter-info">
            <span>CPU Headroom</span>
            <span :class="capacityPlan.headroom_cpu_percent < 20 ? 'text-rose' : 'text-emerald'">
              {{ capacityPlan.headroom_cpu_percent.toFixed(1) }}% ({{ formatCpu(capacityPlan.available_cpu_millis) }} avail)
            </span>
          </div>
          <div class="meter-bar-track">
            <div
              class="meter-bar-fill"
              :class="capacityPlan.headroom_cpu_percent < 20 ? 'meter-danger' : 'meter-safe'"
              :style="{ width: `${Math.min(100, Math.max(5, capacityPlan.headroom_cpu_percent))}%` }"
            ></div>
          </div>
        </div>
      </div>

      <!-- Workload Tiers (Admitted vs Safely Shedded) -->
      <div class="admission-lists-grid">
        <!-- Admitted Workloads -->
        <div class="workload-admission-box admitted">
          <div class="admission-box-title text-emerald">
            <BaseIcon name="check-circle" size="xs" />
            <span>Admitted Workloads ({{ capacityPlan.admitted_pods?.length ?? 0 }})</span>
          </div>
          <ul v-if="capacityPlan.admitted_pods?.length" class="admission-items-list">
            <li v-for="pod in capacityPlan.admitted_pods" :key="pod.name" class="admission-item">
              <span class="pod-name">{{ pod.name }}</span>
              <span class="pod-tier-badge">Tier {{ pod.tier }}: {{ pod.tier_name }}</span>
            </li>
          </ul>
          <div v-else class="text-muted text-xs">No pods scheduled for admission</div>
        </div>

        <!-- Shedded Workloads -->
        <div class="workload-admission-box shedded">
          <div class="admission-box-title text-amber">
            <BaseIcon name="alert-triangle" size="xs" />
            <span>Shedded Non-Critical ({{ capacityPlan.shedded_pods?.length ?? 0 }})</span>
          </div>
          <ul v-if="capacityPlan.shedded_pods?.length" class="admission-items-list">
            <li v-for="pod in capacityPlan.shedded_pods" :key="pod.name" class="admission-item">
              <span class="pod-name">{{ pod.name }}</span>
              <span class="pod-shed-badge">OOM Defense: Suspended</span>
            </li>
          </ul>
          <div v-else class="text-muted text-xs">Zero pods shedded (full admission)</div>
        </div>
      </div>
    </div>
  </div>
</template>
