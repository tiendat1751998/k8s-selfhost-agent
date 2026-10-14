import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
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

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

const { normalizeHistogramBuckets } = await import('../src/api/logging.ts')
const { createMockServer } = await import('../scripts/mock-preview-server.js')

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
  })

  describe('LogVolumeHistogram.vue template & prop safety', () => {
    const componentPath = path.join(rootDir, 'src', 'components', 'logs', 'LogVolumeHistogram.vue')

    it('1. LogVolumeHistogram.vue guards buckets prop with Array.isArray', () => {
      assert.ok(fs.existsSync(componentPath), 'LogVolumeHistogram.vue must exist')
      const content = fs.readFileSync(componentPath, 'utf-8')

      // Must guard against non-array props.buckets before mapping
      assert.match(content, /Array\.isArray\s*\(\s*props\.buckets\s*\)/, 'Must check Array.isArray(props.buckets)')
      // Must support both time_bucket and timestamp
      assert.match(content, /timestamp/, 'Must handle timestamp field parity')
    })
  })

  describe('mock-preview-server.js /api/v1/logs/histogram parity', () => {
    it('1. Responds to /api/v1/logs/histogram with realistic LogAggregationBucket array', async () => {
      const server = createMockServer()
      await new Promise((resolve) => server.listen(0, '127.0.0.1', () => resolve(null)))
      const addr = server.address()
      const port = typeof addr === 'object' && addr ? addr.port : 8080

      try {
        const res = await fetch(`http://127.0.0.1:${port}/api/v1/logs/histogram?interval_seconds=60`)
        assert.equal(res.status, 200, 'HTTP status must be 200')
        const body = (await res.json()) as any

        assert.ok(Array.isArray(body), 'Response must be a direct JSON array matching Go backend')
        assert.ok(body.length > 0, 'Must return at least one bucket')

        const bucket = body[0]
        assert.ok('timestamp' in bucket || 'time_bucket' in bucket, 'Bucket must have timestamp or time_bucket')
        assert.ok('total' in bucket || 'total_count' in bucket, 'Bucket must have total or total_count')
        assert.ok('error_count' in bucket || 'level_count' in bucket, 'Bucket must have error_count or level_count')
      } finally {
        await new Promise((resolve) => server.close(() => resolve(null)))
      }
    })
  })

  describe('File Modularity and Clean Architecture Constraints', () => {
    it('All touched files must strictly stay < 500 lines', () => {
      const filesToCheck = [
        path.join(rootDir, 'src', 'api', 'logging.ts'),
        path.join(rootDir, 'src', 'components', 'logs', 'LogVolumeHistogram.vue'),
        path.join(rootDir, 'scripts', 'mock-preview-server.js'),
        path.join(rootDir, 'tests', 'logExplorerComponents.test.ts'),
      ]

      for (const file of filesToCheck) {
        if (fs.existsSync(file)) {
          const lines = fs.readFileSync(file, 'utf-8').split('\n').length
          assert.ok(lines < 500, `${path.basename(file)} has ${lines} lines, must be < 500`)
        }
      }
    })
  })
})
