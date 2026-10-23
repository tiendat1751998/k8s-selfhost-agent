import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { getTargetBadge, cleanFingerprint, cleanLogMessage, coalesceLogEntries, inferEffectiveLevel } from '../src/components/logs/virtualLogUtils.ts'
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

    it('combines msg and error when both exist and are distinct', () => {
      const raw = '{"error":"command traefik error: field not found, node: entryPoint","level":"error","msg":"Command error","time":"2026-10-23T04:29:10Z"}'
      assert.equal(cleanLogMessage(raw), 'Command error: command traefik error: field not found, node: entryPoint')
    })

    it('cleans docker logfmt format stripping time and level and extracting msg', () => {
      const raw = 'time="2026-10-23T12:03:09.528346220+07:00" level=info msg="NetworkDB stats k8smater(7da4e97006e0) - indexes: 0, entries: 0"'
      assert.equal(cleanLogMessage(raw), 'NetworkDB stats k8smater(7da4e97006e0) - indexes: 0, entries: 0')
    })

    it('strips redundant tag-only log lines like [db] [INFO]', () => {
      assert.equal(cleanLogMessage('[db] [INFO]'), '')
      assert.equal(cleanLogMessage('[postgres] [ERR]'), '')
      assert.equal(cleanLogMessage('[traefik] [DEBUG]'), '')
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

  describe('coalesceLogEntries', () => {
    it('collapses consecutive identical logs and counts repeats', () => {
      const logs: LogEntry[] = [
        {
          id: '1',
          time: '2026-10-23T04:29:10Z',
          level: 'error',
          node: 'k8smater133',
          service: 'traefik.service',
          msg: '{"error":"command traefik error: field not found, node: entryPoint","level":"error","msg":"Command error","time":"2026-10-23T04:29:10Z"}',
        },
        {
          id: '2',
          time: '2026-10-23T04:29:13Z',
          level: 'error',
          node: 'k8smater133',
          service: 'traefik.service',
          msg: '{"error":"command traefik error: field not found, node: entryPoint","level":"error","msg":"Command error","time":"2026-10-23T04:29:13Z"}',
        },
        {
          id: '3',
          time: '2026-10-23T04:29:16Z',
          level: 'error',
          node: 'k8smater133',
          service: 'traefik.service',
          msg: '{"error":"command traefik error: field not found, node: entryPoint","level":"error","msg":"Command error","time":"2026-10-23T04:29:16Z"}',
        },
        {
          id: '4',
          time: '2026-10-23T04:29:20Z',
          level: 'info',
          node: 'k8smater133',
          service: 'traefik.service',
          msg: 'reloaded configuration',
        },
      ]

      const coalesced = coalesceLogEntries(logs)
      assert.equal(coalesced.length, 2)
      assert.equal(coalesced[0].repeatCount, 3)
      assert.equal(coalesced[0].log.time, '2026-10-23T04:29:16Z') // reflects latest timestamp
      assert.equal(coalesced[0].cleanMsg, 'Command error: command traefik error: field not found, node: entryPoint')
      assert.equal(coalesced[1].repeatCount, 1)
      assert.equal(coalesced[1].cleanMsg, 'reloaded configuration')
    })

    it('merges fragmented multi-line Postgres SQL and context lines into parent error', () => {
      const logs: LogEntry[] = [
        {
          id: 'p1',
          time: '2026-10-23T05:03:20Z',
          level: 'error',
          node: 'k8smater133',
          service: 'db',
          msg: '2026-10-23 05:03:20.123 UTC [100] ERROR:  relation "slo_definitions" does not exist',
        },
        {
          id: 'p2',
          time: '2026-10-23T05:03:20Z',
          level: 'info',
          node: 'k8smater133',
          service: 'db',
          msg: 'STATEMENT:  SELECT id, service, slo_type, target_ratio',
        },
        {
          id: 'p3',
          time: '2026-10-23T05:03:20Z',
          level: 'info',
          node: 'k8smater133',
          service: 'db',
          msg: 'FROM slo_definitions',
        },
        {
          id: 'p4',
          time: '2026-10-23T05:03:20Z',
          level: 'info',
          node: 'k8smater133',
          service: 'db',
          msg: 'WHERE tenant_id = $1;',
        },
      ]

      const coalesced = coalesceLogEntries(logs)
      assert.equal(coalesced.length, 1)
      assert.equal(coalesced[0].mergedLinesCount, 3)
      assert.equal(coalesced[0].cleanMsg, '[100] relation "slo_definitions" does not exist')
      assert.ok(coalesced[0].fullMsg.includes('STATEMENT:  SELECT id, service, slo_type, target_ratio'))
      assert.ok(coalesced[0].fullMsg.includes('WHERE tenant_id = $1;'))
    })

    it('filters out empty or tag-only noise logs ([db] [INFO]) during coalescing', () => {
      const logs: LogEntry[] = [
        {
          id: 't1',
          time: '2026-10-23T05:03:25Z',
          level: 'info',
          node: 'k8smater133',
          service: 'db',
          msg: '[db] [INFO]',
        },
        {
          id: 't2',
          time: '2026-10-23T05:03:26Z',
          level: 'info',
          node: 'k8smater133',
          service: 'db',
          msg: 'checkpoint starting: time',
        },
      ]

      const coalesced = coalesceLogEntries(logs)
      assert.equal(coalesced.length, 1)
      assert.equal(coalesced[0].cleanMsg, 'checkpoint starting: time')
    })

    it('coalesces Postgres error with bracketed PID CONTEXT and STATEMENT continuation lines into 1 row', () => {
      const logs: LogEntry[] = [
        {
          id: 'p1',
          time: '2026-10-23 05:58:25.186',
          level: 'INFO', // journald/docker stdout mislabeling
          node: 'k8smater133',
          service: 'db',
          msg: '2026-10-23 05:58:25.186 UTC [677906] ERROR:  invalid input syntax for type uuid: ""',
        },
        {
          id: 'p2',
          time: '2026-10-23 05:58:25.186',
          level: 'INFO',
          node: 'k8smater133',
          service: 'db',
          msg: '2026-10-23 05:58:25.186 UTC [677906] CONTEXT:  unnamed portal parameter $1 = \'\'',
        },
        {
          id: 'p3',
          time: '2026-10-23 05:58:25.186',
          level: 'INFO',
          node: 'k8smater133',
          service: 'db',
          msg: '2026-10-23 05:58:25.186 UTC [677906] STATEMENT:  SELECT id, service, slo_type',
        },
        {
          id: 'p4',
          time: '2026-10-23 05:58:25.186',
          level: 'INFO',
          node: 'k8smater133',
          service: 'db',
          msg: 'FROM slo_definitions',
        },
        {
          id: 'p5',
          time: '2026-10-23 05:58:25.186',
          level: 'INFO',
          node: 'k8smater133',
          service: 'db',
          msg: 'WHERE tenant_id = $1 ORDER BY created_at DESC',
        },
      ]

      const coalesced = coalesceLogEntries(logs)
      assert.equal(coalesced.length, 1, 'Should coalesce all 5 lines into 1 single row')
      assert.equal(coalesced[0].mergedLinesCount, 4, 'Should merge 4 continuation lines')
      assert.equal(coalesced[0].log.level, 'ERROR', 'Parent log level should be promoted to ERROR')
      assert.ok(coalesced[0].fullMsg.includes('CONTEXT:  unnamed portal parameter $1 = \'\''))
      assert.ok(coalesced[0].fullMsg.includes('STATEMENT:  SELECT id, service, slo_type'))
      assert.ok(coalesced[0].fullMsg.includes('WHERE tenant_id = $1 ORDER BY created_at DESC'))
    })
  })

  describe('inferEffectiveLevel', () => {
    it('infers ERROR from JSON payload when rawLevel is INFO', () => {
      const raw = '{"level":"error","error":"command traefik error","msg":"Command error"}'
      assert.equal(inferEffectiveLevel('INFO', raw), 'ERROR')
    })

    it('infers WARN from JSON payload when rawLevel is INFO', () => {
      const raw = '{"level":"warn","msg":"configuration warning"}'
      assert.equal(inferEffectiveLevel('INFO', raw), 'WARN')
    })

    it('infers ERROR from severity field in JSON payload', () => {
      const raw = '{"severity":"error","message":"database failed"}'
      assert.equal(inferEffectiveLevel('INFO', raw), 'ERROR')
    })

    it('infers ERROR from logfmt level=error', () => {
      const raw = 'time="2026-10-23T04:00:00Z" level=error msg="failed to bind port"'
      assert.equal(inferEffectiveLevel('INFO', raw), 'ERROR')
    })

    it('infers ERROR from message matching ERROR: pattern', () => {
      const raw = '2026-10-23 05:58:25.186 UTC [677906] ERROR:  invalid input syntax for type uuid: ""'
      assert.equal(inferEffectiveLevel('INFO', raw), 'ERROR')
    })

    it('infers ERROR from FATAL or PANIC prefix', () => {
      assert.equal(inferEffectiveLevel('INFO', 'FATAL: terminating connection'), 'ERROR')
      assert.equal(inferEffectiveLevel('INFO', 'PANIC: runtime out of memory'), 'ERROR')
    })

    it('infers WARN from WARNING: prefix', () => {
      const raw = '2026-10-23 05:58:25.186 UTC [677906] WARNING: disk space low'
      assert.equal(inferEffectiveLevel('INFO', raw), 'WARN')
    })

    it('preserves non-INFO level unchanged', () => {
      assert.equal(inferEffectiveLevel('WARN', 'some normal text'), 'WARN')
      assert.equal(inferEffectiveLevel('DEBUG', 'some normal text'), 'DEBUG')
      assert.equal(inferEffectiveLevel('ERROR', 'some normal text'), 'ERROR')
    })

    it('returns INFO for normal info logs', () => {
      assert.equal(inferEffectiveLevel('INFO', 'Server listening on :8080'), 'INFO')
      assert.equal(inferEffectiveLevel('info', 'Started successfully'), 'INFO')
    })
  })

  describe('LogTargetItem Badge Visibility Logic', () => {
    function shouldShowCountBadge(hideZeroCount?: boolean, count?: number): boolean {
      return !(hideZeroCount && !count)
    }

    it('hides count badge when hideZeroCount is true and count is 0 or undefined', () => {
      assert.equal(shouldShowCountBadge(true, 0), false)
      assert.equal(shouldShowCountBadge(true, undefined), false)
    })

    it('shows count badge when count is greater than 0 even if hideZeroCount is true', () => {
      assert.equal(shouldShowCountBadge(true, 42), true)
      assert.equal(shouldShowCountBadge(true, 1), true)
    })

    it('shows count badge when hideZeroCount is false or undefined even if count is 0', () => {
      assert.equal(shouldShowCountBadge(false, 0), true)
      assert.equal(shouldShowCountBadge(undefined, 0), true)
    })
  })
})

