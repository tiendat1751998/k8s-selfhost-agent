<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import { isSystemdService } from '../../utils/logTargetCategorizer'

const props = withDefaults(
  defineProps<{
    modelValue?: string
    services?: string[]
    isNodeOffline?: boolean
    initialOpen?: boolean
  }>(),
  {
    modelValue: '',
    services: () => [],
    isNodeOffline: false,
    initialOpen: false,
  }
)

const emit = defineEmits<{
  (e: 'update:modelValue', val: string): void
  (e: 'select', val: string): void
}>()

const isOpen = ref(props.initialOpen || false)
const searchQuery = ref('')
const highlightedIndex = ref(0)
const rootRef = ref<HTMLElement | null>(null)
const searchInputRef = ref<HTMLInputElement | null>(null)

// Sort: App services first, .service daemons second, alphabetically
const sortedServices = computed(() => {
  return [...props.services].sort((a, b) => {
    const aSys = isSystemdService(a)
    const bSys = isSystemdService(b)
    if (aSys !== bSys) return aSys ? 1 : -1
    return a.localeCompare(b)
  })
})

const filteredServices = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return sortedServices.value
  return sortedServices.value.filter((s) => s.toLowerCase().includes(q))
})

watch(isOpen, async (open) => {
  if (open) {
    searchQuery.value = ''
    highlightedIndex.value = 0
    await nextTick()
    searchInputRef.value?.focus()
  }
})

watch(searchQuery, () => {
  highlightedIndex.value = 0
})

function toggleDropdown() {
  isOpen.value = !isOpen.value
}

function closeDropdown() {
  isOpen.value = false
}

function selectService(svc: string) {
  emit('update:modelValue', svc)
  emit('select', svc)
  closeDropdown()
}

function clearService() {
  emit('update:modelValue', '')
  emit('select', '')
}

function handleKeydown(e: KeyboardEvent) {
  if (!isOpen.value) return
  if (e.key === 'Escape') {
    closeDropdown()
    return
  }
  const maxIdx = searchQuery.value.trim() ? filteredServices.value.length - 1 : filteredServices.value.length // includes row 0
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    highlightedIndex.value = highlightedIndex.value >= maxIdx ? 0 : highlightedIndex.value + 1
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    highlightedIndex.value = highlightedIndex.value <= 0 ? maxIdx : highlightedIndex.value - 1
  } else if (e.key === 'Enter') {
    e.preventDefault()
    if (!searchQuery.value.trim() && highlightedIndex.value === 0) {
      selectService('')
    } else {
      const idx = searchQuery.value.trim() ? highlightedIndex.value : highlightedIndex.value - 1
      if (idx >= 0 && idx < filteredServices.value.length) {
        selectService(filteredServices.value[idx])
      }
    }
  }
}

function handleDocumentClick(e: MouseEvent) {
  if (!rootRef.value || !isOpen.value) return
  if (!rootRef.value.contains(e.target as Node)) {
    closeDropdown()
  }
}

onMounted(() => {
  if (typeof window !== 'undefined') {
    window.addEventListener('click', handleDocumentClick)
    window.addEventListener('keydown', handleKeydown)
  }
})

onUnmounted(() => {
  if (typeof window !== 'undefined') {
    window.removeEventListener('click', handleDocumentClick)
    window.removeEventListener('keydown', handleKeydown)
  }
})

defineExpose({
  clearService,
  searchQuery,
  filteredServices,
  sortedServices,
  isOpen,
  toggleDropdown,
  selectService,
})
</script>

<template>
  <div ref="rootRef" class="log-service-combobox font-mono">
    <!-- Trigger Button -->
    <button
      type="button"
      class="log-service-combobox-trigger"
      :class="{ active: Boolean(modelValue), 'is-open': isOpen }"
      aria-haspopup="listbox"
      :aria-expanded="isOpen"
      title="Select log app or service"
      @click="toggleDropdown"
    >
      <BaseIcon name="box" size="xs" class="combobox-trigger-icon" />
      <span class="combobox-trigger-label">{{ modelValue || 'All Apps' }}</span>
      <button
        v-if="modelValue"
        type="button"
        class="btn-clear-service"
        aria-label="Clear active service filter"
        title="Clear active service filter"
        @click.stop="clearService"
      >
        &times;
      </button>
      <BaseIcon name="chevron-down" size="xs" class="combobox-chevron-icon" />
    </button>

    <!-- Popover -->
    <div v-if="isOpen" class="service-combobox-popover" role="listbox">
      <div class="combobox-search-wrap">
        <BaseIcon name="search" size="xs" class="combobox-search-icon" />
        <input
          ref="searchInputRef"
          v-model="searchQuery"
          type="text"
          class="combobox-search-input font-mono"
          placeholder="Search app or service..."
          @click.stop
        />
        <button
          v-if="searchQuery"
          type="button"
          class="combobox-search-clear"
          aria-label="Clear search query"
          @click.stop="searchQuery = ''"
        >
          &times;
        </button>
      </div>

      <div class="combobox-options-list">
        <!-- Row 0: All Apps & Services (when query is empty) -->
        <button
          v-if="!searchQuery.trim()"
          type="button"
          class="combobox-option-item"
          :class="{ active: !modelValue, highlighted: highlightedIndex === 0 }"
          role="option"
          :aria-selected="!modelValue"
          @click="selectService('')"
          @mouseenter="highlightedIndex = 0"
        >
          <BaseIcon name="layers" size="xs" class="opt-icon" />
          <span class="opt-name">All Apps & Services</span>
          <BaseIcon v-if="!modelValue" name="check" size="xs" class="opt-check" />
        </button>

        <!-- Service Options -->
        <button
          v-for="(svc, index) in filteredServices"
          :key="svc"
          type="button"
          class="combobox-option-item"
          :class="{
            active: modelValue.toLowerCase() === svc.toLowerCase(),
            highlighted: highlightedIndex === (!searchQuery.trim() ? index + 1 : index),
          }"
          role="option"
          :aria-selected="modelValue.toLowerCase() === svc.toLowerCase()"
          @click="selectService(svc)"
          @mouseenter="highlightedIndex = !searchQuery.trim() ? index + 1 : index"
        >
          <BaseIcon :name="isSystemdService(svc) ? 'cpu' : 'box'" size="xs" class="opt-icon" />
          <span class="opt-name">{{ svc }}</span>
          <BaseIcon v-if="modelValue.toLowerCase() === svc.toLowerCase()" name="check" size="xs" class="opt-check" />
        </button>

        <!-- Empty State -->
        <div v-if="filteredServices.length === 0" class="combobox-empty-msg">
          {{ isNodeOffline && services.length === 0 ? 'Node offline' : 'No matching app' }}
        </div>
      </div>
    </div>
  </div>
</template>
