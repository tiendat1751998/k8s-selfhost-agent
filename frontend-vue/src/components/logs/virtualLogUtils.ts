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

