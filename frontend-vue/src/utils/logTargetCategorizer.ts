/**
 * Log Target Categorizer & Noise Filter
 *
 * Categorizes log targets into:
 * - Container Workloads (category: "app", subtitle: "Container")
 * - SystemD Services (category: "systemd", subtitle: "SystemD Service", icon: "cpu")
 *
 * Filters out rotated logs (e.g. \.\d+$) and OS maintenance/installer file noise.
 */

import type { LogEntry } from '../stores/logStore'

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

export const HOST_OR_SYSTEM_NAMES: ReadonlySet<string> = new Set([
  'journalctl',
  'systemd-journal',
  'auth',
  'authpriv',
  'kern',
  'syslog',
  'daemon',
  'cron',
  'crond',
  'crontab',
  'atd',
  'irqbalance',
  'acpid',
  'avahi-daemon',
  'accounts-daemon',
  'rtkit-daemon',
  'udisksd',
  'upowerd',
  'modemmanager',
  'networkmanager',
  'wpa_supplicant',
  'unattended-upgrades',
  'multipath',
  'multipathd',
  'networkd-dispatcher',
  'vmware-vmsvc',
  'vmware-vmsvc-root',
  'vmware-vmtoolsd-root',
  'vmtoolsd',
  'qemu-ga',
  'dbus',
  'dbus-daemon',
  'fwupd',
  'packagekit',
  'packagekitd',
  'snapd',
  'lxcfs',
  'rsyslog',
  'rsyslogd',
  'sshd',
  'sudo',
  'su',
  'polkit',
  'chronyd',
  'ntpd',
  'kubelet',
  'kube-proxy',
  'etcd',
  'containerd',
  'containerd-shim',
  'containerd-shim-runc-v2',
  'runc',
  'crio',
  'dockerd',
  'dockerd-rootless',
  'dockerd-rootless.sh',
])

export const SYSTEMD_UNIT_SUFFIXES: readonly string[] = [
  '.service',
  '.socket',
  '.timer',
  '.scope',
  '.slice',
  '.target',
  '.mount',
]

/**
 * Slice 76B: Client-side guard matching systemd unit suffixes, journal targets,
 * and known host OS syslog daemon noise.
 */
export function isHostOrSystemUnit(name?: string | null): boolean {
  if (!name) return false
  const lower = name.trim().toLowerCase()
  if (!lower) return false
  if (SYSTEMD_UNIT_SUFFIXES.some((suffix) => lower.endsWith(suffix))) {
    return true
  }
  if (HOST_OR_SYSTEM_NAMES.has(lower)) {
    return true
  }
  if (
    lower.startsWith('systemd-') ||
    lower.startsWith('systemd_') ||
    lower.startsWith('dbus-') ||
    lower === 'systemd' ||
    lower === 'journald' ||
    lower === 'kernel'
  ) {
    return true
  }
  return false
}

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
  if (OS_NOISE_NAMES.has(lower) || OS_NOISE_NAMES.has(baseName)) return true

  // 3. Filter out systemd scope/slice/user slice noise
  if (lower === 'init.scope' || lower.endsWith('.scope')) return true
  if (lower.endsWith('.slice')) return true
  if (lower.startsWith('user@') || lower.startsWith('user-runtime-dir@')) return true

  return false
}

/**
 * Determines whether a service name represents a systemd service, socket, target, slice, or system daemon.
 */
export function isSystemdService(name?: string | null): boolean {
  if (!name) return false
  const lower = name.trim().toLowerCase()
  if (
    lower.endsWith('.service') ||
    lower.endsWith('.socket') ||
    lower.endsWith('.target') ||
    lower.endsWith('.slice') ||
    lower.endsWith('.scope')
  ) {
    return true
  }
  if (
    lower === 'journald' ||
    lower === 'systemd' ||
    lower.startsWith('systemd-') ||
    lower.startsWith('systemd_') ||
    lower === 'cron' ||
    lower === 'sshd' ||
    lower === 'dbus' ||
    lower.startsWith('dbus-') ||
    lower.startsWith('dbus_') ||
    lower === 'sudo' ||
    lower === 'su' ||
    lower === 'kernel' ||
    lower === 'init.scope'
  ) {
    return true
  }
  return false
}

/**
 * Returns the appropriate icon name for a given target service.
 * SystemD units return 'cpu' and are protected from accidental keyword matches (like 'auth' in 'vgauth.service').
 */
export function getServiceIcon(name?: string | null): string {
  if (!name) return 'sliders'
  const lower = name.trim().toLowerCase()

  // 1. SystemD services must return 'cpu' and never match substring rules like 'auth'
  if (isSystemdService(name) || lower.startsWith('vgauth')) {
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

export interface AvailableServiceItem {
  id: string
  name: string
  category: 'app' | 'systemd'
  icon: string
  count: number
}

export function extractAvailableServices(logs: LogEntry[], effectiveNode?: string): AvailableServiceItem[] {
  const counts = new Map<string, { name: string; category: 'app' | 'systemd'; icon: string; count: number }>()
  const effNode = effectiveNode?.toLowerCase()
  for (const l of logs) {
    if (effNode) {
      const n = (l.node || l.attributes?.node || l.attributes?.node_name || '').toLowerCase()
      if (n && !n.includes(effNode)) continue
    }
    const svc = l.service || l.container || l.attributes?.app || l.attributes?.service || l.attributes?.container_name || l.pod
    if (!svc || isNoiseLog(svc)) continue
    const lower = svc.toLowerCase()
    const existing = counts.get(lower)
    if (existing) {
      existing.count++
    } else {
      const classified = classifyLogTarget(svc)
      counts.set(lower, {
        name: svc,
        category: classified.category,
        icon: classified.icon,
        count: 1,
      })
    }
  }
  const result: AvailableServiceItem[] = []
  for (const [id, item] of counts.entries()) {
    result.push({
      id,
      name: item.name,
      category: item.category,
      icon: item.icon,
      count: item.count,
    })
  }
  result.sort((a, b) => {
    if (a.category !== b.category) {
      return a.category === 'app' ? -1 : 1
    }
    return b.count - a.count
  })
  return result
}

