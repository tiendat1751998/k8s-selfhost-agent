# Rancher — Research Summary

## What is Rancher?
Open-source **multi-cluster Kubernetes management platform**. Manages deployment, lifecycle, and security for K8s clusters across any infrastructure (on-prem, cloud, bare metal, edge).

---

## Architecture

```
┌─────────────────────────────────────────────┐
│         Management Cluster (Upstream)        │
│                                              │
│  ┌──────────────┐  ┌───────────────────────┐ │
│  │ Rancher API  │  │  Custom Controllers   │ │
│  │   Server     │  │  (CRDs + wrangler)    │ │
│  └──────┬───────┘  └───────────┬───────────┘ │
│         │                      │              │
│  ┌──────┴───────┐  ┌──────────┴────────────┐ │
│  │  Auth Proxy  │  │    Fleet Engine       │ │
│  │ (LDAP/SAML/  │  │    (GitOps)           │ │
│  │  OIDC/OAuth) │  │                       │ │
│  └──────────────┘  └───────────────────────┘ │
│         │  etcd (state store)                 │
└─────────┼─────────────────────────────────────┘
          │ TLS Websocket Tunnel (outbound)
          │
    ┌─────┴──────┐     ┌─────────────┐
    │ Downstream │     │ Downstream  │
    │ Cluster A  │     │ Cluster B   │
    │ ┌────────┐ │     │ ┌────────┐  │
    │ │ cattle │ │     │ │ cattle │  │
    │ │ agent  │ │     │ │ agent  │  │
    │ └────────┘ │     │ └────────┘  │
    └────────────┘     └─────────────┘
```

### Key Components
| Component | Role |
|---|---|
| **Rancher Server** | API server + custom controllers, extends K8s via CRDs |
| **cattle-cluster-agent** | Agent inside each downstream cluster, tunnels TLS back to Rancher |
| **Auth Proxy** | Centralized SSO (Okta, AD, GitHub, SAML/OIDC) mapped to RBAC |
| **Fleet** | GitOps engine, syncs Helm charts and manifests from Git |
| **Steve** | Modern K8s API translation layer |
| **Norman** | Legacy v3 API (backward compatibility) |

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | **Go 1.22/1.23** |
| K8s framework | `wrangler` (controllers), `steve` (API), `norman` (legacy) |
| K8s client | `k8s.io/client-go`, `k8s.io/apimachinery` |
| GitOps | `rancher/fleet` |
| Packaging | `helm/helm` |
| Frontend | **Vue.js / Nuxt** (`rancher/dashboard` — separate repo) |
| Build | **Dapper** (containerized build env) |
| CI | GitHub Actions + Drone CI |

---

## Repo Structure (`rancher/rancher`)

```
rancher/rancher/
├── cmd/           # Binary entry points (rancher server, agents)
├── pkg/           # Core business logic
│   ├── controllers/   # K8s controllers (CRDs lifecycle, RBAC, node pools)
│   ├── api/
│   │   ├── steve/     # Modern K8s-native API layer
│   │   └── norman/    # Legacy v3 API
│   ├── apis/          # CRD definitions (provisioning.cattle.io, management.cattle.io)
│   ├── auth/          # Auth middleware (LDAP, SAML, OAuth)
│   ├── provisioningv2/ # Cluster lifecycle & provisioning engine
│   └── generated/     # Auto-generated clientsets
├── charts/        # Bundled Helm charts
├── package/       # Dockerfiles for production images
├── scripts/       # Build, CI, validate, package scripts
├── tests/         # Integration & E2E tests
├── vendor/        # Go module dependencies
└── .github/       # GitHub Actions CI/CD
```

**Multi-repo architecture** — `rancher/rancher` is the main hub, with separated components:
- `rancher/dashboard` → Web UI
- `rancher/steve` → API engine
- `rancher/fleet` → GitOps
- `rancher/rke2` / `rancher/k3s` → K8s engines

---

## Self-Host Deployment

### Production (HA)
```bash
# 1. Setup 3-node K8s cluster (RKE2/K3s recommended)
# 2. Install cert-manager
helm install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace

# 3. Deploy Rancher via Helm
helm install rancher rancher-latest/rancher \
  --namespace cattle-system --create-namespace \
  --set hostname=rancher.example.com
```
- L4 Load Balancer in front (port 80/443)
- Multi-node etcd for fault tolerance

### Dev / PoC (Single Docker)
```bash
docker run -d --restart=unless-stopped \
  -p 80:80 -p 443:443 \
  rancher/rancher:v2.10.x
```
- Runs embedded K3s inside the container
- ⚠️ Non-production use only
