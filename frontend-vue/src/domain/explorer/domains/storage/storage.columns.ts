import type { Column, K8sResource } from '../../types'

export const pvcColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true, render: (r) => r.metadata?.name || (r as any).name || '-' },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true, render: (r) => r.metadata?.namespace || (r as any).namespace || '-' },
  { key: 'status', label: 'Status', width: '110px', sortable: true },
  { key: 'capacity', label: 'Capacity', width: '100px', sortable: true },
  { key: 'accessModes', label: 'Access Modes', width: '140px', sortable: true },
  { key: 'storageClass', label: 'StorageClass', width: '130px', sortable: true },
  { key: 'volume', label: 'Volume', width: '150px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '190px', align: 'right' },
]

export const pvColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true, render: (r) => r.metadata?.name || (r as any).name || '-' },
  { key: 'status', label: 'Status', width: '110px', sortable: true },
  { key: 'capacity', label: 'Capacity', width: '100px', sortable: true },
  { key: 'accessModes', label: 'Access Modes', width: '140px', sortable: true },
  { key: 'reclaimPolicy', label: 'Reclaim Policy', width: '130px', sortable: true },
  { key: 'storageClass', label: 'StorageClass', width: '130px', sortable: true },
  { key: 'claim', label: 'Claim', width: '170px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '190px', align: 'right' },
]

export const storageClassColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true, render: (r) => r.metadata?.name || (r as any).name || '-' },
  { key: 'provisioner', label: 'Provisioner', sortable: true },
  { key: 'reclaimPolicy', label: 'ReclaimPolicy', width: '130px', sortable: true },
  { key: 'volumeBindingMode', label: 'VolumeBindingMode', width: '160px', sortable: true },
  { key: 'defaultClass', label: 'Default', width: '80px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '190px', align: 'right' },
]
