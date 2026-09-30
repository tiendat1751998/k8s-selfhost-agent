/**
 * Log Target Categorizer & Noise Filter
 *
 * Categorizes log targets into:
 * - Container Workloads (category: "app", subtitle: "Container")
 * - SystemD Services (category: "systemd", subtitle: "SystemD Service", icon: "cpu")
 *
 * Filters out rotated logs (e.g. \.\d+$) and OS maintenance/installer file noise.
 */

export const OS_NOISE_NAMES: ReadonlySet<string> = new Set([
  'alternatives',
  'apport',
  'dpkg',
  'bootstrap',
  'cloud-init',
  'cloud-init-output',
  'ubuntu-advantage',
  'fontconfig',
  'faillog',
  'lastlog',
  'wtmp',
  'btmp',
  'vmware-network',
])

/**
 * Determines whether a given log target or file name is rotated OS noise or maintenance file noise.
 */
export function isNoiseLog(rawName?: string | null): boolean {
  if (!rawName) return true
  const lower = rawName.trim().toLowerCase()
  if (!lower) return true

  // 1. Filter out rotated logs matching .\d+$ (e.g. vmware-network.1...7, syslog.1)
  if (/\.\d+$/.test(lower)) return true

  // 2. Filter out OS maintenance / installer file noise (with or without .log extension)
  const baseName = lower.replace(/\.log$/, '')
  return OS_NOISE_NAMES.has(lower) || OS_NOISE_NAMES.has(baseName)
}

/**
 * Determines whether a service name represents a systemd service, socket, target, or slice.
 */
export function isSystemdService(name?: string | null): boolean {
  if (!name) return false
  const lower = name.trim().toLowerCase()
  return (
    lower.endsWith('.service') ||
    lower.endsWith('.socket') ||
    lower.endsWith('.target') ||
    lower.endsWith('.slice')
  )
}

/**
 * Returns the appropriate icon name for a given target service.
 * SystemD units return 'cpu' and are protected from accidental keyword matches (like 'auth' in 'vgauth.service').
 */
export function getServiceIcon(name?: string | null): string {
  if (!name) return 'sliders'
  const lower = name.trim().toLowerCase()

  // 1. SystemD services must return 'cpu' and never match substring rules like 'auth'
  if (
    lower.endsWith('.service') ||
    lower.endsWith('.socket') ||
    lower.endsWith('.target') ||
    lower.endsWith('.slice') ||
    lower.startsWith('vgauth')
  ) {
    return 'cpu'
  }

  // 2. Container and application workload keywords
  if (lower.includes('traefik') || lower.includes('ingress') || lower.includes('gateway') || lower.includes('nginx')) {
    return 'radio'
  }
  if (lower.includes('postg') || lower.includes('sql') || lower.includes('mysql') || lower.includes('redis') || lower.includes('db')) {
    return 'database'
  }
  if (lower.includes('nats') || lower.includes('kafka') || lower.includes('queue') || lower.includes('mq')) {
    return 'zap'
  }
  if (lower.includes('agent')) {
    return 'cloud'
  }
  if (lower.includes('docker') || lower.includes('containerd')) {
    return 'box'
  }
  if (lower.includes('auth') || lower.includes('vault') || lower.includes('security')) {
    return 'lock'
  }
  if (lower.includes('monitor') || lower.includes('prom') || lower.includes('grafana')) {
    return 'activity'
  }

  return 'sliders'
}

export interface ClassifiedTarget {
  category: 'app' | 'systemd'
  type: string
  icon: string
}

/**
 * Classifies a target by name and optional default type into container workload vs systemd daemon.
 */
export function classifyLogTarget(name: string, defaultType?: string): ClassifiedTarget {
  if (isSystemdService(name)) {
    return {
      category: 'systemd',
      type: 'SystemD Service',
      icon: getServiceIcon(name),
    }
  }

  return {
    category: 'app',
    type: defaultType || 'Container',
    icon: getServiceIcon(name),
  }
}
