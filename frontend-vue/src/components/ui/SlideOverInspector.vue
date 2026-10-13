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

<style scoped>
.slide-over-backdrop {
  position: fixed;
  inset: 0;
  width: 100vw;
  height: 100vh;
  min-height: 100dvh;
  background: rgba(0, 0, 0, 0.6);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  z-index: 1050;
  display: flex;
  justify-content: flex-end;
}
.inspector-panel {
  width: var(--drawer-width, 580px);
  max-width: 100vw;
  height: 100vh;
  min-height: 100dvh;
  background: rgba(11, 16, 28, 0.98);
  border-left: 1px solid rgba(255, 255, 255, 0.08);
  box-shadow: -12px 0 36px -4px rgba(0, 0, 0, 0.7);
  display: flex;
  flex-direction: column;
  outline: none;
  overflow: hidden;
  box-sizing: border-box;
}
.inspector-header {
  height: 48px;
  min-height: 48px;
  padding: 0 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  background: rgba(15, 23, 42, 0.75);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  flex-shrink: 0;
}
.header-left { display: flex; align-items: center; gap: 10px; min-width: 0; flex: 1; }
.entity-icon { color: #38bdf8; flex-shrink: 0; }
.header-meta { display: flex; flex-direction: column; justify-content: center; min-width: 0; gap: 1px; }
.header-title {
  margin: 0; font-size: 14px; font-weight: 700; color: #f8fafc; line-height: 1.2;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.header-subtitle {
  font-size: 11px; color: #94a3b8; line-height: 1.2;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.header-status { flex-shrink: 0; }
.header-right { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
.close-button {
  background: rgba(255, 255, 255, 0.04); border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 6px; width: 28px; height: 28px; display: flex; align-items: center;
  justify-content: center; color: #94a3b8; cursor: pointer; padding: 0; transition: all 0.15s ease;
}
.close-button:hover {
  background: rgba(244, 63, 94, 0.15); border-color: rgba(244, 63, 94, 0.4); color: #fb7185;
}
.close-icon { flex-shrink: 0; }
.inspector-tabs {
  display: flex; align-items: center; gap: 2px; padding: 0 12px; height: 38px;
  background: rgba(13, 19, 33, 0.95); border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  overflow-x: auto; flex-shrink: 0; scrollbar-width: none;
}
.inspector-tabs::-webkit-scrollbar { display: none; }
.tab-btn {
  display: inline-flex; align-items: center; gap: 6px; padding: 6px 12px; height: 28px;
  border: 1px solid transparent; border-radius: 6px; background: transparent; color: #94a3b8;
  font-size: 12px; font-weight: 500; cursor: pointer; white-space: nowrap; transition: all 0.15s ease;
}
.tab-btn:hover { color: #e2e8f0; background: rgba(255, 255, 255, 0.04); }
.tab-btn.is-active {
  color: #38bdf8; background: rgba(56, 189, 248, 0.1); border-color: rgba(56, 189, 248, 0.25); font-weight: 600;
}
.tab-icon { flex-shrink: 0; }
.tab-badge {
  display: inline-flex; align-items: center; justify-content: center; padding: 0 5px; height: 16px;
  font-size: 10px; font-weight: 600; border-radius: 9999px; background: rgba(255, 255, 255, 0.1); color: #cbd5e1;
}
.tab-btn.is-active .tab-badge { background: rgba(56, 189, 248, 0.2); color: #38bdf8; }
.inspector-body { flex: 1; padding: 16px; overflow-y: auto; min-height: 0; }
.inspector-footer {
  padding: 12px 16px; border-top: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(15, 23, 42, 0.5); display: flex; align-items: center;
  justify-content: flex-end; gap: 10px; flex-shrink: 0;
}
.inspector-fade-enter-active, .inspector-fade-leave-active { transition: opacity 0.22s ease; }
.inspector-fade-enter-from, .inspector-fade-leave-to { opacity: 0; }
.inspector-slide-enter-active { transition: transform 0.28s cubic-bezier(0.16, 1, 0.3, 1); }
.inspector-slide-leave-active { transition: transform 0.2s cubic-bezier(0.4, 0, 1, 1); }
.inspector-slide-enter-from, .inspector-slide-leave-to { transform: translateX(100%); }
.inspector-slide-enter-to, .inspector-slide-leave-from { transform: translateX(0); }
@media (max-width: 640px) {
  .inspector-panel { width: 100vw !important; max-width: 100vw !important; border-left: none; }
  .inspector-header { height: 48px; padding: 0 12px; }
  .header-subtitle { display: none; }
  .inspector-body { padding: 12px; }
  .inspector-footer { padding: 10px 12px; padding-bottom: max(12px, env(safe-area-inset-bottom, 12px)); }
}
</style>
