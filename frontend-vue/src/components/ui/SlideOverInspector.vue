<script setup lang="ts">
import { watch, onMounted, onUnmounted, computed } from 'vue'
import BaseIcon from './BaseIcon.vue'
import StatusBadge from './StatusBadge.vue'

export interface InspectorTab {
  id: string
  label: string
  icon?: string
  count?: number
}

const props = withDefaults(
  defineProps<{
    show: boolean
    title: string
    subtitle?: string
    icon?: string
    status?: string
    tabs?: InspectorTab[]
    activeTab?: string
    closable?: boolean
    closeOnEsc?: boolean
    closeOnBackdrop?: boolean
  }>(),
  {
    subtitle: undefined,
    icon: undefined,
    status: undefined,
    tabs: () => [],
    activeTab: undefined,
    closable: true,
    closeOnEsc: true,
    closeOnBackdrop: true,
  }
)

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'update:activeTab', tabId: string): void
  (e: 'tab-change', tabId: string): void
  (e: 'close'): void
}>()

const currentTab = computed(() => {
  if (props.activeTab !== undefined) return props.activeTab
  if (props.tabs && props.tabs.length > 0) return props.tabs[0].id
  return ''
})

function handleClose() {
  emit('close')
  emit('update:show', false)
}

function handleBackdropClick() {
  if (props.closeOnBackdrop) handleClose()
}

function handleKeydown(e: KeyboardEvent) {
  if (props.closeOnEsc && e.key === 'Escape' && props.show) handleClose()
}

function selectTab(tabId: string) {
  emit('update:activeTab', tabId)
  emit('tab-change', tabId)
}

function handleTabKeydown(e: KeyboardEvent, tabId: string) {
  if (!props.tabs || props.tabs.length === 0) return
  const currentIdx = props.tabs.findIndex((t) => t.id === tabId)
  if (currentIdx === -1) return

  if (e.key === 'ArrowRight') {
    e.preventDefault()
    const nextIdx = (currentIdx + 1) % props.tabs.length
    selectTab(props.tabs[nextIdx].id)
  } else if (e.key === 'ArrowLeft') {
    e.preventDefault()
    const prevIdx = (currentIdx - 1 + props.tabs.length) % props.tabs.length
    selectTab(props.tabs[prevIdx].id)
  }
}

watch(
  () => props.show,
  (isOpen) => {
    if (typeof document !== 'undefined') {
      document.body.style.overflow = isOpen ? 'hidden' : ''
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
    document.body.style.overflow = ''
  }
})
</script>

<template>
  <Teleport to="body">
    <Transition name="slide-over-fade">
      <div
        v-if="show"
        class="slide-over-backdrop"
        role="presentation"
        @click.self="handleBackdropClick"
      >
        <Transition name="slide-over-panel" appear>
          <aside
            class="slide-over-panel"
            role="dialog"
            aria-modal="true"
            :aria-label="title"
          >
            <!-- Header -->
            <header class="slide-over-header">
              <div class="slide-over-title-area">
                <div v-if="icon" class="slide-over-icon-box">
                  <BaseIcon :name="icon" size="md" />
                </div>
                <div class="slide-over-titles">
                  <div class="slide-over-title-row">
                    <h2 class="slide-over-title">{{ title }}</h2>
                    <StatusBadge v-if="status" :status="status" size="sm" />
                  </div>
                  <p v-if="subtitle" class="slide-over-subtitle">{{ subtitle }}</p>
                </div>
              </div>

              <div class="slide-over-actions">
                <slot name="header-actions"></slot>
                <button
                  v-if="closable"
                  type="button"
                  class="slide-over-close-btn"
                  title="Close inspector (Esc)"
                  aria-label="Close inspector"
                  @click="handleClose"
                >
                  <BaseIcon name="x" size="sm" />
                </button>
              </div>
            </header>

            <!-- Tabs Navigation -->
            <nav
              v-if="tabs && tabs.length > 0"
              class="slide-over-tabs"
              role="tablist"
              aria-label="Inspector tabs"
            >
              <button
                v-for="tab in tabs"
                :key="tab.id"
                type="button"
                role="tab"
                :aria-selected="currentTab === tab.id"
                :tabindex="currentTab === tab.id ? 0 : -1"
                class="slide-over-tab-btn"
                :class="{ 'is-active': currentTab === tab.id }"
                @click="selectTab(tab.id)"
                @keydown="handleTabKeydown($event, tab.id)"
              >
                <BaseIcon v-if="tab.icon" :name="tab.icon" size="xs" />
                <span class="tab-label">{{ tab.label }}</span>
                <span v-if="tab.count !== undefined" class="tab-count-badge">{{ tab.count }}</span>
              </button>
            </nav>

            <!-- Scrollable Body Slot -->
            <div class="slide-over-body">
              <slot :active-tab="currentTab" :close="handleClose"></slot>
            </div>

            <!-- Optional Footer Slot -->
            <footer v-if="$slots.footer" class="slide-over-footer">
              <slot name="footer" :close="handleClose"></slot>
            </footer>
          </aside>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.slide-over-backdrop {
  position: fixed; inset: 0; background: rgba(0, 0, 0, 0.65);
  backdrop-filter: blur(4px); -webkit-backdrop-filter: blur(4px);
  z-index: 1000; display: flex; justify-content: flex-end;
}
.slide-over-panel {
  position: fixed; top: 0; right: 0; bottom: 0; height: 100vh;
  width: 580px; max-width: 100vw; background: #141516;
  border-left: 1px solid rgba(255, 255, 255, 0.08);
  box-shadow: -8px 0 32px rgba(0, 0, 0, 0.6);
  z-index: 1000; display: flex; flex-direction: column;
}
@media (min-width: 768px) and (max-width: 1023px) { .slide-over-panel { width: 480px; } }
@media (max-width: 767px) { .slide-over-panel { width: 100vw; } }

.slide-over-header {
  padding: 16px 20px; border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  display: flex; align-items: center; justify-content: space-between;
  gap: 12px; flex-shrink: 0;
}
.slide-over-title-area { display: flex; align-items: center; gap: 12px; min-width: 0; }
.slide-over-icon-box {
  display: inline-flex; align-items: center; justify-content: center;
  width: 34px; height: 34px; border-radius: 8px; background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.08); color: #38bdf8; flex-shrink: 0;
}
.slide-over-titles { min-width: 0; }
.slide-over-title-row { display: flex; align-items: center; gap: 8px; }
.slide-over-title {
  margin: 0; font-size: 15px; font-weight: 600; color: #f8fafc;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.slide-over-subtitle {
  margin: 2px 0 0; font-size: 12px; color: var(--text-muted, #94a3b8);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.slide-over-actions { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
.slide-over-close-btn {
  display: inline-flex; align-items: center; justify-content: center;
  width: 28px; height: 28px; border-radius: 6px; border: 1px solid transparent;
  background: transparent; color: var(--text-muted, #94a3b8); cursor: pointer;
  transition: all 0.15s ease;
}
.slide-over-close-btn:hover {
  background: rgba(255, 255, 255, 0.08); color: #f8fafc;
  border-color: rgba(255, 255, 255, 0.1);
}

.slide-over-tabs {
  display: flex; gap: 4px; padding: 0 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(0, 0, 0, 0.2); overflow-x: auto; flex-shrink: 0;
}
.slide-over-tab-btn {
  display: inline-flex; align-items: center; gap: 6px; padding: 10px 12px;
  font-size: 12px; font-weight: 500; color: var(--text-muted, #94a3b8);
  border: none; background: transparent; cursor: pointer;
  border-bottom: 2px solid transparent; transition: all 0.15s ease; white-space: nowrap;
}
.slide-over-tab-btn:hover { color: #f8fafc; }
.slide-over-tab-btn.is-active { color: #38bdf8; border-bottom-color: #38bdf8; font-weight: 600; }
.tab-count-badge {
  font-family: var(--font-mono, monospace); font-size: 10px; padding: 1px 6px;
  border-radius: 9999px; background: rgba(255, 255, 255, 0.08); color: #cbd5e1;
}

.slide-over-body { flex: 1; overflow-y: auto; overflow-x: hidden; padding: 20px; }
.slide-over-footer {
  padding: 14px 20px; border-top: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(0, 0, 0, 0.2); display: flex; align-items: center;
  justify-content: flex-end; gap: 8px; flex-shrink: 0;
}

.slide-over-fade-enter-active, .slide-over-fade-leave-active { transition: opacity 0.2s ease; }
.slide-over-fade-enter-from, .slide-over-fade-leave-to { opacity: 0; }
.slide-over-panel-enter-active, .slide-over-panel-leave-active {
  transition: transform 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}
.slide-over-panel-enter-from, .slide-over-panel-leave-to { transform: translateX(100%); }
</style>
