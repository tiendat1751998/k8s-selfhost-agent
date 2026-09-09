<script setup lang="ts">
import CyberDateTimePicker from '../../ui/CyberDateTimePicker.vue'

interface Props {
  from: string
  to: string
  loading?: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:from', val: string): void
  (e: 'update:to', val: string): void
  (e: 'apply-preset', preset: '30m' | '2h' | '6h' | 'today'): void
  (e: 'apply'): void
}>()
</script>

<template>
  <div class="custom-range-bar glass-panel animate-fadeIn">
    <div class="custom-range-inputs">
      <div class="range-field">
        <label class="range-label font-mono">FROM:</label>
        <CyberDateTimePicker
          :modelValue="from"
          placeholder="From datetime"
          @update:modelValue="emit('update:from', $event)"
          @apply="emit('apply')"
        />
      </div>
      <div class="range-field">
        <label class="range-label font-mono">TO:</label>
        <CyberDateTimePicker
          :modelValue="to"
          placeholder="To datetime"
          @update:modelValue="emit('update:to', $event)"
          @apply="emit('apply')"
        />
      </div>
    </div>

    <div class="custom-range-presets">
      <span class="preset-label font-mono">PRESETS:</span>
      <button type="button" class="btn-preset-chip font-mono" @click="emit('apply-preset', '30m')">Last 30m</button>
      <button type="button" class="btn-preset-chip font-mono" @click="emit('apply-preset', '2h')">Last 2h</button>
      <button type="button" class="btn-preset-chip font-mono" @click="emit('apply-preset', '6h')">Last 6h</button>
      <button type="button" class="btn-preset-chip font-mono" @click="emit('apply-preset', 'today')">Today</button>
    </div>

    <div class="custom-range-actions">
      <button
        type="button"
        class="btn-apply-range font-mono"
        :disabled="loading"
        @click="emit('apply')"
      >
        <span class="glow-dot"></span>
        <span>{{ loading ? 'Applying...' : 'Apply Window' }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
@import '../../../assets/styles/components/node-historical-chart.css';
</style>
