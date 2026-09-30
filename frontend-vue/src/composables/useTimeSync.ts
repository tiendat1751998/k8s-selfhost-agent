import { ref, type Ref } from 'vue'

/**
 * Singleton reactive state for synchronized crosshair telemetry.
 * Shared across all CanvasTimeSeries and CanvasSparkline instances in the view.
 */
const activeHoverTime: Ref<number | null> = ref(null)
const activeHoverGroup: Ref<string | null> = ref(null)

export interface TimeSyncContext {
  activeHoverTime: Ref<number | null>
  activeHoverGroup: Ref<string | null>
  setHoverTime: (time: number | null, group?: string) => void
  clearHoverTime: () => void
}

export function useTimeSync(): TimeSyncContext {
  function setHoverTime(time: number | null, group: string = 'default'): void {
    activeHoverTime.value = time
    activeHoverGroup.value = time !== null ? group : null
  }

  function clearHoverTime(): void {
    activeHoverTime.value = null
    activeHoverGroup.value = null
  }

  return {
    activeHoverTime,
    activeHoverGroup,
    setHoverTime,
    clearHoverTime,
  }
}
