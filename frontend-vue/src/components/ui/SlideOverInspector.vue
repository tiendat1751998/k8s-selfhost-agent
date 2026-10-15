<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import BaseIcon from './BaseIcon.vue'
import StatusBadge from './StatusBadge.vue'

export interface InspectorTab {
  id: string
  label: string
  icon?: string
  badge?: number | string
}

interface Props {
  isOpen: boolean
  title: string
  subtitle?: string
  status?: string
  icon?: string
  tabs?: InspectorTab[]
  activeTab?: string
  width?: string
  closeOnBackdrop?: boolean
  closeOnEsc?: boolean
  ariaLabel?: string
}

const props = withDefaults(defineProps<Props>(), {
  subtitle: undefined,
  status: undefined,
  icon: undefined,
  tabs: undefined,
  activeTab: undefined,
  width: '580px',
  closeOnBackdrop: true,
  closeOnEsc: true,
  ariaLabel: undefined
})

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update:isOpen', value: boolean): void
  (e: 'update:activeTab', tabId: string): void
  (e: 'action', actionName: string): void
}>()

const DEFAULT_TABS: InspectorTab[] = [
  { id: 'overview', label: 'Overview', icon: 'eye' },
  { id: 'yaml', label: 'YAML', icon: 'file-code' },
  { id: 'logs', label: 'Logs', icon: 'file-text' },
  { id: 'actions', label: 'Actions', icon: 'sliders' },
]

const availableTabs = computed<InspectorTab[]>(() => {
  return props.tabs && props.tabs.length > 0 ? props.tabs : DEFAULT_TABS
})

const localTab = ref<string>(props.activeTab || availableTabs.value[0]?.id || 'overview')

watch(
  () => props.activeTab,
  (newTab) => {
    if (newTab) {
      localTab.value = newTab
    }
  }
)

const currentTab = computed<string>(() => {
  return props.activeTab || localTab.value
})

function selectTab(tabId: string) {
  localTab.value = tabId
  emit('update:activeTab', tabId)
  if (tabId === 'actions') {
    emit('action', 'tab-actions-opened')
  }
}

function triggerAction(actionName: string) {
  emit('action', actionName)
}

function handleClose() {
  emit('close')
  emit('update:isOpen', false)
}

function handleBackdropClick() {
  if (props.closeOnBackdrop) {
    handleClose()
  }
}

const drawerRef = ref<HTMLElement | null>(null)
let previousActiveElement: HTMLElement | null = null

function handleKeydown(e: KeyboardEvent) {
  if (!props.isOpen) return

  if (props.closeOnEsc && e.key === 'Escape') {
    e.stopPropagation()
    handleClose()
    return
  }

  // Focus trap for accessibility
  if (e.key === 'Tab' && drawerRef.value) {
    const focusable = drawerRef.value.querySelectorAll<HTMLElement>(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
    )
    if (!focusable.length) return
    const first = focusable[0]
    const last = focusable[focusable.length - 1]

    if (e.shiftKey && document.activeElement === first) {
      last.focus()
      e.preventDefault()
    } else if (!e.shiftKey && document.activeElement === last) {
      first.focus()
      e.preventDefault()
    }
  }
}

watch(
  () => props.isOpen,
  (open) => {
    if (typeof document !== 'undefined') {
      if (open) {
        previousActiveElement = document.activeElement as HTMLElement | null
        document.body.style.overflow = 'hidden'
        nextTick(() => {
          drawerRef.value?.focus()
        })
      } else {
        document.body.style.overflow = ''
        previousActiveElement?.focus?.()
      }
    }
  }
)

onMounted(() => {
  if (typeof window !== 'undefined') {
    window.addEventListener('keydown', handleKeydown)
  }
})

onUnmounted(() => {
  if (typeof window !== 'undefined') {
    window.removeEventListener('keydown', handleKeydown)
    if (typeof document !== 'undefined') {
      document.body.style.overflow = ''
    }
  }
})
</script>

<template>
  <Teleport to="body">
    <Transition name="inspector-fade">
      <div
        v-if="isOpen"
        class="slide-over-backdrop"
        @click.self="handleBackdropClick"
        aria-hidden="false"
      >
        <Transition name="inspector-slide" appear>
          <div
            v-if="isOpen"
            ref="drawerRef"
            class="inspector-panel"
            :style="{ '--drawer-width': width || '580px' }"
            role="dialog"
            aria-modal="true"
            :aria-label="ariaLabel || title"
            tabindex="-1"
          >
            <!-- 48px Fixed Header Bar -->
            <header class="inspector-header">
              <div class="header-left">
                <BaseIcon
                  v-if="icon"
                  :name="icon"
                  size="md"
                  class="entity-icon"
                />
                <div class="header-meta">
                  <h3 class="header-title" :title="title">{{ title }}</h3>
                  <span v-if="subtitle" class="header-subtitle" :title="subtitle">
                    {{ subtitle }}
                  </span>
                </div>
                <StatusBadge v-if="status" :status="status" size="sm" class="header-status" />
              </div>

              <div class="header-right">
                <slot name="header-actions" :close="handleClose"></slot>
                <button
                  type="button"
                  class="close-button"
                  title="Close inspector (Esc)"
                  aria-label="Close inspector"
                  @click="handleClose"
                >
                  <BaseIcon name="x-circle" size="md" class="close-icon" />
                </button>
              </div>
            </header>

            <!-- Tab Navigation Strip -->
            <nav class="inspector-tabs" role="tablist" aria-label="Inspector Sections">
              <button
                v-for="tab in availableTabs"
                :key="tab.id"
                type="button"
                role="tab"
                :id="`tab-${tab.id}`"
                :aria-controls="`tabpanel-${tab.id}`"
                :aria-selected="currentTab === tab.id"
                class="tab-btn"
                :class="{ 'is-active': currentTab === tab.id }"
                @click="selectTab(tab.id)"
              >
                <BaseIcon v-if="tab.icon" :name="tab.icon" size="xs" class="tab-icon" />
                <span class="tab-label">{{ tab.label }}</span>
                <span v-if="tab.badge !== undefined" class="tab-badge">{{ tab.badge }}</span>
              </button>
            </nav>

            <!-- Content Area (Active Tab Slot or Fallback) -->
            <main
              class="inspector-body"
              :id="`tabpanel-${currentTab}`"
              role="tabpanel"
              :aria-labelledby="`tab-${currentTab}`"
            >
              <slot :name="currentTab" :action="triggerAction">
                <slot></slot>
              </slot>
            </main>

            <!-- Optional Footer -->
            <footer v-if="$slots.footer" class="inspector-footer">
              <slot name="footer" :close="handleClose" :action="triggerAction"></slot>
            </footer>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped src="../../assets/styles/components/ui/slide-over.css"></style>
<style scoped>
.inspector-header { height: 48px; min-height: 48px; }
.slide-over-backdrop { backdrop-filter: blur(4px); }
.inspector-slide-enter-from, .inspector-slide-leave-to { transform: translateX(100%); }
.inspector-slide-enter-to, .inspector-slide-leave-from { transform: translateX(0); }
@media (max-width: 640px) {
  .inspector-panel { width: 100vw; }
}
</style>
