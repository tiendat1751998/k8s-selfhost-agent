<script setup lang="ts">
import NodeHistoricalKpis from './NodeHistoricalKpis.vue'
import NodeHistoricalCustomToolbar from './NodeHistoricalCustomToolbar.vue'
import { formatPercent, formatBytes, formatIoRate } from './nodeChartMath'
import { useNodeHistoricalChart, type HistoricalChartProps, type HistoricalChartEmit } from './useNodeHistoricalChart'

const props = withDefaults(defineProps<HistoricalChartProps>(), {
  tpsData: null,
  nodeHistoryData: null,
  nodeHistoryLoading: false,
  nodeHistoryRange: '1h',
  customHistFrom: '',
  customHistTo: '',
})

const emit = defineEmits<HistoricalChartEmit>()

const {
  showHistCpu,
  showHistMem,
  showHistDisk,
  showHistPeakEnvelope,
  histViewMode,
  showCustomHistoryPicker,
  customHistoryFrom,
  customHistoryTo,
  hoveredNodeHistIndex,
  isNodeHistHovered,
  nodeHistoryWindowBadge,
  nodeHistoryList,
  nodeHistoryChartCpuPath,
  nodeHistoryChartCpuPeakPath,
  nodeHistoryChartCpuEnvelope,
  nodeHistorySpikeMarkers,
  nodeHistoryChartCpuArea,
  nodeHistoryChartMemPath,
  nodeHistoryChartMemArea,
  nodeHistoryChartDiskPath,
  nodeHistoryTimeMarkers,
  hoveredNodeHistPoint,
  hoveredPointSuspect,
  nodeHistHoverCoords,
  nodeHistTooltipStyle,
  toggleCustomHistoryPicker,
  switchNodeDrawerToHistory,
  applyPreset,
  handleNodeHistChartHover,
  handleNodeHistChartLeave,
  loadNodeHistory,
  syncLogsToPointInTime,
} = useNodeHistoricalChart(props, emit)
</script>

<template>
  <div class="node-historical-container">
    <!-- 1. Top 4 KPI Summary Cards -->
    <NodeHistoricalKpis :node="node" :nodeHistoryData="nodeHistoryData" />

    <!-- 2. Historical Multi-Series Chart -->
    <div class="hist-chart-wrapper glass-panel">
      <!-- 2a. Chart Header with Title, Badge, and Series Toggles -->
      <div class="hist-chart-header">
        <div class="chart-title-group">
          <h4 class="hist-chart-title">📈 {{ node?.node_name || 'Node' }} Hardware Saturation Trends</h4>
          <span class="hist-chart-desc badge-history-window font-mono">{{ nodeHistoryWindowBadge }}</span>
        </div>

        <!-- Interactive Series Toggles with Live Values -->
        <div class="series-toggles-group" v-if="nodeHistoryList.length > 0">
          <button
            type="button"
            class="series-toggle-btn"
            :class="{ 'toggle-active cpu-active': showHistCpu }"
            @click="showHistCpu = !showHistCpu"
            title="Click to toggle CPU curve"
          >
            <span class="toggle-dot bg-violet"></span>
            <span>CPU: <strong>{{ formatPercent(node?.cpu_percent) }}</strong></span>
          </button>

          <button
            type="button"
            class="series-toggle-btn"
            :class="{ 'toggle-active peak-active': showHistPeakEnvelope }"
            @click="showHistPeakEnvelope = !showHistPeakEnvelope"
            title="Toggle Peak Spike Envelope layer"
          >
            <span class="toggle-dot bg-rose"></span>
            <span>🔥 Peak Envelope: <strong>{{ showHistPeakEnvelope ? 'ON' : 'OFF' }}</strong></span>
          </button>

          <button
            type="button"
            class="series-toggle-btn"
            :class="{ 'toggle-active mem-active': showHistMem }"
            @click="showHistMem = !showHistMem"
            title="Click to toggle RAM curve"
          >
            <span class="toggle-dot bg-cyan"></span>
            <span>RAM: <strong>{{ formatPercent(node?.memory_percent) }}</strong></span>
          </button>

          <button
            type="button"
            class="series-toggle-btn"
            :class="{ 'toggle-active disk-active reqs-active': showHistDisk }"
            @click="showHistDisk = !showHistDisk"
            title="Click to toggle Disk curve"
          >
            <span class="toggle-dot bg-emerald"></span>
            <span>Disk: <strong>{{ formatPercent(node?.disk_percent) }}</strong></span>
          </button>
        </div>
      </div>

      <!-- 2b. Integrated Time Range & Sample Bar -->
      <div class="hist-chart-time-bar">
        <div class="chart-time-pills">
          <button
            v-for="r in ['1h', '3h', '6h', '24h', '7d', '30d'] as const"
            :key="r"
            class="btn-chart-range-pill"
            :class="{ active: nodeHistoryRange === r && !showCustomHistoryPicker }"
            @click="switchNodeDrawerToHistory(r)"
          >
            {{ r }}
          </button>
          <button
            class="btn-chart-range-pill"
            :class="{ active: nodeHistoryRange === 'custom' || showCustomHistoryPicker }"
            @click="toggleCustomHistoryPicker"
          >
            📅 Custom
          </button>
        </div>
        <div class="range-right">
          <span class="badge badge-indigo font-mono" v-if="nodeHistoryData?.resolution">
            Sample Rate: {{ nodeHistoryData.resolution }}
          </span>
          <button class="btn btn-secondary btn-xs" @click="emit('range-change', nodeHistoryRange)" :disabled="nodeHistoryLoading">
            ↺ Refresh
          </button>
        </div>
      </div>

      <!-- 2c. Inline Glassmorphic Custom History Toolbar -->
      <NodeHistoricalCustomToolbar
        v-if="showCustomHistoryPicker || nodeHistoryRange === 'custom'"
        v-model:from="customHistoryFrom"
        v-model:to="customHistoryTo"
        :loading="nodeHistoryLoading"
        @apply-preset="applyPreset"
        @apply="loadNodeHistory(node?.node_id || node?.node_name, 'custom', customHistoryFrom, customHistoryTo)"
      />

      <!-- 2d. SVG Chart Canvas / Loading / Empty -->
      <div v-if="nodeHistoryLoading" class="hist-chart-loading glass-panel">
        <span class="spinner-sm"></span> Loading {{ nodeHistoryRange }} telemetry rollups...
      </div>

      <div v-else-if="nodeHistoryList.length > 0" class="hist-chart-container" @mousemove="handleNodeHistChartHover" @mouseleave="handleNodeHistChartLeave">
        <!-- Main SVG Canvas -->
        <svg class="hist-svg-canvas" viewBox="0 0 760 200" preserveAspectRatio="none">
          <defs>
            <linearGradient id="histCpuGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stop-color="#a855f7" stop-opacity="0.35" />
              <stop offset="100%" stop-color="#a855f7" stop-opacity="0.0" />
            </linearGradient>
            <linearGradient id="histCpuPeakEnvelopeGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stop-color="#f43f5e" stop-opacity="0.45" />
              <stop offset="50%" stop-color="#e879f9" stop-opacity="0.25" />
              <stop offset="100%" stop-color="#a855f7" stop-opacity="0.05" />
            </linearGradient>
            <linearGradient id="histMemGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stop-color="#06b6d4" stop-opacity="0.3" />
              <stop offset="100%" stop-color="#06b6d4" stop-opacity="0.0" />
            </linearGradient>
          </defs>

          <!-- Y-Axis Percentage Labels inside SVG -->
          <g class="hist-svg-y-labels" font-size="9" text-anchor="end">
            <text x="44" y="23" fill="#fb7185" font-weight="600">100%</text>
            <text x="44" y="63" fill="#fbbf24" font-weight="600">75%</text>
            <text x="44" y="103" fill="#38bdf8" font-weight="600">50%</text>
            <text x="44" y="143" fill="#94a3b8">25%</text>
            <text x="44" y="183" fill="#64748b">0%</text>
          </g>

          <!-- Horizontal Grid Lines -->
          <line x1="50" y1="20" x2="720" y2="20" stroke="rgba(244,63,94,0.2)" stroke-dasharray="3,3" />
          <line x1="50" y1="60" x2="720" y2="60" stroke="rgba(245,158,11,0.2)" stroke-dasharray="3,3" />
          <line x1="50" y1="100" x2="720" y2="100" stroke="rgba(56,189,248,0.12)" stroke-dasharray="3,3" />
          <line x1="50" y1="140" x2="720" y2="140" stroke="rgba(255,255,255,0.06)" stroke-dasharray="3,3" />
          <line x1="50" y1="180" x2="720" y2="180" stroke="rgba(255,255,255,0.12)" />

          <!-- Area Fills -->
          <path
            v-if="showHistCpu && showHistPeakEnvelope && (histViewMode === 'both' || histViewMode === 'peak') && nodeHistoryChartCpuEnvelope"
            :d="nodeHistoryChartCpuEnvelope"
            fill="url(#histCpuPeakEnvelopeGrad)"
            class="chart-envelope-area"
          />
          <path v-if="showHistCpu && (histViewMode === 'both' || histViewMode === 'avg') && nodeHistoryChartCpuArea" :d="nodeHistoryChartCpuArea" fill="url(#histCpuGrad)" />
          <path v-if="showHistMem && nodeHistoryChartMemArea" :d="nodeHistoryChartMemArea" fill="url(#histMemGrad)" />

          <!-- Trend Lines -->
          <path v-if="showHistDisk && nodeHistoryChartDiskPath" :d="nodeHistoryChartDiskPath" fill="none" stroke="#10b981" stroke-width="1.5" stroke-dasharray="4,4" />
          <path v-if="showHistMem && nodeHistoryChartMemPath" :d="nodeHistoryChartMemPath" fill="none" stroke="#06b6d4" stroke-width="2" />
          <path
            v-if="showHistCpu && (histViewMode === 'both' || histViewMode === 'peak') && (showHistPeakEnvelope || histViewMode === 'peak') && nodeHistoryChartCpuPeakPath"
            :d="nodeHistoryChartCpuPeakPath"
            fill="none"
            stroke="#e879f9"
            stroke-width="1.2"
            stroke-dasharray="3,2"
            class="chart-peak-line"
          />
          <path v-if="showHistCpu && (histViewMode === 'both' || histViewMode === 'avg') && nodeHistoryChartCpuPath" :d="nodeHistoryChartCpuPath" fill="none" stroke="#a855f7" stroke-width="2" />

          <!-- Glowing Spike Pins on Prominent Spikes -->
          <g v-if="showHistCpu && (showHistPeakEnvelope || histViewMode === 'peak')" class="spike-markers-group">
            <g
              v-for="marker in nodeHistorySpikeMarkers"
              :key="'spike-' + marker.index"
              class="spike-pin-item"
              @mouseenter="hoveredNodeHistIndex = marker.index; isNodeHistHovered = true"
              @click.stop="syncLogsToPointInTime(nodeHistoryList[marker.index])"
            >
              <circle
                v-if="marker.peak >= 85"
                :cx="marker.x"
                :cy="marker.y"
                r="7"
                fill="none"
                stroke="#f43f5e"
                stroke-width="1.5"
                class="spike-marker-pulse"
              />
              <circle
                :cx="marker.x"
                :cy="marker.y"
                :r="marker.peak >= 85 ? 4 : 3"
                :fill="marker.peak >= 85 ? '#f43f5e' : '#e879f9'"
                stroke="#ffffff"
                stroke-width="1.2"
              />
            </g>
          </g>

          <!-- Interactive Crosshair -->
          <g v-if="isNodeHistHovered && nodeHistHoverCoords">
            <line
              :x1="nodeHistHoverCoords.x"
              y1="15"
              :x2="nodeHistHoverCoords.x"
              y2="185"
              stroke="#38bdf8"
              stroke-width="1.5"
              stroke-dasharray="3,3"
            />
            <circle v-if="showHistCpu && (histViewMode === 'both' || histViewMode === 'avg')" :cx="nodeHistHoverCoords.x" :cy="nodeHistHoverCoords.yCpu" r="4.5" fill="#a855f7" stroke="#ffffff" stroke-width="2" />
            <circle v-if="showHistCpu && (showHistPeakEnvelope || histViewMode === 'peak') && hoveredNodeHistPoint && (hoveredNodeHistPoint.cpu_peak || 0) > hoveredNodeHistPoint.cpu_percent" :cx="nodeHistHoverCoords.x" :cy="nodeHistHoverCoords.yCpuPeak" r="4" fill="#e879f9" stroke="#ffffff" stroke-width="1.5" />
            <circle v-if="showHistMem" :cx="nodeHistHoverCoords.x" :cy="nodeHistHoverCoords.yMem" r="4" fill="#06b6d4" stroke="#ffffff" stroke-width="1.5" />
            <circle v-if="showHistDisk" :cx="nodeHistHoverCoords.x" :cy="nodeHistHoverCoords.yDisk" r="3.5" fill="#10b981" stroke="#ffffff" stroke-width="1.5" />
          </g>
        </svg>

        <!-- Floating Tooltip Box -->
        <div v-if="isNodeHistHovered && hoveredNodeHistPoint" class="hist-rich-tooltip" :style="nodeHistTooltipStyle">
          <div class="tooltip-time-header">
            🕒 {{ new Date(hoveredNodeHistPoint.recorded_at).toLocaleString() }}
          </div>
          <div class="tooltip-series-row" v-if="showHistCpu">
            <span class="tooltip-dot dot-violet"></span>
            <span class="tooltip-label">CPU:</span>
            <strong class="tooltip-val text-violet">
              {{ formatPercent(hoveredNodeHistPoint.cpu_percent) }}
              <span v-if="(hoveredNodeHistPoint.cpu_peak || 0) > hoveredNodeHistPoint.cpu_percent" class="tooltip-peak-highlight">
                (🔥 Peak: {{ formatPercent(hoveredNodeHistPoint.cpu_peak) }})
              </span>
            </strong>
          </div>
          <div v-if="showHistCpu && (hoveredNodeHistPoint.cpu_peak || 0) >= 85" class="tooltip-spike-alert">
            <span class="spike-alert-tag">⚠️ Critical Spike: {{ formatPercent(hoveredNodeHistPoint.cpu_peak) }}</span>
          </div>
          <div v-if="hoveredPointSuspect" class="tooltip-suspect-row">
            <span class="tooltip-suspect-tag">
              🔥 Top Offender: <strong class="text-rose">{{ hoveredPointSuspect.name }}</strong> ({{ hoveredPointSuspect.reason }})
            </span>
          </div>
          <div class="tooltip-series-row" v-if="showHistMem">
            <span class="tooltip-dot dot-cyan"></span>
            <span class="tooltip-label">Memory:</span>
            <strong class="tooltip-val text-cyan">{{ formatPercent(hoveredNodeHistPoint.mem_percent) }} ({{ formatBytes(hoveredNodeHistPoint.mem_used_bytes) }})</strong>
          </div>
          <div class="tooltip-series-row" v-if="showHistDisk">
            <span class="tooltip-dot dot-emerald"></span>
            <span class="tooltip-label">Disk:</span>
            <strong class="tooltip-val text-emerald">{{ formatPercent(hoveredNodeHistPoint.disk_percent) }} ({{ formatBytes(hoveredNodeHistPoint.disk_used_bytes) }})</strong>
          </div>
          <div class="tooltip-series-row">
            <span class="tooltip-dot dot-indigo"></span>
            <span class="tooltip-label">Net I/O:</span>
            <strong class="tooltip-val text-indigo">↓ {{ formatIoRate(hoveredNodeHistPoint.rx_bytes_per_sec) }} ↑ {{ formatIoRate(hoveredNodeHistPoint.tx_bytes_per_sec) }}</strong>
          </div>
          <button
            type="button"
            class="btn-pit-sync font-mono"
            @click.stop="syncLogsToPointInTime(hoveredNodeHistPoint)"
            title="Sync live and historical failure logs to this point in time (±15 minutes)"
          >
            <span>🔍</span> View Logs at this time
          </button>
        </div>

        <!-- Bottom X-Axis Time Markers -->
        <div class="hist-x-axis">
          <span v-for="marker in nodeHistoryTimeMarkers" :key="marker.x" class="x-tick">
            {{ marker.time }}
          </span>
        </div>
      </div>

      <!-- Empty History Chart Placeholder -->
      <div v-else class="hist-chart-empty">
        <div class="empty-chart-illustration">
          <span class="empty-chart-icon">📈</span>
          <span class="empty-pulse-badge">Awaiting Telemetry Rollups</span>
        </div>
        <h5 class="empty-chart-title">No Telemetry Recorded in {{ nodeHistoryRange }} Window</h5>
        <p class="empty-chart-desc">
          Historical metrics are aggregated periodically by the background collector. Once continuous telemetry is recorded for <code>{{ node?.node_name }}</code>, multi-series saturation curves will display here.
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../../assets/styles/components/node-historical-chart.css';
</style>
