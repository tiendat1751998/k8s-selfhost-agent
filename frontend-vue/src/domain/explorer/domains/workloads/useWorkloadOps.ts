import { ref } from 'vue'
import { k8sApi } from '../../../../api/k8s'
import type { K8sResource } from '../../types'

export function useWorkloadOps() {
  const scalingResource = ref(false)

  async function scaleWorkload(cluster: string, resource: K8sResource, replicas: number): Promise<void> {
    if (!resource.metadata?.name) return
    scalingResource.value = true
    try {
      const ns = resource.metadata?.namespace || 'default'
      const kind = (resource.kind || '').toLowerCase()
      if (kind === 'statefulset') {
        await k8sApi.scaleStatefulSet(cluster, resource.metadata.name, ns, replicas)
      } else {
        await k8sApi.scaleDeployment(cluster, resource.metadata.name, ns, replicas)
      }
    } finally {
      scalingResource.value = false
    }
  }

  async function restartWorkload(cluster: string, resource: K8sResource): Promise<void> {
    if (!resource.metadata?.name) return
    const kind = resource.kind || 'Resource'
    const name = resource.metadata.name
    const ns = resource.metadata.namespace || 'default'
    if (kind.toLowerCase() === 'daemonset') {
      await k8sApi.restartDaemonSet(cluster, name, ns)
    } else {
      await k8sApi.restartDeployment(cluster, name, ns)
    }
  }

  async function triggerCronJob(cluster: string, resource: K8sResource): Promise<void> {
    if (!resource.metadata?.name) return
    const name = resource.metadata.name
    const ns = resource.metadata.namespace || 'default'
    await k8sApi.triggerCronJob(cluster, name, ns)
  }

  async function suspendCronJob(cluster: string, resource: K8sResource, suspendState?: boolean): Promise<boolean> {
    if (!resource.metadata?.name) return false
    const name = resource.metadata.name
    const ns = resource.metadata.namespace || 'default'
    const targetSuspend = suspendState !== undefined
      ? suspendState
      : !Boolean((resource.spec as { suspend?: boolean })?.suspend)
    await k8sApi.suspendCronJob(cluster, name, ns, targetSuspend)
    return targetSuspend
  }

  return {
    scalingResource,
    scaleWorkload,
    restartWorkload,
    triggerCronJob,
    suspendCronJob,
  }
}
