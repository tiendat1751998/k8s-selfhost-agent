import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import * as vueModule from 'vue'
import { h, defineComponent } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { parse, compileScript } from '@vue/compiler-sfc'
import ts from 'typescript'

interface ComponentMocks {
  BaseIcon?: any
  getErrorRadar?: (...args: any[]) => Promise<any>
  isNoiseLog?: (service: string) => boolean
  cleanFingerprint?: (fp?: string) => string
}

/**
 * SFC loader helper: compiles .vue SFC files into runnable Vue 3 component definitions in Node.js
 */
function loadComponent(relativeFilePath: string, mocks: ComponentMocks = {}) {
  const fullPath = path.resolve(process.cwd(), relativeFilePath)
  let source = fs.readFileSync(fullPath, 'utf8')

  // If component doesn't have defineExpose, expose top-level bindings for test inspectability
  if (!source.includes('defineExpose')) {
    source = source.replace(
      '</script>',
      '\ndefineExpose({ fetchRadar, groups, totalGroups, totalOccurrences, activeTimeRange, setTimeRange })\n</script>'
    )
  }

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

  // Transpile TypeScript out of compiled script
  const transpiled = ts.transpileModule(compiled.content, {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText

  let jsCode = transpiled
  jsCode = jsCode.replace(/import\s+['\"][^'\"]+\.css['\"];?/g, '')
  jsCode = jsCode.replace(/import\s+BaseIcon\s+from\s+['\"][^'\"]+['\"];?/g, '')
  jsCode = jsCode.replace(/import\s*\{[^}]*getErrorRadar[^}]*\}\s*from\s*['\"][^'\"]+['\"];?/g, '')
  jsCode = jsCode.replace(/import\s*\{[^}]*isNoiseLog[^}]*\}\s*from\s*['\"][^'\"]+['\"];?/g, '')
  jsCode = jsCode.replace(/import\s*\{[^}]*cleanFingerprint[^}]*\}\s*from\s*['\"][^'\"]+['\"];?/g, '')

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

  const factory = new Function(
    'Vue',
    'BaseIcon',
    'getErrorRadar',
    'isNoiseLog',
    'cleanFingerprint',
    jsCode
  )
  return factory(
    vueModule,
    mocks.BaseIcon || BaseIconStub,
    mocks.getErrorRadar || (async () => ({ groups: [], total_groups: 0, total_occurrences: 0, took_ms: 1 })),
    mocks.isNoiseLog || (() => false),
    mocks.cleanFingerprint || ((fp?: string) => fp || '')
  )
}

describe('Task 074 — Slice 74C: Behavioral Mount & Interactive UI Testing', () => {
  describe('LogTerminalEmptyState Behavioral Component Tests', () => {
    const LogTerminalEmptyState = loadComponent('src/components/logs/LogTerminalEmptyState.vue')

    it('mounts with offlineNodeName and streamMode="live", renders CTA button with icon + text and emits switch-to-history', async () => {
      const emitted: string[] = []
      const vnode = h(LogTerminalEmptyState, {
        offlineNodeName: 'k8s-worker-node-1',
        streamMode: 'live',
        onSwitchToHistory: () => emitted.push('switchToHistory'),
        'onSwitch-to-history': () => emitted.push('switch-to-history'),
      })

      const html = await renderToString(vnode)

      assert.ok(html.includes('offline-node-empty'), 'Must contain offline-node-empty container')
      assert.ok(html.includes('k8s-worker-node-1'), 'Must display the offline node name in title')
      assert.ok(html.includes('btn-switch-history'), 'Must render button with class .btn-switch-history')
      assert.ok(html.includes('data-icon="clock"'), 'Must render clock icon inside button')
      assert.ok(html.includes('Switch to History Logs'), 'Must render "Switch to History Logs" button label')

      const clickHandler = vnode.component?.exposed?.onSwitchToHistory || vnode.component?.setupState?.onSwitchToHistory
      assert.ok(clickHandler, 'Must expose onSwitchToHistory handler')
      clickHandler()

      assert.ok(
        emitted.includes('switch-to-history'),
        `Expected switch-to-history event, received: ${JSON.stringify(emitted)}`
      )
    })

    it('supports backward-compatible isTargetOffline and mode props', async () => {
      const emitted: string[] = []
      const vnode = h(LogTerminalEmptyState, {
        isTargetOffline: true,
        mode: 'live',
        targetName: 'legacy-node-alpha',
        onSwitchToHistory: () => emitted.push('switchToHistory'),
        'onSwitch-to-history': () => emitted.push('switch-to-history'),
      })

      const html = await renderToString(vnode)
      assert.ok(html.includes('offline-node-empty'))
      assert.ok(html.includes('legacy-node-alpha'))
      assert.ok(html.includes('btn-switch-history'))

      const clickHandler = vnode.component?.exposed?.onSwitchToHistory || vnode.component?.setupState?.onSwitchToHistory
      assert.ok(clickHandler, 'Must expose onSwitchToHistory handler')
      clickHandler()
      assert.ok(emitted.includes('switch-to-history') || emitted.includes('switchToHistory'))
    })

    it('renders normal waiting state when target is online in live mode', async () => {
      const vnode = h(LogTerminalEmptyState, {
        isTargetOffline: false,
        mode: 'live',
        targetName: 'cluster',
        isConnected: true,
      })

      const html = await renderToString(vnode)
      assert.ok(!html.includes('offline-node-empty'), 'Must not render offline empty state')
      assert.ok(!html.includes('btn-switch-history'), 'Must not render switch history button when online')
      assert.ok(html.includes('Waiting for logs from [cluster]...'))
    })

    it('does not render CTA button when in historical mode even if node is offline', async () => {
      const vnode = h(LogTerminalEmptyState, {
        offlineNodeName: 'k8s-offline-worker',
        streamMode: 'historical',
      })

      const html = await renderToString(vnode)
      assert.ok(!html.includes('offline-node-empty'), 'Must not render offline live card in historical mode')
      assert.ok(!html.includes('btn-switch-history'), 'Must NOT render history CTA button when already in history mode')
      assert.ok(html.includes('Waiting for logs from [k8s-offline-worker]...'))
    })

    it('falls back to [cluster] label when targetName and offlineNodeName are omitted', async () => {
      const vnode = h(LogTerminalEmptyState, {
        isConnected: true,
        isTargetOffline: false,
      })

      const html = await renderToString(vnode)
      assert.ok(html.includes('Waiting for logs from [cluster]...'))
    })
  })

  describe('LogErrorRadarPanel Behavioral Component Tests (patterns.length > 0)', () => {
    const mockGroups = [
      {
        fingerprint: 'Connection refused to 10.10.10.60:5432',
        service: 'payment-svc',
        node: 'k8s-worker-1',
        count: 42,
        level: 'ERROR',
        last_seen: '2026-11-05T04:15:00.000Z',
        sample_message: 'dial tcp 10.10.10.60:5432: connect: connection refused',
      },
      {
        fingerprint: 'Disk usage exceeds threshold 85%',
        service: 'kubelet.service',
        node: 'k8s-worker-2',
        count: 7,
        level: 'WARN',
        last_seen: '2026-11-05T04:20:00.000Z',
        sample_message: 'Warning: disk usage is at 87% on /var/lib/docker',
      },
    ]

    it('renders empty state when zero patterns are returned', async () => {
      const LogErrorRadarPanel = loadComponent('src/components/logs/LogErrorRadarPanel.vue', {
        getErrorRadar: async () => ({ groups: [], total_groups: 0, total_occurrences: 0, took_ms: 1.5 }),
      })

      const vnode = h(LogErrorRadarPanel, { node: 'worker-1' })
      const html = await renderToString(vnode)

      assert.ok(html.includes('Blind Error Radar'), 'Must render panel title')
      assert.ok(html.includes('radar-empty-state'), 'Must render empty state container')
      assert.ok(html.includes('Zero ERROR/FATAL patterns detected in current scope'))
      assert.ok(html.includes('Node: worker-1'), 'Must render scope badge')
    })

    it('mounts and renders pattern rows, count pills, and inspect action when patterns.length > 0', async () => {
      const inspected: any[] = []
      const closed: boolean[] = []

      const LogErrorRadarPanel = loadComponent('src/components/logs/LogErrorRadarPanel.vue', {
        getErrorRadar: async () => ({
          groups: mockGroups,
          total_groups: 2,
          total_occurrences: 49,
          took_ms: 8.4,
        }),
        cleanFingerprint: (fp?: string) => fp ? fp.replace(/^Connection refused/, 'Refused') : '',
      })

      const vnode = h(LogErrorRadarPanel, {
        node: 'k8s-worker-1',
        service: 'payment-svc',
        onInspectGroup: (group: any, range: string) => inspected.push({ group, range }),
        onClose: () => closed.push(true),
      })

      await renderToString(vnode)
      const exposed = vnode.component?.exposed
      assert.ok(exposed, 'Component exposed bindings must be available')

      await exposed.fetchRadar()

      assert.equal(exposed.groups.value.length, 2, 'Must populate 2 groups')
      assert.equal(exposed.totalGroups.value, 2)
      assert.equal(exposed.totalOccurrences.value, 49)

      const render = vnode.component?.render
      assert.ok(render, 'Render function must exist')
      const cache = (vnode.component as any)?.renderCache || []
      const updatedVNode = render.call(vnode.component.proxy, vnode.component.proxy, cache)
      const html = await renderToString(updatedVNode)

      assert.ok(html.includes('radar-list'), 'Must render radar-list container')
      assert.ok(html.includes('radar-item'), 'Must render radar-item row')

      assert.ok(html.includes('42×'), 'Must render 42× count badge')
      assert.ok(html.includes('7×'), 'Must render 7× count badge')
      assert.ok(html.includes('count-warn'), 'Warn group must have count-warn class')

      assert.ok(html.includes('ERROR'), 'Must render ERROR level badge')
      assert.ok(html.includes('WARN'), 'Must render WARN level badge')

      assert.ok(html.includes('[k8s-worker-1/payment-svc]'), 'Must render scoped node/service badge')
      assert.ok(html.includes('[k8s-worker-2/kubelet.service]'), 'Must render worker-2 badge')

      assert.ok(html.includes('Refused to 10.10.10.60:5432'), 'Must render cleaned fingerprint')
      assert.ok(html.includes('dial tcp 10.10.10.60:5432'), 'Must render sample message')

      vnode.component.emit('inspectGroup', mockGroups[0], exposed.activeTimeRange.value)
      assert.equal(inspected.length, 1)
      assert.equal(inspected[0].group.service, 'payment-svc')
      assert.equal(inspected[0].range, '24h')

      vnode.component.emit('close')
      assert.equal(closed.length, 1)

      exposed.setTimeRange('1h')
      assert.equal(exposed.activeTimeRange.value, '1h')
    })
  })
})
