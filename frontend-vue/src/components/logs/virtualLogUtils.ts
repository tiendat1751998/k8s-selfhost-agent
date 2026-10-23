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

export interface CoalescedLogEntry {
  log: LogEntry
  cleanMsg: string
  fullMsg: string
  repeatCount: number
  mergedLinesCount: number
}

const JSON_IGNORED_KEYS = new Set([
  'level', 'time', 'timestamp', 'ts', 'severity',
  'msg', 'message', 'error', 'err', 'log', 'caller', 'logger',
])

const SQL_CONTINUATION_REGEX = /^(?:CONTEXT:|DETAIL:|HINT:|STATEMENT:|QUERY:|SELECT\b|FROM\b|WHERE\b|JOIN\b|LEFT\b|INNER\b|GROUP BY\b|ORDER BY\b|LIMIT\b|VALUES\b|SET\b|COALESCE\(|created_at\b|updated_at\b|AND\b|OR\b|\)|;)/i

/**
 * Cleans single-line log message preview by extracting primary text from raw JSON
 * or stripping redundant timestamps and severity prefixes from database logs.
 */
export function cleanLogMessage(rawMsg?: string): string {
  if (!rawMsg) return ''
  const trimmed = rawMsg.trim()
  if (!trimmed) return ''

  // Filter out empty placeholders
  if (trimmed === '-- No entries --') return ''

  // Filter out redundant tag-only messages like "[db] [INFO]"
  if (/^\[[^\]]+\]\s*\[(?:INFO|WARN|WARNING|ERR|ERROR|DEBUG)\]$/i.test(trimmed)) {
    return ''
  }
  if (/^(?:\[[a-zA-Z0-9_.\-\s:]+\]\s*)+$/.test(trimmed)) {
    return ''
  }

  // 1. JSON-encoded structured log detection
  if (trimmed.startsWith('{') && trimmed.endsWith('}')) {
    try {
      const obj = JSON.parse(trimmed)
      if (typeof obj === 'object' && obj !== null && !Array.isArray(obj)) {
        const msgField = typeof obj.msg === 'string' ? obj.msg : (typeof obj.message === 'string' ? obj.message : (typeof obj.log === 'string' ? obj.log : ''))
        const errField = typeof obj.error === 'string' ? obj.error : (typeof obj.err === 'string' ? obj.err : '')

        let primary = ''
        if (msgField && errField && !msgField.toLowerCase().includes(errField.toLowerCase())) {
          primary = `${msgField.trim()}: ${errField.trim()}`
        } else {
          primary = (msgField || errField || (typeof obj.msg === 'object' ? JSON.stringify(obj.msg) : '') || '').trim()
        }

        if (primary) {
          const extras: string[] = []
          for (const [k, v] of Object.entries(obj)) {
            if (!JSON_IGNORED_KEYS.has(k.toLowerCase()) && v !== undefined && v !== null && v !== '') {
              extras.push(`${k}=${typeof v === 'object' ? JSON.stringify(v) : v}`)
            }
          }
          return extras.length > 0 ? `${primary} ${extras.join(' ')}` : primary
        }
      }
    } catch {
      // Fall through to regex cleans
    }
  }

  // 2. Docker / container logfmt extraction (e.g. time="..." level=info msg="...")
  if (trimmed.includes('msg="') || (trimmed.includes('level=') && trimmed.includes('time='))) {
    const stripped = trimmed
      .replace(/(?:^|\s)(?:time|ts)="[^"]*"\s*/g, ' ')
      .replace(/(?:^|\s)level=(?:"[^"]*"|[a-zA-Z]+)\s*/gi, ' ')
      .trim()
    const logfmtMatch = stripped.match(/^msg="((?:[^"\\]|\\.)*)"(.*)$/)
    if (logfmtMatch) {
      const unescaped = logfmtMatch[1].replace(/\\"/g, '"').replace(/\\\\/g, '\\')
      const trailing = logfmtMatch[2]?.trim()
      return trailing ? `${unescaped} ${trailing}` : unescaped
    }
  }

  // 3. Postgres / UTC timestamp + PID + Level prefix clean
  const pgMatch = trimmed.match(/^\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?\s+(?:UTC|GMT)\s+(?:(\[\d+\])\s+)?(?:ERROR|LOG|WARNING|FATAL|INFO|DEBUG|NOTICE):\s*(.*)$/i)
  if (pgMatch) {
    const pid = pgMatch[1] ? `${pgMatch[1]} ` : ''
    const msg = pgMatch[2] ? pgMatch[2].trim() : ''
    return `${pid}${msg}`
  }

  // 4. Leading UTC/GMT timestamp without level prefix
  const tsOnlyMatch = trimmed.match(/^\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?\s+(?:UTC|GMT)\s+(.*)$/i)
  if (tsOnlyMatch && tsOnlyMatch[1]) {
    return tsOnlyMatch[1].trim()
  }

  return trimmed
}

/**
 * Coalesces consecutive identical logs (repeat counts) and merges multi-line SQL/Postgres continuation
 * statements into their parent log row, filtering out tag-only noise lines.
 */
export function coalesceLogEntries(logs: LogEntry[]): CoalescedLogEntry[] {
  const result: CoalescedLogEntry[] = []
  if (!logs || logs.length === 0) return result

  for (const log of logs) {
    const cleanMsg = cleanLogMessage(log.msg)
    if (!cleanMsg) continue

    const node = (log.node || '').toLowerCase()
    const service = (log.service || log.container || log.pod || '').toLowerCase()
    const level = (log.level || '').toLowerCase()

    if (result.length > 0) {
      const prev = result[result.length - 1]
      const prevNode = (prev.log.node || '').toLowerCase()
      const prevService = (prev.log.service || prev.log.container || prev.log.pod || '').toLowerCase()

      // Rule A — Multi-line Continuation Merging
      if (prevNode === node && prevService === service && SQL_CONTINUATION_REGEX.test(cleanMsg)) {
        prev.fullMsg += `\n${cleanMsg}`
        prev.mergedLinesCount++
        continue
      }

      // Rule B — Consecutive Duplicate Grouping (xN)
      const prevLevel = (prev.log.level || '').toLowerCase()
      if (prevNode === node && prevService === service && prevLevel === level && prev.cleanMsg === cleanMsg) {
        prev.repeatCount++
        prev.log = log // timestamp reflects latest occurrence
        if (prev.repeatCount <= 10) {
          prev.fullMsg += `\n${log.msg}`
        }
        continue
      }
    }

    result.push({
      log,
      cleanMsg,
      fullMsg: log.msg,
      repeatCount: 1,
      mergedLinesCount: 0,
    })
  }

  return result
}


