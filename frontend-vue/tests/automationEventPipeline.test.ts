import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import {
  extractPipelineStages,
  type PipelineStageInfo,
  type RulePipelineStages,
} from '../src/composables/useAutomationPipeline.ts'
import type { AutomationRule, AutomationExecution } from '../src/api/governance.ts'

describe('Task T068-3.1: Automation WHEN -> IF -> THEN Event Pipeline & Timeline Tests', () => {
  const sampleRule1: AutomationRule = {
    id: 'e1000001-a1b2-c3d4-e5f6-7890abcdef12',
    name: 'Auto-Reap Zombie Processes',
    trigger_type: 'node_pressure',
    trigger_config: { condition: 'Zombie process count > 50', threshold: '50' },
    action_type: 'cordon_node',
    action_config: { target: 'host-k8smasterdeb', grace_period: '30s' },
    enabled: true,
    executions: 14,
    last_triggered: '2026-10-26T03:40:00Z',
    created_at: '2026-10-20T00:00:00Z',
  }

  const sampleRule2: AutomationRule = {
    id: 'e2000002-b2c3-d4e5-f6a7-8901bcdef234',
    name: 'Rollback Degraded CrashLoop Deployments',
    trigger_type: 'deployment_failure',
    trigger_config: { namespace: 'production', threshold: '3_restarts' },
    action_type: 'rollback',
    action_config: { target: 'previous_replicaset' },
    enabled: false,
    executions: 0,
    created_at: '2026-10-22T00:00:00Z',
  }

  const sampleRule3: AutomationRule = {
    id: 'e3000003-c3d4-e5f6-a7b8-9012cdef3456',
    name: 'Auto-Scale High CPU Web Tier',
    trigger_type: 'high_cpu',
    trigger_config: {},
    action_type: 'scale_deployment',
    action_config: {},
    enabled: true,
    executions: 42,
    last_triggered: '2026-10-26T03:50:00Z',
    created_at: '2026-10-15T00:00:00Z',
  }

  it('extracts WHEN -> IF -> THEN 3-stage pipeline descriptors with complete fidelity', () => {
    const stages1 = extractPipelineStages(sampleRule1)
    assert.ok(stages1.when, 'WHEN stage should exist')
    assert.ok(stages1.ifGate, 'IF stage should exist')
    assert.ok(stages1.thenAction, 'THEN stage should exist')

    // WHEN Stage check
    assert.match(stages1.when.label, /WHEN/i)
    assert.ok(stages1.when.subtext.length > 0)
    assert.ok(stages1.when.chip.length > 0)

    // IF Gate Stage check
    assert.match(stages1.ifGate.label, /IF/i)
    assert.match(stages1.ifGate.subtext, /Zombie process count > 50|threshold/i)

    // THEN Action Stage check
    assert.match(stages1.thenAction.label, /THEN/i)
    assert.match(stages1.thenAction.subtext, /Cordon/i)
  })

  it('handles empty or custom trigger/action configs gracefully without throwing', () => {
    const stages3 = extractPipelineStages(sampleRule3)
    assert.ok(stages3.when.subtext)
    assert.ok(stages3.ifGate.subtext)
    assert.ok(stages3.thenAction.subtext)
    assert.equal(typeof stages3.when.chip, 'string')
    assert.equal(typeof stages3.ifGate.chip, 'string')
    assert.equal(typeof stages3.thenAction.chip, 'string')
  })

  it('verifies AutomationHeaderBar.vue has NO duplicate "+ +" and contains view switcher', () => {
    const headerPath = path.resolve(process.cwd(), 'src/components/automation/AutomationHeaderBar.vue')
    const content = fs.readFileSync(headerPath, 'utf8')

    // 1. Must NOT contain "+ + Create" or "+ Create" next to a plus icon
    assert.doesNotMatch(content, /\+ Create Automation Rule/, 'Duplicate plus in button text must be eliminated')
    assert.doesNotMatch(content, />\s*\+\s*Create/, 'Buttons with plus icon should not prefix text with +')

    // 2. Must support viewMode switcher ('pipelines' | 'table')
    assert.match(content, /viewMode/, 'AutomationHeaderBar must support viewMode prop')
    assert.match(content, /pipelines/, 'AutomationHeaderBar must support pipelines view mode')
    assert.match(content, /table/, 'AutomationHeaderBar must support table view mode')
  })

  it('verifies automation.css does not contain undefined var(--color-primary-fg) or raw cyan/dark slop', () => {
    const cssPath = path.resolve(process.cwd(), 'src/assets/styles/views/automation.css')
    const content = fs.readFileSync(cssPath, 'utf8')

    // 1. Must not reference undefined var(--color-primary-fg) without fallback
    assert.doesNotMatch(content, /var\(--color-primary-fg\)(?!;)/, 'Must not reference undefined var(--color-primary-fg)')

    // 2. Must use var(--color-on-primary, #ffffff) for primary button contrast
    assert.match(content, /var\(--color-on-primary/, 'Must use var(--color-on-primary) for button text contrast')

    // 3. Must not hardcode dark-only rgba(15, 23, 42)
    assert.doesNotMatch(content, /rgba\(\s*15\s*,\s*23\s*,\s*42/, 'Must not hardcode dark-only slate rgb(15,23,42)')
  })

  it('verifies strict line limits (< 500 lines, components < 280) on all touched files', () => {
    const files = [
      { file: 'src/views/AutomationView.vue', max: 350 },
      { file: 'src/components/automation/AutomationHeaderBar.vue', max: 250 },
      { file: 'src/components/automation/AutomationPipelineBoard.vue', max: 280 },
      { file: 'src/components/automation/AutomationTimelineFeed.vue', max: 250 },
      { file: 'src/assets/styles/views/automation.css', max: 500 },
      { file: 'src/composables/useAutomationEngine.ts', max: 500 },
    ]

    for (const { file, max } of files) {
      const fullPath = path.resolve(process.cwd(), file)
      assert.ok(fs.existsSync(fullPath), `Target file ${file} must exist`)
      const lines = fs.readFileSync(fullPath, 'utf8').split('\n').length
      assert.ok(
        lines <= max,
        `File ${file} has ${lines} lines, exceeding strict limit of ${max}`
      )
    }
  })
})
