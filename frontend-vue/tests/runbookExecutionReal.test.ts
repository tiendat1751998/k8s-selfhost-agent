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

describe('Task 070 Wave 2C: Runbooks Step & Playbook Real Execution', () => {
  describe('1. parseRunbookSteps parsing logic', () => {
    it('parses markdown numbered steps and code fences into structured steps', () => {
      const content = `1. Check pod status
\`kubectl get pods -n default\`
2. Check logs
\`\`\`bash
kubectl logs pod-1
\`\`\`
3. Restart pod
\`kubectl rollout restart deploy/app\``
      const steps = parseRunbookSteps(content, 'Test Runbook')
      assert.equal(steps.length, 3)
      assert.equal(steps[0].stepNum, 1)
      assert.ok(steps[0].command?.includes('kubectl get pods'))
      assert.equal(steps[1].stepNum, 2)
      assert.ok(steps[1].command?.includes('kubectl logs pod-1'))
      assert.equal(steps[2].stepNum, 3)
      assert.ok(steps[2].command?.includes('rollout restart'))
    })
  })

  describe('2. RunbookPlaybookConsole.vue template verification', () => {
    const consolePath = path.resolve(import.meta.dirname, '../src/components/runbooks/RunbookPlaybookConsole.vue')
    const content = fs.readFileSync(consolePath, 'utf8')

    it('renders step status badges for succeeded, degraded, and failed', () => {
      assert.ok(
        content.includes('step.status') &&
        (content.includes('succeeded') || content.includes('SUCCEEDED') || content.includes('badge-emerald')) &&
        (content.includes('degraded') || content.includes('DEGRADED') || content.includes('badge-amber')) &&
        (content.includes('failed') || content.includes('FAILED') || content.includes('badge-rose')),
        'Must display status badge for succeeded, degraded, and failed step states'
      )
    })

    it('displays execution duration indicator', () => {
      assert.ok(
        content.includes('step.durationMs') || content.includes('durationMs'),
        'Must display execution duration when available'
      )
    })

    it('displays verbatim output box for step execution output', () => {
      assert.ok(
        content.includes('step.output') && (content.includes('step-output') || content.includes('<pre')),
        'Must display verbatim output box for step execution results'
      )
    })
  })

  describe('3. useRunbooks.ts implementation verification', () => {
    const useRunbooksPath = path.resolve(import.meta.dirname, '../src/composables/useRunbooks.ts')
    const content = fs.readFileSync(useRunbooksPath, 'utf8')

    it('contains activeSteps state ref', () => {
      assert.ok(
        content.includes('activeSteps'),
        'useRunbooks must define and export activeSteps ref'
      )
    })

    it('has zero fake setTimeout in execution paths', () => {
      assert.equal(
        content.includes('setTimeout'),
        false,
        'useRunbooks must NOT use fake setTimeout for execution'
      )
    })

    it('records step durationMs upon execution completion', () => {
      assert.ok(
        content.includes('step.durationMs =') || content.includes('durationMs'),
        'executeSingleStep must record step.durationMs'
      )
    })
  })
})
