<script setup lang="ts">
import { ref } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'

interface Props {
  controllerName: string
  title?: string
  description?: string
  command?: string
  actionText?: string
  actionLink?: string
  icon?: string
}

const props = withDefaults(defineProps<Props>(), {
  title: 'Controller Not Connected',
  description: '',
  command: '',
  actionText: '',
  actionLink: '',
  icon: 'shield',
})

const emit = defineEmits<{
  (e: 'retry'): void
}>()

const copied = ref(false)

function copyCommand() {
  if (!props.command) return
  navigator.clipboard.writeText(props.command).then(() => {
    copied.value = true
    setTimeout(() => { copied.value = false }, 2000)
  }).catch(() => {})
}
</script>

<template>
  <div class="integration-empty-state glass-panel animate-fade-in" role="region" aria-label="Integration Controller Unavailable">
    <div class="empty-icon-wrap">
      <BaseIcon :name="icon" size="md" class="empty-icon" />
    </div>

    <div class="empty-content">
      <div class="empty-header">
        <h3 class="empty-title">{{ title }}</h3>
        <span class="controller-badge font-mono">Requires {{ controllerName }}</span>
      </div>

      <p class="empty-desc">
        {{ description || `The ${controllerName} is not installed or returned HTTP 501 (Not Implemented). External controller components must be deployed to enable this capability.` }}
      </p>

      <div v-if="command" class="command-box font-mono">
        <span class="cmd-prompt">$</span>
        <code class="cmd-text">{{ command }}</code>
        <button
          type="button"
          class="btn-copy"
          :title="copied ? 'Copied!' : 'Copy installation command'"
          aria-label="Copy installation command"
          @click="copyCommand"
        >
          <BaseIcon :name="copied ? 'check' : 'copy'" size="xs" />
          <span>{{ copied ? 'Copied' : 'Copy' }}</span>
        </button>
      </div>

      <div class="empty-actions">
        <button
          type="button"
          class="btn btn-secondary btn-xs"
          @click="emit('retry')"
        >
          <BaseIcon name="refresh" size="xs" />
          <span>Check Again</span>
        </button>

        <router-link
          v-if="actionLink"
          :to="actionLink"
          class="btn btn-primary btn-xs"
        >
          <span>{{ actionText || 'Configure in Settings' }}</span>
        </router-link>
      </div>
    </div>
  </div>
</template>

<style scoped>
.integration-empty-state {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 20px 24px;
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.6);
  border: 1px solid rgba(245, 158, 11, 0.25);
  box-shadow: 0 4px 20px -2px rgba(0, 0, 0, 0.4);
  margin: 12px 0;
}
.empty-icon-wrap {
  width: 40px;
  height: 40px;
  border-radius: 8px;
  background: rgba(245, 158, 11, 0.1);
  border: 1px solid rgba(245, 158, 11, 0.2);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: #fbbf24;
}
.empty-content {
  flex: 1;
  min-width: 0;
}
.empty-header {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 6px;
}
.empty-title {
  font-size: 14px;
  font-weight: 700;
  color: #f8fafc;
  margin: 0;
}
.controller-badge {
  font-size: 10.5px;
  font-weight: 600;
  padding: 2px 7px;
  border-radius: 4px;
  background: rgba(245, 158, 11, 0.15);
  border: 1px solid rgba(245, 158, 11, 0.3);
  color: #fbbf24;
}
.empty-desc {
  font-size: 12px;
  line-height: 1.5;
  color: #94a3b8;
  margin: 0 0 12px 0;
}
.command-box {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  background: rgba(0, 0, 0, 0.5);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 6px;
  font-size: 11px;
  margin-bottom: 12px;
  overflow-x: auto;
}
.cmd-prompt {
  color: #64748b;
  user-select: none;
}
.cmd-text {
  color: #38bdf8;
  flex: 1;
  white-space: nowrap;
}
.btn-copy {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #cbd5e1;
  font-size: 10px;
  cursor: pointer;
  transition: all 0.15s ease;
}
.btn-copy:hover {
  background: rgba(255, 255, 255, 0.12);
  color: #fff;
}
.empty-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
