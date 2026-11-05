import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick, type Ref } from 'vue'

export interface UseVirtualLogScrollOptions {
  autoScroll?: Ref<boolean>
  isScrollLocked?: Ref<boolean>
  hoverPauseEnabled?: Ref<boolean>
  effectiveRowHeight: Ref<number>
  logsCount: Ref<number>
  emitScroll?: (e: Event) => void
  emitScrollToBottom?: () => void
  emitHoverChange?: (hovering: boolean) => void
  emitRegisterTerminal?: (el: HTMLElement | null) => void
}

export function useVirtualLogScroll(options: UseVirtualLogScrollOptions) {
  const viewportRef = ref<HTMLElement | null>(null)
  const scrollTop = ref(0)
  const clientHeight = ref(600)
  const unpinnedNewLogsCount = ref(0)
  const isPinnedToBottom = ref(true)
  const isMobile = ref(typeof window !== 'undefined' ? window.innerWidth < 640 : false)
  const isHovered = ref(false)

  const isHoverPaused = computed(() => !!options.hoverPauseEnabled?.value && isHovered.value)

  let resizeObserver: ResizeObserver | null = null

  function handleResize() {
    if (viewportRef.value) {
      clientHeight.value = viewportRef.value.clientHeight || 600
    }
    if (typeof window !== 'undefined') {
      isMobile.value = window.innerWidth < 640
    }
  }

  function handleViewportScroll(e: Event) {
    options.emitScroll?.(e)
    const el = viewportRef.value
    if (!el) return

    scrollTop.value = el.scrollTop
    const threshold = options.effectiveRowHeight.value * 2
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight <= threshold

    if (atBottom) {
      isPinnedToBottom.value = true
      unpinnedNewLogsCount.value = 0
    } else {
      isPinnedToBottom.value = false
    }
  }

  function scrollToBottom() {
    const el = viewportRef.value
    if (!el) return
    requestAnimationFrame(() => {
      el.scrollTop = el.scrollHeight
      scrollTop.value = el.scrollTop
      isPinnedToBottom.value = true
      unpinnedNewLogsCount.value = 0
      options.emitScrollToBottom?.()
    })
  }

  function handleMouseEnter() {
    isHovered.value = true
    options.emitHoverChange?.(true)
  }

  function handleMouseLeave() {
    isHovered.value = false
    options.emitHoverChange?.(false)
    if (options.autoScroll?.value && isPinnedToBottom.value && !options.isScrollLocked?.value) {
      scrollToBottom()
    }
  }

  onMounted(() => {
    options.emitRegisterTerminal?.(viewportRef.value)
    if (viewportRef.value) {
      clientHeight.value = viewportRef.value.clientHeight || 600
      viewportRef.value.addEventListener('scroll', handleViewportScroll, { passive: true })
      if (typeof ResizeObserver !== 'undefined') {
        resizeObserver = new ResizeObserver((entries) => {
          for (const entry of entries) {
            if (entry.target === viewportRef.value) {
              clientHeight.value = entry.contentRect.height || viewportRef.value.clientHeight || 600
            }
          }
        })
        resizeObserver.observe(viewportRef.value)
      }
    }
    window.addEventListener('resize', handleResize)
    if (options.autoScroll?.value) {
      scrollToBottom()
    }
  })

  onBeforeUnmount(() => {
    if (resizeObserver) {
      resizeObserver.disconnect()
      resizeObserver = null
    }
    if (viewportRef.value) {
      viewportRef.value.removeEventListener('scroll', handleViewportScroll)
    }
    window.removeEventListener('resize', handleResize)
  })

  watch(
    () => options.logsCount.value,
    (newLen, oldLen) => {
      const diff = newLen - (oldLen || 0)
      const isPausedReading = isHoverPaused.value || !!options.isScrollLocked?.value || !isPinnedToBottom.value
      if (diff > 0 && isPausedReading) {
        unpinnedNewLogsCount.value += diff
      }
      if (options.autoScroll?.value && isPinnedToBottom.value && !options.isScrollLocked?.value && !isHoverPaused.value) {
        nextTick(() => scrollToBottom())
      }
    }
  )

  return {
    viewportRef,
    scrollTop,
    clientHeight,
    unpinnedNewLogsCount,
    isPinnedToBottom,
    isMobile,
    isHoverPaused,
    scrollToBottom,
    handleMouseEnter,
    handleMouseLeave,
    handleViewportScroll,
  }
}
