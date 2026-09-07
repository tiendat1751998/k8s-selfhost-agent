import { ref, computed } from 'vue'
import type { NodeMetrics, SystemOverview, TpsSnapshot, TpsServiceMetrics } from '../../../../api/overview'

export type ProcessCategoryType = 'container' | 'host_app' | 'system'

export interface UnifiedProcessItem {
  pid: number | string
  name: string
  command_line?: string
  user?: string
  cpu_percent: number
  memory_bytes: number
  memory_percent?: number
  disk_read_bytes_per_sec?: number
  disk_write_bytes_per_sec?: number
  requests_per_sec: number
  error_rate: number
  rx_bytes_per_sec: number
  tx_bytes_per_sec: number
  state?: string
  priority?: number | string
  is_container?: boolean
  is_app?: boolean
  app_type?: ProcessCategoryType
  container_name?: string
}

export interface NodeProcessesProps {
  node: NodeMetrics
  overview?: SystemOverview | null
  tpsData?: TpsSnapshot | null
}

export function useNodeProcesses(props: NodeProcessesProps) {
  const processCategoryFilter = ref<'all' | 'container' | 'host_daemon' | 'kernel'>('all')
  const processSearch = ref('')
  const processSortBy = ref<'cpu' | 'mem' | 'disk' | 'rps' | 'bandwidth' | 'pid' | 'name'>('cpu')
  const processSortOrder = ref<'asc' | 'desc'>('desc')
  const processPageSize = ref<number>(25)
  const processCurrentPage = ref<number>(1)

  function classifyProcessType(p: { name?: string; command_line?: string; user?: string; is_container?: boolean }): ProcessCategoryType {
    if (p.is_container) return 'container'

    const name = (p.name || '').toLowerCase()
    const cmd = (p.command_line || '').toLowerCase()
    const user = (p.user || '').toLowerCase()

    if (name.startsWith('[') || name.endsWith(']') || cmd.startsWith('[') || cmd.endsWith(']')) {
      return 'system'
    }

    const coreOsSystemDaemons = [
      'systemd', 'journald', 'udevd', 'resolved', 'timesyncd', 'logind', 'dbus-daemon',
      'polkitd', 'accounts-daemon', 'cron', 'crond', 'rsyslogd', 'agetty', 'login',
      'irqbalance', 'multipathd', 'thermald', 'snapd', 'unattended-upgr', 'packagekitd',
      'auditd', 'acpid', 'atd', 'smartd'
    ]
    if (coreOsSystemDaemons.some(d => name === d || name.startsWith(d) || name.endsWith(d))) {
      return 'system'
    }

    if (user && user !== 'root' && user !== 'daemon' && user !== 'sys') {
      return 'host_app'
    }

    const knownAppKeywords = [
      'k8s-agent', 'k8s-master', 'k8s-controller', 'hermes', 'proxysql', 'dockerd', 'containerd',
      'traefik', 'redis', 'nats', 'postgres', 'pgsql', 'mysql', 'mariadb', 'nginx', 'caddy',
      'envoy', 'prometheus', 'grafana', 'vector', 'fluent', 'loki', 'alloy', 'vault', 'consul',
      'etcd', 'kubelet', 'kube-proxy', 'node', 'python', 'java', 'golang', 'ruby', 'php',
      'uvicorn', 'gunicorn', 'vite', 'cargo', 'dotnet', 'celery'
    ]
    if (knownAppKeywords.some(k => name.includes(k) || cmd.includes(k))) {
      return 'host_app'
    }

    if (cmd.includes('/home/') || cmd.includes('/opt/') || cmd.includes('/srv/') || cmd.includes('/app/') || cmd.includes('/usr/local/')) {
      return 'host_app'
    }

    return 'system'
  }

  const rawNodeServices = computed<TpsServiceMetrics[]>(() => {
    if (!props.node || !props.tpsData?.services) return []
    const nodeId = (props.node.node_id || '').toLowerCase().trim()
    const nodeName = (props.node.node_name || '').toLowerCase().trim()
    const isSingleNode = (props.overview?.nodes?.length || 0) === 1

    return props.tpsData.services.filter(s => {
      const sNodeId = (s.node_id || '').toLowerCase().trim()
      const sNodeName = (s.node_name || '').toLowerCase().trim()

      if (sNodeId && (sNodeId === nodeId || sNodeId === nodeName)) return true
      if (sNodeName && (sNodeName === nodeName || sNodeName === nodeId)) return true
      if (sNodeName && nodeName) {
        if (nodeName === sNodeName || nodeName.includes(sNodeName) || sNodeName.includes(nodeName)) return true
        const normS = sNodeName.replace(/^(k8s|node-)/, '')
        const normN = nodeName.replace(/^(k8s|node-)/, '')
        if (normS && normN && (normS === normN || normS.includes(normN) || normN.includes(normS))) return true
      }
      if (isSingleNode && (!s.node_id || sNodeId === nodeId || sNodeId === nodeName)) return true
      return false
    })
  })

  const unifiedNodeProcesses = computed<UnifiedProcessItem[]>(() => {
    const procs = props.node?.top_processes || []
    const services = rawNodeServices.value || []
    const matchedServices = new Set<string>()

    const list: UnifiedProcessItem[] = procs.map(p => {
      const pName = (p.name || '').toLowerCase()
      const pCmd = (p.command_line || '').toLowerCase()

      const matchedSvc = services.find(s => {
        const sName = (s.service_name || '').toLowerCase()
        const sClean = sName.replace(/^(tiki_|k8s_|docker_)/, '')
        return (
          sName === pName ||
          pName.includes(sClean) ||
          pCmd.includes(sName) ||
          pCmd.includes(sClean) ||
          (sClean === 'redis' && (pName.includes('redis') || pCmd.includes('redis'))) ||
          (sClean === 'traefik' && (pName.includes('traefik') || pCmd.includes('traefik'))) ||
          (sClean === 'nats' && (pName.includes('nats') || pCmd.includes('nats'))) ||
          ((sClean === 'postgres' || sClean === 'db') && (pName.includes('postgres') || pCmd.includes('postgres')))
        )
      })

      if (matchedSvc) {
        matchedServices.add(matchedSvc.service_name)
        return {
          pid: p.pid,
          name: matchedSvc.service_name,
          command_line: p.command_line || p.name,
          user: p.user || 'root',
          cpu_percent: Math.max(p.cpu_percent || 0, matchedSvc.cpu_percent || 0),
          memory_bytes: Math.max(p.memory_bytes || 0, (matchedSvc.memory_used_mb || 0) * 1024 * 1024),
          memory_percent: p.memory_percent,
          disk_read_bytes_per_sec: p.read_bytes_per_sec || 0,
          disk_write_bytes_per_sec: p.write_bytes_per_sec || 0,
          requests_per_sec: matchedSvc.requests_per_sec || 0,
          error_rate: matchedSvc.error_rate || 0,
          rx_bytes_per_sec: matchedSvc ? (matchedSvc.rx_bytes_per_sec || 0) : 0,
          tx_bytes_per_sec: matchedSvc ? (matchedSvc.tx_bytes_per_sec || 0) : 0,
          state: matchedSvc.status === 'healthy' || matchedSvc.status === 'running' ? 'healthy' : (p.state || 'running'),
          is_container: true,
          is_app: true,
          app_type: 'container',
          container_name: matchedSvc.service_name,
        }
      }

      const appType = classifyProcessType({ name: p.name, command_line: p.command_line, user: p.user, is_container: false })
      return {
        pid: p.pid,
        name: p.name,
        command_line: p.command_line,
        user: p.user || 'root',
        cpu_percent: p.cpu_percent || 0,
        memory_bytes: p.memory_bytes || 0,
        memory_percent: p.memory_percent,
        disk_read_bytes_per_sec: p.read_bytes_per_sec || 0,
        disk_write_bytes_per_sec: p.write_bytes_per_sec || 0,
        requests_per_sec: 0,
        error_rate: 0,
        rx_bytes_per_sec: 0,
        tx_bytes_per_sec: 0,
        state: p.state || 'running',
        is_container: false,
        is_app: appType !== 'system',
        app_type: appType,
      }
    })

    for (const s of services) {
      if (!matchedServices.has(s.service_name)) {
        list.push({
          pid: 'CTR',
          name: s.service_name,
          command_line: `Container Service: ${s.service_name}`,
          user: 'docker',
          cpu_percent: s.cpu_percent || 0,
          memory_bytes: (s.memory_used_mb || 0) * 1024 * 1024,
          disk_read_bytes_per_sec: 0,
          disk_write_bytes_per_sec: 0,
          requests_per_sec: s.requests_per_sec || 0,
          error_rate: s.error_rate || 0,
          rx_bytes_per_sec: s.rx_bytes_per_sec || 0,
          tx_bytes_per_sec: s.tx_bytes_per_sec || 0,
          state: s.status || 'healthy',
          is_container: true,
          is_app: true,
          app_type: 'container',
          container_name: s.service_name,
        })
      }
    }

    return list
  })

  const processCategoryCounts = computed(() => {
    const all = unifiedNodeProcesses.value
    let container = 0
    let hostDaemon = 0
    let kernel = 0
    for (const p of all) {
      if (p.app_type === 'container') container++
      else if (p.app_type === 'host_app') hostDaemon++
      else kernel++
    }
    return { total: all.length, container, hostDaemon, kernel }
  })

  const filteredNodeProcesses = computed<UnifiedProcessItem[]>(() => {
    let list = [...unifiedNodeProcesses.value]

    if (processCategoryFilter.value === 'container') {
      list = list.filter(p => p.app_type === 'container')
    } else if (processCategoryFilter.value === 'host_daemon') {
      list = list.filter(p => p.app_type === 'host_app')
    } else if (processCategoryFilter.value === 'kernel') {
      list = list.filter(p => p.app_type === 'system')
    }

    if (processSearch.value.trim()) {
      const q = processSearch.value.toLowerCase().trim()
      list = list.filter(p =>
        p.name.toLowerCase().includes(q) ||
        (p.command_line && p.command_line.toLowerCase().includes(q)) ||
        (p.user && p.user.toLowerCase().includes(q)) ||
        String(p.pid).toLowerCase().includes(q)
      )
    }

    const isAsc = processSortOrder.value === 'asc'
    const multiplier = isAsc ? 1 : -1

    if (processSortBy.value === 'cpu') {
      list.sort((a, b) => {
        const diff = ((a.cpu_percent || 0) - (b.cpu_percent || 0)) * multiplier
        if (diff !== 0) return diff
        return ((b.memory_bytes || 0) - (a.memory_bytes || 0))
      })
    } else if (processSortBy.value === 'mem') {
      list.sort((a, b) => {
        const diff = ((a.memory_bytes || 0) - (b.memory_bytes || 0)) * multiplier
        if (diff !== 0) return diff
        return ((b.cpu_percent || 0) - (a.cpu_percent || 0))
      })
    } else if (processSortBy.value === 'disk') {
      list.sort((a, b) => {
        const aDisk = (a.disk_read_bytes_per_sec || 0) + (a.disk_write_bytes_per_sec || 0)
        const bDisk = (b.disk_read_bytes_per_sec || 0) + (b.disk_write_bytes_per_sec || 0)
        const diff = (aDisk - bDisk) * multiplier
        if (diff !== 0) return diff
        return ((b.cpu_percent || 0) - (a.cpu_percent || 0)) || ((b.memory_bytes || 0) - (a.memory_bytes || 0))
      })
    } else if (processSortBy.value === 'rps') {
      list.sort((a, b) => {
        const diff = ((a.requests_per_sec || 0) - (b.requests_per_sec || 0)) * multiplier
        if (diff !== 0) return diff
        return ((b.cpu_percent || 0) - (a.cpu_percent || 0)) || ((b.memory_bytes || 0) - (a.memory_bytes || 0))
      })
    } else if (processSortBy.value === 'bandwidth') {
      list.sort((a, b) => {
        const aBw = (a.rx_bytes_per_sec || 0) + (a.tx_bytes_per_sec || 0)
        const bBw = (b.rx_bytes_per_sec || 0) + (b.tx_bytes_per_sec || 0)
        const diff = (aBw - bBw) * multiplier
        if (diff !== 0) return diff
        return ((b.cpu_percent || 0) - (a.cpu_percent || 0)) || ((b.memory_bytes || 0) - (a.memory_bytes || 0))
      })
    } else if (processSortBy.value === 'pid') {
      list.sort((a, b) => {
        const pA = typeof a.pid === 'number' ? a.pid : 999999
        const pB = typeof b.pid === 'number' ? b.pid : 999999
        return (pA - pB) * multiplier
      })
    } else if (processSortBy.value === 'name') {
      list.sort((a, b) => a.name.localeCompare(b.name) * multiplier)
    }

    return list
  })

  const totalProcessPages = computed(() => {
    return Math.ceil(filteredNodeProcesses.value.length / processPageSize.value) || 1
  })

  const paginatedNodeProcesses = computed(() => {
    const start = (processCurrentPage.value - 1) * processPageSize.value
    return filteredNodeProcesses.value.slice(start, start + processPageSize.value)
  })

  function getProcessCpuColor(cpu?: number): string {
    if (!cpu) return 'slate'
    if (cpu >= 70) return 'rose'
    if (cpu >= 40) return 'amber'
    if (cpu >= 15) return 'cyan'
    return 'slate'
  }

  function getUnifiedProcessState(proc: UnifiedProcessItem): { label: string; class: string; dotClass: string } {
    const s = (proc.state || '').toLowerCase()
    if (s === 'healthy' || s === 'running' || s === 'r') {
      return { label: 'RUNNING', class: 'badge-emerald', dotClass: 'dot-emerald' }
    }
    if (s === 'sleeping' || s === 's') {
      return { label: 'SLEEP', class: 'badge-indigo', dotClass: 'dot-indigo' }
    }
    if (s === 'zombie' || s === 'z') {
      return { label: 'ZOMBIE', class: 'badge-rose', dotClass: 'dot-rose' }
    }
    if (s === 'disk_sleep' || s === 'd') {
      return { label: 'D-SLEEP', class: 'badge-amber', dotClass: 'dot-amber' }
    }
    if (s === 'stopped' || s === 't') {
      return { label: 'STOPPED', class: 'badge-slate', dotClass: 'dot-slate' }
    }
    return { label: s.toUpperCase() || 'ACTIVE', class: 'badge-cyan', dotClass: 'dot-cyan' }
  }

  function getProcessOriginBadge(proc: UnifiedProcessItem): { label: string; class: string; icon: string } {
    if (proc.app_type === 'container') {
      return { label: 'CONTAINER', class: 'tag-container', icon: '📦' }
    }
    if (proc.app_type === 'host_app') {
      return { label: 'HOST APP', class: 'tag-host-daemon', icon: '⚙️' }
    }
    return { label: 'KERNEL', class: 'tag-kernel', icon: '🐧' }
  }

  return {
    processCategoryFilter,
    processSearch,
    processSortBy,
    processSortOrder,
    processPageSize,
    processCurrentPage,
    processCategoryCounts,
    filteredNodeProcesses,
    totalProcessPages,
    paginatedNodeProcesses,
    getProcessCpuColor,
    getUnifiedProcessState,
    getProcessOriginBadge,
  }
}
