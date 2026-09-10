<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import BaseIcon from './BaseIcon.vue'

export interface FilterOption {
  value: string
  label: string
}

export interface FilterFacet {
  key: string
  label: string
  options: FilterOption[]
}

interface Props {
  modelValue?: string
  placeholder?: string
  facets?: FilterFacet[]
  activeFilters?: Record<string, string>
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: '',
  placeholder: 'Filter resources by name, label, status... (Press / to focus)',
  facets: () => [],
  activeFilters: () => ({})
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'update:activeFilters', filters: Record<string, string>): void
  (e: 'clear'): void
}>()

const searchInputRef = ref<HTMLInputElement | null>(null)
const openFacetKey = ref<string | null>(null)

const activeTokens = computed(() => {
  const tokens: Array<{ key: string; label: string; value: string; displayValue: string }> = []
  if (!props.facets || !props.activeFilters) return tokens
  for (const facet of props.facets) {
    const val = props.activeFilters[facet.key]
    if (val && val !== 'all') {
      const opt = facet.options.find(o => o.value === val)
      tokens.push({
        key: facet.key,
        label: facet.label,
        value: val,
        displayValue: opt ? opt.label : val
      })
    }
  }
  return tokens
})

const hasActiveFilters = computed(() => {
  return activeTokens.value.length > 0 || Boolean(props.modelValue && props.modelValue.trim())
})

let debounceTimer: ReturnType<typeof setTimeout> | null = null

function onSearchInput(e: Event) {
  const val = (e.target as HTMLInputElement).value
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    emit('update:modelValue', val)
  }, 150)
}

function removeFilter(key: string) {
  const updated = { ...props.activeFilters }
  delete updated[key]
  emit('update:activeFilters', updated)
}

function selectFacetOption(facetKey: string, val: string) {
  const updated = { ...props.activeFilters }
  if (!val || val === 'all') delete updated[facetKey]
  else updated[facetKey] = val
  emit('update:activeFilters', updated)
  openFacetKey.value = null
}

function clearAll() {
  if (debounceTimer) { clearTimeout(debounceTimer); debounceTimer = null }
  emit('update:modelValue', '')
  emit('update:activeFilters', {})
  emit('clear')
}

function toggleFacet(key: string) {
  openFacetKey.value = openFacetKey.value === key ? null : key
}

function handleGlobalKeydown(e: KeyboardEvent) {
  const target = e.target as HTMLElement | null
  const isEditing = target && (
    target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable
  )
  if (e.key === '/' && !isEditing && !e.ctrlKey && !e.metaKey && !e.altKey) {
    e.preventDefault(); searchInputRef.value?.focus(); searchInputRef.value?.select()
  } else if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'f' && !isEditing) {
    e.preventDefault(); searchInputRef.value?.focus(); searchInputRef.value?.select()
  } else if (e.key === 'Escape' && openFacetKey.value) {
    openFacetKey.value = null
  }
}

function onDocClick(e: MouseEvent) {
  if (!openFacetKey.value) return
  const t = e.target as HTMLElement | null
  if (t && !t.closest('.facet-menu-container')) openFacetKey.value = null
}

onMounted(() => {
  window.addEventListener('keydown', handleGlobalKeydown)
  document.addEventListener('click', onDocClick)
})

onUnmounted(() => {
  if (debounceTimer) { clearTimeout(debounceTimer); debounceTimer = null }
  window.removeEventListener('keydown', handleGlobalKeydown)
  document.removeEventListener('click', onDocClick)
})
</script>

<template>
  <div class="faceted-filter-bar">
    <div class="filter-main-row">
      <div class="search-input-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          ref="searchInputRef" type="text" :value="modelValue" :placeholder="placeholder"
          class="filter-search-input font-mono" @input="onSearchInput"
        />
        <kbd class="shortcut-badge" title="Focus search bar">/</kbd>
      </div>

      <div v-if="facets.length > 0" class="facet-dropdowns">
        <div v-for="facet in facets" :key="facet.key" class="facet-menu-container">
          <button
            type="button" class="facet-btn"
            :class="{ 'has-selection': Boolean(activeFilters[facet.key] && activeFilters[facet.key] !== 'all') }"
            @click.stop="toggleFacet(facet.key)"
          >
            <span>{{ facet.label }}</span>
            <svg class="facet-chevron" width="8" height="6" viewBox="0 0 8 6" fill="none"><path d="M1 1.5L4 4.5L7 1.5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/></svg>
          </button>

          <div v-if="openFacetKey === facet.key" class="facet-popover" @click.stop>
            <button
              type="button" class="facet-opt"
              :class="{ 'is-active': !activeFilters[facet.key] || activeFilters[facet.key] === 'all' }"
              @click="selectFacetOption(facet.key, 'all')"
            >
              All {{ facet.label }}s
            </button>
            <button
              v-for="opt in facet.options" :key="opt.value" type="button" class="facet-opt"
              :class="{ 'is-active': activeFilters[facet.key] === opt.value }"
              @click="selectFacetOption(facet.key, opt.value)"
            >
              {{ opt.label }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <div v-if="hasActiveFilters" class="filter-chips-row">
      <div class="token-chips">
        <span v-for="token in activeTokens" :key="token.key" class="filter-token-chip">
          <span class="token-key">{{ token.label }}:</span>
          <span class="token-val font-mono">{{ token.displayValue }}</span>
          <button
            type="button" class="token-remove-btn"
            :aria-label="`Remove ${token.label} filter`" @click="removeFilter(token.key)"
          >
            <BaseIcon name="x" size="xs" />
          </button>
        </span>
      </div>

      <button type="button" class="clear-all-btn" @click="clearAll">
        Clear All
      </button>
    </div>
  </div>
</template>

<style scoped>
.faceted-filter-bar {
  display: flex; flex-direction: column; gap: 8px; padding: 12px 16px;
  background: rgba(15, 23, 42, 0.4); border-bottom: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
}
.filter-main-row { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.search-input-wrap {
  position: relative; display: flex; align-items: center; flex: 1; min-width: 220px; height: 32px;
  background: rgba(255, 255, 255, 0.04); border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.12));
  border-radius: 6px; padding: 0 10px; transition: all 0.15s ease;
}
.search-input-wrap:focus-within { border-color: rgba(6, 182, 212, 0.5); box-shadow: 0 0 0 1px rgba(6, 182, 212, 0.2); }
.search-icon { color: var(--text-muted, #94a3b8); margin-right: 8px; flex-shrink: 0; }
.filter-search-input { flex: 1; background: transparent; border: none; outline: none; color: var(--text-primary, #f1f5f9); font-size: 12px; }
.shortcut-badge {
  padding: 1px 6px; background: rgba(255, 255, 255, 0.08); border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 4px; color: var(--text-secondary, #94a3b8); font-size: 10px; font-family: var(--font-mono, monospace); line-height: 1;
}
.facet-dropdowns { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.facet-menu-container { position: relative; }
.facet-btn {
  display: inline-flex; align-items: center; gap: 6px; height: 32px; padding: 0 10px;
  background: rgba(255, 255, 255, 0.04); border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.1));
  border-radius: 6px; color: var(--text-secondary, #94a3b8); font-size: 12px; font-weight: 500; cursor: pointer; transition: all 0.15s ease;
}
.facet-btn:hover, .facet-btn.has-selection { background: rgba(255, 255, 255, 0.08); color: var(--text-primary, #fff); }
.facet-btn.has-selection { border-color: rgba(6, 182, 212, 0.4); color: #38bdf8; }
.facet-chevron { opacity: 0.6; }
.facet-popover {
  position: absolute; top: calc(100% + 4px); left: 0; z-index: 60; min-width: 140px; background: #0f172a;
  border: 1px solid rgba(255, 255, 255, 0.12); border-radius: 6px; box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
  padding: 4px; display: flex; flex-direction: column; gap: 2px;
}
.facet-opt {
  padding: 5px 8px; background: transparent; border: none; border-radius: 4px;
  color: #94a3b8; font-size: 11px; text-align: left; cursor: pointer;
}
.facet-opt:hover { background: rgba(255, 255, 255, 0.08); color: #fff; }
.facet-opt.is-active { color: #38bdf8; font-weight: 600; }
.filter-chips-row { display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap; }
.token-chips { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.filter-token-chip {
  display: inline-flex; align-items: center; gap: 5px; padding: 3px 8px; background: rgba(6, 182, 212, 0.12);
  border: 1px solid rgba(6, 182, 212, 0.3); border-radius: 9999px; font-size: 11px; color: #e2e8f0;
}
.token-key { color: #38bdf8; font-weight: 600; }
.token-remove-btn {
  display: inline-flex; align-items: center; justify-content: center; background: transparent;
  border: none; color: #94a3b8; cursor: pointer; padding: 0; margin-left: 2px; line-height: 1;
}
.token-remove-btn:hover { color: #fff; }
.clear-all-btn { background: transparent; border: none; color: #fb7185; font-size: 11px; font-weight: 600; cursor: pointer; padding: 2px 6px; }
.clear-all-btn:hover { text-decoration: underline; }
</style>
