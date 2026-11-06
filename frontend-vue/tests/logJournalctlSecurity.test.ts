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
        'kubelet.service', 'cron.service', 'systemd-journal.service', 'docker.socket', 'systemd-tmpfiles-clean.timer',
        'user-1000.slice', 'session-1.scope', 'multi-user.target', 'proc-sys-fs-binfmt_misc.mount', 'journalctl',
        'systemd-journal', 'auth', 'authpriv', 'kern', 'syslog', 'daemon', 'cron', 'crontab', 'multipath',
        'vmware-vmsvc-root', 'vmware-vmtoolsd-root', 'ModemManager', 'NetworkManager', 'dbus', 'packagekit',
        'rsyslog', 'sshd', 'sudo', 'polkit', 'kubelet', 'containerd', 'dockerd',
      ]
      for (const u of units) assert.equal(isHostOrSystemUnit(u), true, `Expected "${u}" to be host unit`)
    })

    it('preserves legitimate application, container, and pod workloads', () => {
      const apps = ['my-nginx', 'db', 'postgres', 'tiki_redis', 'auth-api', 'frontend-ui', 'checkout-service', 'worker-queue']
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
})
