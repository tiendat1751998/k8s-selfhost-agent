import type { LogEntry } from '../../stores/logStore'
import type { StackTraceInfo } from '../../utils/stackTraceParser'

export interface VisibleLogItem {
  log: LogEntry
  actualIndex: number
  stackInfo: StackTraceInfo
  isExpanded: boolean
}

export function formatTime(t: string): string {
  if (!t) return ''
  if (t.includes('T')) {
    const p = t.split('T')[1]
    return p ? p.slice(0, 8) : t
  }
  if (t.length > 8 && t[2] === ':' && t[5] === ':') {
    return t.slice(0, 8)
  }
  return t
}

export function getTargetBadge(log: LogEntry, currentTarget?: string): string {
  // If target name or active node matches current log, compact down to service/pod
  if (currentTarget && log.node && (currentTarget === log.node || currentTarget.includes(log.node))) {
    if (log.service) return log.service
    if (log.pod) return (log.namespace && log.namespace !== 'default') ? `${log.namespace}/${log.pod}` : log.pod
  }
  if (log.node && log.service && log.node !== log.service) return `${log.node}/${log.service}`
  if (log.node && log.pod && log.node !== log.pod) return `${log.node}/${log.pod}`
  if (log.service) return log.service
  if (log.node) return log.node
  if (log.pod) return (log.namespace && log.namespace !== 'default') ? `${log.namespace}/${log.pod}` : log.pod
  return log.namespace || 'system'
}

/**
 * Strips leading wildcard/concrete timestamp noise, UTC labels, and bracketed worker/PID tags
 * from error fingerprint patterns so titles read cleanly in Error Radar.
 */
export function cleanFingerprint(fp?: string): string {
  if (!fp) return ''
  // Strip leading timestamps like "*-*-* *:*:*.*", "2026-10-23 04:12:34.123", "2026-10-23T11:20:00.000Z"
  // and trailing "UTC", "GMT", and "[*]", "[12345]", "[worker-1]"
  const cleaned = fp
    .replace(/^(\*|[\d\-:T.Z\s]+(UTC|GMT)?)+(\[[^\]]*\])?\s*/i, '')
    .replace(/^(\*[\s\-:*.]*)+/g, '')
    .replace(/^\[[^\]]*\]\s*/, '')
    .trim()
  return cleaned || fp
}

const JSON_IGNORED_KEYS = new Set([
  'level', 'time', 'timestamp', 'ts', 'severity',
  'msg', 'message', 'error', 'log', 'caller', 'logger',
])

/**
 * Cleans single-line log message preview by extracting primary text from raw JSON
 * or stripping redundant timestamps and severity prefixes from database logs.
 */
export function cleanLogMessage(rawMsg?: string): string {
  if (!rawMsg) return ''
  const trimmed = rawMsg.trim()
  if (!trimmed) return ''

  // 1. JSON-encoded structured log detection
  if (trimmed.startsWith('{') && trimmed.endsWith('}')) {
    try {
      const obj = JSON.parse(trimmed)
      if (typeof obj === 'object' && obj !== null && !Array.isArray(obj)) {
        const primary = obj.msg || obj.message || obj.error || obj.log
        if (typeof primary === 'string' && primary.trim()) {
          const extras: string[] = []
          for (const [k, v] of Object.entries(obj)) {
            if (!JSON_IGNORED_KEYS.has(k.toLowerCase()) && v !== undefined && v !== null && v !== '') {
              extras.push(`${k}=${typeof v === 'object' ? JSON.stringify(v) : v}`)
            }
          }
          return extras.length > 0 ? `${primary.trim()} ${extras.join(' ')}` : primary.trim()
        }
      }
    } catch {
      // Fall through to regex cleans
    }
  }

  // 2. Postgres / UTC timestamp + PID + Level prefix clean
  const pgMatch = trimmed.match(/^\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?\s+(?:UTC|GMT)\s+(?:(\[\d+\])\s+)?(?:ERROR|LOG|WARNING|FATAL|INFO|DEBUG|NOTICE):\s*(.*)$/i)
  if (pgMatch) {
    const pid = pgMatch[1] ? `${pgMatch[1]} ` : ''
    const msg = pgMatch[2] ? pgMatch[2].trim() : ''
    return `${pid}${msg}`
  }

  // 3. Leading UTC/GMT timestamp without level prefix
  const tsOnlyMatch = trimmed.match(/^\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?\s+(?:UTC|GMT)\s+(.*)$/i)
  if (tsOnlyMatch && tsOnlyMatch[1]) {
    return tsOnlyMatch[1].trim()
  }

  return trimmed
}


