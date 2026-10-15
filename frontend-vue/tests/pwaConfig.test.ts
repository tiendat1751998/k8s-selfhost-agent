import { describe, it } from 'node:test'
import assert from 'node:assert/strict'

export interface PwaManifestIcon {
  src: string
  sizes: string
  type: string
  purpose?: string
}

export interface PwaManifest {
  name: string
  short_name: string
  start_url: string
  display: 'standalone' | 'fullscreen' | 'minimal-ui' | 'browser'
  background_color: string
  theme_color: string
  icons: PwaManifestIcon[]
}

export function parsePngDimensions(buffer: Buffer): { width: number; height: number } {
  if (buffer.length < 24) {
    throw new Error('Buffer too small for PNG IHDR header')
  }
  const pngSig = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a])
  if (buffer.subarray(0, 8).compare(pngSig) !== 0) {
    throw new Error('File does not have a valid PNG signature')
  }
  const ihdrChunkType = buffer.toString('ascii', 12, 16)
  if (ihdrChunkType !== 'IHDR') {
    throw new Error('Expected IHDR chunk after signature')
  }
  const width = buffer.readUInt32BE(16)
  const height = buffer.readUInt32BE(20)
  return { width, height }
}

export function createMockPngBuffer(width: number, height: number): Buffer {
  const buf = Buffer.alloc(24)
  // PNG Signature
  const pngSig = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]
  for (let i = 0; i < pngSig.length; i++) {
    buf[i] = pngSig[i]
  }
  // IHDR chunk length (13 bytes)
  buf.writeUInt32BE(13, 8)
  // IHDR type
  buf.write('IHDR', 12, 'ascii')
  // Width & Height
  buf.writeUInt32BE(width, 16)
  buf.writeUInt32BE(height, 20)
  return buf
}

export function validatePwaManifest(manifest: Partial<PwaManifest>): {
  valid: boolean
  errors: string[]
} {
  const errors: string[] = []
  if (!manifest.name) errors.push('Missing manifest name')
  if (!manifest.short_name) errors.push('Missing manifest short_name')
  if (manifest.start_url !== '/') errors.push('start_url must be "/"')
  if (manifest.display !== 'standalone') errors.push('display must be "standalone"')
  if (!manifest.theme_color) errors.push('Missing theme_color')
  if (!manifest.background_color) errors.push('Missing background_color')

  const icons = manifest.icons || []
  const has192 = icons.some(i => i.sizes === '192x192' && i.purpose?.includes('maskable'))
  const has512 = icons.some(i => i.sizes === '512x512' && i.purpose?.includes('maskable'))

  if (!has192) errors.push('Missing 192x192 maskable icon')
  if (!has512) errors.push('Missing 512x512 maskable icon')

  return {
    valid: errors.length === 0,
    errors
  }
}

describe('PWA Manifest & Icon Specification Logic Tests', () => {
  it('parsePngDimensions correctly extracts dimensions from valid PNG IHDR buffer', () => {
    const icon192 = createMockPngBuffer(192, 192)
    assert.deepEqual(parsePngDimensions(icon192), { width: 192, height: 192 })

    const icon512 = createMockPngBuffer(512, 512)
    assert.deepEqual(parsePngDimensions(icon512), { width: 512, height: 512 })

    const appleIcon = createMockPngBuffer(180, 180)
    assert.deepEqual(parsePngDimensions(appleIcon), { width: 180, height: 180 })
  })

  it('parsePngDimensions throws on corrupted signature or truncated buffer', () => {
    const badSig = Buffer.alloc(24)
    assert.throws(() => parsePngDimensions(badSig), /valid PNG signature/)

    const tooShort = Buffer.alloc(10)
    assert.throws(() => parsePngDimensions(tooShort), /Buffer too small/)
  })

  it('validatePwaManifest validates standard PWA manifest specifications', () => {
    const validManifest: PwaManifest = {
      name: 'K8S Control Platform',
      short_name: 'K8SControl',
      start_url: '/',
      display: 'standalone',
      background_color: '#0b0f19',
      theme_color: '#0b0f19',
      icons: [
        { src: '/pwa-192x192.png', sizes: '192x192', type: 'image/png', purpose: 'any maskable' },
        { src: '/pwa-512x512.png', sizes: '512x512', type: 'image/png', purpose: 'any maskable' }
      ]
    }
    const result = validatePwaManifest(validManifest)
    assert.equal(result.valid, true)
    assert.equal(result.errors.length, 0)
  })

  it('validatePwaManifest identifies missing required fields and icons', () => {
    const incomplete: Partial<PwaManifest> = {
      name: 'Incomplete PWA',
      display: 'browser', // invalid
      start_url: '/subpath' // invalid
    }
    const result = validatePwaManifest(incomplete)
    assert.equal(result.valid, false)
    assert.ok(result.errors.length >= 4)
  })
})
