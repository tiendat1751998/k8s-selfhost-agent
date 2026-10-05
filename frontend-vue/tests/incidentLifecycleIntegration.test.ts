import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

describe('Incident Lifecycle UI Integration & Anti-Mock Verification', () => {
  const incidentsApiPath = path.join(rootDir, 'src', 'api', 'incidents.ts')
  const useIncidentsPath = path.join(rootDir, 'src', 'composables', 'useIncidents.ts')
  const incidentHelpersPath = path.join(rootDir, 'src', 'composables', 'incidentHelpers.ts')
  const incidentDetailPanePath = path.join(rootDir, 'src', 'components', 'incidents', 'IncidentDetailPane.vue')
  const incidentTriageDrawerPath = path.join(rootDir, 'src', 'components', 'incidents', 'IncidentTriageDrawer.vue')
  const incidentsViewPath = path.join(rootDir, 'src', 'views', 'IncidentsView.vue')

  const targetFiles = [
    incidentsApiPath,
    useIncidentsPath,
    incidentHelpersPath,
    incidentDetailPanePath,
    incidentTriageDrawerPath,
    incidentsViewPath
  ]

  it('CON-17: Purge hardcoded 95% confidence score from incident files and normalization', () => {
    const apiContent = fs.readFileSync(incidentsApiPath, 'utf-8')
    assert.strictEqual(apiContent.includes(': 0.95'), false, 'incidents.ts must not have default 0.95 fallback')
    assert.ok(apiContent.includes('confidence?: number'), 'RCAReport interface must have optional confidence')

    for (const filePath of targetFiles) {
      const content = fs.readFileSync(filePath, 'utf-8')
      assert.strictEqual(content.includes('0.95'), false, `${path.basename(filePath)} must not contain hardcoded 0.95`)
      assert.strictEqual(content.includes('0.94'), false, `${path.basename(filePath)} must not contain hardcoded 0.94`)
      assert.strictEqual(content.includes('95%'), false, `${path.basename(filePath)} must not contain hardcoded 95%`)
    }
  })

  it('CON-32: Purge fake PR #104 fallback from incident files and normalization', () => {
    const apiContent = fs.readFileSync(incidentsApiPath, 'utf-8')
    assert.strictEqual(apiContent.includes('|| 104'), false, 'incidents.ts must not have fallback to PR 104')

    for (const filePath of targetFiles) {
      const content = fs.readFileSync(filePath, 'utf-8')
      assert.strictEqual(content.includes('104'), false, `${path.basename(filePath)} must not contain fake PR 104`)
    }
  })

  it('Eradicate fake setTimeout delays simulating backend mutations', () => {
    const useIncidentsContent = fs.readFileSync(useIncidentsPath, 'utf-8')
    assert.strictEqual(useIncidentsContent.includes('setTimeout(resolve'), false, 'useIncidents must not contain fake Promise setTimeout delays')
    assert.strictEqual(/new Promise\s*\(\s*resolve\s*=>\s*setTimeout/.test(useIncidentsContent), false, 'useIncidents must not contain simulated latency delays')

    const detailPaneContent = fs.readFileSync(incidentDetailPanePath, 'utf-8')
    assert.strictEqual(detailPaneContent.includes('setTimeout'), false, 'IncidentDetailPane must not contain setTimeout')

    const triageDrawerContent = fs.readFileSync(incidentTriageDrawerPath, 'utf-8')
    assert.strictEqual(triageDrawerContent.includes('setTimeout'), false, 'IncidentTriageDrawer must not contain setTimeout')
  })

  it('Bind Mitigate and Resolve actions to real API endpoints with notes payload', () => {
    const apiContent = fs.readFileSync(incidentsApiPath, 'utf-8')
    assert.ok(apiContent.includes('/incidents/${id}/mitigate'), 'Must call POST /incidents/${id}/mitigate')
    assert.ok(apiContent.includes('/incidents/${id}/resolve'), 'Must call POST /incidents/${id}/resolve')
    assert.ok(apiContent.includes('notes: payload?.notes'), 'Must pass notes payload in mitigate and resolve')

    const useIncidentsContent = fs.readFileSync(useIncidentsPath, 'utf-8')
    assert.ok(useIncidentsContent.includes('handleMitigateIncident'), 'useIncidents must export handleMitigateIncident')
    assert.ok(useIncidentsContent.includes('handleResolveIncident'), 'useIncidents must export handleResolveIncident')
    assert.ok(useIncidentsContent.includes('await fetchIncidents()'), 'Must refresh incident status dynamically after mitigate/resolve')

    const viewContent = fs.readFileSync(incidentsViewPath, 'utf-8')
    assert.ok(viewContent.includes('@mitigate="handleMitigateIncident"'), 'IncidentsView must wire @mitigate')
    assert.ok(viewContent.includes('@resolve="handleResolveIncident"'), 'IncidentsView must wire @resolve')
  })

  it('RCA & PR 404 response handling: render clean UI empty states without errors or toasts', () => {
    const apiContent = fs.readFileSync(incidentsApiPath, 'utf-8')
    assert.ok(apiContent.includes('RCA report not found') || apiContent.includes('not found'), 'incidentsApi must gracefully handle not found report')
    assert.ok(apiContent.includes('PR not found') || apiContent.includes('not found'), 'incidentsApi must gracefully handle not found PR')

    const detailPaneContent = fs.readFileSync(incidentDetailPanePath, 'utf-8')
    assert.ok(detailPaneContent.includes('No RCA report available'), 'IncidentDetailPane must render No RCA report available empty state')
    assert.ok(detailPaneContent.includes('No PR generated'), 'IncidentDetailPane must render No PR generated notice')

    const triageDrawerContent = fs.readFileSync(incidentTriageDrawerPath, 'utf-8')
    assert.ok(triageDrawerContent.includes('No RCA report available'), 'IncidentTriageDrawer must render No RCA report available')
    assert.ok(triageDrawerContent.includes('No PR generated'), 'IncidentTriageDrawer must render No PR generated')
  })

  it('Line counts: all target files are strictly under 500 lines', () => {
    for (const filePath of targetFiles) {
      const lineCount = fs.readFileSync(filePath, 'utf-8').split('\n').length
      assert.ok(lineCount < 500, `${path.basename(filePath)} (${lineCount} lines) must be strictly under 500 lines`)
    }
  })
})
