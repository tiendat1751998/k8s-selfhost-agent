import { ref } from 'vue'
import { k8sApi } from '../../../../api/k8s'
import type { DrainOptions, K8sResource, NodeTaint } from '../../types'

export function useNodeOps() {
  const operatingNode = ref(false)
  const drainingNode = ref(false)
  const updatingTaints = ref(false)
  const updatingLabels = ref(false)

  async function cordonNode(cluster: string, name: string): Promise<void> {
    operatingNode.value = true
    try {
      await k8sApi.cordonNode(cluster, name)
    } finally {
      operatingNode.value = false
    }
  }

  async function uncordonNode(cluster: string, name: string): Promise<void> {
    operatingNode.value = true
    try {
      await k8sApi.uncordonNode(cluster, name)
    } finally {
      operatingNode.value = false
    }
  }

  async function drainNode(cluster: string, name: string, options: DrainOptions): Promise<void> {
    drainingNode.value = true
    try {
      await k8sApi.drainNode(cluster, name, options)
    } finally {
      drainingNode.value = false
    }
  }

  async function addTaint(cluster: string, node: K8sResource, taint: NodeTaint): Promise<void> {
    const name = node.metadata?.name
    if (!name) return
    updatingTaints.value = true
    try {
      const currentTaints = (node.spec as { taints?: NodeTaint[] })?.taints || []
      const newTaints = [...currentTaints, taint]
      await k8sApi.updateNodeTaints(cluster, name, newTaints)
    } finally {
      updatingTaints.value = false
    }
  }

  async function removeTaint(cluster: string, node: K8sResource, index: number): Promise<void> {
    const name = node.metadata?.name
    if (!name) return
    updatingTaints.value = true
    try {
      const currentTaints = [...((node.spec as { taints?: NodeTaint[] })?.taints || [])]
      currentTaints.splice(index, 1)
      await k8sApi.updateNodeTaints(cluster, name, currentTaints)
    } finally {
      updatingTaints.value = false
    }
  }

  async function saveNodeLabels(cluster: string, node: K8sResource, labels: Record<string, string>): Promise<void> {
    const name = node.metadata?.name
    if (!name) return
    updatingLabels.value = true
    try {
      await k8sApi.updateNodeLabels(cluster, name, labels)
    } finally {
      updatingLabels.value = false
    }
  }

  return {
    operatingNode,
    drainingNode,
    updatingTaints,
    updatingLabels,
    cordonNode,
    uncordonNode,
    drainNode,
    addTaint,
    removeTaint,
    saveNodeLabels,
  }
}
