<script setup lang="ts">
import { computed } from 'vue'

export type IconSize = 'xs' | 'sm' | 'md' | 'lg' | 'xl' | number | string

export type { KnownIconName, IconName } from './iconPaths'

export interface BaseIconProps {
  name: string
  size?: IconSize
  strokeWidth?: number | string
  class?: string | Record<string, boolean | undefined | null> | (string | Record<string, boolean | undefined | null>)[]
}

const props = withDefaults(defineProps<BaseIconProps>(), {
  size: 'sm',
  strokeWidth: 1.5,
})

const sizeMap: Record<'xs' | 'sm' | 'md' | 'lg' | 'xl', number> = {
  xs: 12,
  sm: 14,
  md: 16,
  lg: 20,
  xl: 24,
}

const pixelSize = computed<string>(() => {
  if (typeof props.size === 'number') return `${props.size}px`
  if (typeof props.size === 'string') {
    if (props.size in sizeMap) {
      return `${sizeMap[props.size as keyof typeof sizeMap]}px`
    }
    const parsed = Number(props.size)
    if (!Number.isNaN(parsed)) return `${parsed}px`
  }
  return `${sizeMap.sm}px`
})

import { ICONS, ALIAS_MAP, FALLBACK_SVG } from './iconPaths'

const svgContent = computed<string>(() => {
  if (!props.name) return FALLBACK_SVG
  const rawKey = String(props.name).trim().toLowerCase()
  const key = ICONS[rawKey] ? rawKey : (ALIAS_MAP[rawKey] || rawKey)
  return ICONS[key] || FALLBACK_SVG
})
</script>

<template>
  <svg
    xmlns="http://www.w3.org/2000/svg"
    :width="pixelSize"
    :height="pixelSize"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    :stroke-width="strokeWidth"
    stroke-linecap="round"
    stroke-linejoin="round"
    :class="['base-icon', props.class]"
    aria-hidden="true"
    v-html="svgContent"
  />
</template>

<style scoped>
.base-icon {
  display: inline-block;
  vertical-align: middle;
  flex-shrink: 0;
}
</style>
