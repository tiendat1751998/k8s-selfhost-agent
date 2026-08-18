<script setup lang="ts">
defineProps<{
  status: string
  label?: string
  size?: 'sm' | 'md'
}>()

function normalizeStatus(s: string) {
  if (!s) return 'unknown'
  const lower = s.toLowerCase()
  if (['healthy', 'active', 'pass', 'success', 'resolved', 'deployed', 'completed', 'verified', 'running', 'armed'].includes(lower)) {
    return 'emerald'
  }
  if (['warning', 'pending', 'standby', 'polling', 'inprogress', 'in_progress', 'promoting', 'open'].includes(lower)) {
    return 'amber'
  }
  if (['critical', 'danger', 'failed', 'error', 'degraded', 'rejected', 'blocked', 'mismatch', 'down'].includes(lower)) {
    return 'rose'
  }
  if (['info', 'analyzing', 'remediating', 'generating', 'idle', 'draft'].includes(lower)) {
    return 'cyan'
  }
  return 'violet'
}
</script>

<template>
  <span 
    class="status-badge" 
    :class="[`badge-${normalizeStatus(status)}`, size === 'sm' ? 'badge-sm' : 'badge-md']"
  >
    <span class="status-dot" :class="`dot-${normalizeStatus(status)}`"></span>
    <span class="status-text">{{ label || status }}</span>
  </span>
</template>

<style scoped>
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border-radius: 9999px;
  font-weight: 600;
  letter-spacing: 0.03em;
  text-transform: uppercase;
  font-family: var(--font-mono);
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
}

.badge-emerald {
  background: rgba(16, 185, 129, 0.12);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
}
.dot-emerald {
  background-color: #10b981;
  box-shadow: 0 0 6px #10b981;
}

.badge-amber {
  background: rgba(245, 158, 11, 0.12);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.3);
}
.dot-amber {
  background-color: #f59e0b;
  box-shadow: 0 0 6px #f59e0b;
}

.badge-rose {
  background: rgba(244, 63, 94, 0.12);
  color: #fb7185;
  border: 1px solid rgba(244, 63, 94, 0.3);
}
.dot-rose {
  background-color: #f43f5e;
  box-shadow: 0 0 6px #f43f5e;
}

.badge-cyan {
  background: rgba(6, 182, 212, 0.12);
  color: #38bdf8;
  border: 1px solid rgba(6, 182, 212, 0.3);
}
.dot-cyan {
  background-color: #06b6d4;
  box-shadow: 0 0 6px #06b6d4;
}

.badge-violet {
  background: rgba(139, 92, 246, 0.12);
  color: #a78bfa;
  border: 1px solid rgba(139, 92, 246, 0.3);
}
.dot-violet {
  background-color: #8b5cf6;
  box-shadow: 0 0 6px #8b5cf6;
}
</style>
