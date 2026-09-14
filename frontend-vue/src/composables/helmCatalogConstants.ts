import type { HelmChart, HelmRelease, InstallReleaseRequest, UpgradeReleaseRequest } from '../api/helm'

export type ActiveTab = 'releases' | 'charts' | 'repos'
export type ActiveDrawerTab = 'overview' | 'values' | 'manifest' | 'history'

export interface RepoPreset {
  name: string
  url: string
  icon: string
}

export interface CategoryTag {
  key: string
  label: string
  icon: string
}

export interface ToastMessage {
  text: string
  type: 'success' | 'error'
}

export interface InstallFormData {
  releaseName: string
  namespace: string
  createNamespace: boolean
  version: string
  values: string
  loadingDefaultValues: boolean
}

export const REPO_PRESETS: RepoPreset[] = [
  { name: 'bitnami', url: 'https://charts.bitnami.com/bitnami', icon: '🍱' },
  { name: 'ingress-nginx', url: 'https://kubernetes.github.io/ingress-nginx', icon: '🌐' },
  { name: 'prometheus-community', url: 'https://prometheus-community.github.io/helm-charts', icon: '📊' },
  { name: 'grafana', url: 'https://grafana.github.io/helm-charts', icon: '📈' },
  { name: 'jetstack', url: 'https://charts.jetstack.io', icon: '🔒' },
  { name: 'traefik', url: 'https://traefik.github.io/charts', icon: '🚦' },
  { name: 'hashicorp', url: 'https://helm.releases.hashicorp.com', icon: '🔷' },
]

export const CATEGORY_TAGS: CategoryTag[] = [
  { key: 'all', label: 'All Categories', icon: '✨' },
  { key: 'database', label: 'Databases', icon: '🗄️' },
  { key: 'networking', label: 'Ingress & Mesh', icon: '🌐' },
  { key: 'monitoring', label: 'Observability', icon: '📊' },
  { key: 'security', label: 'Security & Auth', icon: '🛡️' },
  { key: 'storage', label: 'Storage & Backup', icon: '📦' },
  { key: 'web', label: 'Web & APIs', icon: '⚡' },
]

export function sanitizeReleaseName(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9-]/g, '-').replace(/^-+|-+$/g, '')
}

export function downloadAsFile(filename: string, content: string): void {
  const blob = new Blob([content], { type: 'text/yaml;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

export function formatReleaseDate(dateStr?: string): string {
  if (!dateStr) return '—'
  try {
    const d = new Date(dateStr)
    if (isNaN(d.getTime())) return dateStr
    const diff = Date.now() - d.getTime()
    const mins = Math.floor(diff / 60000)
    if (mins < 1) return 'Just now'
    if (mins < 60) return `${mins}m ago`
    const hours = Math.floor(mins / 60)
    if (hours < 24) return `${hours}h ago`
    const days = Math.floor(Math.abs(diff) / (1000 * 60 * 60 * 24))
    return days < 30 ? `${days}d ago` : d.toLocaleDateString()
  } catch {
    return dateStr
  }
}

export function getChartIcon(chart: HelmChart): string {
  const name = (chart.name || '').toLowerCase()
  if (name.includes('nginx') || name.includes('ingress') || name.includes('traefik')) return '🌐'
  if (name.includes('postgres') || name.includes('mysql') || name.includes('mariadb') || name.includes('redis') || name.includes('mongo')) return '🗄️'
  if (name.includes('prom') || name.includes('grafana') || name.includes('loki') || name.includes('metric')) return '📊'
  if (name.includes('cert') || name.includes('vault') || name.includes('auth') || name.includes('keycloak')) return '🛡️'
  if (name.includes('kafka') || name.includes('rabbit') || name.includes('queue') || name.includes('nats')) return '⚡'
  if (name.includes('elastic') || name.includes('search') || name.includes('opensearch')) return '🔍'
  if (name.includes('ai') || name.includes('ollama') || name.includes('vllm') || name.includes('llm')) return '🤖'
  return '📦'
}

export function getStatusType(status: string): string {
  const s = (status || '').toLowerCase()
  if (s.includes('deploy') || s === 'active' || s === 'success') return 'deployed'
  if (s.includes('fail') || s.includes('error')) return 'failed'
  if (s.includes('pend') || s.includes('upgrad') || s.includes('install')) return 'pending'
  if (s.includes('super') || s.includes('uninst')) return 'superseded'
  return 'unknown'
}

export function getErrorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback
}

export function filterReleases(releases: HelmRelease[], filter: string, search: string): HelmRelease[] {
  let list = [...releases]
  if (filter !== 'all') {
    const s = filter.toLowerCase()
    list = list.filter(r => {
      const rawInfo = (r as any)?.info || {}
      const status = String(r?.status || rawInfo?.status || '').toLowerCase()
      return status.includes(s)
    })
  }
  if (search.trim()) {
    const q = search.toLowerCase().trim()
    list = list.filter(r => {
      const rawInfo = (r as any)?.info || {}
      const desc = r.description || rawInfo?.description || ''
      return (
        (r.name || '').toLowerCase().includes(q) ||
        (r.chart || '').toLowerCase().includes(q) ||
        (r.namespace || '').toLowerCase().includes(q) ||
        desc.toLowerCase().includes(q)
      )
    })
  }
  return list
}

export function filterCharts(charts: HelmChart[], repoFilter: string, tagFilter: string): HelmChart[] {
  let list = [...charts]
  if (repoFilter !== 'all') list = list.filter(c => c.repo === repoFilter)
  if (tagFilter !== 'all') {
    const tag = tagFilter.toLowerCase()
    list = list.filter(c =>
      (c.name || '').toLowerCase().includes(tag) ||
      (c.description || '').toLowerCase().includes(tag) ||
      (c.keywords || []).some(k => k.toLowerCase().includes(tag))
    )
  }
  return list
}

export function computeReleaseStats(releases: HelmRelease[]) {
  const total = releases.length
  const deployed = releases.filter(r => {
    const rawInfo = (r as any)?.info || {}
    const status = String(r?.status || rawInfo?.status || '').toLowerCase()
    return status === 'deployed' || status.includes('deploy')
  }).length
  const failed = releases.filter(r => {
    const rawInfo = (r as any)?.info || {}
    const status = String(r?.status || rawInfo?.status || '').toLowerCase()
    return status.includes('fail') || status.includes('error')
  }).length
  const rate = total === 0 ? '100%' : `${Math.round((deployed / total) * 100)}%`
  return { total, deployed, failed, rate }
}

export function populateUpgradeForm(form: UpgradeReleaseRequest, release: HelmRelease): void {
  form.chart = release.chart || ''
  form.repo = (release.chart || '').includes('/') ? release.chart.split('/')[0] : ''
  form.version = release.version || ''
  form.namespace = release.namespace || 'default'
  form.resetValues = false
  form.reuseValues = true
  form.values = typeof release.values === 'string' ? release.values : ''
}

export function initInstallWizardForm(chart: HelmChart, selectedNamespace: string): InstallFormData {
  const sanitizedName = sanitizeReleaseName(chart.name)
  return {
    releaseName: `${sanitizedName}-${Date.now().toString(36)}`,
    namespace: selectedNamespace !== 'all' ? selectedNamespace : 'default',
    createNamespace: true,
    version: chart.version,
    values: '',
    loadingDefaultValues: true,
  }
}

export function buildInstallPayload(chart: HelmChart, form: InstallFormData): InstallReleaseRequest {
  return {
    releaseName: form.releaseName.trim(),
    chart: chart.name,
    repo: chart.repo,
    version: form.version || chart.version,
    namespace: form.namespace.trim(),
    values: form.values,
    createNamespace: form.createNamespace,
  }
}
