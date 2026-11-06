<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import { isHostOrSystemUnit } from '../../utils/logTargetCategorizer'

const props = withDefaults(
  defineProps<{
    modelValue?: string
    services?: string[]
    isNodeOffline?: boolean
    showJournalctl?: boolean
    initialOpen?: boolean
  }>(),
  { modelValue: '', services: () => [], isNodeOffline: false, showJournalctl: false, initialOpen: false }
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

// Purge individual systemd / OS units, keeping pure application workloads
const pureAppServices = computed(() => props.services.filter((s) => !isHostOrSystemUnit(s)))
const sortedAppServices = computed(() => [...pureAppServices.value].sort((a, b) => a.localeCompare(b)))
const filteredAppServices = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return q ? sortedAppServices.value.filter((s) => s.toLowerCase().includes(q)) : sortedAppServices.value
})

const matchesAllApps = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return !q || 'all apps'.includes(q) || 'all'.includes(q) || 'apps'.includes(q)
})

const matchesJournalctl = computed(() => {
  if (!props.showJournalctl) return false
  const q = searchQuery.value.trim().toLowerCase()
  return !q || 'journalctl'.includes(q) || 'system journal'.includes(q) || q.includes('system') || q.includes('journal')
})

interface ComboboxOption {
  value: string
  label: string
  isJournalctl?: boolean
  isAllApps?: boolean
}

const visibleOptions = computed<ComboboxOption[]>(() => {
  const list: ComboboxOption[] = []
  if (matchesAllApps.value) list.push({ value: '', label: 'All Apps', isAllApps: true })
  if (matchesJournalctl.value) list.push({ value: 'journalctl', label: 'journalctl', isJournalctl: true })
  for (const app of filteredAppServices.value) list.push({ value: app, label: app })
  return list
})

const allAppsIndex = computed(() => visibleOptions.value.findIndex(o => o.isAllApps))
const journalctlIndex = computed(() => visibleOptions.value.findIndex(o => o.isJournalctl))
const getAppIndex = (idx: number) => (matchesAllApps.value ? 1 : 0) + (matchesJournalctl.value ? 1 : 0) + idx

watch(isOpen, async (open) => {
  if (open) {
    searchQuery.value = ''
    highlightedIndex.value = 0
    await nextTick()
    searchInputRef.value?.focus()
  }
})

watch(searchQuery, () => { highlightedIndex.value = 0 })

function toggleDropdown() { isOpen.value = !isOpen.value }
function closeDropdown() { isOpen.value = false }
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
  if (e.key === 'Escape') { closeDropdown(); return }
  const total = visibleOptions.value.length
  if (total === 0) return
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    highlightedIndex.value = highlightedIndex.value >= total - 1 ? 0 : highlightedIndex.value + 1
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    highlightedIndex.value = highlightedIndex.value <= 0 ? total - 1 : highlightedIndex.value - 1
  } else if (e.key === 'Enter') {
    e.preventDefault()
    if (highlightedIndex.value >= 0 && highlightedIndex.value < total) {
      selectService(visibleOptions.value[highlightedIndex.value].value)
    }
  }
}

function handleDocumentClick(e: MouseEvent) {
  if (!rootRef.value || !isOpen.value) return
  if (!rootRef.value.contains(e.target as Node)) closeDropdown()
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
  clearService, searchQuery, filteredServices: filteredAppServices,
  sortedServices: sortedAppServices, pureAppServices, visibleOptions,
  isOpen, toggleDropdown, selectService,
})
</script>

<template>
  <div ref="rootRef" class="log-service-combobox font-mono">
    <!-- Trigger Button -->
    <button
      type="button"
      class="log-service-combobox-trigger"
      :class="{ active: Boolean(modelValue), 'is-journalctl': modelValue === 'journalctl', 'is-open': isOpen }"
      aria-haspopup="listbox"
      :aria-expanded="isOpen"
      title="Select log app or service"
      @click="toggleDropdown"
    >
      <BaseIcon :name="modelValue === 'journalctl' ? 'terminal' : 'box'" size="xs" class="combobox-trigger-icon" />
      <span class="combobox-trigger-label">{{ modelValue || 'All Apps' }}</span>
      <button
        v-if="modelValue"
        type="button"
        class="btn-clear-service"
        aria-label="Clear active service filter"
        title="Clear active service filter"
        @click.stop="clearService"
      >&times;</button>
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
        >&times;</button>
      </div>

      <div class="combobox-options-list">
        <!-- Row 0: All Apps (fixed top option when query empty or matches) -->
        <button
          v-if="matchesAllApps"
          type="button"
          class="combobox-option-item"
          :class="{ active: !modelValue, highlighted: highlightedIndex === allAppsIndex }"
          role="option"
          :aria-selected="!modelValue"
          @click="selectService('')"
          @mouseenter="highlightedIndex = allAppsIndex"
        >
          <BaseIcon name="layers" size="xs" class="opt-icon" />
          <span class="opt-name">All Apps</span>
          <BaseIcon v-if="!modelValue" name="check" size="xs" class="opt-check" />
        </button>

        <!-- Dedicated System Logs row for platform_admin ONLY -->
        <template v-if="matchesJournalctl">
          <div v-if="matchesAllApps" class="combobox-divider" />
          <button
            type="button"
            class="combobox-option-item item-journalctl"
            :class="{ active: modelValue === 'journalctl', highlighted: highlightedIndex === journalctlIndex }"
            role="option"
            :aria-selected="modelValue === 'journalctl'"
            @click="selectService('journalctl')"
            @mouseenter="highlightedIndex = journalctlIndex"
          >
            <BaseIcon name="terminal" size="xs" class="opt-icon text-terminal" />
            <span class="opt-name">journalctl</span>
            <span class="opt-badge font-mono">System Journal</span>
            <BaseIcon v-if="modelValue === 'journalctl'" name="check" size="xs" class="opt-check" />
          </button>
        </template>

        <!-- Subtle divider between System Logs and pure Applications -->
        <div v-if="(matchesAllApps || matchesJournalctl) && filteredAppServices.length > 0" class="combobox-divider" />

        <!-- Pure Application / Container / Pod options -->
        <button
          v-for="(svc, index) in filteredAppServices"
          :key="svc"
          type="button"
          class="combobox-option-item"
          :class="{
            active: modelValue.toLowerCase() === svc.toLowerCase(),
            highlighted: highlightedIndex === getAppIndex(index),
          }"
          role="option"
          :aria-selected="modelValue.toLowerCase() === svc.toLowerCase()"
          @click="selectService(svc)"
          @mouseenter="highlightedIndex = getAppIndex(index)"
        >
          <BaseIcon name="box" size="xs" class="opt-icon" />
          <span class="opt-name">{{ svc }}</span>
          <BaseIcon v-if="modelValue.toLowerCase() === svc.toLowerCase()" name="check" size="xs" class="opt-check" />
        </button>

        <!-- Zero app containers on this node note when search is empty -->
        <div
          v-if="pureAppServices.length === 0 && !searchQuery.trim()"
          :class="isNodeOffline ? 'combobox-empty-msg' : 'combobox-no-apps-note'"
        >
          {{ isNodeOffline ? 'Node offline' : 'No app containers on this node' }}
        </div>

        <!-- Empty search result message -->
        <div v-else-if="visibleOptions.length === 0" class="combobox-empty-msg">
          {{ isNodeOffline && pureAppServices.length === 0 ? 'Node offline' : 'No matching app' }}
        </div>
      </div>
    </div>
  </div>
</template>
