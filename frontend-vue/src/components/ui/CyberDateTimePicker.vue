<script setup lang="ts">
import {
  MONTH_NAMES,
  DAY_NAMES,
  pad,
  useCyberDateTimePicker
} from './cyberDatePickerHelpers'

interface Props {
  modelValue?: string
  label?: string
  placeholder?: string
  min?: string
  max?: string
  disabled?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: '',
  label: '',
  placeholder: 'YYYY-MM-DD HH:mm',
  min: '',
  max: '',
  disabled: false,
})

const emit = defineEmits<{
  (e: 'update:modelValue', val: string): void
  (e: 'change', val: string): void
  (e: 'apply', val: string): void
}>()

const {
  isOpen,
  pickerContainerRef,
  tempYear,
  tempMonth,
  tempDay,
  tempHour,
  tempMinute,
  viewYear,
  viewMonth,
  displayValue,
  calendarDays,
  prevMonth,
  nextMonth,
  prevYear,
  nextYear,
  selectDayCell,
  setTimeHours,
  setTimeMinutes,
  applyDatePreset,
  applyQuickTime,
  setNowFull,
  togglePicker,
  handleCancel,
  handleApply
} = useCyberDateTimePicker(props, (event, val) => emit(event as any, val))
void pickerContainerRef
</script>

<template>
  <div class="cyber-datetime-wrapper" :class="{ 'is-open': isOpen, 'is-disabled': disabled }" ref="pickerContainerRef">
    <div
      class="cyber-trigger-box"
      :class="{ 'is-open': isOpen, 'is-disabled': disabled }"
      @click="togglePicker"
      tabindex="0"
      :aria-expanded="isOpen"
      aria-label="Select Date and Time"
    >
      <div class="trigger-left">
        <span class="cyber-cal-icon">📅</span>
        <span v-if="displayValue" class="trigger-value font-mono">{{ displayValue }}</span>
        <span v-else class="trigger-placeholder font-mono">{{ placeholder }}</span>
      </div>
      <div class="trigger-right">
        <span class="cyber-arrow-icon">{{ isOpen ? '▲' : '▼' }}</span>
      </div>
    </div>

    <transition name="cyber-popover">
      <div v-if="isOpen" class="cyber-picker-popup" role="dialog" aria-modal="true">
        <div class="popup-top-bar">
          <div class="month-nav-controls">
            <button type="button" class="btn-nav-arrow" @click.stop="prevYear" title="Previous Year">«</button>
            <button type="button" class="btn-nav-arrow" @click.stop="prevMonth" title="Previous Month">◀</button>
            <div class="current-month-year-badge">
              <span class="badge-month">{{ MONTH_NAMES[viewMonth] }}</span>
              <span class="badge-year font-mono">{{ viewYear }}</span>
            </div>
            <button type="button" class="btn-nav-arrow" @click.stop="nextMonth" title="Next Month">▶</button>
            <button type="button" class="btn-nav-arrow" @click.stop="nextYear" title="Next Year">»</button>
          </div>

          <div class="quick-date-chips">
            <button type="button" class="btn-date-chip" @click.stop="applyDatePreset('today')">Today</button>
            <button type="button" class="btn-date-chip" @click.stop="applyDatePreset('yesterday')">Yesterday</button>
            <button type="button" class="btn-date-chip" @click.stop="applyDatePreset('minus7d')">-7d</button>
          </div>
        </div>

        <div class="popup-main-body">
          <div class="calendar-column">
            <div class="calendar-week-header">
              <span v-for="d in DAY_NAMES" :key="d" class="week-name">{{ d }}</span>
            </div>

            <div class="calendar-grid">
              <button
                v-for="(cell, idx) in calendarDays"
                :key="`${cell.year}-${cell.month}-${cell.day}-${idx}`"
                type="button"
                class="calendar-cell font-mono"
                :class="{
                  'other-month': !cell.isCurrentMonth,
                  'is-selected': cell.isSelected,
                  'is-today': cell.isToday
                }"
                @click.stop="selectDayCell(cell)"
              >
                <span class="cell-number">{{ cell.day }}</span>
                <span v-if="cell.isToday && !cell.isSelected" class="today-indicator-dot"></span>
              </button>
            </div>
          </div>

          <div class="column-divider"></div>

          <div class="time-column">
            <div class="time-header-box">
              <span class="time-title font-mono">⏱️ TIME (24H)</span>
              <div class="time-digital-display font-mono">
                <span class="time-digit">{{ pad(tempHour) }}</span>
                <span class="time-colon">:</span>
                <span class="time-digit">{{ pad(tempMinute) }}</span>
              </div>
            </div>

            <div class="time-quick-chips">
              <button type="button" class="btn-time-chip" @click.stop="applyQuickTime('now')">Now</button>
              <button type="button" class="btn-time-chip" @click.stop="applyQuickTime('00:00')">00:00</button>
              <button type="button" class="btn-time-chip" @click.stop="applyQuickTime('12:00')">12:00</button>
              <button type="button" class="btn-time-chip" @click.stop="applyQuickTime('minus15m')">-15m</button>
              <button type="button" class="btn-time-chip" @click.stop="applyQuickTime('minus1h')">-1h</button>
            </div>

            <div class="time-pickers-split">
              <div class="time-sub-section">
                <div class="sub-section-title">HOUR (0-23)</div>
                <div class="hours-scroll-grid">
                  <button
                    v-for="h in 24"
                    :key="`h-${h - 1}`"
                    type="button"
                    class="btn-time-unit font-mono"
                    :class="{ 'is-selected': tempHour === (h - 1) }"
                    @click.stop="setTimeHours(h - 1)"
                  >
                    {{ pad(h - 1) }}
                  </button>
                </div>
              </div>

              <div class="time-sub-section">
                <div class="sub-section-title">MIN (0-55)</div>
                <div class="minutes-scroll-grid">
                  <button
                    v-for="mStep in [0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55]"
                    :key="`m-${mStep}`"
                    type="button"
                    class="btn-time-unit font-mono"
                    :class="{ 'is-selected': tempMinute === mStep }"
                    @click.stop="setTimeMinutes(mStep)"
                  >
                    {{ pad(mStep) }}
                  </button>
                </div>
                <div class="exact-minute-stepper">
                  <label class="font-mono">Exact:</label>
                  <input
                    type="number"
                    min="0"
                    max="59"
                    :value="tempMinute"
                    @input="e => setTimeMinutes(parseInt((e.target as HTMLInputElement).value || '0', 10))"
                    class="exact-min-input font-mono"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="popup-footer-bar">
          <div class="footer-left">
            <button type="button" class="btn-picker-quicknow font-mono" @click.stop="setNowFull" title="Set to current date and time">
              ⚡ Set Now
            </button>
            <div class="preview-tag font-mono">
              {{ tempYear }}-{{ pad(tempMonth + 1) }}-{{ pad(tempDay) }} {{ pad(tempHour) }}:{{ pad(tempMinute) }}
            </div>
          </div>

          <div class="footer-right">
            <button type="button" class="btn-picker-cancel font-mono" @click.stop="handleCancel">
              ✕ Cancel
            </button>
            <button type="button" class="btn-picker-ok font-mono" @click.stop="handleApply">
              <span class="glow-check">✓</span>
              <span>OK / Apply</span>
            </button>
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/cyber-date-time-picker.css';
</style>
