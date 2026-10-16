<script setup lang="ts">
import { computed } from 'vue'
import { useTrendChart } from '../../../composables/useTrendChart'
import BaseIcon from '../../ui/BaseIcon.vue'

export interface TrendPoint {
  time: string
  cpu: number
  mem: number
  disk: number
  reqs: number
}

interface Props {
  trendHistory: TrendPoint[]
  effectiveHttpRps?: number
  httpActiveConns?: number
  httpQueuedReqs?: number
  clusterAvgLatencyMs?: number
  httpErrorRate?: number
}

const props = withDefaults(defineProps<Props>(), {
  effectiveHttpRps: 0,
  httpActiveConns: 0,
  httpQueuedReqs: 0,
  clusterAvgLatencyMs: 0,
  httpErrorRate: 0,
})

const emit = defineEmits<{
  (e: 'openDeepDive'): void
}>()

const {
  latestCpu,
  latestMem,
  latestReqs,
  trendChartMaxReqs,
  trendChartCpuPath,
  trendChartCpuArea,
  trendChartMemPath,
  trendChartMemArea,
  trendChartReqsPath,
  trendChartReqsArea,
  trendTimeAxisMarkers,
  hoveredPoint,
  hoveredElapsed,
  hoverCoords,
  tooltipStyle,
  handleChartHover,
  handleChartLeave,
  formatMetricRate,
} = useTrendChart({
  trendHistory: computed(() => props.trendHistory),
  effectiveHttpRps: computed(() => props.effectiveHttpRps),
})
</script>

<template>
  <section class="trend-chart-card glass-panel trend-card-clickable" @click="emit('openDeepDive')">
    <div class="trend-chart-header">
      <div class="trend-title-wrap">
        <div class="trend-title-top-row">
          <div class="trend-title-left">
            <h3 class="sidebar-card-title">
              <span class="title-full">
                <BaseIcon name="trending-up" size="sm" /> 5-Min Saturation Trends
              </span>
              <span class="title-mobile">
                <BaseIcon name="trending-up" size="sm" /> Trends
              </span>
            </h3>
            <span class="badge badge-muted font-mono">LIVE BUFFER</span>
          </div>
        </div>
        <span class="trend-chart-subtitle">
          Rolling 30-sample sliding window across CPU, RAM, and gateway RPS
        </span>
      </div>

      <div class="trend-header-actions">
        <!-- Gateway Ingress Metrics Pills Group -->
        <div class="trend-ingress-metrics font-mono">
          <span class="trend-metric-pill" title="Active HTTP Connections">
            <span class="pill-icon">
              <BaseIcon name="globe" size="xs" />
            </span>
            <span class="pill-val">{{ httpActiveConns.toLocaleString() }}</span>
            <span class="pill-lbl">active</span>
          </span>

          <span
            class="trend-metric-pill"
            :class="{ 'pill-warning': httpQueuedReqs > 0 }"
            title="Queued Gateway Requests"
          >
            <span class="pill-icon">
              <BaseIcon name="activity" size="xs" />
            </span>
            <span class="pill-val">{{ httpQueuedReqs.toLocaleString() }}</span>
            <span class="pill-lbl">queued</span>
          </span>

          <span class="trend-metric-pill" title="Cluster Average Latency">
            <span class="pill-icon">
              <BaseIcon name="activity" size="xs" />
            </span>
            <span class="pill-val">{{ clusterAvgLatencyMs > 0 ? clusterAvgLatencyMs.toFixed(1) + 'ms' : '--' }}</span>
            <span class="pill-lbl">latency</span>
          </span>

          <span
            class="trend-metric-pill"
            :class="{ 'pill-critical': httpErrorRate >= 5, 'pill-warning': httpErrorRate > 0 && httpErrorRate < 5 }"
            title="HTTP Error Rate"
          >
            <span class="pill-icon">
              <BaseIcon :name="httpErrorRate >= 5 ? 'x-circle' : httpErrorRate > 0 ? 'alert-triangle' : 'shield'" size="xs" />
            </span>
            <span class="pill-val">{{ httpErrorRate.toFixed(1) }}%</span>
            <span class="pill-lbl">err</span>
          </span>
        </div>

        <!-- Legend Pills Group -->
        <div class="trend-legend font-mono">
          <span class="legend-pill pill-cpu">
            <span class="legend-dot-circle bg-blue"></span>
            <span>CPU ({{ Math.round(latestCpu) }}%)</span>
          </span>
          <span class="legend-pill pill-mem">
            <span class="legend-dot-circle bg-sky"></span>
            <span>RAM ({{ Math.round(latestMem) }}%)</span>
          </span>
          <span class="legend-pill pill-reqs">
            <span class="legend-dot-circle bg-emerald"></span>
            <span><span class="legend-name-full">Throughput </span>({{ Math.round(latestReqs).toLocaleString() }} req/s)</span>
          </span>
        </div>

        <!-- Single Clean Deep-Dive Action Button (Desktop) -->
        <button
          class="trend-expand-badge font-mono trend-btn-desktop"
          type="button"
          @click.stop="emit('openDeepDive')"
          title="Click to open cluster telemetry deep-dive modal"
        >
          <BaseIcon name="search" size="xs" />
          <span>Deep-Dive</span>
        </button>
      </div>
    </div>

    <!-- HTML-Overlay Grid Layout Chart with Zero Distortion -->
    <div class="trend-chart-frame">
      <div class="trend-chart-body">
        <!-- Left Y-Axis (Percentages) -->
        <div class="trend-y-axis y-axis-left font-mono">
          <span class="y-tick">100%</span>
          <span class="y-tick">75%</span>
          <span class="y-tick">50%</span>
          <span class="y-tick">25%</span>
          <span class="y-tick">0%</span>
        </div>

        <!-- Chart Plot Canvas -->
        <div
          class="trend-plot-canvas"
          @mousemove="handleChartHover"
          @mouseleave="handleChartLeave"
        >
          <svg viewBox="0 0 1000 200" preserveAspectRatio="none" class="trend-plot-svg">
            <defs>
              <linearGradient id="trendCpuGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                <stop offset="0%" stop-color="#3b82f6" stop-opacity="0.08" />
                <stop offset="100%" stop-color="#3b82f6" stop-opacity="0.0" />
              </linearGradient>
              <linearGradient id="trendMemGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                <stop offset="0%" stop-color="#0284c7" stop-opacity="0.08" />
                <stop offset="100%" stop-color="#0284c7" stop-opacity="0.0" />
              </linearGradient>
              <linearGradient id="trendReqsGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                <stop offset="0%" stop-color="#10b981" stop-opacity="0.08" />
                <stop offset="100%" stop-color="#10b981" stop-opacity="0.0" />
              </linearGradient>
            </defs>

            <!-- 5 Horizontal Grid Lines -->
            <line x1="0" y1="10" x2="1000" y2="10" stroke="var(--color-border-subtle)" stroke-dasharray="4 4" />
            <line x1="0" y1="56" x2="1000" y2="56" stroke="var(--color-border-subtle)" stroke-dasharray="4 4" />
            <line x1="0" y1="102" x2="1000" y2="102" stroke="var(--color-border-subtle)" stroke-dasharray="4 4" />
            <line x1="0" y1="148" x2="1000" y2="148" stroke="var(--color-border-subtle)" stroke-dasharray="4 4" />
            <line x1="0" y1="194" x2="1000" y2="194" stroke="var(--color-border-subtle)" stroke-width="1" />

            <!-- Series 1: CPU Area & Line (Enterprise Sapphire Blue #3b82f6) -->
            <path v-if="trendChartCpuArea" :d="trendChartCpuArea" fill="url(#trendCpuGrad)" />
            <path v-if="trendChartCpuPath" :d="trendChartCpuPath" fill="none" stroke="#3b82f6" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />

            <!-- Series 2: RAM Area & Line (Slate/Sky #0284c7) -->
            <path v-if="trendChartMemArea" :d="trendChartMemArea" fill="url(#trendMemGrad)" />
            <path v-if="trendChartMemPath" :d="trendChartMemPath" fill="none" stroke="#0284c7" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />

            <!-- Series 3: Throughput Area & Line (Refined Emerald #10b981) -->
            <path v-if="trendChartReqsArea" :d="trendChartReqsArea" fill="url(#trendReqsGrad)" />
            <path v-if="trendChartReqsPath" :d="trendChartReqsPath" fill="none" stroke="#10b981" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />

            <!-- Interactive Hover Crosshair & Series Markers -->
            <g v-if="hoverCoords">
              <line
                :x1="hoverCoords.x"
                y1="10"
                :x2="hoverCoords.x"
                y2="194"
                stroke="rgba(255, 255, 255, 0.25)"
                stroke-width="1"
                stroke-dasharray="3 3"
              />
              <g>
                <circle :cx="hoverCoords.x" :cy="hoverCoords.yCpu" r="5" fill="var(--bg-card)" stroke="#3b82f6" stroke-width="2" />
              </g>
              <g>
                <circle :cx="hoverCoords.x" :cy="hoverCoords.yMem" r="5" fill="var(--bg-card)" stroke="#0284c7" stroke-width="2" />
              </g>
              <g>
                <circle :cx="hoverCoords.x" :cy="hoverCoords.yReq" r="5" fill="var(--bg-card)" stroke="#10b981" stroke-width="2" />
              </g>
            </g>
          </svg>

          <!-- Floating Rich Tooltip Box -->
          <div v-if="hoveredPoint" class="trend-rich-tooltip" :style="tooltipStyle">
            <div class="tooltip-time-header">
              <span class="tooltip-time-icon">
                <BaseIcon name="activity" size="xs" />
              </span>
              <span class="tooltip-time-text font-mono font-bold">{{ hoveredPoint.time }}</span>
              <span class="tooltip-time-elapsed">({{ hoveredElapsed }})</span>
            </div>
            <div class="tooltip-metrics-list">
              <div class="tooltip-metric-row">
                <span class="metric-indicator bg-blue"></span>
                <span class="metric-name">Avg Cluster CPU:</span>
                <span class="metric-val font-mono text-blue font-bold">{{ Math.round(hoveredPoint.cpu) }}%</span>
              </div>
              <div class="tooltip-metric-row">
                <span class="metric-indicator bg-sky"></span>
                <span class="metric-name">Avg Cluster RAM:</span>
                <span class="metric-val font-mono text-sky font-bold">{{ Math.round(hoveredPoint.mem) }}%</span>
              </div>
              <div class="tooltip-metric-row">
                <span class="metric-indicator bg-emerald"></span>
                <span class="metric-name">Throughput:</span>
                <span class="metric-val font-mono text-emerald font-bold">{{ Math.round(hoveredPoint.reqs) }} req/s</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Right Y-Axis (Throughput RPS) -->
        <div class="trend-y-axis y-axis-right font-mono text-emerald">
          <span class="y-tick">{{ formatMetricRate(trendChartMaxReqs) }}</span>
          <span class="y-tick">{{ formatMetricRate(trendChartMaxReqs * 0.75) }}</span>
          <span class="y-tick">{{ formatMetricRate(trendChartMaxReqs * 0.5) }}</span>
          <span class="y-tick">{{ formatMetricRate(trendChartMaxReqs * 0.25) }}</span>
          <span class="y-tick">0 rps</span>
        </div>
      </div>

      <!-- Bottom X-Axis (Timestamps) -->
      <div class="trend-x-axis font-mono">
        <span v-for="marker in trendTimeAxisMarkers" :key="marker.x" class="x-tick">
          {{ marker.time }}
        </span>
      </div>
    </div>
  </section>
</template>

<style scoped src="../../../assets/styles/overview-saturation-trends.css"></style>
