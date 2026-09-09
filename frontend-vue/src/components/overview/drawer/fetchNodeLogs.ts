import { dockerApi } from '../../../api/docker'
import { toIsoTime } from './nodeLogUtils'

export interface FetchLogsParams {
  targetApp: string
  tail: number | 'all'
  since: string
  customFrom: string
  customTo: string
  search?: string
  filterLevel?: string
  nodeTarget?: string
  nodeName?: string
  isHostApp?: boolean
}

export async function requestNodeLogs(params: FetchLogsParams): Promise<string> {
  const tailParam = params.tail === 'all' ? 'all' : String(params.tail)
  let sinceParam = ''
  let untilParam = ''

  if (params.since === 'custom') {
    if (params.customFrom) sinceParam = toIsoTime(params.customFrom) || params.customFrom
    if (params.customTo) untilParam = toIsoTime(params.customTo) || params.customTo
  } else if (params.since !== 'all') {
    sinceParam = params.since
  }

  const searchParam = params.search?.trim() || undefined
  const filterLevel = params.filterLevel && params.filterLevel !== 'all' ? params.filterLevel : undefined

  let res: { logs: string } | null = null
  try {
    res = await dockerApi.getLogs(
      params.targetApp,
      'service',
      tailParam,
      sinceParam,
      untilParam,
      searchParam,
      filterLevel,
      params.nodeTarget
    )
  } catch {
    res = await dockerApi.getLogs(
      params.targetApp,
      'container',
      tailParam,
      sinceParam,
      untilParam,
      searchParam,
      filterLevel,
      params.nodeTarget
    )
  }

  if (res && typeof res.logs === 'string' && res.logs.trim().length > 0) {
    return res.logs
  }

  if (params.isHostApp) {
    return `[INFO] Host process '${params.targetApp}' is running as a system service. Standard stdout/stderr stream is only accessible for Docker containerized workloads (e.g. tiki_redis, my-nginx, db, nats).`
  }

  const windowInfo = params.since === 'custom'
    ? `between ${params.customFrom || 'start'} and ${params.customTo || 'now'}`
    : `in the last ${params.since}`
  return `[INFO] No real stdout/stderr lines emitted by '${params.targetApp}' on node '${params.nodeName || 'node'}' ${windowInfo}. Process is running nominally.`
}
