import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { getTargetBadge, cleanFingerprint, cleanLogMessage } from '../src/components/logs/virtualLogUtils.ts'
import type { LogEntry } from '../src/stores/logStore.ts'

describe('Log Decluttering Helpers (virtualLogUtils)', () => {
  describe('cleanLogMessage', () => {
    it('cleans json logs with level and error fields', () => {
      const raw = '{"level":"error","error":"command traefik error: field not found, node: entryPoint","time":"2026-10-23T04:29:10Z"}'
      assert.equal(cleanLogMessage(raw), 'command traefik error: field not found, node: entryPoint')
    })

    it('cleans json logs with message and extra tags', () => {
      const raw = '{"ts":"2026-10-23T04:00:00Z","level":"info","msg":"HTTP server started","port":8080}'
      assert.equal(cleanLogMessage(raw), 'HTTP server started port=8080')
    })

    it('cleans postgres UTC timestamp and redundant ERROR prefix with PID', () => {
      const raw = '2026-10-23 02:38:25.172 UTC [668659] ERROR:  invalid input syntax for type uuid: ""'
      assert.equal(cleanLogMessage(raw), '[668659] invalid input syntax for type uuid: ""')
    })

    it('cleans postgres UTC timestamp and redundant LOG prefix without PID', () => {
      const raw = '2026-10-23 02:38:25.172 UTC LOG:  database system is ready to accept connections'
      assert.equal(cleanLogMessage(raw), 'database system is ready to accept connections')
    })

    it('preserves clean text logs as is', () => {
      const raw = 'worker pool initialized with 4 workers'
      assert.equal(cleanLogMessage(raw), 'worker pool initialized with 4 workers')
    })

    it('safely handles empty or null values', () => {
      assert.equal(cleanLogMessage(''), '')
      assert.equal(cleanLogMessage(undefined as any), '')
    })
  })

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

  describe('Service Tree Decluttering Logic', () => {
    interface ServiceItem {
      id: string
      name: string
      category: 'app' | 'systemd'
      count: number
    }

    const services: ServiceItem[] = [
      { id: 'dbus.service', name: 'dbus.service', category: 'systemd', count: 0 },
      { id: 'postgres', name: 'postgres', category: 'app', count: 12 },
      { id: 'modemmanager.service', name: 'ModemManager.service', category: 'systemd', count: 0 },
      { id: 'traefik.service', name: 'traefik.service', category: 'systemd', count: 45 },
      { id: 'api-server', name: 'api-server', category: 'app', count: 5 },
      { id: 'apt-daily-upgrade.service', name: 'apt-daily-upgrade.service', category: 'systemd', count: 0 },
    ]

    it('sorts app containers before systemd daemons, then by count descending', () => {
      const sorted = [...services].sort((a, b) => {
        if (a.category !== b.category) {
          return a.category === 'app' ? -1 : 1
        }
        return b.count - a.count
      })

      assert.deepEqual(
        sorted.map((s) => s.id),
        ['postgres', 'api-server', 'traefik.service', 'dbus.service', 'modemmanager.service', 'apt-daily-upgrade.service']
      )
    })

    it('filters out 0-log idle services unless selected or muted', () => {
      const activeService = 'dbus.service' // selected
      const mutedServices = ['modemmanager.service'] // muted

      const activeList = services.filter((s) => {
        const isSelected = activeService.toLowerCase() === s.id.toLowerCase()
        const isMuted = mutedServices.some((m) => m.toLowerCase() === s.id.toLowerCase())
        return s.count > 0 || isSelected || isMuted
      })

      const idleList = services.filter((s) => {
        const isSelected = activeService.toLowerCase() === s.id.toLowerCase()
        const isMuted = mutedServices.some((m) => m.toLowerCase() === s.id.toLowerCase())
        return s.count === 0 && !isSelected && !isMuted
      })

      assert.equal(activeList.length, 5) // postgres, api-server, traefik, plus dbus & modemmanager because selected/muted
      assert.equal(idleList.length, 1) // apt-daily-upgrade.service is isolated as idle
      assert.equal(idleList[0].id, 'apt-daily-upgrade.service')
    })

    it('hides all 0-log services when none are selected or muted', () => {
      const activeList = services.filter((s) => s.count > 0)
      const idleList = services.filter((s) => s.count === 0)

      assert.equal(activeList.length, 3)
      assert.deepEqual(activeList.map((s) => s.id), ['postgres', 'traefik.service', 'api-server'])
      assert.equal(idleList.length, 3)
      assert.deepEqual(idleList.map((s) => s.id), ['dbus.service', 'modemmanager.service', 'apt-daily-upgrade.service'])
    })

  })
})

