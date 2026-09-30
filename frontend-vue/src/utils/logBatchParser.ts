import type { LogEntry } from '../stores/logStore'

export type WebSocketParsedResult =
  | { type: 'logs'; entries: LogEntry[] }
  | { type: 'telemetry'; droppedCount: number; streamRate: number }
  | { type: 'ignore' }

export function mapRawToLogEntry(raw: any, defaultNamespace = 'default', defaultService?: string): LogEntry | null {
  if (!raw) return null
  const msg = raw.message || raw.msg || raw.log || (typeof raw === 'string' ? raw : JSON.stringify(raw))
  if (!msg || msg === '-- No entries --' || (typeof msg === 'string' && msg.trim() === '-- No entries --')) {
    return null
  }

  const time = raw.timestamp || raw.time || new Date().toISOString()
  const level = (raw.log_level || raw.level || raw.severity || 'INFO').toUpperCase()
  const namespace = raw.namespace || raw.ns || defaultNamespace
  const container = raw.container_name || raw.container
  const pod = raw.pod_name || raw.pod || container || raw.service || 'system'
  const node = raw.node || raw.host || raw.attributes?.node || raw.attributes?.node_name
  const service = raw.service || raw.app || raw.attributes?.service || raw.attributes?.app || container || defaultService
  const traceId = raw.traceId || raw.trace_id || raw.attributes?.trace_id || raw.attributes?.traceId
  const stream = raw.stream || 'stdout'

  return {
    time,
    level,
    namespace,
    pod,
    container,
    node,
    service,
    msg: typeof msg === 'string' ? msg : String(msg),
    traceId,
    stream,
    attributes: raw.attributes,
  }
}

export function parseWebSocketFrame(data: string | unknown, defaultNamespace = 'default', defaultService?: string): WebSocketParsedResult {
  if (typeof data !== 'string') {
    if (Array.isArray(data)) {
      const entries: LogEntry[] = []
      for (const item of data) {
        const mapped = mapRawToLogEntry(item, defaultNamespace, defaultService)
        if (mapped) entries.push(mapped)
      }
      return { type: 'logs', entries }
    }
    if (data && typeof data === 'object') {
      const obj = data as Record<string, any>
      if (obj.type === 'stream_telemetry' || typeof obj.dropped_count === 'number' || typeof obj.droppedCount === 'number') {
        return {
          type: 'telemetry',
          droppedCount: Number(obj.dropped_count ?? obj.droppedCount ?? 0),
          streamRate: Number(obj.stream_rate ?? obj.streamRate ?? 0),
        }
      }
      const entry = mapRawToLogEntry(obj, defaultNamespace, defaultService)
      return entry ? { type: 'logs', entries: [entry] } : { type: 'ignore' }
    }
    return { type: 'ignore' }
  }

  const trimmed = data.trim()
  if (!trimmed || trimmed === '-- No entries --') {
    return { type: 'ignore' }
  }

  try {
    const parsed = JSON.parse(trimmed)
    if (Array.isArray(parsed)) {
      const entries: LogEntry[] = []
      for (const item of parsed) {
        const mapped = mapRawToLogEntry(item, defaultNamespace, defaultService)
        if (mapped) entries.push(mapped)
      }
      return { type: 'logs', entries }
    }

    if (parsed && typeof parsed === 'object') {
      if (parsed.type === 'stream_telemetry' || typeof parsed.dropped_count === 'number' || typeof parsed.droppedCount === 'number') {
        return {
          type: 'telemetry',
          droppedCount: Number(parsed.dropped_count ?? parsed.droppedCount ?? 0),
          streamRate: Number(parsed.stream_rate ?? parsed.streamRate ?? 0),
        }
      }
      const mapped = mapRawToLogEntry(parsed, defaultNamespace, defaultService)
      return mapped ? { type: 'logs', entries: [mapped] } : { type: 'ignore' }
    }
  } catch {
    // Non-JSON plain text string
    const entry: LogEntry = {
      time: new Date().toISOString(),
      level: 'INFO',
      namespace: defaultNamespace,
      pod: 'system',
      service: defaultService,
      msg: trimmed,
      stream: 'stdout',
    }
    return { type: 'logs', entries: [entry] }
  }

  return { type: 'ignore' }
}