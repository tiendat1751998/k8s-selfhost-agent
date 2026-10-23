import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { getTargetBadge, cleanFingerprint } from '../src/components/logs/virtualLogUtils.ts'
import type { LogEntry } from '../src/stores/logStore.ts'

describe('Log Decluttering Helpers (virtualLogUtils)', () => {
  describe('cleanFingerprint', () => {
    it('strips wildcard timestamp and bracketed worker noise', () => {
      const raw = '*-*-* *:*:*.* UTC [*] ERROR: invalid input syntax for type integer'
      assert.equal(cleanFingerprint(raw), 'ERROR: invalid input syntax for type integer')
    })

    it('strips concrete ISO timestamps and thread identifiers', () => {
      const raw = '2026-10-23 04:12:34.123 UTC [12345] FATAL: terminating connection due to administrator command'
      assert.equal(cleanFingerprint(raw), 'FATAL: terminating connection due to administrator command')
    })

    it('strips ISO8601 timestamps with T and Z', () => {
      const raw = '2026-10-23T11:20:00.000Z [worker-1] Error: ECONNREFUSED 127.0.0.1:5432'
      assert.equal(cleanFingerprint(raw), 'Error: ECONNREFUSED 127.0.0.1:5432')
    })

    it('preserves clean error messages unchanged', () => {
      const raw = 'ERROR: relation "workloads" does not exist'
      assert.equal(cleanFingerprint(raw), 'ERROR: relation "workloads" does not exist')
    })

    it('handles empty or undefined safely', () => {
      assert.equal(cleanFingerprint(''), '')
      assert.equal(cleanFingerprint(undefined as any), '')
    })
  })

  describe('getTargetBadge', () => {
    const sampleLog: LogEntry = {
      id: 'log-1',
      time: '2026-10-23T11:00:00Z',
      level: 'ERROR',
      msg: 'database connection failed',
      node: 'k8s-node-1',
      service: 'auth-service',
      pod: 'auth-service-7f6c9d',
      namespace: 'production',
    }

    it('returns full node/service when no current target is active', () => {
      assert.equal(getTargetBadge(sampleLog), 'k8s-node-1/auth-service')
    })

    it('returns ultra-compact service when current target matches node', () => {
      assert.equal(getTargetBadge(sampleLog, 'k8s-node-1'), 'auth-service')
    })

    it('returns ultra-compact service when current target name contains node', () => {
      assert.equal(getTargetBadge(sampleLog, 'k8s-node-1 (Control-Plane)'), 'auth-service')
    })

    it('returns service directly when node is not present', () => {
      const noNodeLog: LogEntry = {
        ...sampleLog,
        node: '',
      }
      assert.equal(getTargetBadge(noNodeLog), 'auth-service')
    })

    it('falls back to namespace/pod when service is empty', () => {
      const podLog: LogEntry = {
        id: 'log-2',
        time: '2026-10-23T11:00:00Z',
        level: 'INFO',
        msg: 'started',
        node: '',
        service: '',
        pod: 'backend-pod-xyz',
        namespace: 'monitoring',
      }
      assert.equal(getTargetBadge(podLog), 'monitoring/backend-pod-xyz')
    })
  })
})
