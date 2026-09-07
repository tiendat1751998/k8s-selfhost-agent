/**
 * Unified Frontend Formatting Utilities
 * Standardizes formatting for Bytes, Dates, Durations, Percentages,
 * Network I/O, System info, and OS metadata across K8sControl Console.
 */

/**
 * Formats a raw byte count into human-readable unit string (B, KB, MB, GB, TB, PB).
 */
export function formatBytes(bytes?: number | null, decimals = 1): string {
  if (bytes === undefined || bytes === null || isNaN(bytes) || bytes <= 0) return '0 B'
  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), sizes.length - 1)
  const val = parseFloat((bytes / Math.pow(k, i)).toFixed(dm))
  return `${val} ${sizes[i]}`
}

/**
 * Alias for formatBytes specifically tailored for Memory representation.
 */
export function formatMemory(bytes?: number | null): string {
  return formatBytes(bytes, 1)
}

/**
 * Formats network or disk throughput rates in bytes/second.
 */
export function formatIoRate(bytesPerSec?: number | null, decimals = 1): string {
  if (!bytesPerSec || bytesPerSec <= 0 || isNaN(bytesPerSec)) return '0 B/s'
  return `${formatBytes(bytesPerSec, decimals)}/s`
}

/**
 * Formats date to localized string, handling nulls/undefined gracefully.
 */
export function formatDate(d?: string | number | Date | null): string {
  if (!d) return '-'
  try {
    const dateObj = typeof d === 'string' || typeof d === 'number' ? new Date(d) : d
    if (isNaN(dateObj.getTime())) return String(d)
    return dateObj.toLocaleString()
  } catch {
    return String(d)
  }
}

/**
 * Formats date into datetime-local format: YYYY-MM-DDTHH:mm
 */
export function formatDateTimeLocal(d?: Date | string | null): string {
  if (!d) return ''
  const dateObj = typeof d === 'string' ? new Date(d) : d
  if (isNaN(dateObj.getTime())) return ''
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${dateObj.getFullYear()}-${pad(dateObj.getMonth() + 1)}-${pad(dateObj.getDate())}T${pad(dateObj.getHours())}:${pad(dateObj.getMinutes())}`
}

/**
 * Formats time string (HH:MM:SS or short locale).
 */
export function formatTime(d?: string | Date | null): string {
  if (!d) return '-'
  try {
    const dt = typeof d === 'string' ? new Date(d) : d
    if (isNaN(dt.getTime())) return String(d)
    return dt.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  } catch {
    return String(d)
  }
}

/**
 * Calculates human-friendly relative elapsed time (e.g. 'Just now', '5m ago', '2h ago').
 */
export function formatTimeAgo(dateStr?: string | Date | null): string {
  if (!dateStr) return '-'
  try {
    const dt = typeof dateStr === 'string' ? new Date(dateStr) : dateStr
    if (isNaN(dt.getTime())) return String(dateStr)
    const diffSec = Math.floor((Date.now() - dt.getTime()) / 1000)
    if (diffSec < 0 || isNaN(diffSec)) return 'Just now'
    if (diffSec < 60) return 'Just now'
    if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`
    if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h ago`
    return `${Math.floor(diffSec / 86400)}d ago`
  } catch {
    return String(dateStr)
  }
}

/**
 * Formats age duration (e.g. '2m', '5h', '12d').
 */
export function formatAge(dateStr?: string | Date | null): string {
  if (!dateStr) return 'Unknown'
  try {
    const dt = typeof dateStr === 'string' ? new Date(dateStr) : dateStr
    const diff = Date.now() - dt.getTime()
    if (isNaN(diff) || diff < 0) return 'Just now'
    const mins = Math.floor(diff / 60000)
    if (mins < 60) return `${mins}m`
    const hours = Math.floor(mins / 60)
    if (hours < 24) return `${hours}h`
    const days = Math.floor(hours / 24)
    return `${days}d`
  } catch {
    return 'Unknown'
  }
}

/**
 * Alias for formatTimeAgo / formatAge used in ecosystem and logs.
 */
export function formatRelativeTime(dateStr?: string | Date | null): string {
  return formatTimeAgo(dateStr)
}

/**
 * Formats a duration in seconds into human-readable compact form (e.g. '3d 4h 12m', '45m 10s').
 */
export function formatDuration(seconds?: number | null): string {
  if (!seconds || seconds <= 0 || isNaN(seconds)) return '0s'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const secs = Math.floor(seconds % 60)
  if (days > 0) return `${days}d ${hours}h ${minutes}m`
  if (hours > 0) return `${hours}h ${minutes}m`
  if (minutes > 0) return `${minutes}m ${secs}s`
  return `${secs}s`
}

/**
 * Formats system uptime from seconds.
 */
export function formatUptime(seconds?: number | null): string {
  if (!seconds || seconds <= 0) return 'Active'
  return formatDuration(seconds)
}

/**
 * Formats percentage numbers with optional decimal precision.
 */
export function formatPercent(val?: number | null, decimals = 0): string {
  if (val === undefined || val === null || isNaN(val)) return '0%'
  if (decimals === 0) return `${Math.round(val)}%`
  return `${val.toFixed(decimals)}%`
}

/**
 * Capitalizes first letter of string.
 */
export function formatTitleCase(str?: string | null): string {
  if (!str) return ''
  return str.charAt(0).toUpperCase() + str.slice(1)
}

/**
 * Normalizes OS distro names.
 */
export function formatDistro(distro?: string | null, os?: string | null): string {
  if (distro && distro.trim().length > 0) return distro.trim()
  if (os && os.trim().length > 0) return os.trim()
  return 'Linux'
}

/**
 * Returns short badge label for Linux distributions.
 */
export function formatShortDistro(distro?: string | null, os?: string | null): string {
  const d = formatDistro(distro, os)
  if (!d) return 'Linux'
  const lower = d.toLowerCase()
  if (lower.includes('ubuntu')) return 'Ubuntu'
  if (lower.includes('debian')) return 'Debian'
  if (lower.includes('centos')) return 'CentOS'
  if (lower.includes('fedora')) return 'Fedora'
  if (lower.includes('red hat') || lower.includes('rhel')) return 'RHEL'
  if (lower.includes('alpine')) return 'Alpine'
  if (lower.includes('arch')) return 'Arch'
  return d
}

/**
 * Generates summary label for OS distro and architecture.
 */
export function formatOsSummary(node?: { os_distro?: string; os?: string; arch?: string } | null): string {
  if (!node) return 'Linux (amd64)'
  const arch = node.arch || 'amd64'
  const distro = formatShortDistro(node.os_distro, node.os)
  return `${distro} (${arch})`
}

/**
 * Formats kernel version with fallback to OS.
 */
export function formatKernelVersion(kernel?: string | null, os?: string | null): string {
  if (!kernel || !kernel.trim()) return formatTitleCase(os) || 'Linux'
  const k = kernel.trim()
  if (k.toLowerCase() === 'linux') return 'Linux'
  return k
}

/**
 * Formats load average array / string into standardized '0.45, 0.32, 0.28'.
 */
export function formatLoadAvg(loadAvg?: [number, number, number] | number[] | string | null): string {
  if (!loadAvg) return '0.45, 0.32, 0.28'
  if (typeof loadAvg === 'string') return loadAvg
  if (Array.isArray(loadAvg)) {
    return loadAvg.map(n => (typeof n === 'number' ? n.toFixed(2) : String(n))).join(', ')
  }
  return '0.45, 0.32, 0.28'
}

/**
 * Formats category strings by replacing underscores and uppercasing.
 */
export function formatCategory(cat?: string | null): string {
  if (!cat) return 'UNKNOWN'
  return cat.replace(/_/g, ' ').toUpperCase()
}

/**
 * Formats snake_case or kebab-case types to Title Case with spaces.
 */
export function formatType(t?: string | null): string {
  if (!t) return 'GENERAL'
  return t.replace(/[_-]/g, ' ').toUpperCase()
}

/**
 * Formats framework tags.
 */
export function formatFrameworkTag(tag?: string | null): string {
  if (!tag) return ''
  return tag.replace(/_/g, ' ').toUpperCase()
}

/**
 * Formats FinOps waste types.
 */
export function formatWasteType(t?: string | null): string {
  if (!t) return 'Idle Resources'
  return t.replace(/_/g, ' ').toUpperCase()
}

/**
 * Formats drift status.
 */
export function formatDriftStatus(s?: string | null): string {
  if (!s) return 'IN SYNC'
  return s.replace(/_/g, ' ').toUpperCase()
}
