import { describe, it } from 'node:test'
import assert from 'node:assert/strict'

export interface WorkloadLogRouteQuery {
  app?: string
  pod?: string
  namespace?: string
  cluster?: string
}

export function parseWorkloadLogParams(query: Record<string, any>): {
  appName: string
  selectedPod: string
  namespace: string
  clusterId: string
} {
  return {
    appName: typeof query.app === 'string' ? query.app.trim() : '',
    selectedPod: typeof query.pod === 'string' ? query.pod.trim() : 'all',
    namespace: typeof query.namespace === 'string' ? query.namespace.trim() : 'default',
    clusterId: typeof query.cluster === 'string' ? query.cluster.trim() : ''
  }
}

export function buildPodSelectorOptions(pods: string[]): Array<{ label: string; value: string }> {
  const options = [{ label: 'All Pods (Aggregated)', value: 'all' }]
  for (const pod of pods) {
    if (pod && pod !== 'all') {
      options.push({ label: pod, value: pod })
    }
  }
  return options
}

export function buildWorkloadLogEndpoint(clusterId: string, namespace: string, appName: string): string {
  const c = encodeURIComponent(clusterId || 'default')
  const ns = encodeURIComponent(namespace || 'default')
  const app = encodeURIComponent(appName)
  return `/k8s/${c}/namespaces/${ns}/workloads/${app}/logs`
}

describe('Task 015: Workload Logs Drawer & Log Stream Integration Specification', () => {
  it('parseWorkloadLogParams extracts app, pod, namespace, and cluster safely', () => {
    const query = {
      app: 'frontend-checkout',
      pod: 'frontend-checkout-84d9f-x1',
      namespace: 'production',
      cluster: 'k8s-prod-us'
    }
    const parsed = parseWorkloadLogParams(query)
    assert.equal(parsed.appName, 'frontend-checkout')
    assert.equal(parsed.selectedPod, 'frontend-checkout-84d9f-x1')
    assert.equal(parsed.namespace, 'production')
    assert.equal(parsed.clusterId, 'k8s-prod-us')
  })

  it('parseWorkloadLogParams falls back to defaults on missing parameters', () => {
    const emptyParsed = parseWorkloadLogParams({})
    assert.equal(emptyParsed.appName, '')
    assert.equal(emptyParsed.selectedPod, 'all')
    assert.equal(emptyParsed.namespace, 'default')
    assert.equal(emptyParsed.clusterId, '')
  })

  it('buildPodSelectorOptions prepends All Pods (Aggregated) option', () => {
    const pods = ['core-api-0', 'core-api-1', 'core-api-2']
    const options = buildPodSelectorOptions(pods)
    assert.equal(options.length, 4)
    assert.deepEqual(options[0], { label: 'All Pods (Aggregated)', value: 'all' })
    assert.deepEqual(options[1], { label: 'core-api-0', value: 'core-api-0' })
    assert.deepEqual(options[2], { label: 'core-api-1', value: 'core-api-1' })
    assert.deepEqual(options[3], { label: 'core-api-2', value: 'core-api-2' })
  })

  it('buildWorkloadLogEndpoint constructs standard REST endpoint without rogue prefixes', () => {
    const endpoint = buildWorkloadLogEndpoint('prod-eu', 'finance', 'payment-gateway')
    assert.equal(endpoint, '/k8s/prod-eu/namespaces/finance/workloads/payment-gateway/logs')
  })
})
