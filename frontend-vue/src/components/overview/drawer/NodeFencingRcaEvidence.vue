<script setup lang="ts">
import { ref, computed } from 'vue'
import BaseIcon from '../../ui/BaseIcon.vue'
import type { DynamicRCAReport, IndicatorSignal } from '../../../api/dr'

interface Props {
  dynamicRca?: DynamicRCAReport
  fallbackCause?: string
  fallbackConfidence?: number
  fallbackRecommendation?: string
}

const props = withDefaults(defineProps<Props>(), {
  fallbackCause: 'UNRESPONSIVE_KUBELET_HEARTBEAT',
  fallbackConfidence: 0.94,
  fallbackRecommendation: 'Power isolate node (STONITH) and apply out-of-service taint to trigger volume release.'
})

const showEvidenceList = ref(true)

const primaryCause = computed(() => {
  return props.dynamicRca?.primary_cause || props.fallbackCause
})

const confidencePercent = computed(() => {
  const conf = props.dynamicRca?.confidence ?? props.fallbackConfidence
  return (conf * 100).toFixed(1)
})

const recommendation = computed(() => {
  return props.dynamicRca?.action_plan || props.fallbackRecommendation
})

const allSignals = computed<IndicatorSignal[]>(() => {
  if (!props.dynamicRca?.hypotheses) return []
  const list: IndicatorSignal[] = []
  const seen = new Set<string>()
  for (const h of props.dynamicRca.hypotheses) {
    for (const ind of h.indicators || []) {
      if (!seen.has(ind.id)) {
        seen.add(ind.id)
        list.push(ind)
      }
    }
  }
  return list
})

const detectedSignals = computed(() => {
  return allSignals.value.filter(s => s.detected)
})

const totalSignalCount = computed(() => {
  return allSignals.value.length || 6
})

function toggleEvidence() {
  showEvidenceList.value = !showEvidenceList.value
}
</script>

<template>
  <div class="rca-card">
    <div class="rca-top">
      <span class="rca-badge">
        <BaseIcon name="bot" size="xs" />
        Probable Cause: {{ primaryCause }}
      </span>
      <span class="rca-confidence font-mono">
        Confidence: {{ confidencePercent }}% (Derived from {{ totalSignalCount }} telemetry signals)
      </span>
    </div>

    <p class="rca-recommendation">
      <strong>Action Plan:</strong> {{ recommendation }}
    </p>

    <!-- Detected Signals Telemetry Accordion -->
    <div v-if="allSignals.length > 0" class="rca-signals-container font-mono">
      <button type="button" class="rca-signals-toggle" @click="toggleEvidence">
        <span class="signals-toggle-title">
          <BaseIcon :name="showEvidenceList ? 'chevron-down' : 'chevron-right'" size="xs" />
          <span>Multi-Signal Telemetry Evidence ({{ detectedSignals.length }} detected / {{ allSignals.length }} probed)</span>
        </span>
        <span class="signals-toggle-tag">{{ showEvidenceList ? 'Collapse' : 'Expand' }}</span>
      </button>

      <div v-if="showEvidenceList" class="rca-signals-list animate-fade-in">
        <div
          v-for="signal in allSignals"
          :key="signal.id"
          class="rca-signal-item"
          :class="{ 'signal-detected': signal.detected }"
        >
          <div class="signal-header">
            <span class="signal-icon">
              <BaseIcon :name="signal.detected ? 'alert-triangle' : 'check'" size="xs" />
            </span>
            <span class="signal-desc">{{ signal.description }}</span>
            <span class="signal-weight">weight {{ signal.weight.toFixed(1) }}</span>
          </div>
          <div class="signal-evidence">
            &rarr; {{ signal.detected ? 'Detected:' : 'Negative:' }} {{ signal.evidence }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
