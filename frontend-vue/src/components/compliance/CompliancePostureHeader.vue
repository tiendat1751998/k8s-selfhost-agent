<template>
  <div class="compliance-posture-cockpit">
    <!-- 1. Radial Compliance Framework Scorecards (3 Frameworks) -->
    <div class="framework-cards-grid" role="region" aria-label="Compliance Framework Scorecards">
      <div
        v-for="fw in displayedFrameworks"
        :key="fw.id"
        class="framework-card"
        :class="{ active: selectedFrameworkId === fw.id }"
        role="button"
        tabindex="0"
        :aria-pressed="selectedFrameworkId === fw.id"
        @click="emit('select-framework', selectedFrameworkId === fw.id ? '' : fw.id)"
        @keydown.enter="emit('select-framework', selectedFrameworkId === fw.id ? '' : fw.id)"
        @keydown.space.prevent="emit('select-framework', selectedFrameworkId === fw.id ? '' : fw.id)"
      >
        <!-- SVG Radial Score Ring -->
        <div class="score-ring-wrap" aria-hidden="true">
          <svg class="score-ring-svg" viewBox="0 0 36 36">
            <path
              class="ring-bg"
              d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
            />
            <path
              class="ring-progress"
              :stroke="getScoreRingColor(fw.score)"
              :stroke-dasharray="`${Math.min(Math.max(fw.score, 0), 100)}, 100`"
              d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
            />
            <text x="18" y="20.5" class="ring-text">{{ Math.round(fw.score) }}%</text>
          </svg>
        </div>

        <!-- Framework Content & Telemetry -->
        <div class="fw-details">
          <div class="fw-header-row">
            <span class="fw-name font-bold" :title="fw.name">{{ fw.name }}</span>
            <span v-if="selectedFrameworkId === fw.id" class="fw-active-badge">ACTIVE</span>
          </div>
          <div class="fw-checks font-mono">
            <span class="check-pass">{{ fw.passed_checks }} pass</span>
            <span class="check-sep text-muted">/</span>
            <span class="check-fail">{{ fw.failed_checks }} fail</span>
            <span class="check-total text-muted">({{ fw.total_checks }})</span>
          </div>
          <div class="fw-meta font-mono text-muted">
            Last scan: {{ formatScanDate(fw.last_scan_at) }}
          </div>
        </div>
      </div>
    </div>

    <!-- 2. Proportional Severity Breakdown Bar & Filters -->
    <div class="posture-severity-section">
      <div class="severity-bar-header">
        <div class="bar-title">
          <span class="bar-title-text font-bold">Posture Risk Distribution</span>
          <span class="bar-subtitle text-muted font-mono">
            ({{ distribution.total }} open defects across controls & CVEs)
          </span>
        </div>
        <div class="header-actions">
          <button
            type="button"
            class="btn-cockpit-action"
            :disabled="loading"
            title="Refresh compliance telemetry"
            @click="emit('refresh')"
          >
            <BaseIcon :name="loading ? 'clock' : 'refresh'" size="xs" />
            <span>{{ loading ? 'Syncing...' : 'Sync' }}</span>
          </button>
          <button
            type="button"
            class="btn-cockpit-action btn-scan-action"
            :disabled="isScanning"
            title="Trigger compliance scan"
            @click="emit('trigger-scan')"
          >
            <BaseIcon name="zap" size="xs" />
            <span>{{ isScanning ? 'Scanning...' : 'Scan Now' }}</span>
          </button>
        </div>
      </div>

      <!-- Multi-Segment Horizontal Bar -->
      <div class="proportional-bar" role="progressbar" :aria-valuenow="distribution.total" aria-valuemin="0">
        <div
          v-if="distribution.critical > 0"
          class="bar-segment bar-critical"
          :style="{ width: `${distribution.criticalPct}%` }"
          :title="`Critical: ${distribution.critical} (${distribution.criticalPct.toFixed(1)}%)`"
        />
        <div
          v-if="distribution.high > 0"
          class="bar-segment bar-high"
          :style="{ width: `${distribution.highPct}%` }"
          :title="`High: ${distribution.high} (${distribution.highPct.toFixed(1)}%)`"
        />
        <div
          v-if="distribution.medium > 0"
          class="bar-segment bar-medium"
          :style="{ width: `${distribution.mediumPct}%` }"
          :title="`Medium: ${distribution.medium} (${distribution.mediumPct.toFixed(1)}%)`"
        />
        <div
          v-if="distribution.low > 0"
          class="bar-segment bar-low"
          :style="{ width: `${distribution.lowPct}%` }"
          :title="`Low: ${distribution.low} (${distribution.lowPct.toFixed(1)}%)`"
        />
        <div
          v-if="distribution.total === 0"
          class="bar-segment bar-clean"
          style="width: 100%;"
          title="All controls compliant"
        />
      </div>

      <!-- Interactive Severity Filter Pills -->
      <div class="severity-pills-row" role="group" aria-label="Filter by severity">
        <button
          type="button"
          class="sev-pill"
          :class="{ active: activeSeverity === 'ALL' }"
          @click="emit('filter-severity', 'ALL')"
        >
          <span>All Severities ({{ distribution.total }})</span>
        </button>
        <button
          type="button"
          class="sev-pill pill-critical"
          :class="{ active: activeSeverity === 'CRITICAL' }"
          @click="emit('filter-severity', activeSeverity === 'CRITICAL' ? 'ALL' : 'CRITICAL')"
        >
          <span class="sev-dot dot-critical" />
          <span>Critical ({{ distribution.critical }})</span>
        </button>
        <button
          type="button"
          class="sev-pill pill-high"
          :class="{ active: activeSeverity === 'HIGH' }"
          @click="emit('filter-severity', activeSeverity === 'HIGH' ? 'ALL' : 'HIGH')"
        >
          <span class="sev-dot dot-high" />
          <span>High ({{ distribution.high }})</span>
        </button>
        <button
          type="button"
          class="sev-pill pill-medium"
          :class="{ active: activeSeverity === 'MEDIUM' }"
          @click="emit('filter-severity', activeSeverity === 'MEDIUM' ? 'ALL' : 'MEDIUM')"
        >
          <span class="sev-dot dot-medium" />
          <span>Medium ({{ distribution.medium }})</span>
        </button>
        <button
          type="button"
          class="sev-pill pill-low"
          :class="{ active: activeSeverity === 'LOW' }"
          @click="emit('filter-severity', activeSeverity === 'LOW' ? 'ALL' : 'LOW')"
        >
          <span class="sev-dot dot-low" />
          <span>Low ({{ distribution.low }})</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import type { ComplianceFramework, ComplianceViolation, AuditFinding } from '../../api/governance'
import {
  type SeverityFilter,
  computeSeverityDistribution,
  getScoreRingColor,
} from '../../composables/useCompliance'
import '../../assets/styles/views/compliance.css'

const props = withDefaults(
  defineProps<{
    frameworks: ComplianceFramework[]
    violations: ComplianceViolation[]
    findings: AuditFinding[]
    selectedFrameworkId?: string
    activeSeverity?: SeverityFilter
    loading?: boolean
    isScanning?: boolean
  }>(),
  {
    selectedFrameworkId: '',
    activeSeverity: 'ALL',
    loading: false,
    isScanning: false,
  }
)

const emit = defineEmits<{
  (e: 'select-framework', id: string): void
  (e: 'filter-severity', sev: SeverityFilter): void
  (e: 'trigger-scan'): void
  (e: 'refresh'): void
}>()

const displayedFrameworks = computed(() => {
  return props.frameworks && props.frameworks.length > 0 ? props.frameworks : []
})

const distribution = computed(() => {
  return computeSeverityDistribution(props.violations, props.findings)
})

function formatScanDate(d?: string): string {
  if (!d) return 'Continuous'
  try {
    const dt = new Date(d)
    return dt.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) + ' ' + dt.toLocaleDateString([], { month: 'short', day: 'numeric' })
  } catch {
    return d
  }
}
</script>
