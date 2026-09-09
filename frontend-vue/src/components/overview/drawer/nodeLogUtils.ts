export interface ParsedLogLine {
  id: number
  raw: string
  cleanText: string
  level: 'error' | 'warn' | 'info' | 'debug'
  timestamp?: string
}

export const LINE_HEIGHT = 22
export const OVERSCAN = 15

export const ANSI_REGEX = /\u001b\[[0-9;]*m/g

export function stripAnsi(str: string): string {
  if (str.indexOf('\u001b') === -1 && str.indexOf('\x1b') === -1) {
    return str
  }
  return str.replace(ANSI_REGEX, '')
}

export function classifyLogLevel(cleanLower: string): 'error' | 'warn' | 'info' | 'debug' {
  if (
    cleanLower.includes('error') ||
    cleanLower.includes('fatal') ||
    cleanLower.includes('panic') ||
    cleanLower.includes('exception') ||
    cleanLower.includes('fail') ||
    cleanLower.includes('err ') ||
    cleanLower.includes('[err]')
  ) {
    return 'error'
  }
  if (cleanLower.includes('warn') || cleanLower.includes('timeout')) {
    return 'warn'
  }
  if (cleanLower.includes('debug') || cleanLower.includes('trace')) {
    return 'debug'
  }
  return 'info'
}

export function toIsoTime(val?: string): string | undefined {
  if (!val) return undefined
  try {
    const d = new Date(val)
    if (!isNaN(d.getTime())) return d.toISOString()
  } catch {}
  return val
}

export function formatBadgeDate(dt: string): string {
  if (!dt) return ''
  try {
    const d = new Date(dt)
    if (!isNaN(d.getTime())) {
      const pad = (n: number) => String(n).padStart(2, '0')
      return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
    }
  } catch {}
  return dt.replace('T', ' ')
}

export function formatDateTimeLocal(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  const yyyy = d.getFullYear()
  const MM = pad(d.getMonth() + 1)
  const dd = pad(d.getDate())
  const hh = pad(d.getHours())
  const mm = pad(d.getMinutes())
  return `${yyyy}-${MM}-${dd}T${hh}:${mm}`
}
