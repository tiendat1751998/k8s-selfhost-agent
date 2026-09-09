<script setup lang="ts">
import { onMounted, watch } from 'vue';

const props = withDefaults(
  defineProps<{
    modelValue: 'grid' | 'table';
    gridLabel?: string;
    tableLabel?: string;
    storageKey?: string;
  }>(),
  { gridLabel: 'Grid / Cards', tableLabel: 'Data Table', storageKey: undefined }
);
const emit = defineEmits<{ (e: 'update:modelValue', value: 'grid' | 'table'): void }>();

const setMode = (mode: 'grid' | 'table') => {
  if (props.modelValue !== mode) emit('update:modelValue', mode);
  if (props.storageKey) {
    try { localStorage.setItem(props.storageKey, mode); }
    catch (e) { console.warn('Failed to save view mode to localStorage', e); }
  }
};
const handleKey = (e: KeyboardEvent) => {
  if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') { e.preventDefault(); setMode('table'); }
  else if (e.key === 'ArrowRight' || e.key === 'ArrowDown') { e.preventDefault(); setMode('grid'); }
};
watch(() => props.modelValue, (val) => {
  if (props.storageKey && (val === 'grid' || val === 'table')) {
    try { localStorage.setItem(props.storageKey, val); }
    catch (e) { console.warn('Failed to sync view mode to localStorage', e); }
  }
});
onMounted(() => {
  if (props.storageKey) {
    try {
      const saved = localStorage.getItem(props.storageKey);
      if (saved === 'grid' || saved === 'table') emit('update:modelValue', saved);
    } catch (e) { console.warn('Failed to read view mode from localStorage', e); }
  }
});
</script>

<template>
  <div class="view-mode-switcher" role="radiogroup" aria-label="Layout view mode" @keydown="handleKey">
    <button
      type="button" role="radio" :aria-checked="modelValue === 'table'"
      :tabindex="modelValue === 'table' ? 0 : -1" class="mode-btn"
      :class="{ active: modelValue === 'table' }" @click="setMode('table')"
    >
      <svg class="mode-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M3 3h18v18H3z"/><path d="M3 9h18M3 15h18M9 3v18"/>
      </svg>
      <span>{{ tableLabel }}</span>
    </button>
    <button
      type="button" role="radio" :aria-checked="modelValue === 'grid'"
      :tabindex="modelValue === 'grid' ? 0 : -1" class="mode-btn"
      :class="{ active: modelValue === 'grid' }" @click="setMode('grid')"
    >
      <svg class="mode-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/>
        <rect x="14" y="14" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/>
      </svg>
      <span>{{ gridLabel }}</span>
    </button>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/ui/common.css';
</style>
