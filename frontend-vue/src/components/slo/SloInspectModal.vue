<script setup lang="ts">
import { computed } from 'vue'
import type { SLODefinition, SLOSnapshot } from '../../api/compute'

const props = defineProps<{
  show: boolean
  inspectSLO: { def?: SLODefinition; snap?: SLOSnapshot } | null
  actionInProgress?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'triggerAlert', id: string, service: string): void
}>()

function formatPercent(val?: number): string {
  if (val === undefined || val === null || isNaN(val)) return '0.00%'
  const pct = val > 1 ? val : val * 100
  return `${pct.toFixed(2)}%`
}

function getBurnRateColor(rate: number): string {
  if (rate <= 1.0) return 'text-emerald'
  if (rate <= 2.5) return 'text-amber'
  return 'text-rose'
}

const currentBurnRate = computed(() => {
  const r = props.inspectSLO?.snap?.burn_rate
  return (r !== undefined && r !== null && !isNaN(r)) ? r : 0
})

const currentTarget = computed(() => {
  return props.inspectSLO?.def?.target || props.inspectSLO?.snap?.target || 99.9
})

const currentActual = computed(() => {
  return props.inspectSLO?.snap?.actual || currentTarget.value
})

const currentBudget = computed(() => {
  const b = props.inspectSLO?.snap?.error_budget
  return (b !== undefined && b !== null && !isNaN(b)) ? b : 100
})

const targetService = computed(() => {
  return props.inspectSLO?.def?.service || props.inspectSLO?.snap?.service || 'Service'
})

const targetId = computed(() => {
  return props.inspectSLO?.def?.id || props.inspectSLO?.snap?.slo_id || ''
})
</script>

<template>
  <div v-if="show && inspectSLO" class="modal-overlay animate-fade-in" @click.self="emit('update:show', false)">
    <div class="modal-card inspect-modal glass-panel animate-scale-up">
      <div class="modal-header">
        <div class="modal-title-group">
          <span class="modal-icon">🔍</span>
          <div>
            <h2 class="modal-title">SLI Telemetry Inspector: {{ targetService }}</h2>
            <span class="modal-subtitle">PromQL evaluation formula & multi-window compliance breakdown</span>
          </div>
        </div>
        <button class="modal-close-btn" @click="emit('update:show', false)">✕</button>
      </div>

      <div class="modal-body inspect-body">
        <!-- Top Stats Row -->
        <div class="inspect-stats-grid">
          <div class="inspect-stat-card">
            <span class="stat-lbl">Target Objective</span>
            <span class="stat-val font-mono text-emerald">
              {{ formatPercent(currentTarget) }}
            </span>
          </div>
          <div class="inspect-stat-card">
            <span class="stat-lbl">Actual Compliance SLI</span>
            <span class="stat-val font-mono" :class="currentActual >= currentTarget ? 'text-emerald' : 'text-rose'">
              {{ formatPercent(currentActual) }}
            </span>
          </div>
          <div class="inspect-stat-card">
            <span class="stat-lbl">Remaining Error Budget</span>
            <span class="stat-val font-mono" :class="currentBudget > 20 ? 'text-emerald' : currentBudget > 0 ? 'text-amber' : 'text-rose'">
              {{ currentBudget.toFixed(1) }}%
            </span>
          </div>
          <div class="inspect-stat-card">
            <span class="stat-lbl">Burn Rate Multiplier</span>
            <span class="stat-val font-mono" :class="getBurnRateColor(currentBurnRate)">
              {{ currentBurnRate.toFixed(2) }}x
            </span>
          </div>
        </div>

        <!-- PromQL Query Display -->
        <div class="inspect-section">
          <span class="section-tag-label">PromQL Telemetry Stream Query</span>
          <div class="promql-box font-mono">
            <code>{{ inspectSLO.def?.query || 'sum(rate(http_requests_total{status=~"2..|3.."}[5m])) / sum(rate(http_requests_total[5m])) * 100' }}</code>
          </div>
        </div>

        <!-- Error Budget Math Formula -->
        <div class="inspect-section">
          <span class="section-tag-label">Google SRE Error Budget Math Model</span>
          <div class="formula-box font-mono">
            <div class="formula-line">
              <span class="formula-term">Target Error Rate:</span>
              <span class="formula-calc">1.0 - (Target / 100) = {{ ((100 - currentTarget) / 100).toFixed(4) }}</span>
            </div>
            <div class="formula-line">
              <span class="formula-term">Burn Rate:</span>
              <span class="formula-calc">Actual Error Rate / Target Error Rate = {{ currentBurnRate.toFixed(2) }}x</span>
            </div>
            <div class="formula-line">
              <span class="formula-term">Remaining Budget:</span>
              <span class="formula-calc">(1.0 - Consumed Errors / Total Budget) × 100% = {{ currentBudget.toFixed(1) }}%</span>
            </div>
          </div>
        </div>

        <!-- Multi-Window Burn Comparison Table -->
        <div class="inspect-section">
          <span class="section-tag-label">Multi-Window Alerting Thresholds</span>
          <table class="inspect-table font-mono">
            <thead>
              <tr>
                <th>Window</th>
                <th>Alert Severity</th>
                <th>Burn Multiplier</th>
                <th>Budget Impact</th>
                <th>Current State</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td>1 Hour</td>
                <td><span class="badge badge-rose">P1 FAST BURN</span></td>
                <td>14.4x</td>
                <td>2% in 1h</td>
                <td>
                  <span :class="currentBurnRate >= 14.4 ? 'text-rose' : 'text-emerald'">
                    {{ currentBurnRate >= 14.4 ? `ALERT (${currentBurnRate.toFixed(2)}x)` : `NOMINAL (${currentBurnRate.toFixed(2)}x)` }}
                  </span>
                </td>
              </tr>
              <tr>
                <td>6 Hours</td>
                <td><span class="badge badge-amber">P2 SLOW BURN</span></td>
                <td>6.0x</td>
                <td>5% in 6h</td>
                <td>
                  <span :class="currentBurnRate >= 6.0 ? 'text-amber' : 'text-emerald'">
                    {{ currentBurnRate >= 6.0 ? `ALERT (${currentBurnRate.toFixed(2)}x)` : `NOMINAL (${currentBurnRate.toFixed(2)}x)` }}
                  </span>
                </td>
              </tr>
              <tr>
                <td>24 Hours</td>
                <td><span class="badge badge-cyan">P3 COMPOUND</span></td>
                <td>2.0x</td>
                <td>10% in 24h</td>
                <td>
                  <span :class="currentBurnRate >= 2.0 ? 'text-amber' : 'text-emerald'">
                    {{ currentBurnRate >= 2.0 ? `ELEVATED (${currentBurnRate.toFixed(2)}x)` : `NOMINAL (${currentBurnRate.toFixed(2)}x)` }}
                  </span>
                </td>
              </tr>
              <tr>
                <td>30 Days</td>
                <td><span class="badge badge-emerald">BASELINE</span></td>
                <td>1.0x</td>
                <td>100% in 30d</td>
                <td>
                  <span :class="currentBurnRate > 1.0 ? 'text-amber' : 'text-emerald'">
                    {{ currentBurnRate > 1.0 ? 'ELEVATED CONSUMPTION' : 'OPTIMAL COMPLIANCE' }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Modal Footer -->
        <div class="modal-footer">
          <button 
            type="button" 
            class="btn btn-secondary"
            :disabled="actionInProgress"
            @click="emit('triggerAlert', targetId, targetService)"
          >
            <span>⚡ Test Burn Alert</span>
          </button>
          <button type="button" class="btn btn-primary" @click="emit('update:show', false)">
            Close Inspector
          </button>
        </div>
      </div>
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
  max-width: 720px;
  background: #161f30;
  border: 1px solid var(--border-medium);
  border-radius: 12px;
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.6);
  overflow: hidden;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
}

.modal-header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.modal-title-group { display: flex; align-items: center; gap: 10px; }
.modal-icon { font-size: 18px; }
.modal-title { font-size: 15px; font-weight: 700; color: #fff; margin: 0; }
.modal-subtitle { font-size: 11px; color: var(--text-muted); }
.modal-close-btn { background: none; border: none; color: var(--text-muted); cursor: pointer; font-size: 16px; }

.modal-body {
  padding: 20px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.inspect-stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 10px;
}

@media (max-width: 640px) {
  .inspect-stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

.inspect-stat-card {
  background: #0b0f19;
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  padding: 10px 12px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.stat-lbl { font-size: 10px; color: var(--text-muted); text-transform: uppercase; }
.stat-val { font-size: 15px; font-weight: 700; }

.inspect-section { display: flex; flex-direction: column; gap: 6px; }
.section-tag-label { font-size: 11px; font-weight: 700; color: var(--text-muted); text-transform: uppercase; }

.promql-box,
.formula-box {
  background: #0b0f19;
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  padding: 10px 14px;
  font-size: 11px;
}

.formula-box {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.formula-line { display: flex; gap: 8px; }
.formula-term { color: var(--accent-cyan); min-width: 140px; }
.formula-calc { color: var(--text-secondary); }

.inspect-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 11px;
}

.inspect-table th,
.inspect-table td {
  padding: 8px 10px;
  text-align: left;
  border-bottom: 1px solid var(--border-subtle);
}

.inspect-table th {
  color: var(--text-muted);
  font-weight: 600;
  text-transform: uppercase;
  font-size: 10px;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--border-subtle);
}

.text-emerald { color: #10b981; }
.text-amber { color: #f59e0b; }
.text-rose { color: #f43f5e; }
.font-mono { font-family: var(--font-mono); }
</style>