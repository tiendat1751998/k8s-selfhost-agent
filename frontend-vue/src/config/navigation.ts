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
    icon: '📊',
    items: [
      { path: '/', name: 'Overview', icon: '🌐', sub: 'Fleet Health & Mesh' },
      { path: '/incidents', name: 'Incidents & RCA', icon: '🚨', sub: 'Autonomous Triage' },
      { path: '/slo', name: 'SLOs & Budgets', icon: '🎯', sub: 'PromQL SLI Tracking' },
      { path: '/logs', name: 'Real-Time Logs', icon: '📜', sub: 'Live WebSocket Stream', badge: 'LIVE' },
    ]
  },
  {
    key: 'compute',
    label: 'COMPUTE & FLEET',
    icon: '🌐',
    items: [
      { path: '/fleet', name: 'Fleet Clusters', icon: '☸️', sub: 'Multi-Region Mesh' },
      { path: '/hosts', name: 'Infrastructure Hosts', icon: '🖥️', sub: 'Compute & Database Nodes' },
      { path: '/deployments', name: 'Deployments & Apps', icon: '🚀', sub: 'Canary, Blue-Green & Workloads' },
      { path: '/promotions', name: 'Promotions', icon: '🔄', sub: 'Dev → Stage → Prod' },
      { path: '/explorer', name: 'Cluster Explorer', icon: '🔍', sub: 'Live CRD & Pod Trees' },
      { path: '/helm', name: 'Helm Catalog', icon: '⛵', sub: 'Repositories, Charts & Releases' },
    ]
  },
  {
    key: 'governance',
    label: 'GOVERNANCE & SECURITY',
    icon: '🛡️',
    items: [
      { path: '/audit', name: 'Audit & CVEs', icon: '🛡️', sub: 'Trivy & IaC Scanner' },
      { path: '/compliance', name: 'Compliance & CIS', icon: '📋', sub: 'SOC2 / CIS Level 2' },
      { path: '/drift', name: 'Config Drift', icon: '⚡', sub: 'GitOps Reconciliation' },
      { path: '/backup', name: 'Disaster Recovery', icon: '📦', sub: 'Dual-Sync NVMe + S3' },
    ]
  },
  {
    key: 'automation',
    label: 'AUTOMATION & FINOPS',
    icon: '⚡',
    items: [
      { path: '/automation', name: 'Automation Rules', icon: '⚙️', sub: 'Event-Driven SRE' },
      { path: '/runbooks', name: 'SRE Runbooks', icon: '📖', sub: 'One-Click Remediation' },
      { path: '/cost', name: 'Cost Optimization', icon: '💰', sub: 'FinOps Node Bin-Packing' },
      { path: '/capacity', name: 'Capacity Forecast', icon: '📈', sub: 'ARIMA Time-Series' },
    ]
  },
  {
    key: 'management',
    label: 'MANAGEMENT',
    icon: '🏢',
    items: [
      { path: '/tenancy', name: 'Tenancy & RBAC', icon: '🏢', sub: 'Org Isolation Matrix' },
      { path: '/ai-hub', name: 'AI Provider Hub', icon: '🧠', sub: 'LLM Gateway & Breakers' },
      { path: '/changes', name: 'Change Requests', icon: '📝', sub: 'RFC Approvals & Windows' },
      { path: '/alerts', name: 'Alerts & Channels', icon: '🔥', sub: 'Slack, Telegram, Email' },
      { path: '/reports', name: 'Reports Center', icon: '📊', sub: 'Executive SOC2 & Cost' },
      { path: '/catalog', name: 'Service Catalog', icon: '📚', sub: 'Developer Portal & APIs' },
      { path: '/scaffolder', name: 'Scaffolder Templates', icon: '🪄', sub: '1-Click App Deployment' },
      { path: '/ecosystem', name: 'Ecosystem Tools', icon: '⚡', sub: 'Auto-Detected Stack' },
      { path: '/plugins', name: 'Plugin Hub', icon: '🧩', sub: 'JS Extension Runtime' },
      { path: '/settings', name: 'System Settings', icon: '🔧', sub: 'MetalLB & KEDA Nodes' },
    ]
  }
]
