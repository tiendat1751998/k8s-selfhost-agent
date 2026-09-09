<script setup lang="ts">
import { ref, watch } from 'vue'

interface Props {
  memoryLimit: string
  memoryReservation: string
  cpuLimit: string
  cpuReservation: string
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:memoryLimit', value: string): void
  (e: 'update:memoryReservation', value: string): void
  (e: 'update:cpuLimit', value: string): void
  (e: 'update:cpuReservation', value: string): void
}>()

const customMemLimitValue = ref<number | string>(1)
const customMemLimitUnit = ref<'MiB' | 'GiB'>('GiB')

watch(
  () => props.memoryLimit,
  (mem) => {
    if (!mem) return
    const clean = mem.trim()
    if (/gi/i.test(clean)) {
      const num = parseFloat(clean.replace(/[^0-9.]/g, '')) || 1
      customMemLimitValue.value = num
      customMemLimitUnit.value = 'GiB'
    } else if (/mi/i.test(clean)) {
      const num = parseFloat(clean.replace(/[^0-9.]/g, '')) || 512
      customMemLimitValue.value = num
      customMemLimitUnit.value = 'MiB'
    }
  },
  { immediate: true }
)

function setMemoryLimitPreset(preset: string) {
  const clean = preset.replace(/\s+/g, '')
  emit('update:memoryLimit', clean)
  if (clean.endsWith('GiB')) {
    customMemLimitValue.value = parseFloat(clean.replace('GiB', '')) || 1
    customMemLimitUnit.value = 'GiB'
  } else if (clean.endsWith('MiB')) {
    customMemLimitValue.value = parseFloat(clean.replace('MiB', '')) || 512
    customMemLimitUnit.value = 'MiB'
  }
}

function onCustomMemLimitInput() {
  const val = customMemLimitValue.value
  if (val !== '' && val !== null && val !== undefined) {
    emit('update:memoryLimit', `${val}${customMemLimitUnit.value}`)
  }
}

function setCustomMemLimitUnit(unit: 'MiB' | 'GiB') {
  customMemLimitUnit.value = unit
  const val = customMemLimitValue.value || 1
  emit('update:memoryLimit', `${val}${unit}`)
}

function setMemoryReservationPreset(preset: string) {
  emit('update:memoryReservation', preset.replace(/\s+/g, ''))
}

function setCpuLimitPreset(preset: string) {
  if (preset.includes('0.5')) {
    emit('update:cpuLimit', '0.5')
  } else {
    const num = preset.replace(/[^0-9.]/g, '')
    emit('update:cpuLimit', num ? `${num} Cores` : '1 Core')
  }
}

function isCpuLimitActive(preset: string): boolean {
  if (preset.includes('0.5')) {
    return props.cpuLimit === '0.5' || props.cpuLimit === '500m' || props.cpuLimit === '0.5 Core'
  }
  const num = preset.replace(/[^0-9.]/g, '')
  return props.cpuLimit === num ||
    props.cpuLimit === `${num} Core` ||
    props.cpuLimit === `${num} Cores` ||
    props.cpuLimit === `${parseInt(num || '1') * 1000}m`
}

function setCpuReservationPreset(preset: string) {
  if (preset.includes('Core')) {
    const num = preset.replace(/[^0-9.]/g, '') || '1'
    emit('update:cpuReservation', `${num} Core`)
  } else {
    emit('update:cpuReservation', preset.trim())
  }
}

function isCpuResActive(preset: string): boolean {
  if (preset.includes('Core')) {
    const num = preset.replace(/[^0-9.]/g, '') || '1'
    return props.cpuReservation === num || props.cpuReservation === `${num} Core` || props.cpuReservation === '1000m'
  }
  return props.cpuReservation === preset.trim()
}
</script>

<template>
  <div class="scale-section-wrap vertical-tuning-wrap animate-fade-in">
    <!-- 1. RAM Memory Limit -->
    <div class="resource-block">
      <div class="resource-header">
        <div class="resource-label-group">
          <span class="resource-name">RAM Memory Limit</span>
          <span class="resource-hint text-muted">Hard allocation limit before container OOM kill</span>
        </div>
        <span class="current-resource-val font-mono text-cyan">{{ memoryLimit }}</span>
      </div>

      <div class="presets-row">
        <button
          v-for="preset in ['256 MiB', '512 MiB', '1 GiB', '2 GiB', '4 GiB', '8 GiB']"
          :key="preset"
          type="button"
          class="preset-btn"
          :class="{ active: memoryLimit === preset.replace(/\s+/g, '') }"
          @click="setMemoryLimitPreset(preset)"
        >
          {{ preset }}
        </button>
      </div>

      <!-- Custom RAM Limit Input with Unit Toggle -->
      <div class="custom-input-row">
        <span class="custom-label">Custom RAM Limit:</span>
        <div class="custom-input-group">
          <input
            v-model.number="customMemLimitValue"
            type="number"
            min="1"
            placeholder="e.g. 1024"
            class="custom-number-input font-mono"
            @input="onCustomMemLimitInput"
          />
          <div class="unit-toggle-group">
            <button
              type="button"
              class="unit-btn"
              :class="{ active: customMemLimitUnit === 'MiB' }"
              @click="setCustomMemLimitUnit('MiB')"
            >MiB</button>
            <button
              type="button"
              class="unit-btn"
              :class="{ active: customMemLimitUnit === 'GiB' }"
              @click="setCustomMemLimitUnit('GiB')"
            >GiB</button>
          </div>
        </div>
      </div>
    </div>

    <!-- 2. RAM Memory Reservation -->
    <div class="resource-block">
      <div class="resource-header">
        <div class="resource-label-group">
          <span class="resource-name">RAM Reservation</span>
          <span class="resource-hint text-muted">Guaranteed baseline RAM reserved per pod</span>
        </div>
        <span class="current-resource-val font-mono text-emerald">{{ memoryReservation }}</span>
      </div>

      <div class="presets-row">
        <button
          v-for="preset in ['128 MiB', '256 MiB', '512 MiB', '1 GiB']"
          :key="preset"
          type="button"
          class="preset-btn"
          :class="{ active: memoryReservation === preset.replace(/\s+/g, '') }"
          @click="setMemoryReservationPreset(preset)"
        >
          {{ preset }}
        </button>
      </div>
    </div>

    <!-- 3. CPU Core Limit -->
    <div class="resource-block">
      <div class="resource-header">
        <div class="resource-label-group">
          <span class="resource-name">CPU Core Limit</span>
          <span class="resource-hint text-muted">Maximum compute throttling ceiling</span>
        </div>
        <span class="current-resource-val font-mono text-amber">{{ cpuLimit }}</span>
      </div>

      <div class="presets-row">
        <button
          v-for="preset in ['0.5 Core', '1 Core', '2 Cores', '4 Cores', '8 Cores']"
          :key="preset"
          type="button"
          class="preset-btn"
          :class="{ active: isCpuLimitActive(preset) }"
          @click="setCpuLimitPreset(preset)"
        >
          {{ preset }}
        </button>
      </div>

      <div class="custom-input-row">
        <span class="custom-label">Custom CPU Limit:</span>
        <div class="custom-input-group single-field">
          <input
            :value="cpuLimit"
            type="text"
            placeholder="e.g. 2, 4, 500m"
            class="custom-number-input font-mono"
            @input="emit('update:cpuLimit', ($event.target as HTMLInputElement).value)"
          />
        </div>
      </div>
    </div>

    <!-- 4. CPU Core Reservation -->
    <div class="resource-block">
      <div class="resource-header">
        <div class="resource-label-group">
          <span class="resource-name">CPU Core Reservation</span>
          <span class="resource-hint text-muted">Guaranteed compute capacity requested on node</span>
        </div>
        <span class="current-resource-val font-mono text-indigo">{{ cpuReservation }}</span>
      </div>

      <div class="presets-row">
        <button
          v-for="preset in ['100m', '250m', '500m', '1 Core']"
          :key="preset"
          type="button"
          class="preset-btn"
          :class="{ active: isCpuResActive(preset) }"
          @click="setCpuReservationPreset(preset)"
        >
          {{ preset }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/scale-modal.css';
</style>
