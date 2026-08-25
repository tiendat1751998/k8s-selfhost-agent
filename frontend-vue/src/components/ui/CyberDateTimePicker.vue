<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'

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

const isOpen = ref(false)
const pickerContainerRef = ref<HTMLElement | null>(null)

// Working temporary selection state
const tempYear = ref(2026)
const tempMonth = ref(7) // 0-indexed (0=Jan, 7=Aug)
const tempDay = ref(25)
const tempHour = ref(12)
const tempMinute = ref(0)

// Calendar View navigation state
const viewYear = ref(2026)
const viewMonth = ref(7)

const MONTH_NAMES = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December'
]

const DAY_NAMES = ['Su', 'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa']

const pad = (n: number) => String(n).padStart(2, '0')

function parseDateString(val?: string) {
  if (!val || typeof val !== 'string') {
    const now = new Date()
    return {
      year: now.getFullYear(),
      month: now.getMonth(),
      day: now.getDate(),
      hour: now.getHours(),
      minute: now.getMinutes(),
      isValid: false
    }
  }

  // Regex matches YYYY-MM-DDTHH:mm or YYYY-MM-DD HH:mm
  const match = val.match(/^(\d{4})-(\d{1,2})-(\d{1,2})[T\s](\d{1,2}):(\d{1,2})/)
  if (match) {
    const y = parseInt(match[1], 10)
    const m = parseInt(match[2], 10) - 1
    const d = parseInt(match[3], 10)
    const hh = parseInt(match[4], 10)
    const mm = parseInt(match[5], 10)
    return { year: y, month: m, day: d, hour: hh, minute: mm, isValid: true }
  }

  // Fallback to Date object parsing
  const dObj = new Date(val)
  if (!isNaN(dObj.getTime())) {
    return {
      year: dObj.getFullYear(),
      month: dObj.getMonth(),
      day: dObj.getDate(),
      hour: dObj.getHours(),
      minute: dObj.getMinutes(),
      isValid: true
    }
  }

  const fallback = new Date()
  return {
    year: fallback.getFullYear(),
    month: fallback.getMonth(),
    day: fallback.getDate(),
    hour: fallback.getHours(),
    minute: fallback.getMinutes(),
    isValid: false
  }
}

function syncFromProps() {
  const parsed = parseDateString(props.modelValue)
  tempYear.value = parsed.year
  tempMonth.value = parsed.month
  tempDay.value = parsed.day
  tempHour.value = parsed.hour
  tempMinute.value = parsed.minute

  viewYear.value = parsed.year
  viewMonth.value = parsed.month
}

watch(() => props.modelValue, () => {
  if (!isOpen.value) {
    syncFromProps()
  }
}, { immediate: true })

// Display Text in input box
const displayValue = computed(() => {
  if (!props.modelValue) return ''
  const p = parseDateString(props.modelValue)
  if (!p.isValid && !props.modelValue.trim()) return ''
  return `${p.year}-${pad(p.month + 1)}-${pad(p.day)} ${pad(p.hour)}:${pad(p.minute)}`
})

const tempFormattedValue = computed(() => {
  return `${tempYear.value}-${pad(tempMonth.value + 1)}-${pad(tempDay.value)}T${pad(tempHour.value)}:${pad(tempMinute.value)}`
})

// Calendar Grid Generation (42 cells: 6 rows x 7 cols)
interface CalendarDayCell {
  year: number
  month: number
  day: number
  isCurrentMonth: boolean
  isPrevMonth: boolean
  isNextMonth: boolean
  isSelected: boolean
  isToday: boolean
}

const calendarDays = computed<CalendarDayCell[]>(() => {
  const now = new Date()
  const todayY = now.getFullYear()
  const todayM = now.getMonth()
  const todayD = now.getDate()

  const y = viewYear.value
  const m = viewMonth.value

  const firstDayOfWeek = new Date(y, m, 1).getDay() // 0 = Sun
  const daysInCurrentMonth = new Date(y, m + 1, 0).getDate()
  const daysInPrevMonth = new Date(y, m, 0).getDate()

  const cells: CalendarDayCell[] = []

  // 1. Previous month trailing days
  for (let i = firstDayOfWeek - 1; i >= 0; i--) {
    const prevMonthIdx = m === 0 ? 11 : m - 1
    const prevYearVal = m === 0 ? y - 1 : y
    const dayNum = daysInPrevMonth - i
    const isSelected = tempYear.value === prevYearVal && tempMonth.value === prevMonthIdx && tempDay.value === dayNum
    const isToday = todayY === prevYearVal && todayM === prevMonthIdx && todayD === dayNum

    cells.push({
      year: prevYearVal,
      month: prevMonthIdx,
      day: dayNum,
      isCurrentMonth: false,
      isPrevMonth: true,
      isNextMonth: false,
      isSelected,
      isToday
    })
  }

  // 2. Current month days
  for (let d = 1; d <= daysInCurrentMonth; d++) {
    const isSelected = tempYear.value === y && tempMonth.value === m && tempDay.value === d
    const isToday = todayY === y && todayM === m && todayD === d

    cells.push({
      year: y,
      month: m,
      day: d,
      isCurrentMonth: true,
      isPrevMonth: false,
      isNextMonth: false,
      isSelected,
      isToday
    })
  }

  // 3. Next month leading days (fill up to 42 cells)
  const remaining = 42 - cells.length
  for (let d = 1; d <= remaining; d++) {
    const nextMonthIdx = m === 11 ? 0 : m + 1
    const nextYearVal = m === 11 ? y + 1 : y
    const isSelected = tempYear.value === nextYearVal && tempMonth.value === nextMonthIdx && tempDay.value === d
    const isToday = todayY === nextYearVal && todayM === nextMonthIdx && todayD === d

    cells.push({
      year: nextYearVal,
      month: nextMonthIdx,
      day: d,
      isCurrentMonth: false,
      isPrevMonth: false,
      isNextMonth: true,
      isSelected,
      isToday
    })
  }

  return cells
})

// Navigation Handlers
function prevMonth() {
  if (viewMonth.value === 0) {
    viewMonth.value = 11
    viewYear.value -= 1
  } else {
    viewMonth.value -= 1
  }
}

function nextMonth() {
  if (viewMonth.value === 11) {
    viewMonth.value = 0
    viewYear.value += 1
  } else {
    viewMonth.value += 1
  }
}

function prevYear() {
  viewYear.value -= 1
}

function nextYear() {
  viewYear.value += 1
}

function selectDayCell(cell: CalendarDayCell) {
  tempYear.value = cell.year
  tempMonth.value = cell.month
  tempDay.value = cell.day

  if (!cell.isCurrentMonth) {
    viewYear.value = cell.year
    viewMonth.value = cell.month
  }
}

function setTimeHours(h: number) {
  tempHour.value = Math.max(0, Math.min(23, h))
}

function setTimeMinutes(m: number) {
  tempMinute.value = Math.max(0, Math.min(59, m))
}

// Quick Presets
function applyDatePreset(preset: 'today' | 'yesterday' | 'minus7d') {
  const target = new Date()
  if (preset === 'yesterday') {
    target.setDate(target.getDate() - 1)
  } else if (preset === 'minus7d') {
    target.setDate(target.getDate() - 7)
  }

  tempYear.value = target.getFullYear()
  tempMonth.value = target.getMonth()
  tempDay.value = target.getDate()

  viewYear.value = tempYear.value
  viewMonth.value = tempMonth.value
}

function applyQuickTime(mode: 'now' | '00:00' | '12:00' | 'minus15m' | 'minus1h') {
  if (mode === 'now') {
    const now = new Date()
    tempHour.value = now.getHours()
    tempMinute.value = now.getMinutes()
  } else if (mode === '00:00') {
    tempHour.value = 0
    tempMinute.value = 0
  } else if (mode === '12:00') {
    tempHour.value = 12
    tempMinute.value = 0
  } else if (mode === 'minus15m') {
    let m = tempMinute.value - 15
    let h = tempHour.value
    if (m < 0) {
      m += 60
      h = h > 0 ? h - 1 : 23
    }
    tempMinute.value = m
    tempHour.value = h
  } else if (mode === 'minus1h') {
    tempHour.value = tempHour.value > 0 ? tempHour.value - 1 : 23
  }
}

function setNowFull() {
  const now = new Date()
  tempYear.value = now.getFullYear()
  tempMonth.value = now.getMonth()
  tempDay.value = now.getDate()
  tempHour.value = now.getHours()
  tempMinute.value = now.getMinutes()

  viewYear.value = tempYear.value
  viewMonth.value = tempMonth.value
}

// Popup Open / Close / Apply
function togglePicker() {
  if (props.disabled) return
  if (!isOpen.value) {
    syncFromProps()
    isOpen.value = true
  } else {
    closePicker()
  }
}

function closePicker() {
  isOpen.value = false
}

function handleCancel() {
  syncFromProps()
  closePicker()
}

function handleApply() {
  const result = tempFormattedValue.value
  emit('update:modelValue', result)
  emit('change', result)
  emit('apply', result)
  closePicker()
}

// Click outside handling
function handleClickOutside(event: MouseEvent) {
  if (pickerContainerRef.value && !pickerContainerRef.value.contains(event.target as Node)) {
    if (isOpen.value) {
      handleCancel()
    }
  }
}

function handleKeydown(event: KeyboardEvent) {
  if (!isOpen.value) return
  if (event.key === 'Escape') {
    handleCancel()
  } else if (event.key === 'Enter') {
    handleApply()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside, true)
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside, true)
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div class="cyber-datetime-wrapper" :class="{ 'is-open': isOpen, 'is-disabled': disabled }" ref="pickerContainerRef">
    <!-- Trigger Button / Input Display -->
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

    <!-- Dark Glassmorphism Popover Modal -->
    <transition name="cyber-popover">
      <div v-if="isOpen" class="cyber-picker-popup" role="dialog" aria-modal="true">
        <!-- 1. Top Header Presets & Month Navigation -->
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

        <!-- 2. Main Two-Column Body: Left Calendar, Right Time Selector -->
        <div class="popup-main-body">
          <!-- Column 1: Calendar Grid -->
          <div class="calendar-column">
            <!-- Day of Week Header -->
            <div class="calendar-week-header">
              <span v-for="d in DAY_NAMES" :key="d" class="week-name">{{ d }}</span>
            </div>

            <!-- 6x7 Matrix Grid -->
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

          <!-- Divider -->
          <div class="column-divider"></div>

          <!-- Column 2: Time Selector (24H) -->
          <div class="time-column">
            <!-- Active Time Display -->
            <div class="time-header-box">
              <span class="time-title font-mono">⏱️ TIME (24H)</span>
              <div class="time-digital-display font-mono">
                <span class="time-digit">{{ pad(tempHour) }}</span>
                <span class="time-colon">:</span>
                <span class="time-digit">{{ pad(tempMinute) }}</span>
              </div>
            </div>

            <!-- Quick Time Actions -->
            <div class="time-quick-chips">
              <button type="button" class="btn-time-chip" @click.stop="applyQuickTime('now')">Now</button>
              <button type="button" class="btn-time-chip" @click.stop="applyQuickTime('00:00')">00:00</button>
              <button type="button" class="btn-time-chip" @click.stop="applyQuickTime('12:00')">12:00</button>
              <button type="button" class="btn-time-chip" @click.stop="applyQuickTime('minus15m')">-15m</button>
              <button type="button" class="btn-time-chip" @click.stop="applyQuickTime('minus1h')">-1h</button>
            </div>

            <!-- Hour Selector Grid / List -->
            <div class="time-pickers-split">
              <!-- Hours Section -->
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

              <!-- Minutes Section -->
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
                <!-- Custom Minute Stepper Slider/Input -->
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

        <!-- 3. Bottom Action Bar with OK / Apply Button -->
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
.cyber-datetime-wrapper {
  position: relative;
  display: inline-block;
  user-select: none;
}

.cyber-datetime-wrapper.is-open {
  z-index: 1000;
}

/* Trigger Box */
.cyber-trigger-box {
  display: inline-flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-width: 165px;
  background: rgba(0, 0, 0, 0.45);
  border: 1px solid rgba(56, 189, 248, 0.28);
  color: #38bdf8;
  font-family: monospace;
  font-size: 11px;
  border-radius: 6px;
  padding: 4px 9px;
  cursor: pointer;
  outline: none;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.5);
}

.cyber-trigger-box:hover {
  background: rgba(15, 23, 42, 0.7);
  border-color: #38bdf8;
  box-shadow: 0 0 10px rgba(56, 189, 248, 0.25), inset 0 1px 3px rgba(0, 0, 0, 0.4);
}

.cyber-trigger-box.is-open {
  border-color: #38bdf8;
  background: rgba(15, 23, 42, 0.85);
  box-shadow: 0 0 12px rgba(56, 189, 248, 0.35);
}

.cyber-trigger-box.is-disabled {
  opacity: 0.5;
  cursor: not-allowed;
  pointer-events: none;
}

.trigger-left {
  display: flex;
  align-items: center;
  gap: 6px;
}

.cyber-cal-icon {
  font-size: 11px;
  filter: drop-shadow(0 0 4px rgba(56, 189, 248, 0.5));
}

.trigger-value {
  color: #38bdf8;
  font-weight: 600;
  letter-spacing: 0.03em;
}

.trigger-placeholder {
  color: #64748b;
}

.trigger-right {
  display: flex;
  align-items: center;
}

.cyber-arrow-icon {
  font-size: 8px;
  color: #94a3b8;
  transition: transform 0.2s ease;
}

/* Dark Glassmorphism Popover Modal */
.cyber-picker-popup {
  position: absolute;
  top: calc(100% + 5px);
  left: 0;
  z-index: 99999;
  width: 395px;
  max-width: 94vw;
  background: rgba(10, 16, 30, 0.97);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border: 1px solid rgba(56, 189, 248, 0.35);
  border-radius: 10px;
  box-shadow: 0 16px 36px rgba(0, 0, 0, 0.75), 0 0 20px rgba(56, 189, 248, 0.15);
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* 1. Header Navigation */
.popup-top-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  padding-bottom: 6px;
  border-bottom: 1px solid rgba(56, 189, 248, 0.15);
  flex-wrap: wrap;
}

.month-nav-controls {
  display: flex;
  align-items: center;
  gap: 3px;
}

.btn-nav-arrow {
  background: rgba(0, 0, 0, 0.4);
  border: 1px solid rgba(56, 189, 248, 0.2);
  color: #94a3b8;
  padding: 2px 5px;
  border-radius: 4px;
  font-size: 10px;
  cursor: pointer;
  transition: all 0.15s ease;
  line-height: 1;
}

.btn-nav-arrow:hover {
  background: rgba(56, 189, 248, 0.2);
  border-color: #38bdf8;
  color: #ffffff;
  box-shadow: 0 0 6px rgba(56, 189, 248, 0.4);
}

.current-month-year-badge {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 2px 7px;
  background: rgba(56, 189, 248, 0.1);
  border: 1px solid rgba(56, 189, 248, 0.25);
  border-radius: 4px;
}

.badge-month {
  font-size: 11px;
  font-weight: 700;
  color: #e2e8f0;
}

.badge-year {
  font-size: 11px;
  font-weight: 700;
  color: #38bdf8;
}

.quick-date-chips {
  display: flex;
  align-items: center;
  gap: 3px;
}

.btn-date-chip {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.12);
  color: #cbd5e1;
  font-size: 9.5px;
  font-family: monospace;
  padding: 2px 5px;
  border-radius: 3px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-date-chip:hover {
  background: rgba(56, 189, 248, 0.2);
  border-color: #38bdf8;
  color: #38bdf8;
}

/* 2. Main Two Column Body */
.popup-main-body {
  display: flex;
  gap: 8px;
  align-items: stretch;
}

/* Calendar Column */
.calendar-column {
  flex: 1.15;
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.calendar-week-header {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 2px;
  text-align: center;
}

.week-name {
  font-size: 10px;
  font-weight: 700;
  color: #818cf8;
  padding: 1px 0;
}

.calendar-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 2px;
}

.calendar-cell {
  position: relative;
  aspect-ratio: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(15, 23, 42, 0.6);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 4px;
  color: #f1f5f9;
  font-size: 10.5px;
  cursor: pointer;
  outline: none;
  transition: all 0.12s ease;
  padding: 0;
  min-height: 24px;
}

.calendar-cell:hover {
  background: rgba(56, 189, 248, 0.2);
  border-color: rgba(56, 189, 248, 0.4);
  color: #ffffff;
  transform: translateY(-1px);
}

.calendar-cell.other-month {
  opacity: 0.3;
  color: #64748b;
}

.calendar-cell.other-month:hover {
  opacity: 0.75;
}

.calendar-cell.is-today {
  box-shadow: inset 0 0 0 1px #38bdf8;
  font-weight: 700;
}

.calendar-cell.is-selected {
  background: linear-gradient(135deg, #38bdf8, #818cf8) !important;
  border-color: #38bdf8 !important;
  color: #0f172a !important;
  font-weight: 800;
  box-shadow: 0 0 10px rgba(56, 189, 248, 0.5);
}

.today-indicator-dot {
  position: absolute;
  bottom: 2px;
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: #38bdf8;
  box-shadow: 0 0 4px #38bdf8;
}

/* Divider */
.column-divider {
  width: 1px;
  background: linear-gradient(180deg, rgba(56, 189, 248, 0.1), rgba(56, 189, 248, 0.3), rgba(56, 189, 248, 0.1));
}

/* Time Column */
.time-column {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.time-header-box {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 3px 6px;
  background: rgba(0, 0, 0, 0.4);
  border: 1px solid rgba(56, 189, 248, 0.2);
  border-radius: 5px;
}

.time-title {
  font-size: 9.5px;
  font-weight: 700;
  color: #94a3b8;
}

.time-digital-display {
  font-size: 13px;
  font-weight: 800;
  color: #38bdf8;
  text-shadow: 0 0 6px rgba(56, 189, 248, 0.6);
  letter-spacing: 0.5px;
}

.time-quick-chips {
  display: flex;
  gap: 2px;
  flex-wrap: wrap;
}

.btn-time-chip {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #94a3b8;
  font-size: 9px;
  font-family: monospace;
  padding: 1px 4px;
  border-radius: 3px;
  cursor: pointer;
  transition: all 0.12s ease;
}

.btn-time-chip:hover {
  background: rgba(129, 140, 248, 0.2);
  border-color: #818cf8;
  color: #c7d2fe;
}

.time-pickers-split {
  display: flex;
  gap: 6px;
  flex: 1;
}

.time-sub-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.sub-section-title {
  font-size: 9px;
  font-weight: 700;
  color: #64748b;
  font-family: monospace;
}

.hours-scroll-grid,
.minutes-scroll-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 2px;
  max-height: 120px;
  overflow-y: auto;
  padding-right: 1px;
}

/* Custom mini scrollbars */
.hours-scroll-grid::-webkit-scrollbar,
.minutes-scroll-grid::-webkit-scrollbar {
  width: 3px;
}

.hours-scroll-grid::-webkit-scrollbar-thumb,
.minutes-scroll-grid::-webkit-scrollbar-thumb {
  background: rgba(56, 189, 248, 0.3);
  border-radius: 2px;
}

.btn-time-unit {
  background: rgba(15, 23, 42, 0.5);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 3px;
  color: #cbd5e1;
  font-size: 9.5px;
  padding: 2px 0;
  text-align: center;
  cursor: pointer;
  outline: none;
  transition: all 0.12s ease;
  line-height: 1.2;
}

.btn-time-unit:hover {
  background: rgba(56, 189, 248, 0.2);
  border-color: rgba(56, 189, 248, 0.4);
  color: #ffffff;
}

.btn-time-unit.is-selected {
  background: #38bdf8 !important;
  border-color: #38bdf8 !important;
  color: #0f172a !important;
  font-weight: 800;
  box-shadow: 0 0 6px rgba(56, 189, 248, 0.6);
}

.exact-minute-stepper {
  display: flex;
  align-items: center;
  gap: 3px;
  margin-top: 3px;
  font-size: 9px;
  color: #94a3b8;
}

.exact-min-input {
  width: 34px;
  height: 18px;
  background: rgba(0, 0, 0, 0.5);
  border: 1px solid rgba(56, 189, 248, 0.3);
  border-radius: 3px;
  color: #38bdf8;
  font-size: 9.5px;
  padding: 0 2px;
  outline: none;
  text-align: center;
}

.exact-min-input:focus {
  border-color: #38bdf8;
  box-shadow: 0 0 5px rgba(56, 189, 248, 0.3);
}

/* 3. Footer Bar with OK / Apply */
.popup-footer-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  padding-top: 6px;
  border-top: 1px solid rgba(56, 189, 248, 0.15);
}

.footer-left {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.btn-picker-quicknow {
  background: rgba(56, 189, 248, 0.1);
  border: 1px solid rgba(56, 189, 248, 0.3);
  color: #38bdf8;
  font-size: 9.5px;
  font-weight: 600;
  padding: 3px 6px;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.12s ease;
  white-space: nowrap;
}

.btn-picker-quicknow:hover {
  background: rgba(56, 189, 248, 0.25);
  box-shadow: 0 0 6px rgba(56, 189, 248, 0.3);
  color: #ffffff;
}

.preview-tag {
  font-size: 9.5px;
  color: #64748b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.footer-right {
  display: flex;
  align-items: center;
  gap: 4px;
}

.btn-picker-cancel {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.15);
  color: #94a3b8;
  font-size: 10px;
  font-weight: 600;
  padding: 3px 8px;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.12s ease;
}

.btn-picker-cancel:hover {
  background: rgba(244, 63, 94, 0.15);
  border-color: rgba(244, 63, 94, 0.4);
  color: #fda4af;
}

.btn-picker-ok {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: linear-gradient(135deg, #10b981, #059669);
  border: 1px solid #34d399;
  color: #ffffff;
  font-size: 10px;
  font-weight: 700;
  padding: 3px 10px;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.15s ease;
  box-shadow: 0 0 8px rgba(16, 185, 129, 0.4);
}

.btn-picker-ok:hover {
  background: linear-gradient(135deg, #34d399, #10b981);
  box-shadow: 0 0 12px rgba(52, 211, 153, 0.6);
  transform: translateY(-1px);
}

.glow-check {
  font-weight: 800;
}

/* Animations */
.cyber-popover-enter-active,
.cyber-popover-leave-active {
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.cyber-popover-enter-from,
.cyber-popover-leave-to {
  opacity: 0;
  transform: translateY(-6px) scale(0.98);
}
</style>
