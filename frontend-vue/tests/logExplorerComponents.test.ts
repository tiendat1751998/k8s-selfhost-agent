import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { register } from 'node:module'

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

export async function load(url, context, nextLoad) {
  if (url.includes('client.ts') || url.includes('/client')) {
    return {
      format: 'module',
      shortCircuit: true,
      source: 'export const api = { get: async () => ({ status: "ok", data: [] }), post: async () => ({}) };'
    };
  }
  return nextLoad(url, context);
}
`))

if (typeof globalThis.window === 'undefined') {
  // @ts-expect-error Mock window for test environment
  globalThis.window = {
    location: { protocol: 'http:', host: 'localhost:3000', pathname: '/', search: '' }
  }
}

const { normalizeHistogramBuckets } = await import('../src/api/logging.ts')

describe('Phase 8B: Log Histogram Response Unwrapping & Mock Parity', () => {
  describe('normalizeHistogramBuckets unwrapping logic', () => {
    it('1. Handles direct array of buckets without throwing', () => {
      const direct = [
        {
          timestamp: '2026-10-14T03:00:00Z',
          total: 10,
          error_count: 2,
          warn_count: 1,
          info_count: 5,
          debug_count: 2,
        },
      ]
      const result = normalizeHistogramBuckets(direct)
      assert.ok(Array.isArray(result), 'Result must be an array')
      assert.equal(result.length, 1)
      assert.equal(result[0].total, 10)
    })

    it('2. Unwraps wrapped { status: "ok", data: [...] } without calling map on object', () => {
      const wrapped = {
        status: 'ok',
        data: [
          {
            time_bucket: '2026-10-14T03:00:00Z',
            total_count: 15,
            level_count: { error: 1, info: 14 },
          },
        ],
      }
      const result = normalizeHistogramBuckets(wrapped)
      assert.ok(Array.isArray(result), 'Result must be an array')
      assert.equal(result.length, 1)
      assert.equal(result[0].total_count, 15)
    })

    it('3. Unwraps wrapped { buckets: [...] } objects correctly', () => {
      const wrapped = {
        buckets: [
          {
            timestamp: '2026-10-14T03:05:00Z',
            total: 8,
          },
        ],
      }
      const result = normalizeHistogramBuckets(wrapped)
      assert.ok(Array.isArray(result), 'Result must be an array')
      assert.equal(result.length, 1)
      assert.equal(result[0].total, 8)
    })

    it('4. Gracefully handles null, undefined, empty objects, and primitives returning []', () => {
      assert.deepEqual(normalizeHistogramBuckets(null), [])
      assert.deepEqual(normalizeHistogramBuckets(undefined), [])
      assert.deepEqual(normalizeHistogramBuckets({}), [])
      assert.deepEqual(normalizeHistogramBuckets({ status: 'ok' }), [])
      assert.deepEqual(normalizeHistogramBuckets('not an array'), [])
      assert.deepEqual(normalizeHistogramBuckets(12345), [])
      assert.deepEqual(normalizeHistogramBuckets(false), [])
    })

    it('5. Correctly handles multi-item buckets with mixed time properties', () => {
      const mixed = [
        { time_bucket: '2026-10-14T04:00:00Z', total_count: 25 },
        { timestamp: '2026-10-14T04:01:00Z', total: 42 }
      ]
      const result = normalizeHistogramBuckets(mixed)
      assert.equal(result.length, 2)
      assert.equal(result[0].total_count, 25)
      assert.equal(result[1].total, 42)
    })
  })
})
