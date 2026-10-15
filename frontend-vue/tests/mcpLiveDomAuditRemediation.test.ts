import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const srcDir = path.resolve(__dirname, '../src')

function getAllSourceFiles(dir: string): string[] {
  const files: string[] = []
  const entries = fs.readdirSync(dir, { withFileTypes: true })
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      files.push(...getAllSourceFiles(fullPath))
    } else if (entry.isFile() && (entry.name.endsWith('.ts') || entry.name.endsWith('.vue'))) {
      files.push(fullPath)
    }
  }
  return files
}

describe('Live Chrome DevTools MCP Audit: 3 UI/DOM Defect Remediation', () => {
  it('Criterion 1: SettingsView tabs have zero hardcoded fake counts and zero fake keys in telemetry', () => {
    const settingsPath = path.join(srcDir, 'views/SettingsView.vue')
    const content = fs.readFileSync(settingsPath, 'utf-8')

    // Must not have hardcoded count: 3
    assert.strictEqual(
      content.includes('count: 3'),
      false,
      'SettingsView tabs must not contain hardcoded count: 3'
    )
    // Must not have hardcoded "3 Keys" in micro-telemetry
    assert.strictEqual(
      content.includes('3 Keys'),
      false,
      'SettingsView micro-telemetry must not display hardcoded "3 Keys"'
    )
  })

  it('Criterion 2: ZeroTrustDrawer initializes with honest empty state and no fake KMS key', () => {
    const ztPath = path.join(srcDir, 'components/layout/ZeroTrustDrawer.vue')
    const content = fs.readFileSync(ztPath, 'utf-8')

    // Must not have hardcoded fake root key or version
    assert.strictEqual(
      content.includes('kms-k8s-root-9f82c'),
      false,
      'ZeroTrustDrawer must not contain hardcoded fake key ID kms-k8s-root-9f82c'
    )
    assert.strictEqual(
      content.includes("activeKeyId = ref('--')"),
      true,
      'ZeroTrustDrawer activeKeyId must initialize with uninitialized default "--"'
    )
    assert.strictEqual(
      content.includes('keyVersion = ref(0)'),
      true,
      'ZeroTrustDrawer keyVersion must initialize with 0'
    )
  })

  it('Criterion 3: Zero residual prod-us-east-1, prod-eu-west-1, staging-us-west-2, or kms-k8s-root strings in src/', () => {
    const allFiles = getAllSourceFiles(srcDir)
    const forbiddenRegex = /prod-us-east-1|prod-eu-west-1|staging-us-west-2|kms-k8s-root/g

    const violations: { file: string; line: number; text: string }[] = []

    for (const file of allFiles) {
      const relPath = path.relative(srcDir, file)
      const content = fs.readFileSync(file, 'utf-8')
      const lines = content.split('\n')
      lines.forEach((line, index) => {
        if (forbiddenRegex.test(line)) {
          violations.push({
            file: relPath,
            line: index + 1,
            text: line.trim()
          })
        }
      })
    }

    assert.deepStrictEqual(
      violations,
      [],
      `Found residual strings in src/:\n${violations.map(v => `  ${v.file}:${v.line} -> ${v.text}`).join('\n')}`
    )
  })
})
