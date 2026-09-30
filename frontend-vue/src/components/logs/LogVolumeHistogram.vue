<script setup lang="ts">
import { ref, computed } from 'vue'
import type { LogAggregationBucket } from '../../api/logging'
import BaseIcon from '../ui/BaseIcon.vue'

interface Props {
  buckets?: LogAggregationBucket[]
  height?: number
  selectedRange?: { start: string; end: string } | null
}

const props = withDefaults(defineProps<Props>(), {
  buckets: () => [],
  height: 80,
  selectedRange: null,
})

const emit = defineEmits<{
  (e: 'filterRange', range: { start: string; end: string }): void
  (e: 'clearFilter'): void
}>()

interface ParsedBucket {
  key: string
  start: string
  end: string
  label: string
  total: number
  error: number
  warn: number
  info: number
  debug: number
}

const hoveredBucket = ref<ParsedBucket | null>(null)
const activeBucketKey = ref<string | null>(null)

const parsedBuckets = computed<ParsedBucket[]>(() => {
  if (!props.buckets || props.buckets.length === 0) return []

  return props.buckets.map((b, idx, arr) => {
    const counts = b.level_count || {}
    const errCount = (counts.error || counts.ERROR || 0) + (counts.fatal || counts.FATAL || 0)
    const warnCount = counts.warn || counts.WARN || 0
    const infoCount = counts.info || counts.INFO || 0
    const debugCount = counts.debug || counts.DEBUG || 0
    const total = b.total_count || (errCount + warnCount + infoCount + debugCount)

    const start = b.time_bucket
    let end: string
    if (idx < arr.length - 1 && arr[idx + 1].time_bucket) {
      end = arr[idx + 1].time_bucket
    } else {
      const d = new Date(start)
      end = new Date(d.getTime() + 60000).toISOString()
    }

    const label = start.includes('T') ? start.split('T')[1].slice(0, 8) : start

    return {
      key: `bucket-${idx}-${start}`,
      start,
      end,
      label,
      total,
      error: errCount,
      warn: warnCount,
      info: infoCount,
      debug: debugCount,
    }
  })
})

const maxVolume = computed(() => {
  const max = Math.max(...parsedBuckets.value.map(b => b.total), 1)
  return max
})

function onBucketClick(bucket: ParsedBucket) {
  if (activeBucketKey.value === bucket.key) {
    activeBucketKey.value = null
    emit('clearFilter')
  } else {
    activeBucketKey.value = bucket.key
    emit('filterRange', { start: bucket.start, end: bucket.end })
  }
}
</script>

<template>
  <div class="log-histogram-container glass-panel font-mono" :style="{ height: `${height}px` }" role="region" aria-label="Log Volume Histogram">
    <div class="histogram-header">
      <div class="histogram-legend">
        <span class="legend-title">LOG VOLUME</span>
        <span class="legend-item"><span class="legend-dot bg-rose" />ERR</span>
        <span class="legend-item"><span class="legend-dot bg-amber" />WARN</span>
        <span class="legend-item"><span class="legend-dot bg-cyan" />INFO</span>
        <span class="legend-item"><span class="legend-dot bg-slate" />DEBUG</span>
      </div>
      <div class="histogram-meta">
        <span v-if="hoveredBucket" class="tooltip-text">
          <strong>{{ hoveredBucket.label }}</strong>:
          <span class="text-rose">{{ hoveredBucket.error }} E</span> ·
          <span class="text-amber">{{ hoveredBucket.warn }} W</span> ·
          <span class="text-cyan">{{ hoveredBucket.info }} I</span> ·
          <span class="text-slate">{{ hoveredBucket.debug }} D</span>
        </span>
        <span v-else class="peak-label">Peak: {{ maxVolume }} logs</span>
        <button v-if="activeBucketKey" type="button" class="btn-reset-filter" @click="activeBucketKey = null; emit('clearFilter')">Reset Range</button>
      </div>
    </div>

    <!-- Interactive Stacked SVG Bar Chart -->
    <div class="histogram-svg-wrap">
      <svg class="histogram-svg" viewBox="0 0 1000 52" preserveAspectRatio="none">
        <g v-for="(b, idx) in parsedBuckets" :key="b.key" class="bucket-group" @click="onBucketClick(b)" @mouseenter="hoveredBucket = b" @mouseleave="hoveredBucket = null">
          <!-- Bucket click background hit area -->
          <rect :x="idx * (1000 / parsedBuckets.length)" y="0" :width="1000 / parsedBuckets.length" height="52" fill="transparent" class="bucket-hit-area" />
          <!-- Active highlight border -->
          <rect v-if="activeBucketKey === b.key" :x="idx * (1000 / parsedBuckets.length)" y="0" :width="1000 / parsedBuckets.length" height="52" fill="rgba(56, 189, 248, 0.15)" stroke="#38bdf8" stroke-width="1" />
          <!-- Stacked segments (Debug -> Info -> Warn -> Error from bottom to top) -->
          <template v-if="b.total > 0">
            <!-- Debug (Slate) -->
            <rect
              :x="idx * (1000 / parsedBuckets.length) + 1"
              :y="52 - ((b.debug / maxVolume) * 50)"
              :width="Math.max(1, (1000 / parsedBuckets.length) - 2)"
              :height="(b.debug / maxVolume) * 50"
              fill="#64748b"
            />
            <!-- Info (Cyan) -->
            <rect
              :x="idx * (1000 / parsedBuckets.length) + 1"
              :y="52 - (((b.debug + b.info) / maxVolume) * 50)"
              :width="Math.max(1, (1000 / parsedBuckets.length) - 2)"
              :height="(b.info / maxVolume) * 50"
              fill="#06b6d4"
            />
            <!-- Warn (Warm Amber) -->
            <rect
              :x="idx * (1000 / parsedBuckets.length) + 1"
              :y="52 - (((b.debug + b.info + b.warn) / maxVolume) * 50)"
              :width="Math.max(1, (1000 / parsedBuckets.length) - 2)"
              :height="(b.warn / maxVolume) * 50"
              fill="#f59e0b"
            />
            <!-- Error (Rose) -->
            <rect
              :x="idx * (1000 / parsedBuckets.length) + 1"
              :y="52 - (((b.debug + b.info + b.warn + b.error) / maxVolume) * 50)"
              :width="Math.max(1, (1000 / parsedBuckets.length) - 2)"
              :height="(b.error / maxVolume) * 50"
              fill="#ef4444"
            />
          </template>
        </g>
      </svg>
      <div v-if="parsedBuckets.length === 0" class="empty-histogram font-mono">
        <BaseIcon name="activity" size="xs" />
        <span>Aggregating real-time severity buckets...</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.log-histogram-container {
  padding: 6px 12px;
  background: rgba(11, 15, 25, 0.75);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  box-sizing: border-box;
}
.histogram-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 10px;
  height: 16px;
}
.histogram-legend { display: flex; align-items: center; gap: 8px; }
.legend-title { font-weight: 700; color: #cbd5e1; letter-spacing: 0.05em; }
.legend-item { display: inline-flex; align-items: center; gap: 3px; font-size: 9px; color: #94a3b8; }
.legend-dot { width: 5px; height: 5px; border-radius: 50%; }
.bg-rose { background-color: #ef4444; }
.bg-amber { background-color: #f59e0b; }
.bg-cyan { background-color: #06b6d4; }
.bg-slate { background-color: #64748b; }
.text-rose { color: #f87171; }
.text-amber { color: #f59e0b; }
.text-cyan { color: #06b6d4; }
.text-slate { color: #94a3b8; }
.histogram-meta { display: flex; align-items: center; gap: 8px; font-size: 10px; color: #64748b; }
.peak-label { font-size: 9px; }
.btn-reset-filter { background: rgba(56, 189, 248, 0.15); border: 1px solid rgba(56, 189, 248, 0.3); color: #38bdf8; font-size: 9px; padding: 1px 6px; border-radius: 3px; cursor: pointer; }
.histogram-svg-wrap { height: 48px; width: 100%; position: relative; }
.histogram-svg { width: 100%; height: 100%; display: block; overflow: visible; }
.bucket-group { cursor: pointer; }
.bucket-group:hover rect:not(.bucket-hit-area) { filter: brightness(1.25); }
.empty-histogram { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; gap: 6px; font-size: 10px; color: #64748b; }
</style>
