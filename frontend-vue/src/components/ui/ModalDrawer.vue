<script setup lang="ts">
import { watch, onMounted, onUnmounted } from 'vue'

const props = withDefaults(
  defineProps<{
    show: boolean
    title?: string
    subtitle?: string
    mode?: 'modal' | 'drawer'
    placement?: 'right' | 'left'
    maxWidth?: string
    closable?: boolean
    closeOnEsc?: boolean
    closeOnBackdrop?: boolean
  }>(),
  {
    mode: 'modal',
    placement: 'right',
    maxWidth: undefined,
    closable: true,
    closeOnEsc: true,
    closeOnBackdrop: true,
  }
)

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update:show', value: boolean): void
}>()

function handleClose() {
  emit('close')
  emit('update:show', false)
}

function handleBackdropClick() {
  if (props.closeOnBackdrop) {
    handleClose()
  }
}

function handleKeydown(e: KeyboardEvent) {
  if (props.closeOnEsc && e.key === 'Escape' && props.show) {
    handleClose()
  }
}

watch(
  () => props.show,
  (isOpen) => {
    if (typeof document !== 'undefined') {
      if (isOpen) {
        document.body.style.overflow = 'hidden'
      } else {
        document.body.style.overflow = ''
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
    document.body.style.overflow = ''
  }
})
</script>

<template>
  <Teleport to="body">
    <Transition name="overlay-fade">
      <div 
        v-if="show" 
        class="modal-drawer-backdrop" 
        :class="{ 'is-drawer': mode === 'drawer' }"
        @click.self="handleBackdropClick"
        role="dialog"
        aria-modal="true"
      >
        <!-- Modal Mode Content -->
        <Transition v-if="mode === 'modal'" name="modal-scale" appear>
          <div 
            class="modal-panel glass-panel" 
            :style="{ maxWidth: maxWidth || '580px' }"
          >
            <div class="panel-header">
              <div class="header-titles">
                <slot name="header-prefix"></slot>
                <div class="title-wrap">
                  <h3 class="panel-title">{{ title || 'Dialog' }}</h3>
                  <p v-if="subtitle" class="panel-subtitle">{{ subtitle }}</p>
                </div>
              </div>
              <div class="header-actions">
                <slot name="header-actions"></slot>
                <button 
                  v-if="closable" 
                  class="close-button" 
                  title="Close dialog (Esc)" 
                  @click="handleClose"
                  type="button"
                >
                  <span class="close-icon">✕</span>
                </button>
              </div>
            </div>

            <div class="panel-body">
              <slot></slot>
            </div>

            <div v-if="$slots.footer" class="panel-footer">
              <slot name="footer" :close="handleClose"></slot>
            </div>
          </div>
        </Transition>

        <!-- Drawer Mode Content -->
        <Transition v-else name="drawer-slide" appear>
          <div 
            class="drawer-panel" 
            :class="`drawer-${placement}`"
            :style="{ width: maxWidth || '520px' }"
          >
            <div class="panel-header">
              <div class="header-titles">
                <slot name="header-prefix"></slot>
                <div class="title-wrap">
                  <h3 class="panel-title">{{ title || 'Details' }}</h3>
                  <p v-if="subtitle" class="panel-subtitle">{{ subtitle }}</p>
                </div>
              </div>
              <div class="header-actions">
                <slot name="header-actions"></slot>
                <button 
                  v-if="closable" 
                  class="close-button" 
                  title="Close drawer (Esc)" 
                  @click="handleClose"
                  type="button"
                >
                  <span class="close-icon">✕</span>
                </button>
              </div>
            </div>

            <div class="panel-body drawer-body">
              <slot></slot>
            </div>

            <div v-if="$slots.footer" class="panel-footer">
              <slot name="footer" :close="handleClose"></slot>
            </div>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.modal-drawer-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(4, 6, 12, 0.78);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.modal-drawer-backdrop.is-drawer {
  padding: 0;
  justify-content: flex-end;
}

/* Modal Panel */
.modal-panel {
  width: 100%;
  background: rgba(13, 19, 33, 0.95);
  border: 1px solid var(--border-medium);
  border-radius: 18px;
  box-shadow: 0 25px 60px -15px rgba(0, 0, 0, 0.8), 0 0 0 1px rgba(56, 189, 248, 0.1);
  display: flex;
  flex-direction: column;
  max-height: 90vh;
  overflow: hidden;
}

/* Drawer Panel */
.drawer-panel {
  height: 100vh;
  max-width: 100vw;
  background: rgba(11, 16, 28, 0.98);
  border-left: 1px solid var(--border-medium);
  box-shadow: -15px 0 45px rgba(0, 0, 0, 0.7);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  z-index: 1001;
}

.drawer-left {
  position: absolute;
  left: 0;
  border-left: none;
  border-right: 1px solid var(--border-medium);
}

.panel-header {
  padding: 18px 24px;
  border-bottom: 1px solid var(--border-subtle);
  background: rgba(15, 23, 42, 0.6);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.header-titles {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.title-wrap {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.panel-title {
  font-size: 16px;
  font-weight: 700;
  color: #fff;
  letter-spacing: -0.01em;
  margin: 0;
}

.panel-subtitle {
  font-size: 12px;
  color: var(--text-muted);
  margin: 0;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.close-button {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.15s ease;
}

.close-button:hover {
  background: rgba(244, 63, 94, 0.15);
  border-color: rgba(244, 63, 94, 0.4);
  color: #fb7185;
}

.close-icon {
  font-size: 12px;
  font-weight: bold;
}

.panel-body {
  padding: 24px;
  overflow-y: auto;
  flex: 1;
}

.drawer-body {
  padding: 24px 28px;
}

.panel-footer {
  padding: 16px 24px;
  border-top: 1px solid var(--border-subtle);
  background: rgba(15, 23, 42, 0.4);
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
}

/* Animations */
.overlay-fade-enter-active,
.overlay-fade-leave-active {
  transition: opacity 0.22s ease;
}
.overlay-fade-enter-from,
.overlay-fade-leave-to {
  opacity: 0;
}

.modal-scale-enter-active {
  transition: all 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}
.modal-scale-leave-active {
  transition: all 0.18s ease-in;
}
.modal-scale-enter-from {
  opacity: 0;
  transform: scale(0.94) translateY(10px);
}
.modal-scale-leave-to {
  opacity: 0;
  transform: scale(0.97) translateY(-6px);
}

.drawer-slide-enter-active {
  transition: transform 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}
.drawer-slide-leave-active {
  transition: transform 0.22s ease-in;
}
.drawer-slide-enter-from {
  transform: translateX(100%);
}
.drawer-slide-leave-to {
  transform: translateX(100%);
}

@media (max-width: 640px) {
  .modal-panel {
    width: calc(100vw - 20px) !important;
    max-width: 100% !important;
    margin: 10px;
    border-radius: 12px;
  }

  .drawer-panel {
    width: 100vw !important;
    max-width: 100vw !important;
    border-radius: 0;
  }

  .panel-header,
  .panel-body,
  .panel-footer {
    padding: 12px 14px;
  }

  .drawer-body {
    padding: 12px 14px;
  }
}
</style>
