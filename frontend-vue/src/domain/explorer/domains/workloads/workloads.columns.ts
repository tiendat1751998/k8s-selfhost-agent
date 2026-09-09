import type { Column, K8sResource } from '../../types'

export const podColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true, render: (r) => r.metadata?.name || (r as any).name || '-' },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true, render: (r) => r.metadata?.namespace || (r as any).namespace || '-' },
  { key: 'status', label: 'Status', width: '120px', sortable: true },
  { key: 'ready', label: 'Ready', width: '80px', sortable: true },
  { key: 'restarts', label: 'Restarts', width: '85px', sortable: true },
  { key: 'node', label: 'Node', width: '130px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '270px', align: 'right' },
]

export const deploymentColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true, render: (r) => r.metadata?.name || (r as any).name || '-' },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true, render: (r) => r.metadata?.namespace || (r as any).namespace || '-' },
  { key: 'replicas', label: 'Replicas (Ready/Desired)', width: '180px', sortable: true },
  { key: 'image', label: 'Image', sortable: true },
  { key: 'selector', label: 'Selector', width: '160px', sortable: false },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '270px', align: 'right' },
]

export const statefulSetColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true, render: (r) => r.metadata?.name || (r as any).name || '-' },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true, render: (r) => r.metadata?.namespace || (r as any).namespace || '-' },
  { key: 'replicas', label: 'Replicas (Ready/Desired)', width: '180px', sortable: true },
  { key: 'image', label: 'Image', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '240px', align: 'right' },
]

export const daemonSetColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true, render: (r) => r.metadata?.name || (r as any).name || '-' },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true, render: (r) => r.metadata?.namespace || (r as any).namespace || '-' },
  { key: 'desired', label: 'Desired', width: '80px', sortable: true },
  { key: 'current', label: 'Current', width: '80px', sortable: true },
  { key: 'ready', label: 'Ready', width: '80px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '240px', align: 'right' },
]

export const jobColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true, render: (r) => r.metadata?.name || (r as any).name || '-' },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true, render: (r) => r.metadata?.namespace || (r as any).namespace || '-' },
  { key: 'completions', label: 'Completions', width: '120px', sortable: true },
  { key: 'duration', label: 'Duration', width: '110px', sortable: true },
  { key: 'status', label: 'Status', width: '120px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '190px', align: 'right' },
]

export const cronJobColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true, render: (r) => r.metadata?.name || (r as any).name || '-' },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true, render: (r) => r.metadata?.namespace || (r as any).namespace || '-' },
  { key: 'schedule', label: 'Schedule', width: '130px', sortable: true },
  { key: 'suspend', label: 'Suspend', width: '90px', sortable: true },
  { key: 'active', label: 'Active', width: '80px', sortable: true },
  { key: 'lastSchedule', label: 'Last Schedule', width: '120px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '280px', align: 'right' },
]
