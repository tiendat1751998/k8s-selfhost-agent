import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { register } from 'node:module'
import * as vueModule from 'vue'
import { h, defineComponent } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { parse, compileScript } from '@vue/compiler-sfc'
import ts from 'typescript'
import { isHostOrSystemUnit } from '../src/utils/logTargetCategorizer.ts'

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

function resolveFilePath(relativeFilePath: string): string {
  const candidates = [
    path.resolve(process.cwd(), relativeFilePath),
    path.resolve(process.cwd(), 'frontend-vue', relativeFilePath),
    path.resolve(process.cwd(), '..', relativeFilePath),
  ]
  for (const c of candidates) {
    if (fs.existsSync(c)) return c
  }
  return path.resolve(process.cwd(), relativeFilePath)
}

function loadComponent(relativeFilePath: string) {
  const fullPath = resolveFilePath(relativeFilePath)
  const source = fs.readFileSync(fullPath, 'utf8')

  const { descriptor } = parse(source, { filename: path.basename(fullPath) })
  const compiled = compileScript(descriptor, {
    id: 'test-sfc-' + Math.random().toString(36).slice(2),
    inlineTemplate: true,
  })

  const BaseIconStub = defineComponent({
    name: 'BaseIcon',
    props: ['name', 'size', 'class'],
    render() {
      return h('svg', {
        class: ['base-icon', `base-icon-${this.name}`, this.class],
        'data-icon': this.name,
        'data-size': this.size,
      })
    },
  })

  const transpiled = ts.transpileModule(compiled.content, {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText

  let jsCode = transpiled
  jsCode = jsCode.replace(/import\s+['\"][^'\"]+\.css['\"];?/g, '')
  jsCode = jsCode.replace(/import\s+BaseIcon\s+from\s+['\"][^'\"]+['\"];?/g, '')
  jsCode = jsCode.replace(/import\s*\{[^}]*\}\s*from\s*['\"][^'\"]+logTargetCategorizer['\"];?/g, '')
  jsCode = jsCode.replace(/import\s*\{[^}]*\}\s*from\s*['\"][^'\"]+useLogTargets['\"];?/g, '')
  jsCode = jsCode.replace(/import\s*\{\s*([^}]+)\s*\}\s*from\s*['"]vue['"];?/g, (_, imports) => {
    const mapped = imports
      .split(',')
      .map((i: string) => {
        const parts = i.trim().split(/\s+as\s+/)
        return parts.length === 2 ? `${parts[0].trim()}: ${parts[1].trim()}` : parts[0].trim()
      })
      .filter(Boolean)
      .join(', ')
    return `const { ${mapped} } = Vue;`
  })
  jsCode = jsCode.replace(/export\s+default\s+/, 'return ')

  const factory = new Function('Vue', 'BaseIcon', 'isHostOrSystemUnit', jsCode)
  return factory(vueModule, BaseIconStub, isHostOrSystemUnit)
}

describe('Slice 76B: Admin-Only Journalctl Target & Pure App Catalog Security', () => {
  describe('1. isHostOrSystemUnit Client-Side Guard Specification', () => {
    it('correctly classifies systemd units, suffixes, and OS syslog daemons as host units', () => {
      const systemUnits = [
        'kubelet.service',
        'cron.service',
        'systemd-journal.service',
        'docker.socket',
        'systemd-tmpfiles-clean.timer',
        'user-1000.slice',
        'session-1.scope',
        'multi-user.target',
        'proc-sys-fs-binfmt_misc.mount',
        'journalctl',
        'systemd-journal',
        'auth',
        'authpriv',
        'kern',
        'syslog',
        'daemon',
        'cron',
        'crontab',
        'multipath',
        'vmware-vmsvc-root',
        'vmware-vmtoolsd-root',
        'ModemManager',
        'NetworkManager',
        'dbus',
        'packagekit',
        'rsyslog',
        'sshd',
        'sudo',
        'polkit',
        'kubelet',
        'containerd',
        'dockerd',
      ]

      for (const unit of systemUnits) {
        assert.equal(
          isHostOrSystemUnit(unit),
          true,
          `Expected "${unit}" to be classified as a host or system unit`
        )
      }
    })

    it('preserves legitimate application, container, and pod workloads', () => {
      const pureApps = [
        'my-nginx',
        'db',
        'postgres',
        'tiki_redis',
        'auth-api',
        'frontend-ui',
        'checkout-service',
        'worker-queue',
      ]

      for (const app of pureApps) {
        assert.equal(
          isHostOrSystemUnit(app),
          false,
          `Expected "${app}" to NOT be classified as host/system unit`
        )
      }
    })
  })

  describe('2. LogServiceCombobox.vue Admin-Only Journalctl Option & Noise Purging', () => {
    const LogServiceCombobox = loadComponent('src/components/logs/LogServiceCombobox.vue')

    const mixedServices = [
      'db',
      'kubelet.service',
      'my-nginx',
      'cron.service',
      'auth',
      'kern',
      'vmware-vmsvc-root',
      'tiki_redis',
    ]

    it('renders exactly ONE journalctl (System Journal) option and pure apps when showJournalctl=true', async () => {
      const vnode = h(LogServiceCombobox, {
        modelValue: '',
        services: mixedServices,
        showJournalctl: true,
        initialOpen: true,
      })
      const html = await renderToString(vnode)

      // Must include All Apps
      assert.ok(html.includes('All Apps'), 'Must render All Apps')

      // Must include single consolidated journalctl option with System Journal badge
      assert.ok(html.includes('journalctl'), 'Must render journalctl')
      assert.ok(html.includes('System Journal'), 'Must render System Journal badge')
      const matches = html.match(/class="[^"]*item-journalctl[^"]*"/g) || []
      assert.equal(matches.length, 1, 'Must render exactly ONE consolidated journalctl row')

      // Must include pure application containers
      assert.ok(html.includes('db'), 'Must include app "db"')
      assert.ok(html.includes('my-nginx'), 'Must include app "my-nginx"')
      assert.ok(html.includes('tiki_redis'), 'Must include app "tiki_redis"')

      // MUST NOT include individual system units or OS noise in app options
      assert.ok(!html.includes('kubelet.service'), 'kubelet.service must be purged from app list')
      assert.ok(!html.includes('cron.service'), 'cron.service must be purged from app list')
      assert.ok(!html.includes('vmware-vmsvc-root'), 'vmware-vmsvc-root must be purged from app list')
    })

    it('does NOT render journalctl at all when showJournalctl=false (non-admin)', async () => {
      const vnode = h(LogServiceCombobox, {
        modelValue: '',
        services: mixedServices,
        showJournalctl: false,
        initialOpen: true,
      })
      const html = await renderToString(vnode)

      assert.ok(!html.includes('journalctl'), 'Non-admin must not see journalctl')
      assert.ok(!html.includes('System Journal'), 'Non-admin must not see System Journal')
      assert.ok(html.includes('All Apps'), 'Must render All Apps')
      assert.ok(html.includes('my-nginx'), 'Must render pure app')
    })

    it('renders [terminal journalctl × ▾] trigger pill and clears to All Apps in 1 click', async () => {
      const emitted: string[] = []
      const vnode = h(LogServiceCombobox, {
        modelValue: 'journalctl',
        services: mixedServices,
        showJournalctl: true,
        'onUpdate:modelValue': (val: string) => emitted.push(val),
      })
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
      const vnode = h(LogServiceCombobox, {
        modelValue: '',
        services: ['kubelet.service', 'cron.service'], // Only system units -> 0 pure apps
        showJournalctl: true,
        initialOpen: true,
      })
      const html = await renderToString(vnode)

      assert.ok(html.includes('No app containers on this node'), 'Must show note when 0 app containers')
      assert.ok(html.includes('journalctl'), 'journalctl must remain accessible for platform_admin')
    })
  })

  describe('3. Journalctl Stream Catalog Pollution Prevention (useLogTargets)', async () => {
    const { useLogTargets, enrichDiscoveredServices } = await import('../src/composables/useLogTargets.ts')
    type LogEntry = import('../src/stores/logStore.ts').LogEntry
    type LogTarget = import('../src/composables/useLogTargets.ts').LogTarget

    it('streaming journalctl with kubelet.service and cron.service NEVER pollutes app catalog', () => {
      const target: LogTarget = { type: 'node', id: 'worker-node-1', name: 'worker-node-1' }

      // Initial clean app log
      const appLogs: LogEntry[] = [
        { time: '2026-11-06T00:00:00Z', level: 'INFO', node: 'worker-node-1', service: 'my-nginx', msg: 'nginx started' },
      ]
      enrichDiscoveredServices(appLogs)

      // Admin selects journalctl: streaming logs arrives with OS/systemd units
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
      const vnode = h(LogTerminalEmptyState, {
        isTargetOffline: false,
        mode: 'live',
        activeService: '',
        showJournalctl: true,
        'onSelect-journalctl': () => emitted.push('select-journalctl'),
      })
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
      const vnode = h(LogTerminalEmptyState, {
        isTargetOffline: false,
        mode: 'live',
        activeService: '',
        showJournalctl: false,
      })
      const html = await renderToString(vnode)

      assert.ok(html.includes('No application container logs in current scope'), 'Must show container empty message')
      assert.ok(!html.includes('Switch to journalctl'), 'Non-admin must NOT see journalctl CTA button')
    })
  })
})
