import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { register } from 'node:module'
import { ref } from 'vue'

// Register loader hook for headless node resolution of typescript extensions
register('data:text/javascript,' + encodeURIComponent(`
export async function resolve(specifier, context, nextResolve) {
  try {
    return await nextResolve(specifier, context);
  } catch (err) {
    if (specifier.startsWith('.')) {
      try {
        return await nextResolve(specifier + '.ts', context);
      } catch {}
    }
    throw err;
  }
}
`))

const { useLogTargets } = await import('../src/composables/useLogTargets.ts')
import type { LogTarget } from '../src/composables/useLogTargets.ts'
import type { LogEntry } from '../src/stores/logStore.ts'

describe('useLogTargets', () => {
  it('computes nodeServicesMap and sorts app category before systemd category', () => {
    const selectedTarget: LogTarget = { type: 'node', id: 'k8smater133', name: 'k8smater133' }
    const logs: LogEntry[] = [
      {
        time: '2026-10-22T00:00:01Z',
        level: 'INFO',
        namespace: 'default',
        pod: 'pod-1',
        node: 'k8smater133',
        service: 'traefik.service',
        msg: 'traefik starting'
      },
      {
        time: '2026-10-22T00:00:02Z',
        level: 'INFO',
        namespace: 'default',
        pod: 'pod-2',
        node: 'k8smater133',
        service: 'postgres_db',
        msg: 'database ready'
      },
      {
        time: '2026-10-22T00:00:03Z',
        level: 'INFO',
        namespace: 'default',
        pod: 'pod-3',
        node: 'k8smater133',
        service: 'init.scope', // noise -> should be ignored!
        msg: 'noise line'
      },
      {
        time: '2026-10-22T00:00:04Z',
        level: 'INFO',
        namespace: 'default',
        pod: 'pod-4',
        node: 'k8sworker',
        service: 'my-nginx',
        msg: 'nginx ready'
      }
    ]

    let emittedTarget: LogTarget | null = null
    const { getNodeServices, getNodeServiceCount } = useLogTargets(
      { modelValue: selectedTarget, logs },
      (event: string, target: LogTarget) => {
        emittedTarget = target
      }
    )

    const masterServices = getNodeServices('k8smater133')
    assert.equal(masterServices.length, 2, 'Noise log init.scope should be filtered out')
    assert.equal(masterServices[0].id, 'postgres_db')
    assert.equal(masterServices[0].category, 'app', 'App workloads must come first')
    assert.equal(masterServices[1].id, 'traefik.service')
    assert.equal(masterServices[1].category, 'systemd')

    assert.equal(getNodeServiceCount('k8smater133', 'postgres_db'), 1)
    assert.equal(getNodeServiceCount('k8smater133', 'traefik.service'), 1)
    assert.equal(getNodeServiceCount('k8smater133', 'my-nginx'), 0)

    const workerServices = getNodeServices('k8sworker')
    assert.equal(workerServices.length, 1)
    assert.equal(workerServices[0].id, 'my-nginx')
  })
})
