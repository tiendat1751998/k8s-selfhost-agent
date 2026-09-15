import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import {
  isNoiseLog,
  isSystemdService,
  classifyLogTarget,
  getServiceIcon,
} from '../src/utils/logTargetCategorizer.ts'

describe('LogTargetCategorizer', () => {
  describe('isNoiseLog', () => {
    it('filters out rotated logs matching .\d+$', () => {
      assert.equal(isNoiseLog('vmware-network.1'), true)
      assert.equal(isNoiseLog('vmware-network.2'), true)
      assert.equal(isNoiseLog('vmware-network.7'), true)
      assert.equal(isNoiseLog('syslog.1'), true)
      assert.equal(isNoiseLog('auth.log.1'), true)
      assert.equal(isNoiseLog('dpkg.log.2'), true)
    })

    it('filters out OS maintenance and installer file noise', () => {
      const noiseList = [
        'alternatives',
        'apport',
        'dpkg',
        'bootstrap',
        'cloud-init',
        'cloud-init-output',
        'ubuntu-advantage',
        'fontconfig',
        'faillog',
        'lastlog',
        'wtmp',
        'btmp',
      ]
      for (const name of noiseList) {
        assert.equal(isNoiseLog(name), true, `Expected ${name} to be identified as noise`)
        assert.equal(isNoiseLog(`${name}.log`), true, `Expected ${name}.log to be identified as noise`)
      }
    })

    it('allows valid container workloads and systemd services', () => {
      assert.equal(isNoiseLog('postgres_db'), false)
      assert.equal(isNoiseLog('tiki_redis'), false)
      assert.equal(isNoiseLog('nats'), false)
      assert.equal(isNoiseLog('traefik'), false)
      assert.equal(isNoiseLog('my-nginx'), false)
      assert.equal(isNoiseLog('vgauth.service'), false)
      assert.equal(isNoiseLog('dbus.service'), false)
      assert.equal(isNoiseLog('chrony.service'), false)
      assert.equal(isNoiseLog('systemd-resolved.service'), false)
    })
  })

  describe('isSystemdService', () => {
    it('identifies systemd units by suffix (.service, .socket, .target, .slice)', () => {
      assert.equal(isSystemdService('vgauth.service'), true)
      assert.equal(isSystemdService('dbus.service'), true)
      assert.equal(isSystemdService('chrony.service'), true)
      assert.equal(isSystemdService('systemd-resolved.service'), true)
      assert.equal(isSystemdService('docker.socket'), true)
      assert.equal(isSystemdService('multi-user.target'), true)
      assert.equal(isSystemdService('system.slice'), true)
    })

    it('returns false for container workloads', () => {
      assert.equal(isSystemdService('postgres_db'), false)
      assert.equal(isSystemdService('tiki_redis'), false)
      assert.equal(isSystemdService('nats'), false)
      assert.equal(isSystemdService('traefik'), false)
      assert.equal(isSystemdService('my-nginx'), false)
    })
  })

  describe('classifyLogTarget', () => {
    it('classifies container workloads as category app with Container subtitle', () => {
      const target = classifyLogTarget('postgres_db')
      assert.equal(target.category, 'app')
      assert.equal(target.type, 'Container')
      assert.equal(target.icon, 'database')
    })

    it('classifies systemd services as category systemd with SystemD Service subtitle and cpu icon', () => {
      const target = classifyLogTarget('vgauth.service')
      assert.equal(target.category, 'systemd')
      assert.equal(target.type, 'SystemD Service')
      assert.equal(target.icon, 'cpu')
    })
  })

  describe('getServiceIcon', () => {
    it('returns cpu for all systemd services, sockets, and targets', () => {
      assert.equal(getServiceIcon('dbus.service'), 'cpu')
      assert.equal(getServiceIcon('chrony.service'), 'cpu')
      assert.equal(getServiceIcon('systemd-resolved.service'), 'cpu')
      assert.equal(getServiceIcon('udev.socket'), 'cpu')
      assert.equal(getServiceIcon('multi-user.target'), 'cpu')
    })

    it('prevents vgauth.service from matching "auth" and receiving lock icon', () => {
      assert.notEqual(getServiceIcon('vgauth.service'), 'lock')
      assert.equal(getServiceIcon('vgauth.service'), 'cpu')
    })

    it('maps real container services to their respective domain icons', () => {
      assert.equal(getServiceIcon('postgres_db'), 'database')
      assert.equal(getServiceIcon('tiki_redis'), 'database')
      assert.equal(getServiceIcon('traefik'), 'radio')
      assert.equal(getServiceIcon('my-nginx'), 'radio')
      assert.equal(getServiceIcon('nats'), 'zap')
      assert.equal(getServiceIcon('auth-service'), 'lock')
      assert.equal(getServiceIcon('prom-agent'), 'cloud')
      assert.equal(getServiceIcon('monitoring'), 'activity')
      assert.equal(getServiceIcon('unknown-workload'), 'sliders')
    })
  })
})
