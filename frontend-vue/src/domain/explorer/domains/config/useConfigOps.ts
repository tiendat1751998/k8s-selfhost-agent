import { ref } from 'vue'
import { k8sApi } from '../../../../api/k8s'
import type { K8sNamespace, K8sResource, ResourceKind } from '../../types'

export function useConfigOps() {
  const savingConfigMap = ref(false)
  const deletingResource = ref(false)
  const creatingNs = ref(false)

  async function saveConfigMap(
    cluster: string,
    cm: K8sResource,
    data: Record<string, string>
  ): Promise<K8sResource | undefined> {
    const name = cm.metadata?.name
    const ns = cm.metadata?.namespace || 'default'
    if (!name) return undefined
    savingConfigMap.value = true
    try {
      return await k8sApi.updateResource(
        cluster,
        'configmaps',
        name,
        { ...cm, data },
        ns
      )
    } finally {
      savingConfigMap.value = false
    }
  }

  async function deleteResource(
    cluster: string,
    resource: K8sResource,
    currentKind: ResourceKind
  ): Promise<void> {
    if (!resource.metadata?.name) return
    deletingResource.value = true
    try {
      const name = resource.metadata.name
      const ns = resource.metadata.namespace
      let kind = (resource.kind || currentKind).toLowerCase() as ResourceKind
      if (!kind.endsWith('s') && kind !== 'storageclasses' && kind !== 'networkpolicies') {
        kind = (kind + 's') as ResourceKind
      }
      await k8sApi.deleteResource(cluster, kind, name, ns)
    } finally {
      deletingResource.value = false
    }
  }

  async function createNamespace(cluster: string, name: string): Promise<K8sNamespace> {
    creatingNs.value = true
    try {
      return await k8sApi.createNamespace(cluster, name.trim())
    } finally {
      creatingNs.value = false
    }
  }

  return {
    savingConfigMap,
    deletingResource,
    creatingNs,
    saveConfigMap,
    deleteResource,
    createNamespace,
  }
}
