<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import type { MetricAlert } from '../../../api/overview'
import type { MutedAlertConfig } from '../../../stores/alertStore'
import AlertFilterBar, { type AlertTabType } from './AlertFilterBar.vue'
import AlertBatchActionsBar from './AlertBatchActionsBar.vue'
import AlertListTable from './AlertListTable.vue'
import NodeRemediationModal from '../drawer/NodeRemediationModal.vue'
import { useAlertStore } from '../../../stores/alertStore'

interface Props {
  show: boolean
  activeAlerts: MetricAlert[]
  mutedAlertsList: MutedAlertConfig[]
  hasCriticalAlerts: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'mute-alert', alert: MetricAlert, mode: 'restart' | '1h' | '24h' | 'session' | 'forever'): void
  (e: 'unmute-alert', key: string): void
  (e: 'mute-all', mode: 'restart' | '1h' | '24h'): void
  (e: 'unmute-all'): void
  (e: 'dismiss-alert', alert: MetricAlert): void
  (e: 'navigate-to-host', nodeNameOrId: string): void
}>()

const alertStore = useAlertStore()
const activeTab = ref<AlertTabType>('active')
const showRemediationModal = ref(false)
const remediationNodeName = ref('')

function handleRemediateNode(nodeName: string) {
  remediationNodeName.value = nodeName
  showRemediationModal.value = true
}

function handleCloseRemediation() {
  showRemediationModal.value = false
  alertStore.closeRemediation()
}

watch(
  () => alertStore.showRemediationModal,
  (open) => {
    if (open) {
      remediationNodeName.value = alertStore.targetRemediationNode
      showRemediationModal.value = true
    }
  }
)

watch(
  () => props.show,
  (isOpen) => {
    if (isOpen) {
      if (props.activeAlerts.length > 0) {
        activeTab.value = 'active'
      } else if (props.mutedAlertsList.length > 0) {
        activeTab.value = 'muted'
      } else {
        activeTab.value = 'all'
      }
    }
  },
  { immediate: true }
)

function handleKeyDown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.show) {
    emit('close')
  }
}

onMounted(() => document.addEventListener('keydown', handleKeyDown))
onUnmounted(() => document.removeEventListener('keydown', handleKeyDown))
</script>

<template>
  <transition name="modal-fade">
    <div
      v-if="show"
      class="alert-modal-backdrop animate-fade-in"
      role="dialog"
      aria-modal="true"
      aria-labelledby="alert-modal-title"
      @click.self="emit('close')"
    >
      <div class="alert-center-modal glass-panel animate-scale-in">
        <!-- HEADER -->
        <div class="alert-modal-header">
          <div class="alert-modal-title-group">
            <div
              class="alert-modal-icon-badge"
              :class="activeAlerts.length > 0 ? (hasCriticalAlerts ? 'badge-icon-rose' : 'badge-icon-amber') : 'badge-icon-cyan'"
            >
              {{ activeAlerts.length > 0 ? (hasCriticalAlerts ? '🚨' : '⚠️') : '🛡️' }}
            </div>
            <div>
              <div class="title-with-badge">
                <h3 id="alert-modal-title" class="alert-modal-title">
                  Cluster Alert Center &amp; Diagnostics
                </h3>
                <span
                  v-if="activeAlerts.length > 0"
                  class="badge"
                  :class="hasCriticalAlerts ? 'badge-rose' : 'badge-amber'"
                >
                  {{ activeAlerts.length }} Active
                </span>
                <span v-if="mutedAlertsList.length > 0" class="badge badge-muted-tag">
                  {{ mutedAlertsList.length }} Silenced
                </span>
              </div>
              <p class="alert-modal-subtitle">
                Real-time node health violations, autonomous threshold warnings, and persistent silencing policies.
              </p>
            </div>
          </div>

          <div class="alert-header-right-actions">
            <AlertBatchActionsBar
              placement="header"
              :activeCount="activeAlerts.length"
              :mutedCount="mutedAlertsList.length"
              @mute-all="(mode) => emit('mute-all', mode)"
              @unmute-all="emit('unmute-all')"
            />
            <button
              class="btn-modal-close"
              @click="emit('close')"
              title="Close Alert Center"
              aria-label="Close"
            >
              ✕
            </button>
          </div>
        </div>

        <!-- TAB NAVIGATION -->
        <AlertFilterBar
          :activeTab="activeTab"
          :activeCount="activeAlerts.length"
          :mutedCount="mutedAlertsList.length"
          @update:activeTab="(tab) => activeTab = tab"
        />

        <!-- BODY LIST -->
        <AlertListTable
          :activeTab="activeTab"
          :activeAlerts="activeAlerts"
          :mutedAlertsList="mutedAlertsList"
          @mute-alert="(alert, mode) => emit('mute-alert', alert, mode)"
          @unmute-alert="(key) => emit('unmute-alert', key)"
          @dismiss-alert="(alert) => emit('dismiss-alert', alert)"
          @navigate-to-host="(node) => emit('navigate-to-host', node)"
          @remediate-node="handleRemediateNode"
        />

        <!-- FOOTER -->
        <div class="alert-modal-footer">
          <div class="footer-summary-text">
            <span>{{ activeAlerts.length }} active &bull; {{ mutedAlertsList.length }} silenced</span>
          </div>

          <div class="footer-actions">
            <AlertBatchActionsBar
              placement="footer"
              :activeCount="activeAlerts.length"
              :mutedCount="mutedAlertsList.length"
              @mute-all="(mode) => emit('mute-all', mode)"
              @unmute-all="emit('unmute-all')"
            />
            <button class="btn btn-primary btn-footer-close" @click="emit('close')">
              <span>Done</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </transition>

  <NodeRemediationModal
    :show="showRemediationModal"
    :nodeName="remediationNodeName"
    clusterId="default"
    @close="handleCloseRemediation"
    @remediated="handleCloseRemediation"
  />
</template>

<style scoped>
@import '../../../assets/styles/components/alert-center.css';
</style>
