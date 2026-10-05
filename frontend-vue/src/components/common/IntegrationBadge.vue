<script setup lang="ts">
import { ref, computed } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'

interface Props {
  name: string
  status?: 'connected' | 'not_connected' | 'unsupported' | string
  label?: string
  tooltip?: string
  requiredController?: string
  installHint?: string
}

const props = withDefaults(defineProps<Props>(), {
  status: 'not_connected',
  label: '',
  tooltip: '',
  requiredController: '',
  installHint: '',
})

const isConnected = computed(() => props.status === 'connected')
const showPopover = ref(false)

const badgeText = computed(() => {
  if (props.label) return props.label
  return isConnected.value ? `${props.name}: Connected` : `${props.name}: Not Connected`
})

const defaultTooltip = computed(() => {
  if (props.tooltip) return props.tooltip
  if (isConnected.value) {
    return `${props.name} controller is active and synchronized.`
  }
  return `Requires external controller: ${props.requiredController || props.name}. (HTTP 501 Not Implemented)`
})
</script>

<template>
  <div
    class="integration-badge-container font-mono"
    @mouseenter="showPopover = true"
    @mouseleave="showPopover = false"
  >
    <div
      class="integration-badge-pill"
      :class="isConnected ? 'pill-connected' : 'pill-disconnected'"
      role="status"
      :aria-label="badgeText"
      tabindex="0"
      @focus="showPopover = true"
      @blur="showPopover = false"
    >
      <span class="status-dot" :class="isConnected ? 'dot-emerald' : 'dot-amber'" />
      <span class="badge-label">{{ badgeText }}</span>
      <BaseIcon name="info" size="xs" class="info-icon" />
    </div>

    <!-- Informative Popover / Tooltip -->
    <div v-if="showPopover" class="integration-popover glass-panel animate-scale-in" role="tooltip">
      <div class="popover-header">
        <BaseIcon :name="isConnected ? 'check-circle' : 'alert-triangle'" size="xs" :class="isConnected ? 'text-emerald' : 'text-amber'" />
        <span class="popover-title font-bold">{{ name }} Integration</span>
      </div>
      <p class="popover-desc">{{ defaultTooltip }}</p>
      <div v-if="installHint && !isConnected" class="install-snippet">
        <code>{{ installHint }}</code>
      </div>
    </div>
  </div>
</template>

<style scoped>
.integration-badge-container {
  position: relative;
  display: inline-flex;
  align-items: center;
}
.integration-badge-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 24px;
  padding: 0 8px;
  border-radius: 9999px;
  font-size: 11px;
  font-weight: 500;
  cursor: pointer;
  user-select: none;
  transition: all 0.15s ease;
}
.pill-connected {
  background: rgba(16, 185, 129, 0.1);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
}
.pill-disconnected {
  background: rgba(245, 158, 11, 0.1);
  border: 1px solid rgba(245, 158, 11, 0.3);
  color: #fbbf24;
}
.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}
.dot-emerald {
  background: #10b981;
  box-shadow: 0 0 6px rgba(16, 185, 129, 0.6);
}
.dot-amber {
  background: #f59e0b;
  box-shadow: 0 0 6px rgba(245, 158, 11, 0.5);
}
.badge-label {
  white-space: nowrap;
}
.info-icon {
  opacity: 0.6;
}
.integration-popover {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  z-index: 50;
  min-width: 240px;
  max-width: 320px;
  padding: 10px 12px;
  border-radius: 8px;
  background: #0f172a;
  border: 1px solid rgba(148, 163, 184, 0.2);
  box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.6);
  pointer-events: none;
}
.popover-header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 4px;
  font-size: 11.5px;
}
.popover-title {
  color: #f1f5f9;
}
.popover-desc {
  font-size: 11px;
  color: #94a3b8;
  margin: 0;
  line-height: 1.4;
  white-space: normal;
}
.install-snippet {
  margin-top: 6px;
  padding: 4px 6px;
  background: rgba(0, 0, 0, 0.4);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 4px;
  font-size: 10px;
  color: #38bdf8;
  word-break: break-all;
}
.text-emerald { color: #34d399; }
.text-amber { color: #fbbf24; }
</style>
