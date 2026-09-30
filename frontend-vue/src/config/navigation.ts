export interface NavItem {
  path: string
  name: string
  icon: string
  sub: string
  badge?: string
}

export interface NavGroup {
  key: string
  label: string
  icon: string
  items: NavItem[]
}

export const navGroups: NavGroup[] = [
  {
    key: 'observability',
    label: 'OBSERVABILITY & OPS',
    icon: 'activity',
    items: [
      { path: '/', name: 'Overview', icon: 'globe', sub: 'Fleet Health & Mesh' },
      { path: '/incidents', name: 'Incidents & RCA', icon: 'alert-triangle', sub: 'Autonomous Triage' },
      { path: '/slo', name: 'SLOs & Budgets', icon: 'activity', sub: 'PromQL SLI Tracking' },
      { path: '/logs', name: 'Real-Time Logs', icon: 'terminal', sub: 'Live WebSocket Stream', badge: 'LIVE' },
    ]
  },
  {
    key: 'compute',
    label: 'COMPUTE & FLEET',
    icon: 'server',
    items: [
      { path: '/fleet', name: 'Fleet Clusters', icon: 'anchor', sub: 'Multi-Region Mesh' },
      { path: '/hosts', name: 'Infrastructure Hosts', icon: 'server', sub: 'Compute & Database Nodes' },
      { path: '/deployments', name: 'Deployments & Workloads', icon: 'play', sub: 'Canary, Blue-Green & Workloads' },
      { path: '/explorer', name: 'Cluster Explorer', icon: 'search', sub: 'Live CRD & Pod Trees' },
      { path: '/helm', name: 'Helm Catalog', icon: 'box', sub: 'Repositories, Charts & Releases' },
    ]
  },
  {
    key: 'governance',
    label: 'GOVERNANCE & SECURITY',
    icon: 'shield',
    items: [
      { path: '/audit', name: 'Audit & CVEs', icon: 'shield', sub: 'Trivy & IaC Scanner' },
    ]
  },
  {
    key: 'automation',
    label: 'AUTOMATION',
    icon: 'zap',
    items: [
      { path: '/automation', name: 'Automation Rules', icon: 'sliders', sub: 'Event-Driven SRE' },
      { path: '/runbooks', name: 'SRE Runbooks', icon: 'book-open', sub: 'One-Click Remediation' },
    ]
  },
  {
    key: 'management',
    label: 'MANAGEMENT',
    icon: 'layers',
    items: [
      { path: '/tenancy', name: 'Tenancy & RBAC', icon: 'lock', sub: 'Org Isolation Matrix' },
      { path: '/alerts', name: 'Alerts & Channels', icon: 'flame', sub: 'Slack, Telegram, Email' },
      { path: '/settings', name: 'System Settings', icon: 'sliders', sub: 'MetalLB & KEDA Nodes' },
    ]
  }
]
