<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import BaseIcon from './BaseIcon.vue'

export interface FilterToken {
  id: string
  category: string
  label: string
  value: string
}

export interface FacetOption {
  label: string
  value: string
  count?: number
}

export interface FacetCategory {
  category: string
  label: string
  options: FacetOption[]
}

interface Props {
  modelValue?: string
  filters?: FilterToken[]
  availableFacets?: FacetCategory[]
  placeholder?: string
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: '',
  filters: () => [],
  availableFacets: () => [],
  placeholder: 'Filter resources... (press / to focus)',
})

const emit = defineEmits<{
  (e: 'update:modelValue', query: string): void
  (e: 'update:filters', filters: FilterToken[]): void
  (e: 'clearAll'): void
}>()

const searchQuery = ref(props.modelValue)
const searchInputRef = ref<HTMLInputElement | null>(null)
const facetMenuContainerRef = ref<HTMLDivElement | null>(null)
const isFacetMenuOpen = ref(false)
const selectedCategory = ref<FacetCategory | null>(null)
let debounceTimer: ReturnType<typeof setTimeout> | null = null

watch(() => props.modelValue, (newVal) => {
  if (newVal !== searchQuery.value) searchQuery.value = newVal || ''
})

function handleSearchInput(e: Event) {
  const target = e.target as HTMLInputElement
  searchQuery.value = target.value
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    emit('update:modelValue', searchQuery.value)
  }, 150)
}

function handleClearSearch() {
  searchQuery.value = ''
  if (debounceTimer) clearTimeout(debounceTimer)
  emit('update:modelValue', '')
  searchInputRef.value?.focus()
}

function removeFilter(token: FilterToken) {
  const updated = props.filters.filter((f) => f.id !== token.id)
  emit('update:filters', updated)
}

function handleClearAll() {
  searchQuery.value = ''
  if (debounceTimer) clearTimeout(debounceTimer)
  emit('update:modelValue', '')
  emit('update:filters', [])
  emit('clearAll')
}

const hasActiveFilters = computed(() => {
  return props.filters.length > 0 || (searchQuery.value && searchQuery.value.trim().length > 0)
})

function toggleFacetMenu() {
  if (isFacetMenuOpen.value) {
    closeFacetMenu()
  } else {
    isFacetMenuOpen.value = true
    selectedCategory.value = null
  }
}

function closeFacetMenu() {
  isFacetMenuOpen.value = false
  selectedCategory.value = null
}

function selectCategory(category: FacetCategory) {
  selectedCategory.value = category
}

function selectOption(category: FacetCategory, option: FacetOption) {
  const id = `${category.category}:${option.value}`
  if (!props.filters.some((f) => f.id === id)) {
    const newFilter: FilterToken = {
      id,
      category: category.category,
      label: option.label,
      value: option.value,
    }
    emit('update:filters', [...props.filters, newFilter])
  }
  closeFacetMenu()
}

function handlePointerDown(e: PointerEvent) {
  const target = e.target as Node | null
  if (target && facetMenuContainerRef.value && !facetMenuContainerRef.value.contains(target)) {
    closeFacetMenu()
  }
}

function handleWindowKeydown(e: KeyboardEvent) {
  const target = e.target as HTMLElement | null
  const isInputOrTextarea = target && (
    target.tagName === 'INPUT' ||
    target.tagName === 'TEXTAREA' ||
    target.isContentEditable
  )

  if ((e.key === '/' || ((e.ctrlKey || e.metaKey) && (e.key === 'f' || e.key === 'F'))) && !isInputOrTextarea) {
    e.preventDefault()
    searchInputRef.value?.focus()
    searchInputRef.value?.select()
  } else if (e.key === 'Escape') {
    if (isFacetMenuOpen.value) closeFacetMenu()
    else if (document.activeElement === searchInputRef.value) searchInputRef.value?.blur()
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleWindowKeydown)
  window.addEventListener('pointerdown', handlePointerDown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleWindowKeydown)
  window.removeEventListener('pointerdown', handlePointerDown)
  if (debounceTimer) {
    clearTimeout(debounceTimer)
    debounceTimer = null
  }
})
</script>

<template>
  <div class="faceted-filter-bar" role="search" aria-label="Filter toolbar">
    <div class="filter-search-wrapper">
      <BaseIcon name="search" size="14" class="search-icon" />
      <input
        ref="searchInputRef"
        type="search"
        class="filter-search-input"
        :value="searchQuery"
        :placeholder="placeholder"
        aria-label="Filter query"
        @input="handleSearchInput"
      />
      <button
        v-if="searchQuery"
        type="button"
        class="search-clear-btn"
        aria-label="Clear search query"
        @click="handleClearSearch"
      >
        <BaseIcon name="x" size="12" />
      </button>
    </div>

    <div v-if="filters.length > 0" class="filter-chips-stream" role="group" aria-label="Active filters">
      <div v-for="token in filters" :key="token.id" class="filter-chip">
        <span class="chip-category">{{ token.category }}:</span>
        <span class="chip-label">{{ token.label }}</span>
        <button
          type="button"
          class="chip-remove-btn"
          :aria-label="`Remove filter ${token.category}: ${token.label}`"
          @click="removeFilter(token)"
        >
          <BaseIcon name="x" size="12" />
        </button>
      </div>
    </div>

    <div v-if="availableFacets.length > 0" ref="facetMenuContainerRef" class="facet-dropdown-container">
      <button
        type="button"
        class="filter-add-btn"
        :class="{ 'is-active': isFacetMenuOpen }"
        aria-haspopup="true"
        :aria-expanded="isFacetMenuOpen"
        @click="toggleFacetMenu"
      >
        <BaseIcon name="plus" size="12" />
        <span>+ Filter</span>
      </button>

      <div v-if="isFacetMenuOpen" class="facet-menu-popover" role="menu">
        <div v-if="!selectedCategory" class="facet-menu-view">
          <div class="facet-menu-header">Filter by category</div>
          <div class="facet-menu-list">
            <button
              v-for="cat in availableFacets"
              :key="cat.category"
              type="button"
              class="facet-menu-item"
              role="menuitem"
              @click="selectCategory(cat)"
            >
              <span class="facet-item-label">{{ cat.label }}</span>
              <BaseIcon name="chevron-right" size="12" class="facet-item-arrow" />
            </button>
          </div>
        </div>

        <div v-else class="facet-menu-view">
          <button type="button" class="facet-menu-back-btn" @click="selectedCategory = null">
            <span>&larr; {{ selectedCategory.label }}</span>
          </button>
          <div class="facet-menu-list">
            <button
              v-for="opt in selectedCategory.options"
              :key="opt.value"
              type="button"
              class="facet-menu-item"
              role="menuitem"
              @click="selectOption(selectedCategory, opt)"
            >
              <span class="facet-item-label">{{ opt.label }}</span>
              <span v-if="opt.count !== undefined" class="facet-item-count">({{ opt.count }})</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <button
      v-if="hasActiveFilters"
      type="button"
      class="filter-clear-all-btn"
      aria-label="Clear all filters"
      @click="handleClearAll"
    >
      Clear all
    </button>
  </div>
</template>

<style scoped>
.faceted-filter-bar {
  display: flex; flex-wrap: wrap; align-items: center; gap: 8px; padding: 6px 12px;
  background: rgba(20, 21, 23, 0.7); border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px; backdrop-filter: blur(8px); min-height: 40px; box-sizing: border-box;
}
.filter-search-wrapper {
  display: inline-flex; align-items: center; gap: 6px; position: relative; flex: 1 1 200px; min-width: 160px;
}
.search-icon { color: var(--text-muted, #94a3b8); flex-shrink: 0; }
.filter-search-input {
  background: transparent; border: none; outline: none; color: #f8fafc; font-size: 13px; font-family: inherit; width: 100%; padding: 2px 0;
}
.filter-search-input::placeholder { color: rgba(148, 163, 184, 0.65); }
.search-clear-btn {
  display: inline-flex; align-items: center; justify-content: center; background: transparent; border: none; color: #94a3b8; cursor: pointer; padding: 2px; border-radius: 4px;
}
.search-clear-btn:hover { color: #f8fafc; }
.filter-chips-stream { display: inline-flex; flex-wrap: wrap; align-items: center; gap: 6px; }
.filter-chip {
  display: inline-flex; align-items: center; gap: 4px; padding: 2px 8px; background: rgba(56, 189, 248, 0.1);
  border: 1px solid rgba(56, 189, 248, 0.25); border-radius: 9999px; font-size: 12px; line-height: 1.4; color: #f8fafc;
}
.chip-category { color: #38bdf8; font-weight: 600; font-size: 11px; text-transform: uppercase; letter-spacing: 0.02em; }
.chip-label { font-weight: 500; }
.chip-remove-btn {
  display: inline-flex; align-items: center; justify-content: center; background: transparent; border: none;
  color: #94a3b8; cursor: pointer; padding: 0; margin-left: 2px; border-radius: 50%; transition: color 0.15s ease;
}
.chip-remove-btn:hover { color: #fb7185; }
.facet-dropdown-container { position: relative; display: inline-flex; }
.filter-add-btn {
  display: inline-flex; align-items: center; gap: 4px; padding: 3px 9px; background: rgba(255, 255, 255, 0.04);
  border: 1px dashed rgba(255, 255, 255, 0.2); border-radius: 6px; color: #94a3b8; font-size: 12px; font-weight: 500;
  cursor: pointer; transition: all 0.15s ease;
}
.filter-add-btn:hover, .filter-add-btn.is-active { border-color: #38bdf8; color: #f8fafc; background: rgba(56, 189, 248, 0.08); }
.facet-menu-popover {
  position: absolute; top: calc(100% + 4px); left: 0; z-index: 100; min-width: 180px; background: #141516;
  border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 8px; box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
  padding: 4px; backdrop-filter: blur(12px);
}
.facet-menu-header {
  padding: 4px 8px; font-size: 11px; font-weight: 600; color: #94a3b8; text-transform: uppercase;
  letter-spacing: 0.04em; border-bottom: 1px solid rgba(255, 255, 255, 0.06); margin-bottom: 4px;
}
.facet-menu-back-btn {
  display: inline-flex; align-items: center; gap: 4px; width: 100%; padding: 4px 8px; background: transparent;
  border: none; border-bottom: 1px solid rgba(255, 255, 255, 0.06); color: #38bdf8; font-size: 12px; font-weight: 600;
  cursor: pointer; text-align: left; margin-bottom: 4px;
}
.facet-menu-list { display: flex; flex-direction: column; gap: 2px; max-height: 200px; overflow-y: auto; }
.facet-menu-item {
  display: flex; align-items: center; justify-content: space-between; width: 100%; padding: 6px 8px; background: transparent;
  border: none; border-radius: 4px; color: #f8fafc; font-size: 12px; cursor: pointer; text-align: left; transition: background 0.15s ease;
}
.facet-menu-item:hover { background: rgba(255, 255, 255, 0.08); }
.facet-item-count { font-size: 11px; color: #94a3b8; }
.facet-item-arrow { color: #64748b; }
.filter-clear-all-btn {
  background: transparent; border: none; color: #94a3b8; font-size: 12px; cursor: pointer; padding: 3px 6px;
  border-radius: 4px; text-decoration: underline; text-underline-offset: 2px; margin-left: auto; transition: color 0.15s ease;
}
.filter-clear-all-btn:hover { color: #fb7185; }
</style>
