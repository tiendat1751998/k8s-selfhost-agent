# K8sControl Project State — Session Handoff 2026-08-19T16:28

## Running Subagents
- **Infra Hosts Builder** (fe3198db): Building dedicated Infrastructure Hosts page with multi-type support + full CRUD. CHECK if committed.

## Git State (master)
9586bde fix(metrics): ListAll bypass tenant for agent scraping
e2654ce fix(hosts): rename Docker Host to Infrastructure Host + agent test
071a976 fix(deploy): remove sudo from deploy script
243127b docs: deployment guide + deploy script
c534f3c feat(metrics): scrape k8s-agent hosts + merge overview
3c81f3d feat(agent): lightweight monitoring agent binary
545841e feat(frontend): TOTP 2FA login + setup wizard + token refresh
0041b49 feat(auth): TOTP 2FA + JWT refresh tokens + recovery codes
62bba6c security(docker): swarm join tokens protection
5668ef0 feat(docker): node management APIs + multi-host + swarm tokens
2c7ab2e feat(frontend): real-time overview with topology + gauges
e229a5f feat(metrics): Docker stats collector + WebSocket + overview API
684d70f feat(ecosystem): auto-detector + dashboard
432f1f9 feat(plugins): plugin registry + marketplace
e98e229 security(P1-P2): tenant isolation + UUID validation

## Repo
GitHub: https://github.com/tiendat1751998/k8s-selfhost-agent (master)

## Credentials
- Postgres: 10.10.10.133:5432 myuser/mysecretpassword mydatabase
- Docker: tcp://10.10.10.133:2375
- Admin: admin@k8s.local / admin123
- JWT: k8s-selfhost-enterprise-jwt-secret-2026
- Encryption: 0123456789abcdef0123456789abcdef

## Architecture
- Backend: Go + chi, port 8080, Clean Architecture
- Frontend: Vue3 + Vite + Pinia, port 3000
- PlatformHandlers in router.go holds ALL handlers
- WebSocket: WSHub + WSBridge.Broadcast (real-time metrics)
- Metrics collector: goroutine poll Docker + agent hosts 5s
- RBAC: platform_admin, tenant_admin, operator, viewer
- JWT: access 15min + partial 5min (MFA) + refresh 7d httpOnly
- TOTP: pquerna/otp, AES-GCM encrypted, bcrypt recovery codes

## Migrations 033-041
033:cloud_accounts 034:platform_settings 035:service_catalog 036:admin_pw
037:plugins 038:scaffold_templates 039:ecosystem_tools 040:compute_hosts
041:user_mfa+refresh_tokens

## Completed Features
- Cloud Provider Connector, Platform Settings, Service Catalog
- Security Audit 18/18, Plugin System, Scaffolder Templates
- Ecosystem Auto-Detector, Real-time Overview Dashboard
- Node Management (swarm tokens, drain/activate/remove)
- TOTP 2FA + JWT Refresh (2-step login, QR wizard, recovery codes)
- K8s-Agent Binary (cmd/agent/, deploy-agent.sh)
- Agent Host Scraping (ListAll bypass tenant)
- Infrastructure Host UI (agent type, test connectivity)

## IN PROGRESS
- Infrastructure Hosts Page: dedicated /hosts page, CRUD, multi-type
  (agent/docker/k8s/prometheus/git/database/custom), sidebar nav
  Agent fe3198db may have committed — CHECK git log

## BLOCKED (waiting K8s cluster)
Pod Terminal, Helm Catalog, GitOps Visual, Real-time Logs,
Image CVE Scanning, Policy Dashboard, Service Mesh, Network Policy

## TODO (non-K8s)
- Edge Agent, AI + Ecosystem Data integration

## User Preferences
- Quality > speed, always subagents, fix without asking
- Chrome DevTools MCP for UI verify, always UI approach
- Zero-mock, Vietnamese, max 3 parallel subagents

## Technical Notes
- PowerShell: ; not && | Chrome DevTools: evaluate_script for clicks
- BuildTenantQuery adds tenant_id — use ListAll() for background services
- compute_hosts stored under tenant_id='default-tenant'
- 49+ test packages ALL PASS
