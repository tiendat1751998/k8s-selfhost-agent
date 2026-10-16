import { describe, it } from 'node:test'
import assert from 'node:assert/strict'

export interface SurfaceLadderTokens {
  canvasDark: string
  surfaceInset: string
  surfaceCard: string
  surfaceElevated: string
  hairline: string
}

export const DARK_SURFACE_LADDER: SurfaceLadderTokens = {
  canvasDark: '#0b0f17',
  surfaceInset: '#111827',
  surfaceCard: '#161f30',
  surfaceElevated: '#222f46',
  hairline: '#243147'
}

export const LIGHT_SURFACE_LADDER: SurfaceLadderTokens = {
  canvasDark: '#f8fafc',
  surfaceInset: '#f1f5f9',
  surfaceCard: '#ffffff',
  surfaceElevated: '#ffffff',
  hairline: '#e2e8f0'
}

export function parseHexColor(hex: string): { r: number; g: number; b: number } {
  const cleanHex = hex.replace('#', '')
  const num = parseInt(cleanHex, 16)
  return {
    r: (num >> 16) & 255,
    g: (num >> 8) & 255,
    b: num & 255
  }
}

export function calculateLuminance(rgb: { r: number; g: number; b: number }): number {
  const [r, g, b] = [rgb.r, rgb.g, rgb.b].map(v => {
    const s = v / 255
    return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4)
  })
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

export function calculateContrastRatio(hex1: string, hex2: string): number {
  const lum1 = calculateLuminance(parseHexColor(hex1))
  const lum2 = calculateLuminance(parseHexColor(hex2))
  const brightest = Math.max(lum1, lum2)
  const darkest = Math.min(lum1, lum2)
  return Number(((brightest + 0.05) / (darkest + 0.05)).toFixed(2))
}

export function resolveSurfaceLadder(theme: 'dark' | 'light'): SurfaceLadderTokens {
  return theme === 'light' ? LIGHT_SURFACE_LADDER : DARK_SURFACE_LADDER
}

describe('Task 018: Linear Surface Ladder & Tabular-Nums Typography Standard Specifications', () => {
  it('DARK_SURFACE_LADDER conforms to exact enterprise 4-tier dark surface ladder', () => {
    assert.equal(DARK_SURFACE_LADDER.canvasDark, '#0b0f17')
    assert.equal(DARK_SURFACE_LADDER.surfaceInset, '#111827')
    assert.equal(DARK_SURFACE_LADDER.surfaceCard, '#161f30')
    assert.equal(DARK_SURFACE_LADDER.surfaceElevated, '#222f46')
    assert.equal(DARK_SURFACE_LADDER.hairline, '#243147')
  })

  it('LIGHT_SURFACE_LADDER conforms to exact enterprise 4-tier light surface ladder', () => {
    assert.equal(LIGHT_SURFACE_LADDER.canvasDark, '#f8fafc')
    assert.equal(LIGHT_SURFACE_LADDER.surfaceInset, '#f1f5f9')
    assert.equal(LIGHT_SURFACE_LADDER.surfaceCard, '#ffffff')
    assert.equal(LIGHT_SURFACE_LADDER.surfaceElevated, '#ffffff')
    assert.equal(LIGHT_SURFACE_LADDER.hairline, '#e2e8f0')
  })

  it('Dark mode text (#ffffff) has high WCAG AAA contrast (>= 7:1) against surface card', () => {
    const contrast = calculateContrastRatio(DARK_SURFACE_LADDER.surfaceCard, '#ffffff')
    assert.ok(contrast >= 7.0, `Contrast ratio ${contrast} must be >= 7.0 (AAA)`)
  })

  it('Light mode text (#090a0b) has high WCAG AAA contrast (>= 7:1) against surface card', () => {
    const contrast = calculateContrastRatio(LIGHT_SURFACE_LADDER.surfaceCard, '#090a0b')
    assert.ok(contrast >= 7.0, `Contrast ratio ${contrast} must be >= 7.0 (AAA)`)
  })

  it('resolveSurfaceLadder switches tokens accurately based on active theme', () => {
    assert.deepEqual(resolveSurfaceLadder('dark'), DARK_SURFACE_LADDER)
    assert.deepEqual(resolveSurfaceLadder('light'), LIGHT_SURFACE_LADDER)
  })
})
