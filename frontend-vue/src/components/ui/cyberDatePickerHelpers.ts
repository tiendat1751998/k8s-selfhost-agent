import { ref, computed, watch, onMounted, onUnmounted } from 'vue'

export interface ParsedDate {
  year: number
  month: number
  day: number
  hour: number
  minute: number
  isValid: boolean
}

export interface CalendarDayCell {
  year: number
  month: number
  day: number
  isCurrentMonth: boolean
  isPrevMonth: boolean
  isNextMonth: boolean
  isSelected: boolean
  isToday: boolean
}

export const MONTH_NAMES = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December'
]

export const DAY_NAMES = ['Su', 'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa']

export const pad = (n: number) => String(n).padStart(2, '0')

export function parseDateString(val?: string): ParsedDate {
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

  const match = val.match(/^(\d{4})-(\d{1,2})-(\d{1,2})[T\s](\d{1,2}):(\d{1,2})/)
  if (match) {
    const y = parseInt(match[1], 10)
    const m = parseInt(match[2], 10) - 1
    const d = parseInt(match[3], 10)
    const hh = parseInt(match[4], 10)
    const mm = parseInt(match[5], 10)
    return { year: y, month: m, day: d, hour: hh, minute: mm, isValid: true }
  }

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

export function buildCalendarDays(
  viewYear: number,
  viewMonth: number,
  tempYear: number,
  tempMonth: number,
  tempDay: number
): CalendarDayCell[] {
  const now = new Date()
  const todayY = now.getFullYear()
  const todayM = now.getMonth()
  const todayD = now.getDate()

  const y = viewYear
  const m = viewMonth

  const firstDayOfWeek = new Date(y, m, 1).getDay()
  const daysInCurrentMonth = new Date(y, m + 1, 0).getDate()
  const daysInPrevMonth = new Date(y, m, 0).getDate()

  const cells: CalendarDayCell[] = []

  for (let i = firstDayOfWeek - 1; i >= 0; i--) {
    const prevMonthIdx = m === 0 ? 11 : m - 1
    const prevYearVal = m === 0 ? y - 1 : y
    const dayNum = daysInPrevMonth - i
    const isSelected = tempYear === prevYearVal && tempMonth === prevMonthIdx && tempDay === dayNum
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

  for (let d = 1; d <= daysInCurrentMonth; d++) {
    const isSelected = tempYear === y && tempMonth === m && tempDay === d
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

  const remaining = 42 - cells.length
  for (let d = 1; d <= remaining; d++) {
    const nextMonthIdx = m === 11 ? 0 : m + 1
    const nextYearVal = m === 11 ? y + 1 : y
    const isSelected = tempYear === nextYearVal && tempMonth === nextMonthIdx && tempDay === d
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
}

export function useCyberDateTimePicker(
  props: { modelValue?: string; disabled?: boolean },
  emit: (e: 'update:modelValue' | 'change' | 'apply', val: string) => void
) {
  const isOpen = ref(false)
  const pickerContainerRef = ref<HTMLElement | null>(null)

  const tempYear = ref(2026)
  const tempMonth = ref(7)
  const tempDay = ref(25)
  const tempHour = ref(12)
  const tempMinute = ref(0)

  const viewYear = ref(2026)
  const viewMonth = ref(7)

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
    if (!isOpen.value) syncFromProps()
  }, { immediate: true })

  const displayValue = computed(() => {
    if (!props.modelValue) return ''
    const p = parseDateString(props.modelValue)
    if (!p.isValid && !props.modelValue.trim()) return ''
    return `${p.year}-${pad(p.month + 1)}-${pad(p.day)} ${pad(p.hour)}:${pad(p.minute)}`
  })

  const tempFormattedValue = computed(() => {
    return `${tempYear.value}-${pad(tempMonth.value + 1)}-${pad(tempDay.value)}T${pad(tempHour.value)}:${pad(tempMinute.value)}`
  })

  const calendarDays = computed(() =>
    buildCalendarDays(viewYear.value, viewMonth.value, tempYear.value, tempMonth.value, tempDay.value)
  )

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

  function applyDatePreset(preset: 'today' | 'yesterday' | 'minus7d') {
    const target = new Date()
    if (preset === 'yesterday') target.setDate(target.getDate() - 1)
    else if (preset === 'minus7d') target.setDate(target.getDate() - 7)
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

  function handleClickOutside(event: MouseEvent) {
    if (pickerContainerRef.value && !pickerContainerRef.value.contains(event.target as Node)) {
      if (isOpen.value) handleCancel()
    }
  }

  function handleKeydown(event: KeyboardEvent) {
    if (!isOpen.value) return
    if (event.key === 'Escape') handleCancel()
    else if (event.key === 'Enter') handleApply()
  }

  onMounted(() => {
    document.addEventListener('click', handleClickOutside, true)
    document.addEventListener('keydown', handleKeydown)
  })

  onUnmounted(() => {
    document.removeEventListener('click', handleClickOutside, true)
    document.removeEventListener('keydown', handleKeydown)
  })

  return {
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
    tempFormattedValue,
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
    closePicker,
    handleCancel,
    handleApply
  }
}
