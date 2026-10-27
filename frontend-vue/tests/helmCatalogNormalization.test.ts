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

const { normalizeHelmChart } = await import('../src/composables/useHelm.ts')
const { filterCharts, buildInstallPayload } = await import('../src/composables/helmCatalogConstants.ts')

describe('Task 070 Wave 2C: Helm Catalog 120 Charts Normalization & De-Duplication', () => {
  describe('1. normalizeHelmChart: Go Helm SDK & flat shapes', () => {
    it('normalizes Go Helm SDK nested Chart result (bitnami/postgresql)', () => {
      const raw = {
        Name: 'bitnami/postgresql',
        Score: 0,
        Chart: {
          name: 'postgresql',
          version: '12.1.0',
          appVersion: '15.2.0',
          description: 'Chart for PostgreSQL database',
          icon: 'https://bitnami.com/icons/postgresql.png',
          keywords: ['database', 'sql', 'postgres']
        }
      }
      const chart = normalizeHelmChart(raw)
      assert.equal(chart.name, 'postgresql')
      assert.equal(chart.repo, 'bitnami')
      assert.equal(chart.version, '12.1.0')
      assert.equal(chart.appVersion, '15.2.0')
      assert.equal(chart.description, 'Chart for PostgreSQL database')
      assert.equal(chart.icon, 'https://bitnami.com/icons/postgresql.png')
      assert.deepEqual(chart.keywords, ['database', 'sql', 'postgres'])
    })

    it('normalizes Go Helm SDK nested Chart.Metadata result (ingress-nginx)', () => {
      const raw = {
        Name: 'ingress-nginx/ingress-nginx',
        Chart: {
          Metadata: {
            name: 'ingress-nginx',
            version: 'v4.8.0',
            appVersion: '1.9.0',
            description: 'Ingress controller for Kubernetes using NGINX',
            icon: 'https://kubernetes.github.io/ingress-nginx/icon.png'
          }
        }
      }
      const chart = normalizeHelmChart(raw)
      assert.equal(chart.name, 'ingress-nginx')
      assert.equal(chart.repo, 'ingress-nginx')
      assert.equal(chart.version, '4.8.0', 'Leading v should be stripped for clean display')
      assert.equal(chart.description, 'Ingress controller for Kubernetes using NGINX')
    })

    it('normalizes Go Helm SDK PascalCase fields (prometheus-community/prometheus)', () => {
      const raw = {
        Name: 'prometheus-community/prometheus',
        Chart: {
          Name: 'prometheus',
          Version: '25.0.0',
          AppVersion: '2.47.0',
          Description: 'Prometheus monitoring system and time series database'
        }
      }
      const chart = normalizeHelmChart(raw)
      assert.equal(chart.name, 'prometheus')
      assert.equal(chart.repo, 'prometheus-community')
      assert.equal(chart.version, '25.0.0')
      assert.equal(chart.description, 'Prometheus monitoring system and time series database')
    })

    it('normalizes flat camelCase chart object', () => {
      const raw = {
        name: 'redis',
        repo: 'bitnami',
        version: '17.0.0',
        description: 'Redis in-memory data store'
      }
      const chart = normalizeHelmChart(raw)
      assert.equal(chart.name, 'redis')
      assert.equal(chart.repo, 'bitnami')
      assert.equal(chart.version, '17.0.0')
      assert.equal(chart.description, 'Redis in-memory data store')
    })
  })

  describe('2. Chart Filtering & Installation Payload', () => {
    const testCharts = [
      normalizeHelmChart({ Name: 'bitnami/postgresql', Chart: { name: 'postgresql', version: '12.1.0', keywords: ['database'] } }),
      normalizeHelmChart({ Name: 'bitnami/redis', Chart: { name: 'redis', version: '17.0.0', keywords: ['database', 'cache'] } }),
      normalizeHelmChart({ Name: 'ingress-nginx/ingress-nginx', Chart: { name: 'ingress-nginx', version: '4.8.0', keywords: ['networking'] } }),
      normalizeHelmChart({ Name: 'prometheus-community/prometheus', Chart: { name: 'prometheus', version: '25.0.0', keywords: ['monitoring'] } }),
    ]

    it('filters charts by repository', () => {
      const bitnamiOnly = filterCharts(testCharts, 'bitnami', 'all')
      assert.equal(bitnamiOnly.length, 2)
      assert.ok(bitnamiOnly.every(c => c.repo === 'bitnami'))
    })

    it('filters charts by category tag', () => {
      const dbCharts = filterCharts(testCharts, 'all', 'database')
      assert.equal(dbCharts.length, 2)
      const names = dbCharts.map(c => c.name)
      assert.ok(names.includes('postgresql'))
      assert.ok(names.includes('redis'))
    })

    it('buildInstallPayload constructs valid payload with correct repo and chart', () => {
      const chart = testCharts[0] // postgresql
      const form = {
        releaseName: 'prod-pg',
        namespace: 'db-tier',
        createNamespace: true,
        version: '12.1.0',
        values: 'replicaCount: 2',
        loadingDefaultValues: false,
      }
      const payload = buildInstallPayload(chart, form)
      assert.equal(payload.releaseName, 'prod-pg')
      assert.equal(payload.chart, 'postgresql')
      assert.equal(payload.repo, 'bitnami')
      assert.equal(payload.version, '12.1.0')
      assert.equal(payload.namespace, 'db-tier')
      assert.equal(payload.createNamespace, true)
    })
  })

  describe('3. Toolbar De-duplication (LES-003)', () => {
    it('HelmChartsGrid does not contain duplicate search input or repo select', () => {
      const gridFile = path.resolve(import.meta.dirname, '../src/components/helm/HelmChartsGrid.vue')
      const content = fs.readFileSync(gridFile, 'utf8')

      // Must not render a secondary search input
      assert.equal(content.includes('placeholder="Search Helm charts'), false, 'HelmChartsGrid must NOT have duplicate search input')
      assert.equal(content.includes('class="repo-filter-box"'), false, 'HelmChartsGrid must NOT have duplicate repository select')
    })
  })
})
