<script setup lang="ts">
import { ref, computed, nextTick, onUnmounted } from 'vue'
import BaseIcon from './BaseIcon.vue'

export interface ActionItem {
  id: string
  label: string
  icon?: string
  variant?: 'default' | 'danger' | 'warning'
  disabled?: boolean
  separator?: boolean
}

interface Props {
  items: ActionItem[]
  align?: 'left' | 'right'
  triggerTitle?: string
  size?: 'xs' | 'sm' | 'md'
  disabled?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  align: 'right',
  triggerTitle: 'Actions',
  size: 'sm',
  disabled: false,
})

const emit = defineEmits<{
  (e: 'select', itemId: string): void
  (e: 'open'): void
  (e: 'close'): void
}>()

const isOpen = ref(false)
const triggerRef = ref<HTMLButtonElement | null>(null)
const menuRef = ref<HTMLDivElement | null>(null)
const activeIndex = ref<number>(-1)
const menuPosition = ref<{ top: number; left: number }>({ top: 0, left: 0 })

const iconSize = computed(() => (props.size === 'xs' ? 'xs' : props.size === 'md' ? 'md' : 'sm'))

function getSelectableIndices(): number[] {
  return props.items
    .map((item, idx) => (!item.separator && !item.disabled ? idx : -1))
    .filter((idx) => idx !== -1)
}

function updatePosition() {
  if (!triggerRef.value) return
  const r = triggerRef.value.getBoundingClientRect()
  const vw = window.innerWidth
  const vh = window.innerHeight
  const w = menuRef.value?.offsetWidth || 170
  const h = menuRef.value?.offsetHeight || 140

  let top = r.bottom + 4
  if (vh - r.bottom < h && r.top > vh - r.bottom) {
    top = Math.max(8, r.top - h - 4)
  }

  let left = props.align === 'left' ? r.left : r.right - w
  if (left + w > vw - 8) left = vw - w - 8
  if (left < 8) left = 8

  menuPosition.value = { top: Math.round(top), left: Math.round(left) }
}

function handlePointerDown(e: PointerEvent) {
  const t = e.target as Node | null
  if (t && (triggerRef.value?.contains(t) || menuRef.value?.contains(t))) return
  closeMenu()
}

function handleScrollOrResize() {
  if (isOpen.value) closeMenu()
}

function handleTriggerKeydown(e: KeyboardEvent) {
  if (!props.disabled && ['ArrowDown', 'Enter', ' '].includes(e.key) && !isOpen.value) {
    e.preventDefault(); openMenu(true)
  }
}

function handleMenuKeydown(e: KeyboardEvent) {
  if (!isOpen.value) return
  if (e.key === 'Escape') {
    e.preventDefault(); closeMenu(true); return
  }
  const sel = getSelectableIndices()
  if (sel.length === 0) return
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    const pos = sel.indexOf(activeIndex.value)
    activeIndex.value = pos === -1 || pos === sel.length - 1 ? sel[0] : sel[pos + 1]
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    const pos = sel.indexOf(activeIndex.value)
    activeIndex.value = pos <= 0 ? sel[sel.length - 1] : sel[pos - 1]
  } else if (e.key === 'Enter') {
    e.preventDefault()
    const item = props.items[activeIndex.value]
    if (item && !item.disabled && !item.separator) selectItem(item)
  } else if (e.key === 'Tab') {
    closeMenu()
  }
}

function toggleListeners(add: boolean) {
  const fn = add ? window.addEventListener : window.removeEventListener
  fn('pointerdown', handlePointerDown)
  fn('keydown', handleMenuKeydown)
  fn('scroll', handleScrollOrResize, true)
  fn('resize', handleScrollOrResize)
}

async function openMenu(focusFirst = false) {
  if (props.disabled || isOpen.value) return
  isOpen.value = true
  activeIndex.value = -1
  emit('open')
  await nextTick()
  updatePosition()
  toggleListeners(true)
  if (focusFirst) {
    const sel = getSelectableIndices()
    if (sel.length > 0) activeIndex.value = sel[0]
  }
}

function closeMenu(focusTrigger = false) {
  if (!isOpen.value) return
  isOpen.value = false
  activeIndex.value = -1
  toggleListeners(false)
  emit('close')
  if (focusTrigger) triggerRef.value?.focus()
}

function toggleMenu() {
  if (isOpen.value) closeMenu()
  else openMenu()
}

function selectItem(item: ActionItem) {
  if (item.disabled || item.separator) return
  emit('select', item.id)
  closeMenu(true)
}

onUnmounted(() => toggleListeners(false))
</script>

<template>
  <div class="action-dropdown">
    <button
      ref="triggerRef"
      type="button"
      class="action-dropdown-trigger"
      :class="[`trigger-${size}`, { 'is-active': isOpen }]"
      :disabled="disabled"
      :title="triggerTitle"
      :aria-label="triggerTitle"
      :aria-expanded="isOpen"
      aria-haspopup="true"
      @click.stop="toggleMenu"
      @keydown="handleTriggerKeydown"
    >
      <BaseIcon name="more-vertical" :size="iconSize" />
    </button>

    <Teleport to="body">
      <Transition name="action-dropdown-fade">
        <div
          v-if="isOpen"
          ref="menuRef"
          class="action-dropdown-menu"
          :style="{ top: `${menuPosition.top}px`, left: `${menuPosition.left}px` }"
          role="menu"
          :aria-label="triggerTitle"
          @click.stop
        >
          <template v-for="(item, idx) in items" :key="item.id || idx">
            <div v-if="item.separator" class="action-dropdown-separator" role="separator" />
            <button
              v-else
              type="button"
              class="action-dropdown-item"
              :class="[item.variant ? `variant-${item.variant}` : 'variant-default', { 'is-focused': activeIndex === idx }]"
              :disabled="item.disabled"
              role="menuitem"
              @mouseenter="!item.disabled && (activeIndex = idx)"
              @click="selectItem(item)"
            >
              <BaseIcon v-if="item.icon" :name="item.icon" size="xs" class="action-item-icon" />
              <span class="action-item-label">{{ item.label }}</span>
            </button>
          </template>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.action-dropdown { display: inline-flex; align-items: center; position: relative; }
.action-dropdown-trigger {
  display: inline-flex; align-items: center; justify-content: center;
  background: transparent; border: 1px solid transparent; color: var(--text-muted, #94a3b8);
  border-radius: 6px; cursor: pointer; padding: 0; transition: all 0.15s ease; line-height: 1;
}
.action-dropdown-trigger:hover:not(:disabled), .action-dropdown-trigger.is-active {
  color: var(--text-primary, #f8fafc); background: rgba(255, 255, 255, 0.08); border-color: rgba(255, 255, 255, 0.1);
}
.action-dropdown-trigger:disabled { opacity: 0.4; cursor: not-allowed; }
.trigger-xs { width: 22px; height: 22px; }
.trigger-sm { width: 26px; height: 26px; }
.trigger-md { width: 32px; height: 32px; }
</style>

<style>
.action-dropdown-menu {
  position: fixed !important; z-index: 9999; min-width: 170px; background: #141516;
  border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5), 0 2px 6px rgba(0, 0, 0, 0.3);
  padding: 4px; display: flex; flex-direction: column; gap: 2px;
  backdrop-filter: blur(12px); -webkit-backdrop-filter: blur(12px); box-sizing: border-box;
}
.action-dropdown-item {
  display: flex; align-items: center; gap: 8px; padding: 6px 10px; border-radius: 4px;
  font-size: 12px; font-family: var(--font-sans, system-ui, sans-serif); font-weight: 500;
  color: #f8fafc; border: none; background: transparent; cursor: pointer; text-align: left;
  width: 100%; box-sizing: border-box; transition: background 0.15s ease, color 0.15s ease;
}
.action-dropdown-item:hover:not(:disabled) { background: rgba(255, 255, 255, 0.06); color: #ffffff; }
.action-dropdown-item.is-focused:not(:disabled) { background: rgba(255, 255, 255, 0.08); color: #ffffff; }
.action-dropdown-item.variant-danger { color: #fb7185; }
.action-dropdown-item.variant-danger:hover:not(:disabled),
.action-dropdown-item.variant-danger.is-focused:not(:disabled) { background: rgba(244, 63, 94, 0.12); color: #fda4af; }
.action-dropdown-item.variant-warning { color: #fbbf24; }
.action-dropdown-item.variant-warning:hover:not(:disabled),
.action-dropdown-item.variant-warning.is-focused:not(:disabled) { background: rgba(245, 158, 11, 0.14); color: #fde68a; }
.action-dropdown-item:disabled { opacity: 0.4; cursor: not-allowed; }
.action-dropdown-separator { height: 1px; background: rgba(255, 255, 255, 0.06); margin: 4px 0; }
.action-dropdown-fade-enter-active, .action-dropdown-fade-leave-active { transition: opacity 0.12s ease, transform 0.12s ease; }
.action-dropdown-fade-enter-from, .action-dropdown-fade-leave-to { opacity: 0; transform: scale(0.97); }
</style>
