import { computed, type Ref } from 'vue'
import type { KindCategory, KindCategoryItem } from './types'

export const kindCategories: KindCategory[] = [
  {
    title: 'Cluster',
    iconKey: 'server',
    items: [
      { label: 'Nodes', kind: 'nodes', iconKey: 'nodes' },
      { label: 'PersistentVolumes', kind: 'persistentvolumes', iconKey: 'pv' },
      { label: 'StorageClasses', kind: 'storageclasses', iconKey: 'sc' },
    ],
  },
  {
    title: 'Workloads',
    iconKey: 'workloads',
    items: [
      { label: 'Pods', kind: 'pods', iconKey: 'pods' },
      { label: 'Deployments', kind: 'deployments', iconKey: 'deployments' },
      { label: 'StatefulSets', kind: 'statefulsets', iconKey: 'statefulsets' },
      { label: 'DaemonSets', kind: 'daemonsets', iconKey: 'daemonsets' },
      { label: 'Jobs', kind: 'jobs', iconKey: 'jobs' },
      { label: 'CronJobs', kind: 'cronjobs', iconKey: 'cronjobs' },
    ],
  },
  {
    title: 'Config & Storage',
    iconKey: 'database',
    items: [
      { label: 'ConfigMaps', kind: 'configmaps', iconKey: 'configmaps' },
      { label: 'Secrets', kind: 'secrets', iconKey: 'secrets' },
      { label: 'PersistentVolumeClaims', kind: 'persistentvolumeclaims', iconKey: 'pvc' },
      { label: 'HorizontalPodAutoscalers', kind: 'horizontalpodautoscalers', iconKey: 'hpa' },
    ],
  },
  {
    title: 'Networking & Security',
    iconKey: 'network',
    items: [
      { label: 'Services', kind: 'services', iconKey: 'services' },
      { label: 'Ingresses', kind: 'ingresses', iconKey: 'ingresses' },
      { label: 'NetworkPolicies', kind: 'networkpolicies', iconKey: 'networkpolicies' },
      { label: 'ServiceAccounts', kind: 'serviceaccounts', iconKey: 'serviceaccounts' },
    ],
  },
  {
    title: 'Observability',
    iconKey: 'activity',
    items: [
      { label: 'Events', kind: 'events', iconKey: 'events' },
    ],
  },
]

export const allKindItems = computed<KindCategoryItem[]>(() => kindCategories.flatMap(c => c.items))

export function getFilteredKindCategories(query: string): KindCategory[] {
  const q = query.trim().toLowerCase()
  if (!q) return kindCategories
  return kindCategories
    .map(c => ({
      ...c,
      items: c.items.filter(i => i.label.toLowerCase().includes(q) || i.kind.toLowerCase().includes(q)),
    }))
    .filter(c => c.items.length > 0)
}

export function filteredKindCategories(queryRef: Ref<string>) {
  return computed(() => getFilteredKindCategories(queryRef.value))
}
