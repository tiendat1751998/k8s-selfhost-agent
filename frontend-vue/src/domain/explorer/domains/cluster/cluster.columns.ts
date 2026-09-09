import type { Column, K8sResource } from '../../types'

export const nodeColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Node Name', sortable: true, render: (r) => r.metadata?.name || (r as any).name || '-' },
  { key: 'status', label: 'Status', width: '150px', sortable: true },
  { key: 'roles', label: 'Roles', width: '130px', sortable: true },
  { key: 'version', label: 'Version', width: '120px', sortable: true },
  { key: 'internalIP', label: 'Internal IP', width: '130px', sortable: true },
  { key: 'osArch', label: 'OS / Arch', width: '130px', sortable: true },
  { key: 'podsCount', label: 'Pods', width: '90px', sortable: true },
  { key: 'age', label: 'Age', width: '90px', sortable: true },
  { key: 'actions', label: 'Actions', width: '270px', align: 'right' },
]

export const eventColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Resource Name', sortable: true, render: (r) => (r as any).involvedObject ? ((r as any).involvedObject as { name?: string }).name : r.metadata?.name || (r as any).name || '-' },
  { key: 'namespace', label: 'Namespace', width: '120px', sortable: true, render: (r) => (r as any).involvedObject ? ((r as any).involvedObject as { namespace?: string }).namespace : r.metadata?.namespace || (r as any).namespace || '-' },
  { key: 'type', label: 'Type', width: '110px', sortable: true },
  { key: 'reason', label: 'Reason', width: '150px', sortable: true },
  { key: 'involvedObject', label: 'Object', width: '210px', sortable: true },
  { key: 'message', label: 'Message', sortable: false },
  { key: 'count', label: 'Count', width: '80px', sortable: true },
  { key: 'age', label: 'Age', width: '90px', sortable: true },
]

export const standardColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Resource Name', sortable: true, render: (r) => r.metadata?.name || (r as any).name || '-' },
  { key: 'namespace', label: 'Namespace', width: '150px', sortable: true, render: (r) => r.metadata?.namespace || (r as any).namespace || '-' },
  { key: 'status', label: 'Status / Ready', width: '140px', sortable: true },
  { key: 'age', label: 'Age / Created', width: '140px', sortable: true },
  { key: 'actions', label: 'Actions', width: '190px', align: 'right' },
]
