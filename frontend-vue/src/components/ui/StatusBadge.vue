<script setup lang="ts">
import { computed } from 'vue'

export type StatusVariant = 'emerald' | 'amber' | 'rose' | 'slate' | 'cyan' | 'violet'

interface Props {
  status?: string
  label?: string
  pulse?: boolean
  size?: 'sm' | 'md'
  class?: string
}

const props = withDefaults(defineProps<Props>(), {
  status: 'unknown',
  size: 'sm'
})

const EMERALD_STATUSES = new Set([
  'ready', 'healthy', 'active', 'online', 'connected', 'ok', 'live', 'running', 'deployed', 'success',
  'pass', 'resolved', 'mitigated', 'completed', 'verified', 'armed'
])

const AMBER_STATUSES = new Set([
  'warning', 'pending', 'degraded', 'elevated', 'in_progress', 'inprogress', 'standby', 'polling',
  'open', 'promoting', 'mitigating'
])

const ROSE_STATUSES = new Set([
  'failed', 'critical', 'error', 'offline', 'unhealthy', 'disconnected', 'down', 'danger',
  'rejected', 'blocked', 'mismatch'
])

const SLATE_STATUSES = new Set([
  'cordoned', 'draining', 'terminating', 'disabled', 'unknown'
])

const CYAN_STATUSES = new Set([
  'info', 'analyzing', 'remediating', 'idle', 'draft', 'generating'
])

const ACTIVE_PULSE_STATUSES = new Set([
  'ready', 'healthy', 'active', 'running', 'online', 'connected', 'live', 'ok', 'armed'
])

const normalizedStatus = computed<StatusVariant>(() => {
  if (!props.status) return 'slate'
  const lower = props.status.toLowerCase().trim()
  if (EMERALD_STATUSES.has(lower) || lower === 'emerald') return 'emerald'
  if (AMBER_STATUSES.has(lower) || lower === 'amber') return 'amber'
  if (ROSE_STATUSES.has(lower) || lower === 'rose') return 'rose'
  if (SLATE_STATUSES.has(lower) || lower === 'slate') return 'slate'
  if (CYAN_STATUSES.has(lower) || lower === 'cyan') return 'cyan'
  if (lower === 'violet') return 'violet'
  return 'slate'
})

const isPulsing = computed<boolean>(() => {
  if (typeof props.pulse === 'boolean') {
    return props.pulse
  }
  const lower = (props.status || '').toLowerCase().trim()
  return ACTIVE_PULSE_STATUSES.has(lower)
})

const displayLabel = computed<string>(() => {
  if (props.label !== undefined && props.label !== '') {
    return props.label
  }
  const raw = (props.status || 'unknown').trim()
  if (!raw) return 'Unknown'
  return raw.charAt(0).toUpperCase() + raw.slice(1)
})
</script>

<template>
  <span
    class="status-badge"
    :class="[
      `badge-${normalizedStatus}`,
      size === 'md' ? 'badge-md' : 'badge-sm',
      props.class
    ]"
  >
    <span
      class="status-dot"
      :class="[`dot-${normalizedStatus}`, { 'dot-pulse': isPulsing }]"
      aria-hidden="true"
    ></span>
    <span class="status-text">{{ displayLabel }}</span>
  </span>
</template>

<style scoped>
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border-radius: 9999px;
  font-family: 'Inter', var(--font-sans, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif);
  font-weight: 600;
  font-size: 11px;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  line-height: 1;
  white-space: nowrap;
  vertical-align: middle;
  border: 1px solid transparent;
  box-sizing: border-box;
  user-select: none;
}

.badge-sm {
  padding: 2px 8px;
  font-size: 10px;
}

.badge-md {
  padding: 4px 10px;
  font-size: 11px;
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}

@keyframes status-pulse {
  0%, 100% {
    opacity: 1;
    transform: scale(1);
  }
  50% {
    opacity: 0.4;
    transform: scale(0.85);
  }
}

.dot-pulse {
  animation: status-pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite;
}

/* Emerald: Ready, Healthy, Active, Running */
.badge-emerald {
  background-color: rgba(16, 185, 129, 0.12);
  border-color: rgba(16, 185, 129, 0.25);
  color: #10b981;
}
.dot-emerald {
  background-color: #10b981;
}

/* Amber: Pending, Warning, Degraded, Elevated */
.badge-amber {
  background-color: rgba(245, 158, 11, 0.12);
  border-color: rgba(245, 158, 11, 0.25);
  color: #f59e0b;
}
.dot-amber {
  background-color: #f59e0b;
}

/* Rose: Failed, Critical, Offline, Error */
.badge-rose {
  background-color: rgba(244, 63, 94, 0.12);
  border-color: rgba(244, 63, 94, 0.25);
  color: #f43f5e;
}
.dot-rose {
  background-color: #f43f5e;
}

/* Slate: Cordoned, Draining, Terminating, Unknown */
.badge-slate {
  background-color: rgba(148, 163, 184, 0.12);
  border-color: rgba(148, 163, 184, 0.25);
  color: #94a3b8;
}
.dot-slate {
  background-color: #94a3b8;
}

/* Cyan: Info, Analyzing, Remediating */
.badge-cyan {
  background-color: rgba(6, 182, 212, 0.12);
  border-color: rgba(6, 182, 212, 0.25);
  color: #06b6d4;
}
.dot-cyan {
  background-color: #06b6d4;
}

/* Violet */
.badge-violet {
  background-color: rgba(139, 92, 246, 0.12);
  border-color: rgba(139, 92, 246, 0.25);
  color: #8b5cf6;
}
.dot-violet {
  background-color: #8b5cf6;
}

.status-text {
  font-variant-numeric: tabular-nums;
}
</style>
