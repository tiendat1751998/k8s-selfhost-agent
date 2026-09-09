import type { HelmRelease } from '../api/helm'

export interface ParsedHelmChart {
  name: string
  version: string
  description: string
  chartDisplay: string
  truncatedDescription: string
}

/**
 * Parses a Helm release's chart field and description, handling:
 * 1. Raw JSON objects (Helm Go SDK *chart.Chart serialization)
 * 2. Stringified JSON containing { metadata: { name, version, description } }
 * 3. Base64-encoded chart or JSON payloads
 * 4. Standard chart strings (e.g., "cilium-1.20.0" or "cilium")
 */
export function parseHelmChart(
  chartRaw: unknown,
  descRaw?: unknown,
  versionRaw?: unknown
): ParsedHelmChart {
  let name = ''
  let version = typeof versionRaw === 'string' ? versionRaw.trim() : ''
  let description = typeof descRaw === 'string' ? descRaw.trim() : ''

  // Case 1: Chart is already an object
  if (chartRaw && typeof chartRaw === 'object') {
    const obj = chartRaw as Record<string, any>
    const meta = obj.metadata || obj
    name = meta.name || obj.name || ''
    if (!version) version = meta.version || obj.version || ''
    if (!description) description = meta.description || obj.description || ''
  } else if (typeof chartRaw === 'string') {
    let str = chartRaw.trim()

    // Case 2: Base64-encoded string
    if (str && !str.startsWith('{') && !str.startsWith('[')) {
      try {
        const cleanedStr = str.replace(/\s+/g, '')
        if (/^[A-Za-z0-9+/=]+$/.test(cleanedStr) && cleanedStr.length > 20) {
          const decoded = atob(cleanedStr)
          if (decoded.includes('{') || decoded.includes('"name"') || decoded.includes('"metadata"')) {
            str = decoded.trim()
          }
        }
      } catch {
        // Not valid base64, continue with original string
      }
    }

    // Case 3: JSON string
    if (str.startsWith('{') || str.includes('"metadata"') || str.includes('"name"')) {
      try {
        const parsed = JSON.parse(str)
        if (parsed && typeof parsed === 'object') {
          const meta = parsed.metadata || parsed
          name = meta.name || parsed.name || ''
          if (!version) version = meta.version || parsed.version || ''
          if (!description) description = meta.description || parsed.description || ''
        }
      } catch {
        // Fall back to regex parsing if JSON has trailing truncated bytes
        const nameMatch = str.match(/"name"\s*:\s*"([^"]+)"/)
        if (nameMatch) name = nameMatch[1]

        const versionMatch = str.match(/"version"\s*:\s*"([^"]+)"/)
        if (versionMatch && !version) version = versionMatch[1]

        const descMatch = str.match(/"description"\s*:\s*"([^"]+)"/)
        if (descMatch && !description) description = descMatch[1]
      }
    }

    // Case 4: Standard chart identifier string (e.g. "cilium-1.20.0" or "bitnami/nginx")
    if (!name && str) {
      if (str.startsWith('{') || str.length > 100) {
        name = 'chart'
      } else {
        const match = str.match(/^(.+?)-(\d+\.\d+.*)$/)
        if (match) {
          name = match[1]
          if (!version) version = match[2]
        } else {
          name = str
        }
      }
    }
  }

  // Also clean description if it contains raw JSON string
  if (description && (description.startsWith('{') || description.length > 200)) {
    try {
      const parsedDesc = JSON.parse(description)
      if (parsedDesc?.description) description = parsedDesc.description
      else if (parsedDesc?.metadata?.description) description = parsedDesc.metadata.description
    } catch {
      const descMatch = description.match(/"description"\s*:\s*"([^"]+)"/)
      if (descMatch) description = descMatch[1]
    }
  }

  // Format name
  name = name.trim() || 'chart'

  // Format version: normalize to vX.X.X
  version = version.trim()
  const versionDisplay = version ? (version.startsWith('v') ? version : `v${version}`) : ''

  // Format chartDisplay: "cilium v1.20.0" or "cilium"
  const chartDisplay = versionDisplay ? `${name} ${versionDisplay}` : name

  // Truncate description to ~100 characters with ellipsis
  const cleanDesc = (description || '').replace(/\s+/g, ' ').trim()
  const truncatedDescription = cleanDesc.length > 100
    ? `${cleanDesc.slice(0, 97).trim()}...`
    : cleanDesc

  return {
    name,
    version: versionDisplay,
    description: cleanDesc,
    chartDisplay,
    truncatedDescription,
  }
}

/**
 * Returns formatted Chart string for a release: e.g. "cilium v1.20.0"
 */
export function getFormattedReleaseChart(rel: HelmRelease): string {
  const parsed = parseHelmChart(rel.chart, rel.description, rel.version || rel.appVersion || rel.app_version)
  return parsed.chartDisplay
}

/**
 * Returns clean truncated description for a release (~100 chars with ellipsis)
 */
export function getFormattedReleaseDescription(rel: HelmRelease): string {
  const parsed = parseHelmChart(rel.chart, rel.description, rel.version || rel.appVersion || rel.app_version)
  return parsed.truncatedDescription
}

/**
 * Sanitizes a Helm release object so that `chart` and `description` are never raw JSON/base64
 */
export function sanitizeHelmRelease<T extends HelmRelease>(rel: T): T & {
  cleanChart: string
  cleanDescription: string
  chartDisplay: string
} {
  const parsed = parseHelmChart(rel.chart, rel.description, rel.version || rel.appVersion || rel.app_version)
  return {
    ...rel,
    cleanChart: parsed.chartDisplay,
    cleanDescription: parsed.truncatedDescription,
    chartDisplay: parsed.chartDisplay,
    // Ensure version is set if extracted from chart
    version: rel.version || parsed.version || 'v1.0.0',
  }
}

export function useHelm() {
  return {
    parseHelmChart,
    getFormattedReleaseChart,
    getFormattedReleaseDescription,
    sanitizeHelmRelease,
  }
}
