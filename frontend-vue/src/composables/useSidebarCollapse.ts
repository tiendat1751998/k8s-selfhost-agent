import { ref, watch, onMounted, onUnmounted } from 'vue'
import type { NavItem } from '../config/navigation'

const STORAGE_KEY = 'k8s_sidebar_collapsed'

export interface UseSidebarCollapseOptions {
  props: {
    mobileOpen: boolean
    collapsed?: boolean
  }
  emit: {
    (e: 'update:collapsed', val: boolean): void
    (e: 'closeMobile'): void
  }
}

export function useSidebarCollapse(options: UseSidebarCollapseOptions) {
  const { props, emit } = options

  function getSavedCollapseState(): boolean {
    if (typeof window === 'undefined') return false
    const saved = localStorage.getItem(STORAGE_KEY)
    return saved !== null ? saved === 'true' : false
  }

  function getInitialCollapseState(): boolean {
    if (typeof window === 'undefined') return false
    const width = window.innerWidth
    if (width >= 641 && width <= 1200) {
      return true
    }
    if (width <= 640) {
      return false
    }
    return getSavedCollapseState()
  }

  const isCollapsed = ref<boolean>(getInitialCollapseState())

  watch(
    () => props.collapsed,
    (newVal) => {
      if (newVal !== undefined && newVal !== isCollapsed.value) {
        isCollapsed.value = newVal
      }
    }
  )

  function toggleCollapse() {
    isCollapsed.value = !isCollapsed.value
    try {
      localStorage.setItem(STORAGE_KEY, String(isCollapsed.value))
    } catch {
      // quota/sandboxing fallback
    }
    emit('update:collapsed', isCollapsed.value)
  }

  function handleResize() {
    if (typeof window === 'undefined') return
    const width = window.innerWidth
    if (width > 640 && props.mobileOpen) {
      emit('closeMobile')
    }
    if (width >= 641 && width <= 1200) {
      if (!isCollapsed.value) {
        isCollapsed.value = true
        emit('update:collapsed', true)
      }
    } else if (width > 1200) {
      const saved = getSavedCollapseState()
      if (isCollapsed.value !== saved) {
        isCollapsed.value = saved
        emit('update:collapsed', saved)
      }
    } else if (width <= 640) {
      if (isCollapsed.value) {
        isCollapsed.value = false
        emit('update:collapsed', false)
      }
    }
  }

  onMounted(() => {
    handleResize()
    emit('update:collapsed', isCollapsed.value)
    window.addEventListener('resize', handleResize)
  })

  onUnmounted(() => {
    window.removeEventListener('resize', handleResize)
  })

  const collapsedSections = ref<Record<string, boolean>>({
    observability: false,
    compute: false,
    governance: false,
    automation: false,
    management: false,
  })

  function toggleSection(sectionKey: string) {
    collapsedSections.value[sectionKey] = !collapsedSections.value[sectionKey]
  }

  const hoveredItem = ref<{ item: NavItem; top: number } | null>(null)

  function handleItemMouseEnter(item: NavItem, event: MouseEvent) {
    if (!isCollapsed.value) return
    const target = event.currentTarget as HTMLElement | null
    if (!target) return
    const rect = target.getBoundingClientRect()
    hoveredItem.value = {
      item,
      top: rect.top + rect.height / 2,
    }
  }

  function handleItemMouseLeave() {
    hoveredItem.value = null
  }

  function handleItemClick() {
    hoveredItem.value = null
    emit('closeMobile')
  }

  return {
    isCollapsed,
    toggleCollapse,
    collapsedSections,
    toggleSection,
    hoveredItem,
    handleItemMouseEnter,
    handleItemMouseLeave,
    handleItemClick,
  }
}
