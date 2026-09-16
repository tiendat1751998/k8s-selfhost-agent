import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

function readPngDimensions(buffer: Buffer): { width: number; height: number } {
  const pngSig = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a])
  assert.strictEqual(
    buffer.subarray(0, 8).compare(pngSig),
    0,
    'File does not have a valid PNG signature'
  )
  const ihdrChunkType = buffer.toString('ascii', 12, 16)
  assert.strictEqual(ihdrChunkType, 'IHDR', 'Expected IHDR chunk after signature')
  const width = buffer.readUInt32BE(16)
  const height = buffer.readUInt32BE(20)
  return { width, height }
}

describe('PWA Manifest & Icon Specification', () => {
  const manifestPath = path.join(rootDir, 'public', 'manifest.webmanifest')
  const indexHtmlPath = path.join(rootDir, 'index.html')

  it('manifest.webmanifest matches exact PWA requirements', () => {
    assert.ok(fs.existsSync(manifestPath), 'manifest.webmanifest must exist')
    const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf-8'))

    assert.strictEqual(manifest.name, 'K8S Control Platform')
    assert.strictEqual(manifest.short_name, 'K8SControl')
    assert.strictEqual(manifest.start_url, '/')
    assert.strictEqual(manifest.display, 'standalone')
    assert.strictEqual(manifest.background_color, '#0b0f19')
    assert.strictEqual(manifest.theme_color, '#0b0f19')

    const icons = manifest.icons
    assert.ok(Array.isArray(icons), 'icons must be an array')

    const icon192 = icons.find(
      (i: any) => i.sizes === '192x192' && i.src === '/pwa-192x192.png'
    )
    assert.ok(icon192, 'pwa-192x192.png icon entry must exist in manifest')
    assert.strictEqual(icon192.type, 'image/png')
    assert.strictEqual(icon192.purpose, 'any maskable')

    const icon512 = icons.find(
      (i: any) => i.sizes === '512x512' && i.src === '/pwa-512x512.png'
    )
    assert.ok(icon512, 'pwa-512x512.png icon entry must exist in manifest')
    assert.strictEqual(icon512.type, 'image/png')
    assert.strictEqual(icon512.purpose, 'any maskable')

    const iconFavicon = icons.find(
      (i: any) => i.src === '/favicon.svg'
    )
    assert.ok(iconFavicon, 'favicon.svg icon entry must exist in manifest')
    assert.strictEqual(iconFavicon.type, 'image/svg+xml')
    assert.strictEqual(iconFavicon.sizes, 'any')
  })

  it('generates standard PNG icons with exact required dimensions', () => {
    const icon192Path = path.join(rootDir, 'public', 'pwa-192x192.png')
    const icon512Path = path.join(rootDir, 'public', 'pwa-512x512.png')
    const appleIconPath = path.join(rootDir, 'public', 'apple-touch-icon.png')

    assert.ok(fs.existsSync(icon192Path), 'pwa-192x192.png must exist in public/')
    assert.ok(fs.existsSync(icon512Path), 'pwa-512x512.png must exist in public/')
    assert.ok(fs.existsSync(appleIconPath), 'apple-touch-icon.png must exist in public/')

    const dim192 = readPngDimensions(fs.readFileSync(icon192Path))
    assert.strictEqual(dim192.width, 192)
    assert.strictEqual(dim192.height, 192)

    const dim512 = readPngDimensions(fs.readFileSync(icon512Path))
    assert.strictEqual(dim512.width, 512)
    assert.strictEqual(dim512.height, 512)

    const dimApple = readPngDimensions(fs.readFileSync(appleIconPath))
    assert.strictEqual(dimApple.width, 180)
    assert.strictEqual(dimApple.height, 180)
  })

  it('index.html contains apple-touch-icon, theme-color #0b0f19, and manifest link', () => {
    assert.ok(fs.existsSync(indexHtmlPath), 'index.html must exist')
    const html = fs.readFileSync(indexHtmlPath, 'utf-8')

    assert.match(
      html,
      /<link[^>]+rel=["']apple-touch-icon["'][^>]+href=["']\/apple-touch-icon\.png["']|<link[^>]+href=["']\/apple-touch-icon\.png["'][^>]+rel=["']apple-touch-icon["']/,
      'index.html must contain apple-touch-icon link'
    )
    assert.match(
      html,
      /<meta[^>]+name=["']theme-color["'][^>]+content=["']#0b0f19["']|<meta[^>]+content=["']#0b0f19["'][^>]+name=["']theme-color["']/,
      'index.html must contain theme-color #0b0f19'
    )
    assert.match(
      html,
      /<link[^>]+rel=["']manifest["'][^>]+href=["']\/manifest\.webmanifest["']|<link[^>]+href=["']\/manifest\.webmanifest["'][^>]+rel=["']manifest["']/,
      'index.html must contain manifest link'
    )
  })
  it('PwaInstallBanner.vue exists in src/components/pwa/ and is re-exported from common/', () => {
    const pwaBannerPath = path.join(rootDir, 'src', 'components', 'pwa', 'PwaInstallBanner.vue')
    const commonBannerPath = path.join(rootDir, 'src', 'components', 'common', 'PwaInstallBanner.vue')

    assert.ok(fs.existsSync(pwaBannerPath), 'PwaInstallBanner.vue must exist in src/components/pwa/')
    assert.ok(fs.existsSync(commonBannerPath), 'PwaInstallBanner.vue must exist in src/components/common/')

    const pwaContent = fs.readFileSync(pwaBannerPath, 'utf-8')
    assert.match(pwaContent, /usePwaInstall/, 'PwaInstallBanner.vue must use usePwaInstall')
    assert.match(pwaContent, /handleInstallClick/, 'PwaInstallBanner.vue must handle install click')
  })

  it('registerServiceWorker.ts contains enable_pwa_dev logic and exports reactive state', () => {
    const swHelperPath = path.join(rootDir, 'src', 'registerServiceWorker.ts')
    assert.ok(fs.existsSync(swHelperPath), 'registerServiceWorker.ts must exist')
    const content = fs.readFileSync(swHelperPath, 'utf-8')

    assert.match(content, /enable_pwa_dev/, 'Must check enable_pwa_dev flag')
    assert.match(content, /beforeinstallprompt/, 'Must listen to beforeinstallprompt')
    assert.match(content, /setDeferredPrompt/, 'Must export setDeferredPrompt')
    assert.match(content, /export const isInstallable/, 'Must export isInstallable ref')
  })
})
