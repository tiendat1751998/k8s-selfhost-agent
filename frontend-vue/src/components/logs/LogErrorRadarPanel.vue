<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import { getErrorRadar, type ErrorRadarGroup } from '../../api/logging'
import { isNoiseLog } from '../../utils/logTargetCategorizer'
import '../../assets/styles/components/log-error-radar.css'

interface Props {
  node?: string
  service?: string
  selectedTimeRange?: string
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'inspectGroup', group: ErrorRadarGroup, activeRange: string): void
}>()

const isLoading = ref(false)
const errorMessage = ref<string | null>(null)
const groups = ref<ErrorRadarGroup[]>([])
const totalGroups = ref(0)
const totalOccurrences = ref(0)
const tookMs = ref(0)
const activeTimeRange = ref('24h')
const timeRanges = ['All', '1h', '6h', '24h', '7d']

const scopeLabel = computed(() => {
  if (props.node && props.service) return `${props.node}/${props.service}`
  if (props.node) return `Node: ${props.node}`
  if (props.service) return `Service: ${props.service}`
  return 'Cluster-Wide'
})

function getSinceTimestamp(range: string): string | undefined {
  const now = Date.now()
  if (range === '1h') return new Date(now - 3600 * 1000).toISOString()
  if (range === '6h') return new Date(now - 6 * 3600 * 1000).toISOString()
  if (range === '24h') return new Date(now - 24 * 3600 * 1000).toISOString()
  if (range === '7d') return new Date(now - 7 * 86400 * 1000).toISOString()
  return undefined
}

async function fetchRadar() {
  isLoading.value = true
  errorMessage.value = null
  try {
    const since = getSinceTimestamp(activeTimeRange.value)
    const res = await getErrorRadar({
      node: props.node || undefined,
      service: props.service || undefined,
      since,
      limit: 50,
    })
    const rawList = res?.groups || []
    groups.value = rawList.filter((g) => !g.service || !isNoiseLog(g.service))
    totalGroups.value = res?.total_groups ?? groups.value.length
    totalOccurrences.value = res?.total_occurrences ?? groups.value.reduce((acc, g) => acc + g.count, 0)
    tookMs.value = res?.took_ms ?? 0
  } catch (err: unknown) {
    errorMessage.value = err instanceof Error ? err.message : 'Failed to scan error radar'
  } finally {
    isLoading.value = false
  }
}

function setTimeRange(range: string) {
  activeTimeRange.value = range
  fetchRadar()
}

function formatTime(iso?: string): string {
  if (!iso) return '--'
  try {
    const d = new Date(iso)
    return d.toLocaleTimeString([], { hour12: false })
  } catch {
    return iso
  }
}

watch(() => [props.node, props.service], () => fetchRadar())
onMounted(() => fetchRadar())
</script>

<template>
  <section class="log-error-radar-panel" aria-label="Error Radar Panel">
    <!-- Header -->
    <header class="radar-header font-mono">
      <div class="radar-title-group">
        <span class="radar-title">
          <span>🔥</span> <span>Blind Error Radar</span>
        </span>
        <span class="radar-subtitle">
          {{ totalGroups }} unique error patterns ({{ totalOccurrences.toLocaleString() }} occurrences) scanned in {{ tookMs.toFixed(1) }}ms
        </span>
        <span class="radar-scope-badge">{{ scopeLabel }}</span>
      </div>

      <div class="radar-controls-group">
        <div class="radar-time-pills" role="group" aria-label="Time Window">
          <button
            v-for="r in timeRanges"
            :key="r"
            type="button"
            class="radar-pill-btn"
            :class="{ active: activeTimeRange === r }"
            @click="setTimeRange(r)"
          >
            {{ r }}
          </button>
        </div>

        <button
          type="button"
          class="radar-action-btn"
          :disabled="isLoading"
          title="Refresh Error Radar"
          @click="fetchRadar"
        >
          <BaseIcon name="refresh-cw" size="xs" :class="{ 'spin-icon': isLoading }" />
          <span>{{ isLoading ? 'Scanning...' : 'Refresh' }}</span>
        </button>

        <button
          type="button"
          class="radar-close-btn"
          title="Close Error Radar"
          aria-label="Close Error Radar"
          @click="emit('close')"
        >
          &times;
        </button>
      </div>
    </header>

    <!-- Body -->
    <div class="radar-body font-mono">
      <div v-if="isLoading && groups.length === 0" class="radar-loading-state">
        <BaseIcon name="refresh-cw" size="xs" class="spin-icon" />
        <span>Scanning logs across cluster for error patterns...</span>
      </div>

      <div v-else-if="errorMessage" class="radar-error-state">
        <BaseIcon name="alert-triangle" size="xs" />
        <span>{{ errorMessage }}</span>
        <button type="button" class="radar-action-btn" @click="fetchRadar">Retry</button>
      </div>

      <div v-else-if="groups.length === 0" class="radar-empty-state">
        <span>✅ Zero ERROR/FATAL patterns detected in current scope</span>
      </div>

      <div v-else class="radar-list">
        <article
          v-for="group in groups"
          :key="group.fingerprint + group.service"
          class="radar-item"
        >
          <div class="radar-item-left">
            <span
              class="radar-count-badge"
              :class="{ 'count-warn': group.level === 'WARN' }"
            >
              {{ group.count.toLocaleString() }}×
            </span>
            <span class="radar-level-badge">{{ group.level || 'ERROR' }}</span>
            <span class="radar-target-badge" :title="group.service">
              [{{ group.node ? group.node + '/' : '' }}{{ group.service }}]
            </span>
            <div class="radar-pattern-block">
              <span class="radar-fingerprint" :title="group.fingerprint">
                {{ group.fingerprint }}
              </span>
              <div class="radar-sample-subline">
                <span :title="group.sample_message">Sample: {{ group.sample_message }}</span>
                <span>• Last seen: {{ formatTime(group.last_seen) }}</span>
              </div>
            </div>
          </div>

          <button
            type="button"
            class="radar-inspect-btn font-mono"
            title="Inspect matching logs in terminal"
            @click="emit('inspectGroup', group, activeTimeRange)"
          >
            <BaseIcon name="search" size="xs" />
            <span>Inspect</span>
          </button>
        </article>
      </div>
    </div>
  </section>
</template>
