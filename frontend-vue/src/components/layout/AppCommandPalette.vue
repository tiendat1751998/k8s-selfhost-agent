<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import ModalDrawer from '../ui/ModalDrawer.vue'
import BaseIcon from '../ui/BaseIcon.vue'
import { navGroups } from '../../config/navigation'

const props = defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
}>()

const router = useRouter()
const route = useRoute()
const searchQuery = ref('')

interface SearchGroup {
  key: string
  label: string
  items: Array<{
    path: string
    name: string
    icon: string
    sub: string
  }>
}

const groupedSearchResults = computed<SearchGroup[]>(() => {
  const q = searchQuery.value.toLowerCase().trim()
  const groups: SearchGroup[] = []

  for (const group of navGroups) {
    const matchedItems = group.items
      .filter(item => {
        if (!q) return true
        return (
          item.name.toLowerCase().includes(q) ||
          group.label.toLowerCase().includes(q) ||
          item.sub.toLowerCase().includes(q) ||
          item.path.toLowerCase().includes(q)
        )
      })
      .map(item => ({
        path: item.path,
        name: item.name,
        icon: item.icon,
        sub: item.sub,
      }))

    if (matchedItems.length > 0) {
      groups.push({
        key: group.key,
        label: group.label,
        items: matchedItems,
      })
    }
  }

  return groups
})

const totalResultsCount = computed(() =>
  groupedSearchResults.value.reduce((acc, g) => acc + g.items.length, 0)
)

function navigateTo(path: string) {
  emit('update:show', false)
  searchQuery.value = ''
  router.push(path)
}

function handleGlobalKeydown(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    emit('update:show', !props.show)
  }
}

onMounted(() => {
  if (typeof window !== 'undefined') {
    window.addEventListener('keydown', handleGlobalKeydown)
  }
})

onUnmounted(() => {
  if (typeof window !== 'undefined') {
    window.removeEventListener('keydown', handleGlobalKeydown)
  }
})
</script>

<template>
  <ModalDrawer
    :show="show"
    title="Enterprise Command Palette"
    subtitle="Quick search across all enterprise platform routes and resources."
    max-width="640px"
    @update:show="emit('update:show', $event)"
  >
    <div class="command-palette-content">
      <div class="palette-input-wrap">
        <span class="palette-search-icon"><BaseIcon name="search" size="sm" /></span>
        <input
          id="command-palette-search"
          name="command-palette-search"
          aria-label="Search routes and commands"
          v-model="searchQuery"
          type="text"
          placeholder="Type a route, view, or category..."
          class="input-glass palette-search-input"
          autocomplete="off"
          autocorrect="off"
          autocapitalize="off"
          spellcheck="false"
          autofocus
        />
        <span v-if="searchQuery" class="palette-clear" role="button" tabindex="0" aria-label="Clear search" @click="searchQuery = ''" @keydown.enter="searchQuery = ''">✕</span>
      </div>

      <div class="palette-results">
        <div
          v-for="group in groupedSearchResults"
          :key="group.key"
          class="palette-group-section"
        >
          <div class="palette-group-header font-mono">{{ group.label }}</div>
          <div class="palette-group-items">
            <div
              v-for="item in group.items"
              :key="item.path"
              class="palette-item"
              role="button"
              tabindex="0"
              :class="{ 'palette-item-active': route.path === item.path }"
              @click="navigateTo(item.path)"
              @keydown.enter="navigateTo(item.path)"
            >
              <div class="p-item-icon"><BaseIcon :name="item.icon" size="sm" /></div>
              <div class="p-item-info">
                <span class="p-item-name">{{ item.name }}</span>
                <small class="p-item-sub">{{ item.sub }}</small>
              </div>
              <span class="p-item-path font-mono">{{ item.path }}</span>
            </div>
          </div>
        </div>

        <div v-if="totalResultsCount === 0" class="palette-empty">
          <span>No matching commands or routes found for "{{ searchQuery }}"</span>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="palette-footer-tips font-mono">
        <span class="desktop-tips">Navigate with click • <kbd>Esc</kbd> to close</span>
        <span class="mobile-tips">Tap backdrop or ✕ to close</span>
      </div>
    </template>
  </ModalDrawer>
</template>
