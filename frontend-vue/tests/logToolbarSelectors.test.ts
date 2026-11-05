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

interface ComponentMocks {
  BaseIcon?: any
}

function loadComponent(relativeFilePath: string, mocks: ComponentMocks = {}) {
  const fullPath = path.resolve(process.cwd(), relativeFilePath)
  let source = fs.readFileSync(fullPath, 'utf8')

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
  jsCode = jsCode.replace(/import\s*\{[^}]*isSystemdService[^}]*\}\s*from\s*['\"][^'\"]+['\"];?/g, '')
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

  const isSystemdServiceFn = (name?: string | null) => Boolean(name && (name.endsWith('.service') || name.endsWith('.socket')))

  const factory = new Function('Vue', 'BaseIcon', 'isSystemdService', jsCode)
  return factory(vueModule, mocks.BaseIcon || BaseIconStub, isSystemdServiceFn)
}

describe('Slice 75B: Compact Toolbar Node and App Selectors', () => {
  describe('LogNodeDropdown.vue Behavioral Mount Tests', () => {
    const LogNodeDropdown = loadComponent('src/components/logs/LogNodeDropdown.vue')

    const sampleNodes = [
      { id: 'k8sworker-1', name: 'k8sworker-1', online: true },
      { id: 'k8sworker-2', name: 'k8sworker-2', online: true },
      { id: 'k8smasterdeb', name: 'k8smasterdeb', online: false },
    ]

    it('1. Renders "All Nodes" with globe icon when modelValue is empty or "all"', async () => {
      const vnode = h(LogNodeDropdown, {
        modelValue: '',
        nodes: sampleNodes,
      })
      const html = await renderToString(vnode)
      assert.ok(html.includes('All Nodes'), 'Must render "All Nodes" in trigger')
      assert.ok(html.includes('data-icon="globe"'), 'Must render globe icon for All Nodes')
      assert.ok(!html.includes('btn-clear-node'), 'Must not render clear button when All Nodes')
    })

    it('2. Renders online node with emerald status dot when an online node is selected', async () => {
      const vnode = h(LogNodeDropdown, {
        modelValue: 'k8sworker-1',
        nodes: sampleNodes,
      })
      const html = await renderToString(vnode)
      assert.ok(html.includes('k8sworker-1'), 'Must render selected node name in trigger')
      assert.ok(html.includes('pulse-dot-emerald'), 'Must render emerald dot for online node')
      assert.ok(html.includes('btn-clear-node'), 'Must render inline clear button when node is selected')
    })

    it('3. Renders offline node with muted status dot when an offline node is selected', async () => {
      const vnode = h(LogNodeDropdown, {
        modelValue: 'k8smasterdeb',
        nodes: sampleNodes,
      })
      const html = await renderToString(vnode)
      assert.ok(html.includes('k8smasterdeb'), 'Must render selected node name in trigger')
      assert.ok(html.includes('pulse-dot-slate'), 'Must render muted slate dot for offline node')
      assert.ok(html.includes('btn-clear-node'), 'Must render inline clear button when node is selected')
    })

    it('4. Simplicity gate: does NOT render search input when nodes length <= 5', async () => {
      const vnode = h(LogNodeDropdown, {
        modelValue: '',
        nodes: sampleNodes, // 3 nodes <= 5
      })
      const html = await renderToString(vnode)
      assert.ok(!html.includes('node-search-input'), 'Must omit search input when <= 5 nodes')
    })

    it('5. Emits update:modelValue with empty string when inline clear button is clicked', async () => {
      const emitted: string[] = []
      const vnode = h(LogNodeDropdown, {
        modelValue: 'k8sworker-1',
        nodes: sampleNodes,
        'onUpdate:modelValue': (val: string) => emitted.push(val),
      })
      await renderToString(vnode)

      const instance = vnode.component?.exposed || vnode.component?.setupState
      assert.ok(instance?.clearNode, 'Must expose clearNode method')
      instance.clearNode()
      assert.equal(emitted.length, 1)
      assert.equal(emitted[0], '', 'Must clear to empty string (All Nodes)')
    })
  })

  describe('LogServiceCombobox.vue Behavioral Mount Tests', () => {
    const LogServiceCombobox = loadComponent('src/components/logs/LogServiceCombobox.vue')

    const sampleServices = [
      'auth-service',
      'frontend-app',
      'cron.service',
      'kubelet.service',
      'k8s-agent.service',
    ]

    it('1. Renders "All Apps" trigger pill with box icon when no service is selected', async () => {
      const vnode = h(LogServiceCombobox, {
        modelValue: '',
        services: sampleServices,
      })
      const html = await renderToString(vnode)
      assert.ok(html.includes('All Apps'), 'Must render "All Apps" trigger text')
      assert.ok(html.includes('data-icon="box"'), 'Must render box icon')
      assert.ok(!html.includes('btn-clear-service'), 'Must not render clear button when All Apps')
    })

    it('2. Renders active pill with service name and inline × button when service is selected', async () => {
      const vnode = h(LogServiceCombobox, {
        modelValue: 'cron.service',
        services: sampleServices,
      })
      const html = await renderToString(vnode)
      assert.ok(html.includes('cron.service'), 'Must render service name in trigger')
      assert.ok(html.includes('btn-clear-service'), 'Must render inline clear button')
      assert.ok(html.includes('active'), 'Trigger pill must have active class')
    })

    it('3. Clears active service when inline × button is clicked', async () => {
      const emitted: string[] = []
      const vnode = h(LogServiceCombobox, {
        modelValue: 'cron.service',
        services: sampleServices,
        'onUpdate:modelValue': (val: string) => emitted.push(val),
      })
      await renderToString(vnode)

      const instance = vnode.component?.exposed || vnode.component?.setupState
      assert.ok(instance?.clearService, 'Must expose clearService method')
      instance.clearService()
      assert.equal(emitted.length, 1)
      assert.equal(emitted[0], '', 'Must clear to empty string (All Apps)')
    })

    it('4. Filters services by search query case-insensitively', async () => {
      const vnode = h(LogServiceCombobox, {
        modelValue: '',
        services: sampleServices,
      })
      await renderToString(vnode)

      const instance = vnode.component?.exposed || vnode.component?.setupState
      assert.ok(instance?.searchQuery !== undefined, 'Must expose searchQuery')
      instance.searchQuery.value = 'cron'
      const filtered = instance.filteredServices.value || instance.filteredServices
      assert.equal(filtered.length, 1)
      assert.equal(filtered[0], 'cron.service')
    })

    it('5. Renders "Node offline" when node is offline and services list is empty', async () => {
      const vnode = h(LogServiceCombobox, {
        modelValue: '',
        services: [],
        isNodeOffline: true,
        initialOpen: true,
      })
      const html = await renderToString(vnode)
      assert.ok(html.includes('Node offline'), 'Must show "Node offline" empty message')
    })

    it('6. Sorts regular apps before .service daemons', async () => {
      const vnode = h(LogServiceCombobox, {
        modelValue: '',
        services: ['kubelet.service', 'postgres', 'cron.service', 'auth-api'],
      })
      await renderToString(vnode)

      const instance = vnode.component?.exposed || vnode.component?.setupState
      const sorted = instance.sortedServices.value || instance.sortedServices
      assert.deepEqual(sorted, ['auth-api', 'postgres', 'cron.service', 'kubelet.service'])
    })
  })

  describe('Persistent Discovered Service Catalog & Wipe-Out Prevention (useLogTargets)', async () => {
    const { useLogTargets } = await import('../src/composables/useLogTargets.ts')
    type LogEntry = import('../src/stores/logStore.ts').LogEntry
    type LogTarget = import('../src/composables/useLogTargets.ts').LogTarget

    it('1. Retains all discovered node services even when logs filter down to a single service (cron.service)', () => {
      const target: LogTarget = { type: 'node', id: 'k8sworker', name: 'k8sworker' }
      const initialLogs: LogEntry[] = [
        { time: '2026-10-23T01:00:00Z', level: 'INFO', node: 'k8sworker', service: 'kubelet.service', msg: 'kubelet ready' },
        { time: '2026-10-23T01:00:01Z', level: 'INFO', node: 'k8sworker', service: 'k8s-agent.service', msg: 'agent started' },
        { time: '2026-10-23T01:00:02Z', level: 'INFO', node: 'k8sworker', service: 'cron.service', msg: 'cron tick' },
        { time: '2026-10-23T01:00:03Z', level: 'INFO', node: 'k8sworker', service: 'redis-server', msg: 'redis up' },
      ]

      const targets = useLogTargets({ modelValue: target, logs: initialLogs }, () => {})
      const initialServices = targets.getNodeServices('k8sworker').map(s => s.name)

      assert.ok(initialServices.includes('kubelet.service'), 'Must include kubelet.service')
      assert.ok(initialServices.includes('cron.service'), 'Must include cron.service')
      assert.ok(initialServices.includes('redis-server'), 'Must include redis-server')

      // Now simulate user selecting cron.service, which filters logStore.logs down to ONLY cron.service!
      const filteredLogs: LogEntry[] = [
        { time: '2026-10-23T01:00:02Z', level: 'INFO', node: 'k8sworker', service: 'cron.service', msg: 'cron tick' },
      ]

      const targetsAfterFilter = useLogTargets({ modelValue: target, logs: filteredLogs }, () => {})
      const servicesAfterFilter = targetsAfterFilter.getNodeServices('k8sworker').map(s => s.name)

      // CRITICAL ASSERTION: Zero wipe-out! All discovered services must remain!
      assert.ok(servicesAfterFilter.includes('kubelet.service'), 'kubelet.service MUST NOT be wiped out!')
      assert.ok(servicesAfterFilter.includes('k8s-agent.service'), 'k8s-agent.service MUST NOT be wiped out!')
      assert.ok(servicesAfterFilter.includes('redis-server'), 'redis-server MUST NOT be wiped out!')
      assert.ok(servicesAfterFilter.includes('cron.service'), 'cron.service must remain present!')
    })

    it('2. Does not leak services from other nodes to an offline node without services', () => {
      const target: LogTarget = { type: 'node', id: 'k8smasterdeb', name: 'k8smasterdeb', online: false }
      const targets = useLogTargets({ modelValue: target, logs: [] }, () => {})

      const masterServices = targets.getNodeServices('k8smasterdeb')
      // k8smasterdeb has no logs or by_node services, so it must return empty (or only its own), NOT 51 services from k8sworker!
      assert.equal(masterServices.length, 0, 'Offline node without logs must not inherit services from other nodes')
    })
  })
})
