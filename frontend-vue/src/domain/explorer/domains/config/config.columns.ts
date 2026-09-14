import type { Column, K8sResource } from '../../types'

export const configMapColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true, render: (r) => r.metadata?.name || (r as any).name || '-' },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true, render: (r) => r.metadata?.namespace || (r as any).namespace || '-' },
  { key: 'keysCount', label: 'Keys Count', width: '100px', sortable: true },
  { key: 'dataPreview', label: 'Data Preview', sortable: false },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

export const secretColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true, render: (r) => r.metadata?.name || (r as any).name || '-' },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true, render: (r) => r.metadata?.namespace || (r as any).namespace || '-' },
  { key: 'type', label: 'Type', width: '150px', sortable: true },
  { key: 'keysCount', label: 'Keys Count', width: '100px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

export const hpaColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true, render: (r) => r.metadata?.name || (r as any).name || '-' },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true, render: (r) => r.metadata?.namespace || (r as any).namespace || '-' },
  { key: 'reference', label: 'Reference', width: '170px', sortable: true },
  { key: 'targets', label: 'Targets', width: '140px', sortable: true },
  { key: 'minMax', label: 'Min/Max', width: '100px', sortable: true },
  { key: 'replicas', label: 'Replicas', width: '90px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]
