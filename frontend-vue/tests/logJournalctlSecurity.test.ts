import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { register } from 'node:module'
import * as vueModule from 'vue'
import { h, defineComponent, ref } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { parse, compileScript } from '@vue/compiler-sfc'
import ts from 'typescript'
import { isHostOrSystemUnit } from '../src/utils/logTargetCategorizer.ts'

register('data:text/javascript,' + encodeURIComponent(`
export async function resolve(specifier, context, nextResolve) {
  try { return await nextResolve(specifier, context); } catch (err) {
    if (specifier.startsWith('.')) {
      try { return await nextResolve(specifier + '.ts', context); } catch {}
    }
    throw err;
  }
}`))

function resolveFilePath(rel: string): string {
  for (const c of [path.resolve(process.cwd(), rel), path.resolve(process.cwd(), 'frontend-vue', rel), path.resolve(process.cwd(), '..', rel)]) {
    if (fs.existsSync(c)) return c
  }
  return path.resolve(process.cwd(), rel)
}

function loadComponent(rel: string) {
  const fullPath = resolveFilePath(rel)
  const source = fs.readFileSync(fullPath, 'utf8')
  const { descriptor } = parse(source, { filename: path.basename(fullPath) })
  const compiled = compileScript(descriptor, { id: 'test-' + Math.random().toString(36).slice(2), inlineTemplate: true })
  const BaseIconStub = defineComponent({
    props: ['name', 'size', 'class'],
    render() { return h('svg', { class: ['base-icon', `base-icon-${this.name}`, this.class], 'data-icon': this.name }) },
  })
  const transpiled = ts.transpileModule(compiled.content, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 } }).outputText
  let js = transpiled.replace(/import\s+['\"][^'\"]+\.css['\"];?/g, '').replace(/import\s+BaseIcon\s+from\s+['\"][^'\"]+['\"];?/g, '')
    .replace(/import\s*\{[^}]*\}\s*from\s*['\"][^'\"]+(logTargetCategorizer|useLogTargets)['\"];?/g, '')
    .replace(/import\s*\{\s*([^}]+)\s*\}\s*from\s*['"]vue['"];?/g, (_, imp) => {
      const mapped = imp.split(',').map((i: string) => { const p = i.trim().split(/\s+as\s+/); return p.length === 2 ? `${p[0].trim()}: ${p[1].trim()}` : p[0].trim() }).filter(Boolean).join(', ')
      return `const { ${mapped} } = Vue;`
    }).replace(/export\s+default\s+/, 'return ')
  return new Function('Vue', 'BaseIcon', 'isHostOrSystemUnit', js)(vueModule, BaseIconStub, isHostOrSystemUnit)
}

describe('Slice 76B: Admin-Only Journalctl Target & Pure App Catalog Security', () => {
  describe('1. isHostOrSystemUnit Client-Side Guard Specification', () => {
    it('correctly classifies systemd units, suffixes, and OS syslog daemons as host units', () => {
      const units = [
        'dockerd-rootless.sh', 'dockerd-rootless', 'dockerd', 'containerd', 'containerd-shim',
        'containerd-shim-runc-v2', 'runc', 'crio', 'kubelet', 'kube-proxy', 'etcd', 'systemd',
        'dbus-daemon', 'fwupd', 'multipathd', 'networkd-dispatcher', 'rsyslogd', 'cron', 'crond',
        'atd', 'irqbalance', 'acpid', 'avahi-daemon', 'accounts-daemon', 'rtkit-daemon', 'udisksd',
        'upowerd', 'modemmanager', 'wpa_supplicant', 'unattended-upgrades', 'packagekitd', 'snapd',
        'lxcfs', 'vmware-vmsvc', 'vmtoolsd', 'qemu-ga', 'chronyd', 'ntpd',
        'kubelet.service', 'cron.service', 'systemd-journal.service', 'docker.socket', 'systemd-tmpfiles-clean.timer',
        'user-1000.slice', 'session-1.scope', 'multi-user.target', 'proc-sys-fs-binfmt_misc.mount', 'journalctl',
        'systemd-journal', 'auth', 'authpriv', 'kern', 'syslog', 'daemon', 'crontab', 'multipath',
        'vmware-vmsvc-root', 'vmware-vmtoolsd-root', 'ModemManager', 'NetworkManager', 'dbus', 'packagekit',
        'rsyslog', 'sshd', 'sudo', 'polkit',
      ]
      for (const u of units) assert.equal(isHostOrSystemUnit(u), true, `Expected "${u}" to be host unit`)
    })

    it('preserves legitimate application, container, and pod workloads', () => {
      const apps = ['db', 'my-nginx', 'mysql-temp', 'nats', 'registry', 'tiki_drone', 'tiki_proxysql', 'tiki_redis', 'postgres', 'auth-api', 'frontend-ui', 'checkout-service', 'worker-queue']
      for (const a of apps) assert.equal(isHostOrSystemUnit(a), false, `Expected "${a}" to NOT be host unit`)
    })
  })

  describe('2. LogServiceCombobox.vue Admin-Only Journalctl Option & Noise Purging', () => {
    const LogServiceCombobox = loadComponent('src/components/logs/LogServiceCombobox.vue')
    const mixedServices = ['db', 'kubelet.service', 'my-nginx', 'cron.service', 'auth', 'kern', 'vmware-vmsvc-root', 'tiki_redis']

    it('renders exactly ONE journalctl (System Journal) option and pure apps when showJournalctl=true', async () => {
      const html = await renderToString(h(LogServiceCombobox, { modelValue: '', services: mixedServices, showJournalctl: true, initialOpen: true }))
      assert.ok(html.includes('All Apps'), 'Must render All Apps')
      assert.ok(html.includes('journalctl'), 'Must render journalctl')
      assert.ok(html.includes('System Journal'), 'Must render System Journal badge')
      const matches = html.match(/class="[^"]*item-journalctl[^"]*"/g) || []
      assert.equal(matches.length, 1, 'Must render exactly ONE consolidated journalctl row')
      assert.ok(html.includes('db'), 'Must include app "db"')
      assert.ok(html.includes('my-nginx'), 'Must include app "my-nginx"')
      assert.ok(html.includes('tiki_redis'), 'Must include app "tiki_redis"')
      assert.ok(!html.includes('kubelet.service'), 'kubelet.service must be purged from app list')
      assert.ok(!html.includes('cron.service'), 'cron.service must be purged from app list')
      assert.ok(!html.includes('vmware-vmsvc-root'), 'vmware-vmsvc-root must be purged from app list')
    })

    it('does NOT render journalctl at all when showJournalctl=false (non-admin)', async () => {
      const html = await renderToString(h(LogServiceCombobox, { modelValue: '', services: mixedServices, showJournalctl: false, initialOpen: true }))
      assert.ok(!html.includes('journalctl'), 'Non-admin must not see journalctl')
      assert.ok(!html.includes('System Journal'), 'Non-admin must not see System Journal')
      assert.ok(html.includes('All Apps'), 'Must render All Apps')
      assert.ok(html.includes('my-nginx'), 'Must render pure app')
    })

    it('renders [terminal journalctl × ▾] trigger pill and clears to All Apps in 1 click', async () => {
      const emitted: string[] = []
      const vnode = h(LogServiceCombobox, { modelValue: 'journalctl', services: mixedServices, showJournalctl: true, 'onUpdate:modelValue': (val: string) => emitted.push(val) })
      const html = await renderToString(vnode)
      assert.ok(html.includes('journalctl'), 'Must show journalctl in trigger')
      assert.ok(html.includes('data-icon="terminal"'), 'Must render terminal icon for journalctl')
      assert.ok(html.includes('btn-clear-service'), 'Must render inline clear button')
      const instance = vnode.component?.exposed || vnode.component?.setupState
      assert.ok(instance?.clearService, 'Must expose clearService method')
      instance.clearService()
      assert.equal(emitted.length, 1)
      assert.equal(emitted[0], '', 'Clear button must return to All Apps ("")')
    })

    it('shows subtle note "No app containers on this node" when node has 0 apps', async () => {
      const html = await renderToString(h(LogServiceCombobox, { modelValue: '', services: ['kubelet.service', 'cron.service'], showJournalctl: true, initialOpen: true }))
      assert.ok(html.includes('No app containers on this node'), 'Must show note when 0 app containers')
      assert.ok(html.includes('journalctl'), 'journalctl must remain accessible for platform_admin')
    })

    it('matches journalctl query flexibly on system, journal, or journalctl search', async () => {
      const vnode = h(LogServiceCombobox, { modelValue: '', services: mixedServices, showJournalctl: true, initialOpen: true })
      await renderToString(vnode)
      const inst = vnode.component?.exposed || vnode.component?.setupState
      inst.searchQuery.value = 'journal'
      assert.ok(inst.visibleOptions.value.some((o: any) => o.isJournalctl), 'Matches on "journal"')
      inst.searchQuery.value = 'system'
      assert.ok(inst.visibleOptions.value.some((o: any) => o.isJournalctl), 'Matches on "system"')
    })
  })

  describe('3. Journalctl Stream Catalog Pollution Prevention (useLogTargets)', async () => {
    const { useLogTargets, enrichDiscoveredServices } = await import('../src/composables/useLogTargets.ts')
    type LogEntry = import('../src/stores/logStore.ts').LogEntry
    type LogTarget = import('../src/composables/useLogTargets.ts').LogTarget

    it('streaming journalctl with kubelet.service and cron.service NEVER pollutes app catalog', () => {
      const target: LogTarget = { type: 'node', id: 'worker-node-1', name: 'worker-node-1' }
      enrichDiscoveredServices([{ time: '2026-11-06T00:00:00Z', level: 'INFO', node: 'worker-node-1', service: 'my-nginx', msg: 'nginx started' }])
      const journalLogs: LogEntry[] = [
        { time: '2026-11-06T00:00:01Z', level: 'INFO', node: 'worker-node-1', service: 'kubelet.service', msg: 'kubelet sync' },
        { time: '2026-11-06T00:00:02Z', level: 'INFO', node: 'worker-node-1', service: 'cron.service', msg: 'cron minute tick' },
        { time: '2026-11-06T00:00:03Z', level: 'INFO', node: 'worker-node-1', service: 'multipath', msg: 'daemon running' },
      ]
      enrichDiscoveredServices(journalLogs)
      const targets = useLogTargets({ modelValue: target, logs: journalLogs }, () => {})
      const discovered = targets.getDiscoveredServicesForNode('worker-node-1')
      assert.ok(discovered.includes('my-nginx'), 'my-nginx must be preserved')
      assert.ok(!discovered.includes('kubelet.service'), 'kubelet.service must NOT pollute app catalog')
      assert.ok(!discovered.includes('cron.service'), 'cron.service must NOT pollute app catalog')
      assert.ok(!discovered.includes('multipath'), 'multipath must NOT pollute app catalog')
    })
  })

  describe('4. LogTerminalEmptyState.vue Switch to journalctl CTA', () => {
    const LogTerminalEmptyState = loadComponent('src/components/logs/LogTerminalEmptyState.vue')

    it('renders [Switch to journalctl (System Journal)] ONLY when showJournalctl=true on online node in All Apps', async () => {
      const emitted: string[] = []
      const vnode = h(LogTerminalEmptyState, { isTargetOffline: false, mode: 'live', activeService: '', showJournalctl: true, 'onSelect-journalctl': () => emitted.push('select-journalctl') })
      const html = await renderToString(vnode)
      assert.ok(html.includes('No application container logs in current scope'), 'Must show container empty message')
      assert.ok(html.includes('Switch to journalctl (System Journal)'), 'Must render switch to journalctl button')
      const instance = vnode.component?.exposed || vnode.component?.setupState
      assert.ok(instance?.onSwitchToJournalctl, 'Must expose onSwitchToJournalctl method')
      instance.onSwitchToJournalctl()
      assert.equal(emitted.length, 1)
      assert.equal(emitted[0], 'select-journalctl', 'Must emit select-journalctl event')
    })

    it('does NOT render [Switch to journalctl] button when showJournalctl=false (non-admin)', async () => {
      const html = await renderToString(h(LogTerminalEmptyState, { isTargetOffline: false, mode: 'live', activeService: '', showJournalctl: false }))
      assert.ok(html.includes('No application container logs in current scope'), 'Must show container empty message')
      assert.ok(!html.includes('Switch to journalctl'), 'Non-admin must NOT see journalctl CTA button')
    })
  })

  describe('5. matchesTarget and targetFilteredLogs Journalctl & All Apps Isolation', async () => {
    const { matchesTarget } = await import('../src/stores/logStore.ts')
    const { useHistoricalLogSearch } = await import('../src/composables/useHistoricalLogSearch.ts')
    type LogEntry = import('../src/stores/logStore.ts').LogEntry

    const mockLogs: LogEntry[] = [
      { time: '2026-11-06T00:00:01Z', level: 'INFO', node: 'node-1', service: 'kubelet.service', msg: 'kubelet sync', namespace: '__host__' },
      { time: '2026-11-06T00:00:02Z', level: 'INFO', node: 'node-1', service: 'cron.service', msg: 'cron run', namespace: '__host__' },
      { time: '2026-11-06T00:00:03Z', level: 'INFO', node: 'node-1', service: 'my-nginx', msg: 'http request', namespace: 'prod' },
      { time: '2026-11-06T00:00:04Z', level: 'INFO', node: 'node-1', service: 'postgres_db', msg: 'sql query', namespace: 'prod' },
    ]

    it('retains kubelet.service and cron.service when service is journalctl', () => {
      // 1. Direct matchesTarget contract
      assert.equal(matchesTarget(mockLogs[0], { service: 'journalctl' }), true)
      assert.equal(matchesTarget(mockLogs[1], { service: 'journalctl' }), true)
      assert.equal(matchesTarget(mockLogs[2], { service: 'journalctl' }), false)

      // 2. targetFilteredLogs composable filtering
      const mockLogStore = { logs: mockLogs, histogram: [], totalHistoricalCount: 4, hasMoreHistorical: false, isHistoricalLoading: false } as any
      const search = useHistoricalLogSearch(mockLogStore, ref({ type: 'node', id: 'node-1', name: 'node-1' }), ref(''), ref('ALL'))
      search.activeServiceSubFilter.value = 'journalctl'
      const journalLogs = search.targetFilteredLogs.value
      const journalServices = journalLogs.map(l => l.service)
      assert.ok(journalServices.includes('kubelet.service'), 'Must retain kubelet.service')
      assert.ok(journalServices.includes('cron.service'), 'Must retain cron.service')
      assert.ok(!journalServices.includes('my-nginx'), 'Must exclude my-nginx from journalctl')
    })

    it('excludes kubelet.service and __host__ logs when service is empty (All Apps)', () => {
      // 1. Direct matchesTarget contract
      assert.equal(matchesTarget(mockLogs[0], { service: '' }), false)
      assert.equal(matchesTarget(mockLogs[1], { service: '' }), false)
      assert.equal(matchesTarget(mockLogs[2], { service: '' }), true)
      assert.equal(matchesTarget(mockLogs[3], { service: '' }), true)

      // 2. targetFilteredLogs composable filtering
      const mockLogStore = { logs: mockLogs, histogram: [], totalHistoricalCount: 4, hasMoreHistorical: false, isHistoricalLoading: false } as any
      const search = useHistoricalLogSearch(mockLogStore, ref({ type: 'node', id: 'node-1', name: 'node-1' }), ref(''), ref('ALL'))
      search.activeServiceSubFilter.value = ''
      const appLogs = search.targetFilteredLogs.value
      const appServices = appLogs.map(l => l.service)
      assert.ok(appServices.includes('my-nginx'), 'Must retain pure app my-nginx')
      assert.ok(appServices.includes('postgres_db'), 'Must retain pure app postgres_db')
      assert.ok(!appServices.includes('kubelet.service'), 'Must exclude kubelet.service from All Apps')
      assert.ok(!appServices.includes('cron.service'), 'Must exclude cron.service from All Apps')
    })
  })

  describe('6. VirtualLogTerminal.vue Unit Tag Badge Rendering (journalctl and All Apps)', async () => {
    const virtUtils = await import('../src/components/logs/virtualLogUtils.ts')
    const { calculateVirtualWindow } = await import('../src/utils/virtualWindow.ts')
    const { detectStackTrace } = await import('../src/utils/stackTraceParser.ts')
    const { useVirtualLogScroll } = await import('../src/components/logs/useVirtualLogScroll.ts')

    function loadVirtualTerminal() {
      const fullPath = resolveFilePath('src/components/logs/VirtualLogTerminal.vue')
      const source = fs.readFileSync(fullPath, 'utf8')
      const { descriptor } = parse(source, { filename: path.basename(fullPath) })
      const compiled = compileScript(descriptor, { id: 'test-vterm-' + Math.random().toString(36).slice(2), inlineTemplate: true })
      const transpiled = ts.transpileModule(compiled.content, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 } }).outputText

      const js = transpiled
        .replace(/import\s+['\"][^'\"]+\.css['\"];?/g, '')
        .replace(/import\s+BaseIcon\s+from\s+['\"][^'\"]+['\"];?/g, '')
        .replace(/import\s+LogTerminalEmptyState\s+from\s+['\"][^'\"]+['\"];?/g, '')
        .replace(/import\s*\{[^}]*\}\s*from\s*['\"][^'\"]+logStore['\"];?/g, '')
        .replace(/import\s*\{[^}]*\}\s*from\s*['\"][^'\"]+virtualWindow['\"];?/g, '')
        .replace(/import\s*\{[^}]*\}\s*from\s*['\"][^'\"]+stackTraceParser['\"];?/g, '')
        .replace(/import\s*\{[^}]*\}\s*from\s*['\"][^'\"]+virtualLogUtils['\"];?/g, '')
        .replace(/import\s*\{[^}]*\}\s*from\s*['\"][^'\"]+useVirtualLogScroll['\"];?/g, '')
        .replace(/import\s*\{\s*([^}]+)\s*\}\s*from\s*['"]vue['"];?/g, (_, imp) => {
          const mapped = imp.split(',').map((i: string) => { const p = i.trim().split(/\s+as\s+/); return p.length === 2 ? `${p[0].trim()}: ${p[1].trim()}` : p[0].trim() }).filter(Boolean).join(', ')
          return `const { ${mapped} } = Vue;`
        })
        .replace(/export\s+default\s+/, 'return ')

      const BaseIconStub = defineComponent({
        props: ['name', 'size', 'class'],
        render() { return h('svg', { class: ['base-icon', `base-icon-${this.name}`, this.class], 'data-icon': this.name }) },
      })
      const EmptyStub = defineComponent({ render() { return null } })
      const useLogStoreStub = () => ({ logs: [], openTraceDrawer: () => {}, openContextModal: () => {} })

      return new Function(
        'Vue', 'BaseIcon', 'LogTerminalEmptyState', 'useLogStore',
        'calculateVirtualWindow', 'detectStackTrace', 'useVirtualLogScroll',
        'formatTime', 'formatLevelBadge', 'getTargetBadge', 'getRowUnitBadge', 'getBadgeLabel', 'shouldShowBadge',
        'cleanLogMessage', 'coalesceLogEntries',
        js
      )(
        vueModule, BaseIconStub, EmptyStub, useLogStoreStub,
        calculateVirtualWindow, detectStackTrace, useVirtualLogScroll,
        virtUtils.formatTime, virtUtils.formatLevelBadge, virtUtils.getTargetBadge, virtUtils.getRowUnitBadge, virtUtils.getBadgeLabel, virtUtils.shouldShowBadge,
        virtUtils.cleanLogMessage, virtUtils.coalesceLogEntries
      )
    }

    const VirtualLogTerminal = loadVirtualTerminal()
    type LogEntry = import('../src/stores/logStore.ts').LogEntry

    it('getRowUnitBadge correctly extracts unit name and discards redundant labels', () => {
      assert.equal(virtUtils.getRowUnitBadge({ time: '', level: 'INFO', msg: '', service: 'kubelet.service', namespace: '__host__' }), 'kubelet.service')
      assert.equal(virtUtils.getRowUnitBadge({ time: '', level: 'INFO', msg: '', service: 'cron.service', namespace: '__host__' }), 'cron.service')
      assert.equal(virtUtils.getRowUnitBadge({ time: '', level: 'INFO', msg: '', service: 'dockerd-rootless.sh', namespace: '__host__' }), 'dockerd-rootless.sh')
      assert.equal(virtUtils.getRowUnitBadge({ time: '', level: 'INFO', msg: '', container: 'my-nginx', service: 'my-nginx', namespace: 'default', pod: 'my-nginx-pod' }), 'my-nginx')
      assert.equal(virtUtils.getRowUnitBadge({ time: '', level: 'INFO', msg: '', container: 'db', namespace: 'default', pod: 'db-pod' }), 'db')
      assert.equal(virtUtils.getRowUnitBadge({ time: '', level: 'INFO', msg: '', service: 'journalctl', namespace: '__host__' }), '')
      assert.equal(virtUtils.getRowUnitBadge({ time: '', level: 'INFO', msg: '', service: 'unknown', namespace: 'default' }), '')
    })

    it('renders [<unit>] badge for each row when viewing journalctl', async () => {
      const logs: LogEntry[] = [
        { time: '2026-11-06T04:26:00Z', level: 'ERR', service: 'kubelet.service', msg: 'E1106 kubelet failed to sync node status', namespace: '__host__' },
        { time: '2026-11-06T04:26:01Z', level: 'INFO', service: 'cron.service', msg: 'cron info tick', namespace: '__host__' },
        { time: '2026-11-06T04:26:02Z', level: 'INFO', service: 'dockerd-rootless.sh', msg: 'dockerd running rootless', namespace: '__host__' },
      ]
      const html = await renderToString(h(VirtualLogTerminal, { logs, activeService: 'journalctl', hideServiceBadge: false }))
      assert.ok(html.includes('[kubelet.service]'), 'Must render [kubelet.service] badge')
      assert.ok(html.includes('[cron.service]'), 'Must render [cron.service] badge')
      assert.ok(html.includes('[dockerd-rootless.sh]'), 'Must render [dockerd-rootless.sh] badge')
      assert.ok(!html.includes('[journalctl]'), 'Must not render redundant [journalctl] badge')
    })

    it('renders [<app>] badge for each row when viewing All Apps ("")', async () => {
      const logs: LogEntry[] = [
        { time: '2026-11-06T04:26:00Z', level: 'INFO', container: 'db', service: 'db', msg: 'database ready', namespace: 'prod' },
        { time: '2026-11-06T04:26:01Z', level: 'INFO', container: 'my-nginx', service: 'my-nginx', msg: 'nginx started', namespace: 'prod' },
      ]
      const html = await renderToString(h(VirtualLogTerminal, { logs, activeService: '', hideServiceBadge: false }))
      assert.ok(html.includes('[db]'), 'Must render [db] badge in All Apps')
      assert.ok(html.includes('[my-nginx]'), 'Must render [my-nginx] badge in All Apps')
    })
  })

  describe('7. logStore.ts seedRecentLogs Live Backfill Specification', async () => {
    const { useLogStore } = await import('../src/stores/logStore.ts')
    const { api } = await import('../src/api/client.ts')
    const { setActivePinia, createPinia } = await import('pinia')

    it('defines seedRecentLogs on logStore and backfills logs matching target', async () => {
      setActivePinia(createPinia())
      const store = useLogStore()
      assert.equal(typeof store.seedRecentLogs, 'function', 'Must export seedRecentLogs function')
    })

    it('queries /logs/search with limit: 100, maps entries, and deduplicates against live stream', async () => {
      setActivePinia(createPinia())
      const store = useLogStore()

      let requestedParams: any = null
      const origGet = api.get
      api.get = (async (endpoint: string, params: any) => {
        if (endpoint === '/logs/search') {
          requestedParams = params
          return {
            total_count: 2,
            has_more: false,
            entries: [
              { timestamp: '2026-11-06T04:00:00Z', log_level: 'INFO', message: 'seed log 1', service: 'db', node: 'node-1' },
              { timestamp: '2026-11-06T04:00:01Z', log_level: 'INFO', message: 'seed log 2', service: 'db', node: 'node-1' },
            ],
          }
        }
        return origGet.call(api, endpoint, params)
      }) as any

      try {
        // Pre-populate with live entry identical to seed log 1
        store.logs = [
          { time: '2026-11-06T04:00:00Z', level: 'INFO', msg: 'seed log 1', service: 'db', node: 'node-1', namespace: 'default', pod: 'db' },
        ]

        await store.seedRecentLogs({ node: 'node-1', service: 'db' })

        assert.equal(requestedParams.limit, 100, 'Must query with limit: 100')
        assert.equal(requestedParams.service, 'db', 'Must pass service filter')
        assert.equal(requestedParams.node, 'node-1', 'Must pass node filter')

        // Must have deduplicated seed log 1 and added seed log 2 -> total length 2, not 3
        assert.equal(store.logs.length, 2, 'Must deduplicate and retain only 2 unique entries')
        assert.equal(store.logs[1].msg, 'seed log 2')
      } finally {
        api.get = origGet
      }
    })

    it('cancels/ignores stale responses when target changes rapidly', async () => {
      setActivePinia(createPinia())
      const store = useLogStore()

      let resolveSlowReq: ((val: any) => void) | null = null
      const origGet = api.get
      api.get = (async (endpoint: string, params: any) => {
        if (params?.service === 'slow-svc') {
          return new Promise((resolve) => { resolveSlowReq = resolve })
        }
        if (params?.service === 'fast-svc') {
          return {
            total_count: 1,
            has_more: false,
            entries: [
              { timestamp: '2026-11-06T04:10:00Z', log_level: 'INFO', message: 'fast log', service: 'fast-svc', node: 'node-1' },
            ],
          }
        }
        return { total_count: 0, has_more: false, entries: [] }
      }) as any

      try {
        store.logs = []

        // Req 1: slow target
        const slowPromise = store.seedRecentLogs({ node: 'node-1', service: 'slow-svc' })
        // Req 2: fast target fires immediately after
        const fastPromise = store.seedRecentLogs({ node: 'node-1', service: 'fast-svc' })

        await fastPromise
        assert.equal(store.logs.length, 1)
        assert.equal(store.logs[0].service, 'fast-svc')

        // Now slow request resolves later
        resolveSlowReq?.({
          total_count: 1,
          has_more: false,
          entries: [
            { timestamp: '2026-11-06T04:05:00Z', log_level: 'INFO', message: 'slow stale log', service: 'slow-svc', node: 'node-1' },
          ],
        })
        await slowPromise

        // Must STILL only contain fast-svc log; slow-svc log must be discarded!
        assert.equal(store.logs.length, 1, 'Stale slow-svc logs must not overwrite newer target logs')
        assert.equal(store.logs[0].service, 'fast-svc')
      } finally {
        api.get = origGet
      }
    })
  })
})
