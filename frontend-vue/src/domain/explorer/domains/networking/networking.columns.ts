import type { Column, K8sResource } from '../../types'

export const serviceColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true, render: (r) => r.metadata?.name || (r as any).name || '-' },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true, render: (r) => r.metadata?.namespace || (r as any).namespace || '-' },
  { key: 'type', label: 'Type', width: '120px', sortable: true },
  { key: 'clusterIP', label: 'Cluster IP', width: '130px', sortable: true },
  { key: 'externalIP', label: 'External IP', width: '130px', sortable: true },
  { key: 'ports', label: 'Ports', width: '170px', sortable: false },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '190px', align: 'right' },
]

export const ingressColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true, render: (r) => r.metadata?.name || (r as any).name || '-' },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true, render: (r) => r.metadata?.namespace || (r as any).namespace || '-' },
  { key: 'hosts', label: 'Hosts', width: '180px', sortable: true },
  { key: 'paths', label: 'Paths', width: '150px', sortable: false },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '190px', align: 'right' },
]

export const networkPolicyColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true, render: (r) => r.metadata?.name || (r as any).name || '-' },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true, render: (r) => r.metadata?.namespace || (r as any).namespace || '-' },
  { key: 'podSelector', label: 'Pod Selector', width: '200px', sortable: true },
  { key: 'policyTypes', label: 'Policy Types', width: '150px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '190px', align: 'right' },
]

export const serviceAccountColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true, render: (r) => r.metadata?.name || (r as any).name || '-' },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true, render: (r) => r.metadata?.namespace || (r as any).namespace || '-' },
  { key: 'secretsCount', label: 'Secrets', width: '100px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '190px', align: 'right' },
]
