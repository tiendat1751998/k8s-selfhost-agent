import type {
  K8sResource,
  ResourceKind,
  NodeTaint,
  NodeCondition,
  NodeSystemInfo,
  K8sNamespace,
} from '../../api/k8s'
import type { Cluster } from '../../api/fleet'

export type {
  K8sResource,
  ResourceKind,
  NodeTaint,
  NodeCondition,
  NodeSystemInfo,
  K8sNamespace,
  Cluster,
}

export interface Column<T = unknown> {
  key: string
  label: string
  width?: string
  align?: 'left' | 'center' | 'right'
  sortable?: boolean
  render?: (row: T) => unknown
}

export interface KindCategoryItem {
  label: string
  kind: ResourceKind
  iconKey: string
}

export interface KindCategory {
  title: string
  iconKey: string
  items: KindCategoryItem[]
}

export interface DrainOptions {
  ignoreDaemonSets: boolean
  deleteEmptyDirData: boolean
  force: boolean
  gracePeriodSeconds: number
}

export interface ToastMessage {
  text: string
  type: 'success' | 'error'
}
