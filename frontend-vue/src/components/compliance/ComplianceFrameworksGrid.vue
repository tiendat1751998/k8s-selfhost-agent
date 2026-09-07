<template>
  <div class="frameworks-section">
    <!-- Quick Filters Bar for Standards -->
    <div class="framework-filters-bar glass-panel">
      <span class="filter-label">Regulatory Standard:</span>
      <div class="standard-chips">
        <button
          v-for="std in standards"
          :key="std"
          class="filter-pill"
          :class="{ 'filter-active': selectedStandard === std }"
          @click="$emit('select-standard', std)"
        >
          {{ std }}
        </button>
      </div>
    </div>

    <!-- Framework Score Cards Grid -->
    <div v-if="frameworks.length > 0" class="frameworks-grid">
      <div
        v-for="fw in frameworks"
        :key="fw.id"
        class="framework-card glass-panel glass-panel-glow"
        :class="{ 'framework-card-selected': selectedFrameworkId === fw.id }"
        @click="$emit('select-framework', fw.id)"
      >
        <div class="fw-header">
          <div class="fw-icon-box">{{ fw.icon || '🛡️' }}</div>
          <div class="fw-title-group">
            <h3 class="fw-name">{{ fw.name }}</h3>
            <span class="fw-last-scan text-muted font-mono">Scanned: {{ formatDate(fw.last_scan_at) }}</span>
          </div>
          <!-- Percentage Circular Gauge HUD -->
          <div class="gauge-wrapper" :title="`${fw.score.toFixed(1)}% compliance score`">
            <svg class="gauge-svg" viewBox="0 0 36 36">
              <path
                class="gauge-bg"
                d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
              />
              <path
                class="gauge-progress"
                :class="getGaugeStrokeClass(fw.score)"
                :stroke-dasharray="`${Math.min(100, Math.max(0, fw.score))}, 100`"
                d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
              />
            </svg>
            <span class="gauge-text font-mono">{{ fw.score.toFixed(0) }}%</span>
          </div>
        </div>

        <div class="fw-progress-wrapper">
          <div class="progress-bar-bg">
            <div
              class="progress-bar-fill"
              :style="{ width: `${Math.min(100, Math.max(0, fw.score))}%` }"
              :class="getProgressColorClass(fw.score)"
            ></div>
          </div>
        </div>

        <div class="fw-stats-row font-mono">
          <div class="fw-stat">
            <span class="stat-k">Passed:</span>
            <span class="stat-v text-emerald">{{ fw.passed_checks }}</span>
          </div>
          <div class="fw-stat">
            <span class="stat-k">Failed:</span>
            <span class="stat-v text-rose">{{ fw.failed_checks }}</span>
          </div>
          <div class="fw-stat">
            <span class="stat-k">Total Checks:</span>
            <span class="stat-v">{{ fw.total_checks }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Empty Frameworks State -->
    <div v-else-if="!loading" class="empty-frameworks-box glass-panel">
      <span class="empty-icon">🛡️</span>
      <h3 class="empty-title">No Compliance Frameworks Found</h3>
      <p class="empty-desc">
        No matching regulatory benchmarks (CIS Benchmark, NIST SP 800-53, PCI-DSS, SOC 2, HIPAA) detected.
      </p>
      <button class="btn btn-secondary btn-sm" @click="$emit('run-scan')">
        <span>🔄 Run Compliance Scan</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { COMPLIANCE_STANDARDS, type ComplianceStandard } from '../../composables/useCompliance'
import type { ComplianceFramework } from '../../api/governance'

const standards = COMPLIANCE_STANDARDS

defineProps<{
  frameworks: ComplianceFramework[]
  selectedFrameworkId: string
  selectedStandard: ComplianceStandard
  loading: boolean
  getProgressColorClass: (score: number) => string
  formatDate: (d: string) => string
}>()

defineEmits<{
  (e: 'select-framework', id: string): void
  (e: 'select-standard', standard: ComplianceStandard): void
  (e: 'run-scan'): void
}>()

function getGaugeStrokeClass(score: number): string {
  if (score >= 90) return 'stroke-emerald'
  if (score >= 75) return 'stroke-amber'
  return 'stroke-rose'
}
</script>
