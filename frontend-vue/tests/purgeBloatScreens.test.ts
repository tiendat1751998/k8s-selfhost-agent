import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { navGroups } from '../src/config/navigation.ts'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

const PURGED_PATHS = [
  '/promotions',
  '/drift',
  '/compliance',
  '/backup',
  '/ai-hub',
  '/changes',
  '/reports',
  '/scaffolder',
  '/plugins',
  '/catalog',
  '/ecosystem'
]

const ORPHANED_VIEW_FILES = [
  'PromotionsView.vue',
  'DriftView.vue',
  'ComplianceView.vue',
  'BackupRestoreView.vue',
  'AIProviderHubView.vue',
  'ChangesView.vue',
  'ReportsView.vue',
  'ScaffolderView.vue',
  'ScaffolderTemplatesView.vue',
  'PluginsView.vue',
  'PluginHubView.vue',
  'ServiceCatalogView.vue',
  'EcosystemView.vue',
  'EcosystemToolsView.vue'
]

describe('Purge All Fake Bloat Screens Acceptance Specification', () => {
  const navFilePath = path.join(rootDir, 'src', 'config', 'navigation.ts')
  const routerFilePath = path.join(rootDir, 'src', 'router', 'index.ts')
  const appVuePath = path.join(rootDir, 'src', 'App.vue')

  it('A. navigation.ts: navGroups strictly excludes all 11 bloat items', () => {
    const allNavPaths = navGroups.flatMap(g => g.items.map(i => i.path))
    for (const purged of PURGED_PATHS) {
      assert.strictEqual(
        allNavPaths.includes(purged),
        false,
        `navGroups must not contain purged route: ${purged}`
      )
    }
    assert.strictEqual(allNavPaths.includes('/cost'), false, 'navGroups must not contain /cost')
    assert.strictEqual(allNavPaths.includes('/capacity'), false, 'navGroups must not contain /capacity')

    const expectedGroups = [
      {
        key: 'observability',
        items: ['/', '/incidents', '/slo', '/logs']
      },
      {
        key: 'compute',
        items: ['/fleet', '/hosts', '/deployments', '/explorer', '/helm']
      },
      {
        key: 'governance',
        items: ['/audit']
      },
      {
        key: 'automation',
        items: ['/automation', '/runbooks']
      },
      {
        key: 'management',
        items: ['/tenancy', '/alerts', '/settings']
      }
    ]

    assert.strictEqual(navGroups.length, expectedGroups.length, 'Must have exactly 5 nav groups')
    for (let i = 0; i < expectedGroups.length; i++) {
      assert.strictEqual(navGroups[i].key, expectedGroups[i].key)
      const currentPaths = navGroups[i].items.map(it => it.path)
      assert.deepStrictEqual(currentPaths, expectedGroups[i].items)
    }
  })

  it('B. router/index.ts: redirects all 11 deprecated paths to / with no dead imports', () => {
    const routerContent = fs.readFileSync(routerFilePath, 'utf-8')
    for (const purged of PURGED_PATHS) {
      const redirectRegex = new RegExp(`path:\\s*['"]${purged}['"][\\s\\S]*?redirect:\\s*['"]/['"]`)
      assert.ok(
        redirectRegex.test(routerContent),
        `router/index.ts must redirect deprecated path ${purged} to /`
      )
    }

    for (const viewFile of ORPHANED_VIEW_FILES) {
      assert.strictEqual(
        routerContent.includes(viewFile),
        false,
        `router/index.ts must not import orphaned view file: ${viewFile}`
      )
    }
  })

  it('C. App.vue: routeBreadcrumbs removes all purged screen entries', () => {
    const appContent = fs.readFileSync(appVuePath, 'utf-8')
    const breadcrumbMatch = appContent.match(/const routeBreadcrumbs: Record<string, \{ category: string; title: string \}> = \{([\s\S]*?)\n\}/)
    assert.ok(breadcrumbMatch, 'App.vue must define routeBreadcrumbs')
    const breadcrumbBlock = breadcrumbMatch[1]

    for (const purged of PURGED_PATHS) {
      assert.strictEqual(
        breadcrumbBlock.includes(purged),
        false,
        `App.vue routeBreadcrumbs must not contain purged route: ${purged}`
      )
    }
  })

  it('D. src/views: all orphaned view files are completely deleted from disk', () => {
    const viewsDir = path.join(rootDir, 'src', 'views')
    for (const file of ORPHANED_VIEW_FILES) {
      const fullPath = path.join(viewsDir, file)
      assert.strictEqual(
        fs.existsSync(fullPath),
        false,
        `View file ${file} must be deleted from disk`
      )
    }
  })

  it('E. Line counts: all modified files strictly stay < 500 lines', () => {
    const filesToAudit = [
      navFilePath,
      routerFilePath,
      appVuePath,
      __filename
    ]

    for (const file of filesToAudit) {
      const lineCount = fs.readFileSync(file, 'utf-8').split('\n').length
      assert.ok(
        lineCount < 500,
        `File ${path.basename(file)} (${lineCount} lines) must be strictly < 500 lines`
      )
    }
  })
})
