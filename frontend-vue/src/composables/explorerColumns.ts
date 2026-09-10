import type { Column } from '../components/ui/DataTable.vue'
import type { K8sResource } from '../api/k8s'
export const podColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true },
  { key: 'status', label: 'Status', width: '120px', sortable: true },
  { key: 'ready', label: 'Ready', width: '80px', sortable: true },
  { key: 'restarts', label: 'Restarts', width: '85px', sortable: true },
  { key: 'node', label: 'Node', width: '130px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

export const deploymentColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true },
  { key: 'replicas', label: 'Replicas (Ready/Desired)', width: '180px', sortable: true },
  { key: 'image', label: 'Image', sortable: true },
  { key: 'selector', label: 'Selector', width: '160px', sortable: false },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

export const statefulSetColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true },
  { key: 'replicas', label: 'Replicas (Ready/Desired)', width: '180px', sortable: true },
  { key: 'image', label: 'Image', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

export const daemonSetColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true },
  { key: 'desired', label: 'Desired', width: '80px', sortable: true },
  { key: 'current', label: 'Current', width: '80px', sortable: true },
  { key: 'ready', label: 'Ready', width: '80px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

export const jobColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true },
  { key: 'completions', label: 'Completions', width: '120px', sortable: true },
  { key: 'duration', label: 'Duration', width: '110px', sortable: true },
  { key: 'status', label: 'Status', width: '120px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

export const cronJobColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true },
  { key: 'schedule', label: 'Schedule', width: '130px', sortable: true },
  { key: 'suspend', label: 'Suspend', width: '90px', sortable: true },
  { key: 'active', label: 'Active', width: '80px', sortable: true },
  { key: 'lastSchedule', label: 'Last Schedule', width: '120px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

export const serviceColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true },
  { key: 'type', label: 'Type', width: '120px', sortable: true },
  { key: 'clusterIP', label: 'Cluster IP', width: '130px', sortable: true },
  { key: 'externalIP', label: 'External IP', width: '130px', sortable: true },
  { key: 'ports', label: 'Ports', width: '170px', sortable: false },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

export const ingressColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true },
  { key: 'hosts', label: 'Hosts', width: '180px', sortable: true },
  { key: 'paths', label: 'Paths', width: '150px', sortable: false },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

export const configMapColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true },
  { key: 'keysCount', label: 'Keys Count', width: '100px', sortable: true },
  { key: 'dataPreview', label: 'Data Preview', sortable: false },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

export const secretColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true },
  { key: 'type', label: 'Type', width: '150px', sortable: true },
  { key: 'keysCount', label: 'Keys Count', width: '100px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

export const pvcColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true },
  { key: 'status', label: 'Status', width: '110px', sortable: true },
  { key: 'capacity', label: 'Capacity', width: '100px', sortable: true },
  { key: 'accessModes', label: 'Access Modes', width: '140px', sortable: true },
  { key: 'storageClass', label: 'StorageClass', width: '130px', sortable: true },
  { key: 'volume', label: 'Volume', width: '150px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

export const pvColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'status', label: 'Status', width: '110px', sortable: true },
  { key: 'capacity', label: 'Capacity', width: '100px', sortable: true },
  { key: 'accessModes', label: 'Access Modes', width: '140px', sortable: true },
  { key: 'reclaimPolicy', label: 'Reclaim Policy', width: '130px', sortable: true },
  { key: 'storageClass', label: 'StorageClass', width: '130px', sortable: true },
  { key: 'claim', label: 'Claim', width: '170px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

export const storageClassColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'provisioner', label: 'Provisioner', sortable: true },
  { key: 'reclaimPolicy', label: 'ReclaimPolicy', width: '130px', sortable: true },
  { key: 'volumeBindingMode', label: 'VolumeBindingMode', width: '160px', sortable: true },
  { key: 'defaultClass', label: 'Default', width: '80px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

export const networkPolicyColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true },
  { key: 'podSelector', label: 'Pod Selector', width: '200px', sortable: true },
  { key: 'policyTypes', label: 'Policy Types', width: '150px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

export const serviceAccountColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true },
  { key: 'secretsCount', label: 'Secrets', width: '100px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

export const hpaColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '130px', sortable: true },
  { key: 'reference', label: 'Reference', width: '170px', sortable: true },
  { key: 'targets', label: 'Targets', width: '140px', sortable: true },
  { key: 'minMax', label: 'Min/Max', width: '100px', sortable: true },
  { key: 'replicas', label: 'Replicas', width: '90px', sortable: true },
  { key: 'age', label: 'Age', width: '85px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

export const nodeColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Node Name', sortable: true },
  { key: 'status', label: 'Status', width: '150px', sortable: true },
  { key: 'roles', label: 'Roles', width: '130px', sortable: true },
  { key: 'version', label: 'Version', width: '120px', sortable: true },
  { key: 'internalIP', label: 'Internal IP', width: '130px', sortable: true },
  { key: 'osArch', label: 'OS / Arch', width: '130px', sortable: true },
  { key: 'podsCount', label: 'Pods', width: '90px', sortable: true },
  { key: 'age', label: 'Age', width: '90px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

export const eventColumns: Column<K8sResource>[] = [
  { key: 'type', label: 'Type', width: '110px', sortable: true },
  { key: 'reason', label: 'Reason', width: '150px', sortable: true },
  { key: 'involvedObject', label: 'Object', width: '210px', sortable: true },
  { key: 'message', label: 'Message', sortable: false },
  { key: 'count', label: 'Count', width: '80px', sortable: true },
  { key: 'age', label: 'Age', width: '90px', sortable: true },
]

export const standardColumns: Column<K8sResource>[] = [
  { key: 'name', label: 'Resource Name', sortable: true },
  { key: 'namespace', label: 'Namespace', width: '150px', sortable: true },
  { key: 'status', label: 'Status / Ready', width: '140px', sortable: true },
  { key: 'age', label: 'Age / Created', width: '140px', sortable: true },
  { key: 'actions', label: 'Actions', width: '180px', align: 'right' },
]

