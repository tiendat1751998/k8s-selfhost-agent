import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { register } from 'node:module'

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

const { parseRunbookSteps } = await import('../src/composables/useRunbooks.ts')

describe('Runbooks SOP Step Parser & Global UI Hygiene Tests', () => {
  it('parseRunbookSteps correctly parses Markdown headings, numbered steps and code fences', () => {
    const markdownContent = `
## Step 1: Verify Pod Health
Check if the ingress-controller pods are running.
\`\`\`bash
kubectl get pods -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx
\`\`\`

## Step 2: Cordon and Drain Degraded Worker
If worker-node-2 is unresponsive, cordon it first:
\`kubectl cordon worker-node-2\`
Then evict workloads with a 60s timeout:
\`\`\`sh
kubectl drain worker-node-2 --ignore-daemonsets --delete-emptydir-data --force
\`\`\`

3. Verify Cluster Rescheduling
Inspect events across the cluster.
\`kubectl get events -A --sort-by='.metadata.creationTimestamp'\`
`

    const steps = parseRunbookSteps(markdownContent, 'Ingress Triage')
    assert.equal(steps.length, 3, 'Should parse exactly 3 distinct procedure steps')

    // Step 1 assertions
    assert.equal(steps[0].stepNum, 1)
    assert.match(steps[0].title, /Verify Pod Health/i)
    assert.match(steps[0].content, /ingress-controller pods/i)
    assert.equal(steps[0].command, 'kubectl get pods -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx')
    assert.equal(steps[0].status, 'idle')

    // Step 2 assertions (combining inline and fence command)
    assert.equal(steps[1].stepNum, 2)
    assert.match(steps[1].title, /Cordon and Drain Degraded Worker/i)
    assert.ok(steps[1].command?.includes('kubectl cordon worker-node-2'))
    assert.ok(steps[1].command?.includes('kubectl drain worker-node-2'))
    assert.deepEqual(steps[1].dependencies, ['step-1'])

    // Step 3 assertions
    assert.equal(steps[2].stepNum, 3)
    assert.match(steps[2].title, /Verify Cluster Rescheduling/i)
    assert.equal(steps[2].command, "kubectl get events -A --sort-by='.metadata.creationTimestamp'")
    assert.deepEqual(steps[2].dependencies, ['step-2'])
  })

  it('parseRunbookSteps handles empty or plain text gracefully with fallback', () => {
    const emptySteps = parseRunbookSteps('', 'Fallback SOP')
    assert.equal(emptySteps.length, 0)

    const plainText = 'Perform standard hardware diagnosis and check power cords.'
    const plainSteps = parseRunbookSteps(plainText, 'Hardware Diagnosis')
    assert.equal(plainSteps.length, 1)
    assert.equal(plainSteps[0].stepNum, 1)
    assert.equal(plainSteps[0].title, 'Overview Procedure')
    assert.equal(plainSteps[0].content, plainText)
  })

  it('Double-Plus Check: Zero "<BaseIcon name=\\"plus\\"" followed by literal "+ " across all audited files', () => {
    const filesToCheck = [
      '../src/components/alerts/AlertsHeaderBar.vue',
      '../src/components/explorer/ExplorerToolbar.vue',
      '../src/components/reports/ReportsFilterToolbar.vue',
      '../src/components/ui/FacetedFilterBar.vue',
      '../src/views/RunbooksView.vue',
    ]

    for (const file of filesToCheck) {
      const fullPath = path.resolve(import.meta.dirname, file)
      assert.ok(fs.existsSync(fullPath), `File must exist: ${file}`)
      const content = fs.readFileSync(fullPath, 'utf-8')

      // Check for <BaseIcon name="plus" ... /> followed closely by <span>+ ...
      const doublePlusPattern = /<BaseIcon[^>]*name=["']plus["'][^>]*>[\s\r\n]*<span[^>]*>\s*\+\s+/gi
      const matches = content.match(doublePlusPattern)
      assert.equal(
        matches,
        null,
        `File ${file} has redundant "+ " inside span after plus icon: ${matches ? matches.join(', ') : ''}`
      )

      // Also ensure specific audited strings don't exist
      assert.doesNotMatch(content, />\s*\+\s*New Rule</i, `${file} still contains "+ New Rule"`)
      assert.doesNotMatch(content, />\s*\+\s*Add Channel</i, `${file} still contains "+ Add Channel"`)
      assert.doesNotMatch(content, />\s*\+\s*Create Resource</i, `${file} still contains "+ Create Resource"`)
      assert.doesNotMatch(content, />\s*\+\s*Generate Report</i, `${file} still contains "+ Generate Report"`)
      assert.doesNotMatch(content, />\s*\+\s*Filter</i, `${file} still contains "+ Filter"`)
      assert.doesNotMatch(content, />\s*\+\s*New Runbook</i, `${file} still contains "+ New Runbook"`)
    }
  })

  it('tokens.css: --color-primary-fg is defined in :root and [data-theme="light"]', () => {
    const tokensPath = path.resolve(import.meta.dirname, '../src/assets/css/tokens.css')
    const tokensContent = fs.readFileSync(tokensPath, 'utf-8')

    assert.match(
      tokensContent,
      /--color-primary-fg:\s*var\(--color-on-primary\);/,
      'tokens.css must define --color-primary-fg: var(--color-on-primary);'
    )
  })

  it('base.css: toolbar clipping fix allows wrapping and no hard max-height', () => {
    const basePath = path.resolve(import.meta.dirname, '../src/assets/css/base.css')
    const baseContent = fs.readFileSync(basePath, 'utf-8')

    // Find the [class*="-toolbar-sleek"] rule block
    const sleekToolbarMatch = baseContent.match(/\[class\*="-toolbar-sleek"\]\s*\{([^}]+)\}/)
    assert.ok(sleekToolbarMatch, 'base.css must contain [class*="-toolbar-sleek"] block')

    const ruleBlock = sleekToolbarMatch[1]
    assert.match(ruleBlock, /min-height:\s*48px/i, 'Must have min-height: 48px')
    assert.match(ruleBlock, /height:\s*auto/i, 'Must have height: auto !important')
    assert.match(ruleBlock, /max-height:\s*none/i, 'Must have max-height: none !important')
    assert.match(ruleBlock, /flex-wrap:\s*wrap/i, 'Must have flex-wrap: wrap !important')
  })

  it('deployments-topology.css: namespace cards grid uses auto-fit minmax(380px, 1fr)', () => {
    const topologyPath = path.resolve(import.meta.dirname, '../src/assets/styles/views/deployments-topology.css')
    const topologyContent = fs.readFileSync(topologyPath, 'utf-8')

    assert.match(
      topologyContent,
      /repeat\(auto-fit,\s*minmax\(380px,\s*1fr\)\)/,
      'Must use repeat(auto-fit, minmax(380px, 1fr)) for topology cards grid'
    )
  })

  it('HelmCatalogView.vue: Zero legacy .cyber-* classes remain', () => {
    const helmViewPath = path.resolve(import.meta.dirname, '../src/views/HelmCatalogView.vue')
    const helmContent = fs.readFileSync(helmViewPath, 'utf-8')

    assert.doesNotMatch(helmContent, /cyber-toast/, 'HelmCatalogView.vue must not have cyber-toast')
    assert.doesNotMatch(helmContent, /btn-cyber/, 'HelmCatalogView.vue must not have btn-cyber')
    assert.doesNotMatch(helmContent, /cyber-spinner/, 'HelmCatalogView.vue must not have cyber-spinner')
    assert.doesNotMatch(helmContent, /cyber-table/, 'HelmCatalogView.vue must not have cyber-table')
  })

  it('RunbookPlaybookConsole.vue exists and stays under 290 lines', () => {
    const consolePath = path.resolve(import.meta.dirname, '../src/components/runbooks/RunbookPlaybookConsole.vue')
    assert.ok(fs.existsSync(consolePath), 'RunbookPlaybookConsole.vue must exist')

    const content = fs.readFileSync(consolePath, 'utf-8')
    const lineCount = content.split('\n').length
    assert.ok(
      lineCount <= 290,
      `RunbookPlaybookConsole.vue has ${lineCount} lines, exceeding strict limit of 290 lines`
    )
  })

  it('Strict line limits (< 500 lines) across all modified files', () => {
    const filesToCheck = [
      { file: '../src/views/RunbooksView.vue', maxLines: 350 },
      { file: '../src/components/runbooks/RunbookPlaybookConsole.vue', maxLines: 290 },
      { file: '../src/components/alerts/AlertsHeaderBar.vue', maxLines: 500 },
      { file: '../src/components/explorer/ExplorerToolbar.vue', maxLines: 500 },
      { file: '../src/components/reports/ReportsFilterToolbar.vue', maxLines: 500 },
      { file: '../src/components/ui/FacetedFilterBar.vue', maxLines: 500 },
      { file: '../src/views/HelmCatalogView.vue', maxLines: 500 },
      { file: '../src/composables/useRunbooks.ts', maxLines: 500 },
      { file: '../src/assets/css/base.css', maxLines: 500 },
      { file: '../src/assets/css/tokens.css', maxLines: 500 },
      { file: '../src/assets/styles/views/deployments-topology.css', maxLines: 500 },
      { file: '../src/assets/styles/views/helm.css', maxLines: 500 },
      { file: '../src/assets/styles/views/runbooks-console.css', maxLines: 500 },
    ]

    for (const { file, maxLines } of filesToCheck) {
      const fullPath = path.resolve(import.meta.dirname, file)
      if (fs.existsSync(fullPath)) {
        const lines = fs.readFileSync(fullPath, 'utf-8').split('\n').length
        assert.ok(lines <= maxLines, `${file} has ${lines} lines, exceeding limit of ${maxLines}`)
      }
    }
  })
})
