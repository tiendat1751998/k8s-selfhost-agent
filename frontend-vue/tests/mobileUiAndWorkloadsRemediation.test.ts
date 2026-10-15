import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

describe('Mobile UI Defect Remediation & Workload Stream', () => {
  const paletteVuePath = path.join(rootDir, 'src', 'components', 'layout', 'AppCommandPalette.vue')
  const appShellCssPath = path.join(rootDir, 'src', 'assets', 'styles', 'layout', 'app-shell.css')
  const pwaBannerVuePath = path.join(rootDir, 'src', 'components', 'pwa', 'PwaInstallBanner.vue')
  const pwaBannerCssPath = path.join(rootDir, 'src', 'assets', 'styles', 'components', 'pwa-install-banner.css')
  const mockServerJsPath = path.join(rootDir, 'scripts', 'mock-preview-server.js')
  const useDeploymentsTsPath = path.join(rootDir, 'src', 'composables', 'useDeployments.ts')

  it('Command Palette: groups items by category and eliminates badge collision', () => {
    assert.ok(fs.existsSync(paletteVuePath), 'AppCommandPalette.vue must exist')
    const paletteContent = fs.readFileSync(paletteVuePath, 'utf-8')

    // Must have category grouping header
    assert.match(paletteContent, /palette-group-header/, 'Must render palette-group-header for category grouping')
    
    // Must NOT contain per-item p-item-group badge inside p-item-title-row
    assert.doesNotMatch(paletteContent, /<span class="p-item-group/, 'Must not render redundant p-item-group on every card')

    // App shell CSS must eliminate column-reverse hack
    assert.ok(fs.existsSync(appShellCssPath), 'app-shell.css must exist')
    const appShellContent = fs.readFileSync(appShellCssPath, 'utf-8')
    assert.doesNotMatch(
      appShellContent,
      /\.p-item-title-row\s*\{\s*flex-direction:\s*column-reverse/,
      'app-shell.css must eliminate column-reverse rule on .p-item-title-row'
    )
    assert.match(appShellContent, /\.palette-group-header/, 'app-shell.css must style .palette-group-header')
  })

  it('PWA Banner: eliminates mobile truncation and supports compact responsive styling', () => {
    assert.ok(fs.existsSync(pwaBannerVuePath), 'PwaInstallBanner.vue must exist')
    const bannerContent = fs.readFileSync(pwaBannerVuePath, 'utf-8')

    // Must provide short mobile title or wrapping to avoid "Install K8s..." truncation
    assert.match(
      bannerContent,
      /Install App|title-short/,
      'PwaInstallBanner.vue must provide responsive title adaptation for mobile viewports'
    )

    assert.ok(fs.existsSync(pwaBannerCssPath), 'pwa-install-banner.css must exist')
    const bannerCss = fs.readFileSync(pwaBannerCssPath, 'utf-8')

    // Must adapt on mobile <= 640px
    assert.match(bannerCss, /@media\s*\(max-width:\s*640px\)/, 'pwa-install-banner.css must have mobile media query')
    assert.match(bannerCss, /title-short/, 'pwa-install-banner.css must define .title-short display rules')
  })

  it('Mock Preview Server: is completely eradicated to prevent rogue mock servers occupying port 8080', () => {
    assert.ok(!fs.existsSync(mockServerJsPath), 'mock-preview-server.js must not exist in repository')
  })

  it('useDeployments & Mobile Cards: handles empty states cleanly without hardcoded fallback collusion', () => {
    assert.ok(fs.existsSync(useDeploymentsTsPath), 'useDeployments.ts must exist')
    const useDeploymentsContent = fs.readFileSync(useDeploymentsTsPath, 'utf-8')

    // Authentic state management: initializes empty array and provides fetchDeployments
    assert.match(
      useDeploymentsContent,
      /deployments\s*=\s*ref<DeploymentApp\[\]>\(\[\]\)/,
      'useDeployments.ts must initialize deployments with an authentic empty array'
    )
    assert.match(useDeploymentsContent, /fetchDeployments/, 'useDeployments.ts must provide fetchDeployments')

    // Authentic empty state in mobile cards
    const mobileCardsVuePath = path.join(rootDir, 'src', 'components', 'deployments', 'DeploymentsMobileCards.vue')
    assert.ok(fs.existsSync(mobileCardsVuePath), 'DeploymentsMobileCards.vue must exist')
    const mobileCardsContent = fs.readFileSync(mobileCardsVuePath, 'utf-8')
    assert.match(mobileCardsContent, /empty-mobile-state/, 'Mobile cards must render authentic empty state')
    assert.match(mobileCardsContent, /0 Workloads matching active filters/, 'Mobile cards must display clear empty state message')
  })

  it('File line count constraints: all modified files strictly < 500 lines', () => {
    const files = [
      paletteVuePath,
      appShellCssPath,
      pwaBannerVuePath,
      pwaBannerCssPath,
      useDeploymentsTsPath,
    ]

    for (const f of files) {
      if (fs.existsSync(f)) {
        const content = fs.readFileSync(f, 'utf-8')
        const lineCount = content.split('\n').length
        assert.ok(
          lineCount < 500,
          `File ${path.basename(f)} must be under 500 lines (current: ${lineCount})`
        )
      }
    }
  })
})
