import { describe, it } from 'node:test'
import assert from 'node:assert/strict'

export interface FilterToken {
  category: string
  value: string
  label?: string
}

describe('Task 015: FacetedFilterBar Query Parsing & Filter Token Tests', () => {
  function parseFilterQuery(query: string): { freeText: string; tokens: FilterToken[] } {
    const parts = query.trim().split(/\s+/)
    const tokens: FilterToken[] = []
    const freeTextParts: string[] = []

    for (const part of parts) {
      if (!part) continue
      const colonIdx = part.indexOf(':')
      if (colonIdx > 0 && colonIdx < part.length - 1) {
        const category = part.slice(0, colonIdx).toLowerCase()
        const value = part.slice(colonIdx + 1)
        tokens.push({ category, value })
      } else {
        freeTextParts.push(part)
      }
    }

    return {
      freeText: freeTextParts.join(' '),
      tokens
    }
  }

  function serializeFilterQuery(freeText: string, tokens: FilterToken[]): string {
    const parts: string[] = []
    for (const t of tokens) {
      parts.push(`${t.category}:${t.value}`)
    }
    if (freeText.trim()) {
      parts.push(freeText.trim())
    }
    return parts.join(' ')
  }

  function removeFilterToken(tokens: FilterToken[], target: FilterToken): FilterToken[] {
    return tokens.filter(t => !(t.category === target.category && t.value === target.value))
  }

  function addFilterToken(tokens: FilterToken[], newToken: FilterToken): FilterToken[] {
    const exists = tokens.some(t => t.category === newToken.category && t.value === newToken.value)
    if (exists) return tokens
    return [...tokens, newToken]
  }

  it('parseFilterQuery extracts key-value faceted tokens and preserves free text', () => {
    const raw = 'namespace:production status:running nginx ingress'
    const parsed = parseFilterQuery(raw)

    assert.equal(parsed.tokens.length, 2)
    assert.deepEqual(parsed.tokens[0], { category: 'namespace', value: 'production' })
    assert.deepEqual(parsed.tokens[1], { category: 'status', value: 'running' })
    assert.equal(parsed.freeText, 'nginx ingress')
  })

  it('serializeFilterQuery formats tokens and search query into unified string', () => {
    const tokens: FilterToken[] = [
      { category: 'cluster', value: 'eu-west-1' },
      { category: 'tier', value: 'critical' }
    ]
    const serialized = serializeFilterQuery('database', tokens)
    assert.equal(serialized, 'cluster:eu-west-1 tier:critical database')
  })

  it('addFilterToken appends new facet and prevents duplicate filter entries', () => {
    const initial: FilterToken[] = [{ category: 'env', value: 'staging' }]
    const updated = addFilterToken(initial, { category: 'cluster', value: 'c1' })
    assert.equal(updated.length, 2)

    // Attempt duplicate addition
    const dupe = addFilterToken(updated, { category: 'env', value: 'staging' })
    assert.equal(dupe.length, 2, 'Must not add duplicate token')
  })

  it('removeFilterToken removes target token cleanly', () => {
    const initial: FilterToken[] = [
      { category: 'env', value: 'prod' },
      { category: 'status', value: 'error' }
    ]
    const remaining = removeFilterToken(initial, { category: 'status', value: 'error' })
    assert.equal(remaining.length, 1)
    assert.deepEqual(remaining[0], { category: 'env', value: 'prod' })
  })

  it('keyboard shortcut detector triggers focus only outside editable elements', () => {
    function shouldFocusFilterBar(eventKey: string, isCtrl: boolean, targetTagName: string): boolean {
      if (['INPUT', 'TEXTAREA', 'SELECT'].includes(targetTagName.toUpperCase())) {
        return false
      }
      if (eventKey === '/' && !isCtrl) return true
      if (isCtrl && (eventKey === 'f' || eventKey === 'F')) return true
      return false
    }

    assert.equal(shouldFocusFilterBar('/', false, 'BODY'), true)
    assert.equal(shouldFocusFilterBar('f', true, 'DIV'), true)
    assert.equal(shouldFocusFilterBar('/', false, 'INPUT'), false)
    assert.equal(shouldFocusFilterBar('/', false, 'TEXTAREA'), false)
    assert.equal(shouldFocusFilterBar('a', false, 'BODY'), false)
  })
})
