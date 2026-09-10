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
      { path: '/deployments', name: 'Deployments & Apps', icon: 'play', sub: 'Canary, Blue-Green & Workloads' },
      { path: '/promotions', name: 'Promotions', icon: 'git-branch', sub: 'Dev → Stage → Prod' },
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
      { path: '/compliance', name: 'Compliance & CIS', icon: 'file-text', sub: 'SOC2 / CIS Level 2' },
      { path: '/drift', name: 'Config Drift', icon: 'refresh', sub: 'GitOps Reconciliation' },
      { path: '/backup', name: 'Disaster Recovery', icon: 'database', sub: 'Dual-Sync NVMe + S3' },
    ]
  },
  {
    key: 'automation',
    label: 'AUTOMATION & FINOPS',
    icon: 'zap',
    items: [
      { path: '/automation', name: 'Automation Rules', icon: 'sliders', sub: 'Event-Driven SRE' },
      { path: '/runbooks', name: 'SRE Runbooks', icon: 'book-open', sub: 'One-Click Remediation' },
      { path: '/cost', name: 'Cost Optimization', icon: 'trending-up', sub: 'FinOps Node Bin-Packing' },
      { path: '/capacity', name: 'Capacity Forecast', icon: 'activity', sub: 'ARIMA Time-Series' },
    ]
  },
  {
    key: 'management',
    label: 'MANAGEMENT',
    icon: 'layers',
    items: [
      { path: '/tenancy', name: 'Tenancy & RBAC', icon: 'lock', sub: 'Org Isolation Matrix' },
      { path: '/ai-hub', name: 'AI Provider Hub', icon: 'cpu', sub: 'LLM Gateway & Breakers' },
      { path: '/changes', name: 'Change Requests', icon: 'edit', sub: 'RFC Approvals & Windows' },
      { path: '/alerts', name: 'Alerts & Channels', icon: 'flame', sub: 'Slack, Telegram, Email' },
      { path: '/reports', name: 'Reports Center', icon: 'file-text', sub: 'Executive SOC2 & Cost' },
      { path: '/catalog', name: 'Service Catalog', icon: 'book-open', sub: 'Developer Portal & APIs' },
      { path: '/scaffolder', name: 'Scaffolder Templates', icon: 'box', sub: '1-Click App Deployment' },
      { path: '/ecosystem', name: 'Ecosystem Tools', icon: 'plug', sub: 'Auto-Detected Stack' },
      { path: '/plugins', name: 'Plugin Hub', icon: 'plug', sub: 'JS Extension Runtime' },
      { path: '/settings', name: 'System Settings', icon: 'sliders', sub: 'MetalLB & KEDA Nodes' },
    ]
  }
]
