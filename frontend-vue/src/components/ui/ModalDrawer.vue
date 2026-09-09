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
@import '../../assets/styles/components/ui/modal-drawer.css';
</style>
