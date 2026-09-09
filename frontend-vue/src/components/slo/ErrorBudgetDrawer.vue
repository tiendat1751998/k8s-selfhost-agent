<script setup lang="ts">
import type { InspectSLOState, LatencyPercentiles } from '../../composables/useSLOMonitor'

const props = defineProps<{
  show: boolean
  selectedInspect: InspectSLOState | null
  actionInProgress: boolean
  formatPercent: (val?: number) => string
  getEffectiveBurnRate: (rate?: number) => number
  getBurnRateColor: (rate: number) => string
  calculateBurnRateVelocity: (rate: number, window?: string) => { hoursToExhaustion: number; statusText: string }
  getTargetLatencyPercentiles: (target?: number) => LatencyPercentiles
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'trigger-alert', defId: string, serviceName: string): void
}>()
</script>

<template>
  <div v-if="show && selectedInspect" class="modal-overlay animate-fade-in" @click.self="emit('close')">
    <div class="modal-card inspect-modal glass-panel animate-scale-up">
      <div class="modal-header">
        <div class="modal-title-group">
          <span class="modal-icon">🔍</span>
          <div>
            <h2 class="modal-title">SLI Telemetry Inspector: {{ selectedInspect.def?.service || selectedInspect.snap?.service }}</h2>
            <span class="modal-subtitle">PromQL evaluation formula & multi-window compliance breakdown</span>
          </div>
        </div>
        <button class="modal-close-btn" @click="emit('close')">✕</button>
      </div>

      <div class="modal-body inspect-body">
        <!-- Top Stats Row -->
        <div class="inspect-stats-grid">
          <div class="inspect-stat-card">
            <span class="stat-lbl">Target Objective</span>
            <span class="stat-val font-mono text-emerald">
              {{ formatPercent(selectedInspect.def?.target || selectedInspect.snap?.target) }}
            </span>
          </div>
          <div class="inspect-stat-card">
            <span class="stat-lbl">Actual Compliance SLI</span>
            <span class="stat-val font-mono" :class="(selectedInspect.snap?.actual || 0) >= (selectedInspect.def?.target || 0) ? 'text-emerald' : 'text-rose'">
              {{ formatPercent(selectedInspect.snap?.actual || selectedInspect.def?.target) }}
            </span>
          </div>
          <div class="inspect-stat-card">
            <span class="stat-lbl">Remaining Error Budget</span>
            <span class="stat-val font-mono text-cyan">
              {{ (selectedInspect.snap?.error_budget ?? 85.0).toFixed(1) }}%
            </span>
          </div>
          <div class="inspect-stat-card">
            <span class="stat-lbl">Burn Rate Multiplier</span>
            <span class="stat-val font-mono" :class="getBurnRateColor(getEffectiveBurnRate(selectedInspect.snap?.burn_rate ?? 0.8))">
              {{ getEffectiveBurnRate(selectedInspect.snap?.burn_rate ?? 0.8).toFixed(2) }}x
            </span>
          </div>
        </div>

        <!-- Target Latency Percentiles Breakdown -->
        <div class="inspect-section">
          <span class="section-tag-label">Target Latency Percentiles (SLA Model)</span>
          <div class="latency-percentiles-grid font-mono">
            <div class="percentile-box">
              <span class="percentile-name">P50 Median</span>
              <span class="percentile-value text-emerald">{{ getTargetLatencyPercentiles(selectedInspect.def?.target || 99.9).p50 }} ms</span>
            </div>
            <div class="percentile-box">
              <span class="percentile-name">P90 Standard</span>
              <span class="percentile-value text-cyan">{{ getTargetLatencyPercentiles(selectedInspect.def?.target || 99.9).p90 }} ms</span>
            </div>
            <div class="percentile-box">
              <span class="percentile-name">P99 Strict</span>
              <span class="percentile-value text-amber">{{ getTargetLatencyPercentiles(selectedInspect.def?.target || 99.9).p99 }} ms</span>
            </div>
            <div class="percentile-box">
              <span class="percentile-name">P99.9 Extreme</span>
              <span class="percentile-value text-rose">{{ getTargetLatencyPercentiles(selectedInspect.def?.target || 99.9).p999 }} ms</span>
            </div>
          </div>
        </div>

        <!-- PromQL Query Display -->
        <div class="inspect-section">
          <span class="section-tag-label">PromQL Telemetry Stream Query</span>
          <div class="promql-box font-mono">
            <code>{{ selectedInspect.def?.query || 'sum(rate(http_requests_total{status=~"2..|3.."}[5m])) / sum(rate(http_requests_total[5m])) * 100' }}</code>
          </div>
        </div>

        <!-- Error Budget Math Formula -->
        <div class="inspect-section">
          <span class="section-tag-label">Error Budget & Burn Math Model</span>
          <div class="formula-box font-mono">
            <div class="formula-line">
              <span class="formula-term">Target Error Rate:</span>
              <span class="formula-calc">1.0 - (Target / 100) = {{ ((100 - (selectedInspect.def?.target || 99.9)) / 100).toFixed(4) }}</span>
            </div>
            <div class="formula-line">
              <span class="formula-term">Burn Rate:</span>
              <span class="formula-calc">Actual Error Rate / Target Error Rate = {{ (selectedInspect.snap?.burn_rate ?? 0.85).toFixed(2) }}x</span>
            </div>
            <div class="formula-line">
              <span class="formula-term">Remaining Budget:</span>
              <span class="formula-calc">(1.0 - Consumed Errors / Budget) × 100% = {{ (selectedInspect.snap?.error_budget ?? 85.0).toFixed(1) }}%</span>
            </div>
            <div class="formula-line">
              <span class="formula-term">Depletion Velocity:</span>
              <span class="formula-calc text-amber">{{ calculateBurnRateVelocity(selectedInspect.snap?.burn_rate ?? 0.85, selectedInspect.def?.window || '30d').statusText }} (~{{ calculateBurnRateVelocity(selectedInspect.snap?.burn_rate ?? 0.85, selectedInspect.def?.window || '30d').hoursToExhaustion }}h)</span>
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
                <th>Severity</th>
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
                <td><span class="text-emerald">NOMINAL (0.85x)</span></td>
              </tr>
              <tr>
                <td>6 Hours</td>
                <td><span class="badge badge-amber">P2 SLOW BURN</span></td>
                <td>6.0x</td>
                <td>5% in 6h</td>
                <td><span class="text-emerald">NOMINAL (0.85x)</span></td>
              </tr>
              <tr>
                <td>24 Hours</td>
                <td><span class="badge badge-cyan">P3 COMPOUND</span></td>
                <td>2.0x</td>
                <td>10% in 24h</td>
                <td><span class="text-emerald">NOMINAL (0.85x)</span></td>
              </tr>
              <tr>
                <td>30 Days</td>
                <td><span class="badge badge-emerald">BASELINE</span></td>
                <td>1.0x</td>
                <td>100% in 30d</td>
                <td><span class="text-emerald">OPTIMAL COMPLIANCE</span></td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Modal Footer -->
        <div class="modal-footer" style="padding-top: 16px;">
          <button 
            type="button" 
            class="btn btn-warning"
            :disabled="actionInProgress"
            @click="emit('trigger-alert', selectedInspect.def?.id || selectedInspect.snap?.slo_id || '', selectedInspect.def?.service || selectedInspect.snap?.service || '')"
          >
            <span>⚡ Trigger Test Burn Alert</span>
          </button>
          <button type="button" class="btn btn-secondary" @click="emit('close')">
            Close Inspector
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
