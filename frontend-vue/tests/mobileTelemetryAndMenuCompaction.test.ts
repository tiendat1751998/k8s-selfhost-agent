import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

describe('Mobile Telemetry & Header Stack Compaction Specifications', () => {
  const appShellCssPath = path.join(rootDir, 'src', 'assets', 'styles', 'layout', 'app-shell.css')
  const deploymentsViewPath = path.join(rootDir, 'src', 'views', 'DeploymentsView.vue')
  const deploymentsCssPath = path.join(rootDir, 'src', 'assets', 'styles', 'views', 'deployments.css')
  const mobileCardsVuePath = path.join(rootDir, 'src', 'components', 'deployments', 'DeploymentsMobileCards.vue')
  const apiDeploymentsTsPath = path.join(rootDir, 'src', 'api', 'deployments.ts')
  const useDeploymentsTsPath = path.join(rootDir, 'src', 'composables', 'useDeployments.ts')
  const inspectorCssPath = path.join(rootDir, 'src', 'assets', 'styles', 'components', 'workload-inspector.css')

  it('1. app-shell.css: compacts mobile HUD to 42px, padding-left 0, and page-container top padding 6px', () => {
    assert.ok(fs.existsSync(appShellCssPath), 'app-shell.css must exist')
    const css = fs.readFileSync(appShellCssPath, 'utf-8')

    // Mobile HUD height 42px
    assert.match(
      css,
      /--mobile-hud-height:\s*42px|height:\s*42px/,
      'app-shell.css must reduce mobile HUD height to 42px'
    )

    // hud-left on mobile padding-left: 0
    assert.match(
      css,
      /\.hud-left\s*\{[^}]*padding-left:\s*0/s,
      'app-shell.css must set .hud-left padding-left: 0 on mobile'
    )

    // page-container top padding 6px
    assert.match(
      css,
      /\.page-container\s*\{[^}]*padding:\s*6px\s+12px\s+80px/s,
      'app-shell.css must reduce mobile .page-container top padding to 6px'
    )
  })

  it('2. DeploymentsView.vue & deployments.css: 36px mobile command bar, eliminates separate micro-telemetry strip, inline telemetry pill', () => {
    assert.ok(fs.existsSync(deploymentsViewPath), 'DeploymentsView.vue must exist')
    assert.ok(fs.existsSync(deploymentsCssPath), 'deployments.css must exist')

    const viewContent = fs.readFileSync(deploymentsViewPath, 'utf-8')
    const cssContent = fs.readFileSync(deploymentsCssPath, 'utf-8')

    // deployments.css compacts mobile-command-bar height to 36px
    assert.match(
      cssContent,
      /\.mobile-command-bar\s*\{[^}]*height:\s*36px/s,
      'deployments.css must compact .mobile-command-bar height to 36px'
    )

    // DeploymentsView.vue eliminates separate .micro-telemetry-strip row
    assert.doesNotMatch(
      viewContent,
      /<div\s+class="micro-telemetry-strip/,
      'DeploymentsView.vue must eliminate separate micro-telemetry-strip row'
    )

    // DeploymentsView.vue integrates inline telemetry into command bar
    assert.match(
      viewContent,
      /mobile-telemetry-pill|mobile-compact-badge/,
      'DeploymentsView.vue must integrate telemetry into the command bar'
    )
    assert.match(
      viewContent,
      /pods/,
      'Inline telemetry pill must show pods status'
    )
  })

  it('3. api/deployments.ts & useDeployments.ts: adds restarts & age to DeploymentApp and sample workloads', () => {
    assert.ok(fs.existsSync(apiDeploymentsTsPath), 'deployments.ts must exist')
    assert.ok(fs.existsSync(useDeploymentsTsPath), 'useDeployments.ts must exist')

    const apiContent = fs.readFileSync(apiDeploymentsTsPath, 'utf-8')
    const composableContent = fs.readFileSync(useDeploymentsTsPath, 'utf-8')

    // Interface check
    assert.match(
      apiContent,
      /restarts\?:?\s*number/,
      'DeploymentApp interface must have restarts field'
    )
    assert.match(
      apiContent,
      /age\?:?\s*string/,
      'DeploymentApp interface must have age field'
    )

    // FALLBACK_SAMPLE_WORKLOADS checks
    assert.match(
      composableContent,
      /restarts:\s*\d+/,
      'FALLBACK_SAMPLE_WORKLOADS must supply realistic restarts telemetry'
    )
    assert.match(
      composableContent,
      /age:\s*['"][0-9]+[dhms]['"]/,
      'FALLBACK_SAMPLE_WORKLOADS must supply realistic age telemetry'
    )
  })

  it('4. DeploymentsMobileCards.vue: renders CPU, RAM, Restarts, Age, and clear Inspect button', () => {
    assert.ok(fs.existsSync(mobileCardsVuePath), 'DeploymentsMobileCards.vue must exist')
    const content = fs.readFileSync(mobileCardsVuePath, 'utf-8')

    // Telemetry items
    assert.match(content, /app\.cpu/, 'Mobile cards must render CPU telemetry')
    assert.match(content, /app\.memory/, 'Mobile cards must render Memory/RAM telemetry')
    assert.match(content, /app\.restarts/, 'Mobile cards must render Restarts telemetry')
    assert.match(content, /app\.age/, 'Mobile cards must render Age telemetry')

    // Restarts warning styling
    assert.match(content, /text-amber|restarts-warning|restarts-alert/, 'Mobile cards must highlight restarts > 0 in amber')

    // Inspect button
    assert.match(content, /Inspect|Chi tiết/, 'Mobile cards must provide prominent Inspect / Chi tiết button')
    assert.doesNotMatch(content, /btn-quick-inspect[^>]*>\s*<BaseIcon name="zap"/, 'Must replace cryptic zap icon with explicit Inspect action')
  })

  it('5. workload-inspector.css: responsive 2-column quick actions on mobile <= 640px', () => {
    assert.ok(fs.existsSync(inspectorCssPath), 'workload-inspector.css must exist')
    const css = fs.readFileSync(inspectorCssPath, 'utf-8')

    assert.match(
      css,
      /@media\s*\(max-width:\s*640px\)\s*\{[^}]*\.inspector-quick-actions\s*\{[^}]*grid-template-columns:\s*repeat\(2,\s*1fr\)/s,
      'workload-inspector.css must set 2-column grid for .inspector-quick-actions on mobile <= 640px'
    )
  })

  it('6. Line count constraints: all modified files strictly < 500 lines', () => {
    const files = [
      appShellCssPath,
      deploymentsViewPath,
      deploymentsCssPath,
      mobileCardsVuePath,
      apiDeploymentsTsPath,
      useDeploymentsTsPath,
      inspectorCssPath,
    ]

    for (const f of files) {
      const content = fs.readFileSync(f, 'utf-8')
      const lineCount = content.split('\n').length
      assert.ok(
        lineCount < 500,
        `File ${path.basename(f)} has ${lineCount} lines, must be strictly under 500`
      )
    }
  })
})
