<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import ModalDrawer from '../ui/ModalDrawer.vue'
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

const allSearchableRoutes = computed(() => {
  const list: { path: string; name: string; icon: string; group: string; sub: string }[] = []
  for (const group of navGroups) {
    for (const item of group.items) {
      list.push({
        path: item.path,
        name: item.name,
        icon: item.icon,
        group: group.label,
        sub: item.sub,
      })
    }
  }
  return list
})

const filteredSearchResults = computed(() => {
  if (!searchQuery.value.trim()) return allSearchableRoutes.value
  const q = searchQuery.value.toLowerCase().trim()
  return allSearchableRoutes.value.filter(item =>
    item.name.toLowerCase().includes(q) ||
    item.group.toLowerCase().includes(q) ||
    item.sub.toLowerCase().includes(q) ||
    item.path.toLowerCase().includes(q)
  )
})

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
    subtitle="Quick search across all 20+ platform routes, AI models, and resources."
    max-width="640px"
    @update:show="emit('update:show', $event)"
  >
    <div class="command-palette-content">
      <div class="palette-input-wrap">
        <span class="palette-search-icon">🔍</span>
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Type a route, view, or category..."
          class="input-glass palette-search-input"
          autofocus
        />
        <span v-if="searchQuery" class="palette-clear" role="button" tabindex="0" aria-label="Clear search" @click="searchQuery = ''" @keydown.enter="searchQuery = ''">✕</span>
      </div>

      <div class="palette-results">
        <div
          v-for="item in filteredSearchResults"
          :key="item.path"
          class="palette-item"
          role="button"
          tabindex="0"
          :class="{ 'palette-item-active': route.path === item.path }"
          @click="navigateTo(item.path)"
          @keydown.enter="navigateTo(item.path)"
        >
          <div class="p-item-icon">{{ item.icon }}</div>
          <div class="p-item-info">
            <div class="p-item-title-row">
              <span class="p-item-name">{{ item.name }}</span>
              <span class="p-item-group font-mono">{{ item.group }}</span>
            </div>
            <small class="p-item-sub">{{ item.sub }}</small>
          </div>
          <span class="p-item-path font-mono">{{ item.path }}</span>
        </div>

        <div v-if="filteredSearchResults.length === 0" class="palette-empty">
          <span>No matching commands or routes found for "{{ searchQuery }}"</span>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="palette-footer-tips font-mono">
        <span>Navigate with click • <kbd>Esc</kbd> to close</span>
      </div>
    </template>
  </ModalDrawer>
</template>
