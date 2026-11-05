<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'

export interface NodeItem {
  id: string
  name: string
  online?: boolean
}

const props = withDefaults(
  defineProps<{
    modelValue?: string
    nodes?: NodeItem[]
  }>(),
  {
    modelValue: '',
    nodes: () => [],
  }
)

const emit = defineEmits<{
  (e: 'update:modelValue', val: string): void
  (e: 'select', node: NodeItem | null): void
}>()

const isOpen = ref(false)
const searchQuery = ref('')
const rootRef = ref<HTMLElement | null>(null)

const selectedNode = computed(() => {
  if (!props.modelValue || props.modelValue === 'all') return null
  return props.nodes.find((n) => n.id.toLowerCase() === props.modelValue.toLowerCase()) || null
})

const selectedNodeName = computed(() => selectedNode.value ? selectedNode.value.name : props.modelValue)
const isSelectedNodeOnline = computed(() => selectedNode.value?.online !== false)

const filteredNodes = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return props.nodes
  return props.nodes.filter((n) => n.name.toLowerCase().includes(q) || n.id.toLowerCase().includes(q))
})

const onlineNodes = computed(() => filteredNodes.value.filter((n) => n.online !== false))
const offlineNodes = computed(() => filteredNodes.value.filter((n) => n.online === false))

function toggleDropdown() {
  isOpen.value = !isOpen.value
  if (isOpen.value) searchQuery.value = ''
}

function closeDropdown() {
  isOpen.value = false
}

function selectNode(node: NodeItem | null) {
  const val = node ? node.id : ''
  emit('update:modelValue', val)
  emit('select', node)
  closeDropdown()
}

function clearNode() {
  emit('update:modelValue', '')
  emit('select', null)
}

function handleDocumentClick(e: MouseEvent) {
  if (!rootRef.value || !isOpen.value) return
  if (!rootRef.value.contains(e.target as Node)) {
    closeDropdown()
  }
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && isOpen.value) {
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
  clearNode,
  toggleDropdown,
  isOpen,
  selectedNode,
})
</script>

<template>
  <div ref="rootRef" class="log-node-dropdown font-mono">
    <button
      type="button"
      class="log-node-dropdown-trigger"
      :class="{ 'has-node': Boolean(modelValue && modelValue !== 'all'), 'is-open': isOpen }"
      aria-haspopup="listbox"
      :aria-expanded="isOpen"
      title="Select log node"
      @click="toggleDropdown"
    >
      <template v-if="!modelValue || modelValue === 'all'">
        <BaseIcon name="globe" size="xs" class="trigger-icon" />
        <span class="trigger-label">All Nodes</span>
      </template>
      <template v-else>
        <span :class="['pulse-dot', isSelectedNodeOnline ? 'pulse-dot-emerald' : 'pulse-dot-slate']" />
        <span class="trigger-label">{{ selectedNodeName }}</span>
        <button
          type="button"
          class="btn-clear-node"
          aria-label="Reset to all nodes"
          title="Reset to all nodes"
          @click.stop="clearNode"
        >
          &times;
        </button>
      </template>
      <BaseIcon name="chevron-down" size="xs" class="chevron-icon" />
    </button>

    <div v-if="isOpen" class="node-dropdown-popover" role="listbox">
      <div v-if="nodes.length > 5" class="node-search-wrap">
        <input
          v-model="searchQuery"
          type="text"
          class="node-search-input font-mono"
          placeholder="Search nodes..."
          autofocus
          @click.stop
        />
      </div>

      <div class="node-options-list">
        <button
          type="button"
          class="node-option-item"
          :class="{ active: !modelValue || modelValue === 'all' }"
          role="option"
          :aria-selected="!modelValue || modelValue === 'all'"
          @click="selectNode(null)"
        >
          <BaseIcon name="globe" size="xs" class="opt-icon" />
          <span class="opt-name">All Nodes</span>
          <BaseIcon v-if="!modelValue || modelValue === 'all'" name="check" size="xs" class="opt-check" />
        </button>

        <button
          v-for="node in onlineNodes"
          :key="node.id"
          type="button"
          class="node-option-item"
          :class="{ active: modelValue.toLowerCase() === node.id.toLowerCase() }"
          role="option"
          :aria-selected="modelValue.toLowerCase() === node.id.toLowerCase()"
          @click="selectNode(node)"
        >
          <span class="pulse-dot pulse-dot-emerald" />
          <span class="opt-name">{{ node.name }}</span>
          <BaseIcon v-if="modelValue.toLowerCase() === node.id.toLowerCase()" name="check" size="xs" class="opt-check" />
        </button>

        <div v-if="offlineNodes.length > 0" class="node-divider" />
        <button
          v-for="node in offlineNodes"
          :key="node.id"
          type="button"
          class="node-option-item is-offline"
          :class="{ active: modelValue.toLowerCase() === node.id.toLowerCase() }"
          role="option"
          :aria-selected="modelValue.toLowerCase() === node.id.toLowerCase()"
          @click="selectNode(node)"
        >
          <span class="pulse-dot pulse-dot-slate" />
          <span class="opt-name">{{ node.name }}</span>
          <span class="offline-tag">offline</span>
          <BaseIcon v-if="modelValue.toLowerCase() === node.id.toLowerCase()" name="check" size="xs" class="opt-check" />
        </button>

        <div v-if="onlineNodes.length === 0 && offlineNodes.length === 0" class="node-empty-msg">
          No matching nodes
        </div>
      </div>
    </div>
  </div>
</template>
