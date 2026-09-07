import type { DeploymentApp, DockerService, DockerContainer } from '../api/compute'
import { formatContainerName, formatImageName } from '../utils/dockerFormat'

export type FilterTab = 'all' | 'canary' | 'bluegreen' | 'k8s' | 'swarm'

export interface ToastMessage {
  text: string
  type: 'success' | 'error' | 'info'
}

export interface RolloutState {
  desired: number
  ready: number
  updated: number
  pending: number
  isUpdating: boolean
  isPaused: boolean
  percent: number
  statusText: string
  badgeClass: string
  label: string
}

export interface TrafficSplit {
  canaryPercent: number
  stablePercent: number
  bluePercent: number
  greenPercent: number
  activeColor: 'blue' | 'green'
}

export function calculateTrafficSplit(app: DeploymentApp): TrafficSplit {
  const canaryPercent = Math.max(0, Math.min(100, app.canaryWeight ?? 0))
  const stablePercent = 100 - canaryPercent
  const activeColor = app.blueGreenActive === 'green' ? 'green' : 'blue'
  return {
    canaryPercent,
    stablePercent,
    bluePercent: activeColor === 'blue' ? 100 : 0,
    greenPercent: activeColor === 'green' ? 100 : 0,
    activeColor
  }
}

export function getRolloutState(row: DeploymentApp): RolloutState {
  const desired = row.replicas ?? 0
  const ready = row.readyReplicas !== undefined ? row.readyReplicas : (row.status === 'healthy' ? desired : 0)
  const updated = row.updatedReplicas !== undefined ? row.updatedReplicas : desired
  const pending = Math.max(0, desired - ready)
  const isUpdating = row.status === 'updating' || (desired > 0 && (ready < desired || updated < desired))
  const isPaused = Boolean(row.paused)
  const percent = desired > 0 ? Math.min(100, Math.round((ready / desired) * 100)) : 100

  let statusText = `${ready}/${desired} Ready`
  let badgeClass = 'chip-ready'
  let label = '✓ Ready'

  if (isPaused) {
    label = '⏸ Paused'
    badgeClass = 'chip-paused'
    statusText = `${ready}/${desired} Ready (Paused)`
  } else if (isUpdating) {
    label = `⟳ Updating (${pending} pending)`
    badgeClass = 'chip-updating'
    statusText = `${ready}/${desired} Ready • ${updated}/${desired} Updated (${pending} pending)`
  } else if (ready === desired && desired > 0) {
    label = '✓ Ready'
    badgeClass = 'chip-ready'
    statusText = `${desired}/${desired} Ready (Rollout complete)`
  } else if (desired === 0) {
    label = '0 Replicas'
    badgeClass = 'chip-scaled-down'
    statusText = 'Scaled to 0'
  }

  return { desired, ready, updated, pending, isUpdating, isPaused, percent, statusText, badgeClass, label }
}

export function buildSwarmAppItem(s: DockerService): DeploymentApp {
  let parsedPort = 80
  if (s.ports && s.ports.length > 0) {
    const parts = s.ports[0].split(':')
    if (parts.length >= 2) {
      const p = parseInt(parts[0], 10)
      if (!isNaN(p)) parsedPort = p
    }
  }
  return {
    id: s.id,
    rawId: s.id,
    name: s.name,
    team: s.name.startsWith('tiki_') ? 'Tiki Team' : 'Infrastructure',
    env: 'production',
    image: s.image || 'docker.io/library/unknown:latest',
    target: 'swarm-manager',
    namespace: s.name.includes('_') ? s.name.split('_')[0] : 'swarm',
    type: 'swarm',
    replicas: s.replicas || 0,
    readyReplicas: s.replicas || 0,
    availableReplicas: s.replicas || 0,
    updatedReplicas: s.replicas || 0,
    status: (s.replicas && s.replicas > 0) ? 'healthy' : 'down',
    cpu: '500m',
    memory: '512Mi',
    port: parsedPort,
    netType: (s.ports && s.ports.length > 0) ? 'NodePort' : 'Overlay',
    strategy: 'RollingUpdate',
    revision: 1,
  }
}

export function buildDockerContainerAppItem(c: DockerContainer, cleanName: string): DeploymentApp {
  const isRunning = c.state === 'running'
  const isRestarting = c.state === 'restarting'
  const isDegraded = isRestarting || Boolean(c.status && c.status.toLowerCase().includes('unhealthy'))
  const status = isRunning && !isDegraded ? 'healthy' : isDegraded ? 'degraded' : 'down'

  return {
    id: c.id,
    rawId: c.id,
    name: cleanName || c.id.slice(0, 12),
    team: 'DevOps',
    env: 'production',
    image: c.image,
    target: 'docker-engine',
    namespace: 'docker',
    type: 'docker',
    replicas: isRunning ? 1 : 0,
    readyReplicas: isRunning ? 1 : 0,
    availableReplicas: isRunning ? 1 : 0,
    updatedReplicas: isRunning ? 1 : 0,
    status,
    cpu: '250m',
    memory: '256Mi',
    port: 80,
    netType: 'Bridge',
    strategy: 'Recreate',
    revision: 1,
  }
}

export function filterDeploymentsList(
  list: DeploymentApp[],
  activeFilterTab: FilterTab,
  selectedNamespaceFilter: string,
  searchQuery: string
): DeploymentApp[] {
  return list.filter(d => {
    if (activeFilterTab === 'canary' && d.strategy !== 'Canary' && (!d.canaryWeight || d.canaryWeight <= 0)) {
      return false
    }
    if (activeFilterTab === 'bluegreen' && d.strategy !== 'BlueGreen') return false
    if (activeFilterTab === 'k8s' && d.type !== 'kubernetes' && d.type !== 'k8s') return false
    if (activeFilterTab === 'swarm' && d.type !== 'swarm' && d.type !== 'docker' && d.type !== 'container') return false
    if (selectedNamespaceFilter !== 'all' && d.namespace !== selectedNamespaceFilter) return false

    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase().trim()
      const formatted = formatContainerName(d.name)
      const formattedImg = formatImageName(d.image)
      const matchName = d.name.toLowerCase().includes(q) || formatted.serviceName.toLowerCase().includes(q) || Boolean(formatted.slotBadgeText && formatted.slotBadgeText.toLowerCase().includes(q))
      const matchImage = d.image.toLowerCase().includes(q) || formattedImg.display.toLowerCase().includes(q)
      const matchNs = (d.namespace || '').toLowerCase().includes(q)
      const matchTeam = (d.team || '').toLowerCase().includes(q)
      const matchStrategy = (d.strategy || '').toLowerCase().includes(q)
      return matchName || matchImage || matchNs || matchTeam || matchStrategy
    }
    return true
  })
}
