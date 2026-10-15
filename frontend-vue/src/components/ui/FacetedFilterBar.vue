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

<style scoped src="../../assets/styles/components/ui/faceted-filter-bar.css"></style>
<style scoped>
.faceted-filter-bar { display: flex; align-items: center; }
.filter-chip { display: inline-flex; align-items: center; }
</style>
