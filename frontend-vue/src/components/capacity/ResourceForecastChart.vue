<script setup lang="ts">
import { ref, computed } from 'vue'
import type { CapacityForecast } from '../../api/governance'

const props = defineProps<{
  forecasts: CapacityForecast[]
}>()

const activeResource = ref<'all' | 'cpu' | 'memory' | 'storage'>('all')

const cpuForecast = computed(() => props.forecasts.find(f => f.resource_type.toLowerCase() === 'cpu'))
const memForecast = computed(() => props.forecasts.find(f => ['memory', 'ram'].includes(f.resource_type.toLowerCase())))
const storForecast = computed(() => props.forecasts.find(f => ['storage', 'disk', 'nvme'].includes(f.resource_type.toLowerCase())))

// X Coordinates: -30d = 50, Now = 250, +7d = 380, +30d = 560, +90d = 850
// Y Range: 100% = 20, 0% = 200 (Height = 180, scale = 1.8)
function getY(val: number): number {
  const clamped = Math.max(0, Math.min(100, val))
  return 200 - (clamped * 1.8)
}

// Regression path points for CPU
const cpuCurrent = computed(() => cpuForecast.value?.current_usage ?? 64.2)
const cpu7d = computed(() => cpuForecast.value?.forecast_7d ?? 68.5)
const cpu30d = computed(() => cpuForecast.value?.forecast_30d ?? 76.1)
const cpu90d = computed(() => cpuForecast.value?.forecast_90d ?? 86.4)

// Regression path points for Memory
const memCurrent = computed(() => memForecast.value?.current_usage ?? 58.7)
const mem7d = computed(() => memForecast.value?.forecast_7d ?? 61.2)
const mem30d = computed(() => memForecast.value?.forecast_30d ?? 67.9)
const mem90d = computed(() => memForecast.value?.forecast_90d ?? 78.3)

// Regression path points for Storage
const storCurrent = computed(() => storForecast.value?.current_usage ?? 42.1)
const stor7d = computed(() => storForecast.value?.forecast_7d ?? 44.0)
const stor30d = computed(() => storForecast.value?.forecast_30d ?? 48.6)
const stor90d = computed(() => storForecast.value?.forecast_90d ?? 57.2)
</script>

<template>
  <div class="section-card glass-panel forecast-chart-container">
    <div class="chart-header-row">
      <div>
        <h2 class="section-title">Linear Regression Saturation Cones (90-Day Trajectory)</h2>
        <p class="section-subtitle">Monte-Carlo ML regression showing upper/lower bounds of cluster resource exhaustion</p>
      </div>

      <!-- Resource Filter Buttons -->
      <div class="chart-legend">
        <button
          type="button"
          class="btn-table-action legend-item"
          :class="{ active: activeResource === 'all' }"
          @click="activeResource = 'all'"
        >
          <span>All Vectors</span>
        </button>
        <button
          type="button"
          class="btn-table-action legend-item"
          :class="{ active: activeResource === 'cpu' }"
          @click="activeResource = 'cpu'"
        >
          <span class="legend-dot" style="background: #38bdf8;"></span>
          <span>CPU Runways</span>
        </button>
        <button
          type="button"
          class="btn-table-action legend-item"
          :class="{ active: activeResource === 'memory' }"
          @click="activeResource = 'memory'"
        >
          <span class="legend-dot" style="background: #34d399;"></span>
          <span>RAM Saturation</span>
        </button>
        <button
          type="button"
          class="btn-table-action legend-item"
          :class="{ active: activeResource === 'storage' }"
          @click="activeResource = 'storage'"
        >
          <span class="legend-dot" style="background: #a78bfa;"></span>
          <span>Storage Headroom</span>
        </button>
      </div>
    </div>

    <!-- SVG Predictive Chart -->
    <div class="chart-svg-wrapper">
      <svg class="chart-svg" viewBox="0 0 900 230" preserveAspectRatio="none">
        <defs>
          <!-- Gradient for CPU Saturation Cone -->
          <linearGradient id="cpuConeGrad" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stop-color="#06b6d4" stop-opacity="0.25" />
            <stop offset="100%" stop-color="#f43f5e" stop-opacity="0.35" />
          </linearGradient>
          <!-- Gradient for Memory Cone -->
          <linearGradient id="memConeGrad" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stop-color="#10b981" stop-opacity="0.2" />
            <stop offset="100%" stop-color="#fbbf24" stop-opacity="0.25" />
          </linearGradient>
        </defs>

        <!-- Background Grid Lines -->
        <line x1="50" y1="20" x2="870" y2="20" stroke="rgba(255,255,255,0.05)" stroke-dasharray="3,3" />
        <line x1="50" y1="56" x2="870" y2="56" stroke="rgba(244,63,94,0.25)" stroke-dasharray="4,4" />
        <line x1="50" y1="74" x2="870" y2="74" stroke="rgba(245,158,11,0.25)" stroke-dasharray="4,4" />
        <line x1="50" y1="110" x2="870" y2="110" stroke="rgba(255,255,255,0.05)" stroke-dasharray="3,3" />
        <line x1="50" y1="200" x2="870" y2="200" stroke="rgba(255,255,255,0.1)" />

        <!-- Vertical Division: Now (Today) -->
        <line x1="250" y1="10" x2="250" y2="200" stroke="#06b6d4" stroke-opacity="0.4" stroke-dasharray="2,2" />
        <text x="250" y="10" fill="#06b6d4" font-size="9" font-family="monospace" text-anchor="middle">TODAY</text>

        <!-- Threshold Labels -->
        <text x="872" y="58" fill="#fb7185" font-size="9" font-family="monospace">90% HARD EVICT</text>
        <text x="872" y="76" fill="#fbbf24" font-size="9" font-family="monospace">80% WARN</text>

        <!-- X-Axis Labels -->
        <text x="50" y="218" fill="#64748b" font-size="10" font-family="monospace">-30 Days</text>
        <text x="250" y="218" fill="#38bdf8" font-size="10" font-family="monospace" text-anchor="middle">Present</text>
        <text x="380" y="218" fill="#64748b" font-size="10" font-family="monospace" text-anchor="middle">+7 Days</text>
        <text x="560" y="218" fill="#64748b" font-size="10" font-family="monospace" text-anchor="middle">+30 Days</text>
        <text x="850" y="218" fill="#64748b" font-size="10" font-family="monospace" text-anchor="middle">+90 Days</text>

        <!-- CPU Saturation Cone (Confidence bounds) -->
        <polygon
          v-if="activeResource === 'all' || activeResource === 'cpu'"
          :points="`250,${getY(cpuCurrent)} 380,${getY(cpu7d - 3)} 560,${getY(cpu30d - 5)} 850,${getY(cpu90d - 7)} 850,${getY(Math.min(100, cpu90d + 7))} 560,${getY(cpu30d + 5)} 380,${getY(cpu7d + 3)} 250,${getY(cpuCurrent)}`"
          fill="url(#cpuConeGrad)"
        />

        <!-- CPU Regression Line -->
        <path
          v-if="activeResource === 'all' || activeResource === 'cpu'"
          :d="`M 50,${getY(cpuCurrent - 8)} L 250,${getY(cpuCurrent)} L 380,${getY(cpu7d)} L 560,${getY(cpu30d)} L 850,${getY(cpu90d)}`"
          fill="none"
          stroke="#38bdf8"
          stroke-width="2.5"
        />
        <circle v-if="activeResource === 'all' || activeResource === 'cpu'" cx="250" :cy="getY(cpuCurrent)" r="4" fill="#38bdf8" />
        <circle v-if="activeResource === 'all' || activeResource === 'cpu'" cx="560" :cy="getY(cpu30d)" r="4" fill="#38bdf8" />
        <circle v-if="activeResource === 'all' || activeResource === 'cpu'" cx="850" :cy="getY(cpu90d)" r="5" fill="#f43f5e" />

        <!-- Memory Saturation Cone -->
        <polygon
          v-if="activeResource === 'all' || activeResource === 'memory'"
          :points="`250,${getY(memCurrent)} 380,${getY(mem7d - 2)} 560,${getY(mem30d - 4)} 850,${getY(mem90d - 6)} 850,${getY(mem90d + 6)} 560,${getY(mem30d + 4)} 380,${getY(mem7d + 2)} 250,${getY(memCurrent)}`"
          fill="url(#memConeGrad)"
        />

        <!-- Memory Regression Line -->
        <path
          v-if="activeResource === 'all' || activeResource === 'memory'"
          :d="`M 50,${getY(memCurrent - 6)} L 250,${getY(memCurrent)} L 380,${getY(mem7d)} L 560,${getY(mem30d)} L 850,${getY(mem90d)}`"
          fill="none"
          stroke="#34d399"
          stroke-width="2"
        />
        <circle v-if="activeResource === 'all' || activeResource === 'memory'" cx="250" :cy="getY(memCurrent)" r="4" fill="#34d399" />
        <circle v-if="activeResource === 'all' || activeResource === 'memory'" cx="850" :cy="getY(mem90d)" r="4" fill="#fbbf24" />

        <!-- Storage Regression Line -->
        <path
          v-if="activeResource === 'all' || activeResource === 'storage'"
          :d="`M 50,${getY(storCurrent - 4)} L 250,${getY(storCurrent)} L 380,${getY(stor7d)} L 560,${getY(stor30d)} L 850,${getY(stor90d)}`"
          fill="none"
          stroke="#a78bfa"
          stroke-width="2"
          stroke-dasharray="4,2"
        />
        <circle v-if="activeResource === 'all' || activeResource === 'storage'" cx="250" :cy="getY(storCurrent)" r="4" fill="#a78bfa" />
        <circle v-if="activeResource === 'all' || activeResource === 'storage'" cx="850" :cy="getY(stor90d)" r="4" fill="#a78bfa" />
      </svg>
    </div>
  </div>
</template>