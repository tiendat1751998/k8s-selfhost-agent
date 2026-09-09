<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { Incident, RCAReport, PullRequest } from '../../api/compute'
import type { RcaTimelineEvent } from '../../composables/useIncidents'

defineProps<{
  show: boolean
  incident: Incident | null
  report: RCAReport | null
  activePr: PullRequest | null
  events: RcaTimelineEvent[]
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
}>()

function formatTime(d?: string) {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  } catch {
    return d
  }
}

function getIconForType(type: RcaTimelineEvent['type']) {
  switch (type) {
    case 'detection': return '⚠️'
    case 'correlation': return '🔗'
    case 'analysis': return '🧠'
    case 'pr': return '🔀'
    case 'resolution': return '🛡️'
    default: return '⏱️'
  }
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="modal"
    title="🔬 Interactive RCA Chronology & Reasoning Timeline"
    :subtitle="incident ? `Workload: ${incident.pod_name} (${incident.cluster_name} · ns/${incident.namespace})` : 'Root Cause Analysis Timeline'"
    max-width="700px"
    @update:show="emit('update:show', $event)"
  >
    <div class="rca-timeline-modal-body">
      <div v-if="events.length === 0" class="no-events-box text-muted font-mono">
        <span>No chronological events captured yet.</span>
      </div>


      <div v-else class="rca-timeline-stream">
        <div
          v-for="(evt, idx) in events"
          :key="evt.id"
          class="rca-timeline-step animate-fade-in"
          :class="`step-type-${evt.type}`"
        >
          <!-- Timeline Node Line & Icon -->
          <div class="step-rail">
            <div class="step-icon-circle">
              <span>{{ getIconForType(evt.type) }}</span>
            </div>
            <div v-if="idx < events.length - 1" class="step-connector"></div>
          </div>


          <!-- Step Content Body -->
          <div class="step-content glass-panel">
            <div class="step-header">
              <h4 class="step-title">{{ evt.title }}</h4>
              <span class="step-time font-mono">{{ formatTime(evt.timestamp) }}</span>
            </div>


            <p class="step-desc font-mono">{{ evt.description }}</p>


            <!-- Evidence List -->
            <div v-if="evt.evidence && evt.evidence.length > 0" class="step-evidence">
              <div v-for="(ev, evIdx) in evt.evidence" :key="evIdx" class="step-ev-tag font-mono">
                <span class="step-bullet">▸</span>
                <span>{{ ev }}</span>
              </div>
            </div>


            <!-- Metadata Pills -->
            <div v-if="evt.meta" class="step-meta-row font-mono">
              <div v-for="(val, key) in evt.meta" :key="key" class="step-meta-pill">
                <span class="meta-k">{{ key }}:</span>
                <span class="meta-v">{{ val }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>


    <template #footer="{ close }">
      <button type="button" class="btn btn-secondary" @click="close">Close Chronology</button>
    </template>
  </ModalDrawer>
</template>
