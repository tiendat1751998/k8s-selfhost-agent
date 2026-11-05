import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import * as vueModule from 'vue'
import { h, defineComponent } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { parse, compileScript } from '@vue/compiler-sfc'
import ts from 'typescript'

/**
 * Helper to compile and load a Vue 3 SFC component in Node.js
 */
function loadComponent(relativeFilePath: string) {
  const fullPath = path.resolve(process.cwd(), relativeFilePath)
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

  // Transpile TypeScript out of compiled script
  const transpiled = ts.transpileModule(compiled.content, {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText

  let jsCode = transpiled
  jsCode = jsCode.replace(/import\s+BaseIcon\s+from\s+['\"][^'\"]+['\"];?/g, '')
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

  const factory = new Function('Vue', 'BaseIcon', jsCode)
  return factory(vueModule, BaseIconStub)
}

describe('Task 074 — Slice 74C: Offline History CTA Button & Token Grounding', () => {
  describe('Criterion 5: Behavioral Mounting & Event Emission of LogTerminalEmptyState', () => {
    const LogTerminalEmptyState = loadComponent('src/components/logs/LogTerminalEmptyState.vue')

    it('mounts with offlineNodeName and streamMode="live", renders CTA button with icon + text and emits switch-to-history when clicked', async () => {
      const emitted: string[] = []
      const vnode = h(LogTerminalEmptyState, {
        offlineNodeName: 'k8s-worker-node-1',
        streamMode: 'live',
        onSwitchToHistory: () => emitted.push('switchToHistory'),
        'onSwitch-to-history': () => emitted.push('switch-to-history'),
      })

      const html = await renderToString(vnode)

      // 1. Must render offline state with the offlineNodeName
      assert.ok(html.includes('offline-node-empty'), 'Must contain offline-node-empty container')
      assert.ok(html.includes('k8s-worker-node-1'), 'Must display the offline node name in title')

      // 2. Must render the CTA button with .btn-switch-history
      assert.ok(html.includes('btn-switch-history'), 'Must render button with class .btn-switch-history')

      // 3. Must render the clock icon inside the CTA button
      assert.ok(html.includes('data-icon="clock"'), 'Must render clock icon inside button')

      // 4. Must render "Switch to History Logs" text inside CTA button
      assert.ok(html.includes('Switch to History Logs'), 'Must render "Switch to History Logs" button label')

      // 5. Must emit switch-to-history on click
      const clickHandler = vnode.component?.exposed?.onSwitchToHistory || vnode.component?.setupState?.onSwitchToHistory
      assert.ok(clickHandler, 'Must expose onSwitchToHistory handler')
      clickHandler()

      assert.ok(
        emitted.includes('switch-to-history'),
        `Expected switch-to-history event to be emitted, received: ${JSON.stringify(emitted)}`
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
  })

  describe('Criterion 1 & 2: Token Grounding & CSS Specifications', () => {
    const tokensPath = path.resolve(process.cwd(), 'src/assets/css/tokens.css')
    const tokensContent = fs.readFileSync(tokensPath, 'utf8')

    it('tokens.css defines all required semantic variables in both :root and [data-theme="light"]', () => {
      const rootMatch = tokensContent.match(/:root\s*\{([^}]+)\}/s)
      const lightMatch = tokensContent.match(/\[data-theme=["']light["']\]\s*\{([^}]+)\}/s)

      assert.ok(rootMatch, ':root block must exist in tokens.css')
      assert.ok(lightMatch, '[data-theme="light"] block must exist in tokens.css')

      const requiredTokens = [
        '--accent-primary',
        '--accent-hover',
        '--bg-card',
        '--bg-elevated',
        '--text-primary',
        '--text-secondary',
        '--border-subtle',
      ]

      for (const token of requiredTokens) {
        assert.ok(rootMatch[1].includes(token), `:root must define ${token}`)
        assert.ok(lightMatch[1].includes(token), `[data-theme="light"] must define ${token}`)
      }
    })

    it('virtual-log-terminal.css and LogTerminalEmptyState style .btn-switch-history with tokens and 8px gap', () => {
      const emptyStateContent = fs.readFileSync(
        path.resolve(process.cwd(), 'src/components/logs/LogTerminalEmptyState.vue'),
        'utf8'
      )
      const terminalCssContent = fs.readFileSync(
        path.resolve(process.cwd(), 'src/assets/styles/components/virtual-log-terminal.css'),
        'utf8'
      )

      for (const [name, content] of [
        ['LogTerminalEmptyState.vue', emptyStateContent],
        ['virtual-log-terminal.css', terminalCssContent],
      ]) {
        assert.ok(content.includes('.btn-switch-history'), `${name} must contain .btn-switch-history`)
        assert.ok(content.includes('gap: 8px'), `${name} must set gap: 8px for button icon spacing`)
        assert.ok(content.includes('var(--accent-primary'), `${name} must use var(--accent-primary)`)
        assert.ok(content.includes('var(--accent-hover'), `${name} must use var(--accent-hover)`)
        assert.ok(content.includes('flex-shrink: 0'), `${name} must set flex-shrink: 0 on svg`)
      }
    })
  })

  describe('Criterion 3: Zero Undefined CSS Custom Properties in /logs Stylesheets', () => {
    it('has zero undefined CSS variables in virtual-log-terminal, logstream, log-error-radar, and Vue components', () => {
      const tokensContent = fs.readFileSync(
        path.resolve(process.cwd(), 'src/assets/css/tokens.css'),
        'utf8'
      )
      const tokenVars = new Set([...tokensContent.matchAll(/--[a-zA-Z0-9_-]+/g)].map((m) => m[0]))

      const filesToAudit = [
        'src/assets/styles/components/virtual-log-terminal.css',
        'src/assets/styles/components/log-error-radar.css',
        'src/assets/styles/views/logstream.css',
        'src/assets/styles/views/logstream-tree.css',
        'src/assets/styles/views/logstream-terminal.css',
        'src/components/logs/LogTerminalEmptyState.vue',
        'src/components/logs/LogErrorRadarPanel.vue',
        'src/views/LogStreamView.vue',
      ]

      const undefinedFound: { file: string; variable: string }[] = []

      for (const relPath of filesToAudit) {
        const fullPath = path.resolve(process.cwd(), relPath)
        const content = fs.readFileSync(fullPath, 'utf8')
        const matches = [...content.matchAll(/var\(\s*(--[a-zA-Z0-9_-]+)/g)]

        for (const m of matches) {
          const v = m[1]
          if (!tokenVars.has(v) && v !== '--sab' && v !== '--sat') {
            undefinedFound.push({ file: relPath, variable: v })
          }
        }
      }

      assert.deepEqual(undefinedFound, [], `Found undefined CSS variables: ${JSON.stringify(undefinedFound)}`)
    })
  })

  describe('Criterion 4: Strict Architectural Line-Count Limits', () => {
    it('enforces Views <= 350 lines, Components < 300 lines, CSS < 500 lines', () => {
      const limits: Record<string, number> = {
        'src/views/LogStreamView.vue': 350,
        'src/components/logs/LogTerminalEmptyState.vue': 300,
        'src/components/logs/VirtualLogTerminal.vue': 300,
        'src/components/logs/LogErrorRadarPanel.vue': 300,
        'src/components/logs/LogViewerTerminal.vue': 300,
        'src/components/logs/LogTargetTree.vue': 300,
        'src/assets/styles/components/virtual-log-terminal.css': 500,
        'src/assets/styles/components/log-error-radar.css': 500,
        'src/assets/styles/views/logstream.css': 500,
        'src/assets/styles/views/logstream-tree.css': 500,
        'src/assets/styles/views/logstream-terminal.css': 500,
        'tests/task074LogsFix.test.ts': 500,
        'tests/task074LogsCtaAndTokens.test.ts': 500,
      }

      for (const [relPath, maxLines] of Object.entries(limits)) {
        const fullPath = path.resolve(process.cwd(), relPath)
        const content = fs.readFileSync(fullPath, 'utf8')
        const lineCount = content.split('\n').length
        assert.ok(lineCount <= maxLines, `${relPath} has ${lineCount} lines, exceeding ceiling of ${maxLines} lines`)
      }
    })
  })
})
