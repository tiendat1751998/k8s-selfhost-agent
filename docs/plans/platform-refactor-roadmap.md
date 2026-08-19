# Platform Refactor Roadmap — Based on 6-Platform Research

> Research: Rancher, Portainer, Headlamp, KubeSphere, Lens, Backstage
> Full report at conversation://921db223-f606-4593-8bb2-cf947ec72975

**Status**: APPROVED, ready for implementation

## P0: Must-Have (every competitor has these)

| # | Feature | Library/SDK | Effort |
|---|---|---|---|
| 1 | Pod Terminal (exec) | `client-go/remotecommand` + `gorilla/websocket` + xterm.js | 2 days |
| 2 | Helm App Catalog | `helm.sh/helm/v4/pkg/action` + `pkg/repo` | 3 days |
| 3 | ArgoCD/Flux GitOps visual | ArgoCD REST API + Flux CRDs | 2 days |
| 4 | Real-time Log Viewer | K8s pod logs WebSocket stream | 1 day |

## P1: Should-Have (enterprise competitive)

| # | Feature | Library/SDK | Effort |
|---|---|---|---|
| 5 | Image CVE Scanning | Trivy Operator CRDs or CLI | 2 days |
| 6 | Frontend Plugin System | Runtime JS loader (Headlamp pattern) | 3 days |
| 7 | Policy Dashboard | OPA/Kyverno CRDs via `client-go/dynamic` | 2 days |
| 8 | Service Catalog | New table + K8s annotations (Backstage pattern) | 3 days |

## P2: Nice-to-Have (differentiating)

| # | Feature | Effort |
|---|---|---|
| 9 | Service Mesh Topology (Istio/Kiali) | 4 days |
| 10 | Scaffolder Templates (1-click deploy) | 3 days |
| 11 | Network Policy Visual Builder | 3 days |
| 12 | Edge Agent (reverse tunnel) | 4 days |
| 13 | Port-Forward from UI | 1 day |

## Architecture Recommendations
1. Ecosystem Auto-Detector (detect ArgoCD/Trivy/Kyverno/Istio in cluster)
2. Proxy Pattern (Portainer-style: proxy native K8s API for advanced ops)
3. Vue Plugin Architecture (Rancher Dashboard micro-frontend pattern)
4. AI + Ecosystem Data (unique moat: feed Trivy/ArgoCD/Kyverno into AI Agents)
