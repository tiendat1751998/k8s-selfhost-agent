# System Architecture

## Overview

K8sControl (K8S Self-Healing & Infrastructure Control Plane) is an enterprise-grade, multi-tenant hybrid platform built with **Clean Architecture** and **Domain-Driven Design (DDD)** principles. It unifies Kubernetes cluster orchestration, bare-metal server telemetry, dedicated database host monitoring, and automated disaster recovery into a single high-performance system.

The platform combines:
1. **Go Backend (Clean Architecture)**: High-throughput API server running on Chi router, containing **38 Domain Modules**, **17 Application Usecases**, and **39 HTTP REST Handlers**.
2. **Modern Frontend (Vue 3 + TypeScript + Vite + Pinia)**: 34 Modular Views ($\le$ 350 lines per view) with strict Tri-Partition architecture (segregated CSS, composables, sub-components) and 4-Tier Responsive Web Design (Mobile PWA, Tablet, Desktop, 4K).
3. **Tri-Runtime Adaptive Telemetry Agent (`k8s-agent`)**: Single Go binary (~8MB) running natively on Bare-Metal/DB hosts via `systemd`, containerized on Docker hosts, or auto-scaled as a Kubernetes DaemonSet.
4. **Disaster Recovery & Dual-Target Backup**: Pipelined streaming backup engine (`dualsync`) with `zstd` compression and `AES-256-GCM` encryption for 8 databases (PostgreSQL, MySQL, MongoDB, Redis, MariaDB, Oracle, SQL Server) and etcd/Velero.
5. **Shift-Left DevSecOps**: Built-in Trivy container vulnerability scanner, Checkov IaC scanner, and HashiCorp Vault integration.

---

## Architecture Layers

```
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                                  Presentation Layer                                    │
│  ┌───────────────────────────────────────────┐  ┌───────────────────────────────────┐  │
│  │ Frontend Web Portal (frontend-vue)        │  │ Agent & Orchestrator Mesh         │  │
│  │ Vue 3 + TypeScript + Vite + Pinia         │  │ 22 Specialist Agent Roles         │  │
│  │ 34 Modular Views (<= 350 lines)           │  │ Chrome DevTools MCP QA Gate       │  │
│  └─────────────────────┬─────────────────────┘  └─────────────────┬─────────────────┘  │
│                        │                                          │                    │
├────────────────────────┼──────────────────────────────────────────┼────────────────────┤
│                        ▼                                          ▼                    │
│                                Adapter Layer (HTTP & Event)                            │
│  ┌──────────────────────────────────────────────────────────────────────────────────┐  │
│  │ chi Router + JWT Access/Refresh Auth + TOTP 2FA + RBAC Middleware + CORS         │  │
│  │ 39 HTTP REST Handlers (JSON API v1)                                              │  │
│  │ WebSocket Hub & SSE Stream (Real-Time Pod Logs, Terminal xterm, Live Telemetry)  │  │
│  └──────────────────────────────────────────┬───────────────────────────────────────┘  │
│                                             │                                          │
├─────────────────────────────────────────────┼──────────────────────────────────────────┤
│                                             ▼                                          │
│                                      Usecase Layer                                     │
│  ┌──────────────────────────────────────────────────────────────────────────────────┐  │
│  │ 17 Business Logic Services: Cluster Bootstrap, Metrics Collector, GitOps, SRE    │  │
│  │ Remediation, Dual-Target Backup Engine, SLO Forecaster, Scaffolder, Incidents    │  │
│  └──────────────────────────────────────────┬───────────────────────────────────────┘  │
│                                             │                                          │
├─────────────────────────────────────────────┼──────────────────────────────────────────┤
│                                             ▼                                          │
│                                       Domain Layer                                     │
│  ┌──────────────────────────────────────────────────────────────────────────────────┐  │
│  │ 38 Pure Domain Modules (Business Entities, Aggregates & Port Interfaces)          │  │
│  │ ZERO external dependencies allowed (strict boundary enforcement)                │  │
│  └──────────────────────────────────────────┬───────────────────────────────────────┘  │
│                                             │                                          │
├─────────────────────────────────────────────┼──────────────────────────────────────────┤
│                                             ▼                                          │
│                                   Infrastructure Layer                                 │
│  ┌────────────┐ ┌─────────────┐ ┌──────────────┐ ┌───────────────┐ ┌────────────────┐  │
│  │ PostgreSQL │ │ Redis Cache │ │ NATS Streams │ │ K8s client-go │ │ Docker/Swarm   │  │
│  │ (54 Migs)  │ │ (In-Memory) │ │ (Event Bus)  │ │ (Dynamic/CRI) │ │ (Engine API)   │  │
│  ├────────────┤ ├─────────────┤ ├──────────────┤ ├───────────────┤ ├────────────────┤  │
│  │ k8s-agent  │ │ DualSync DB │ │ Trivy/Checkov│ │ Longhorn CSI  │ │ Traefik / Envoy│  │
│  │ (/proc 9100│ │ (zstd/AES)  │ │ (SecOps Hub) │ │ (3x Replicas) │ │ (L4/L7 Ingress)│  │
│  └────────────┘ └─────────────┘ └──────────────┘ └───────────────┘ └────────────────┘  │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## Directory Structure

```
├── cmd/
│   ├── agent/            # k8s-agent native Linux /proc telemetry binary (port 9100)
│   ├── probe/            # Static health probe binary
│   ├── server/           # Full-featured enterprise API server
│   └── standalone/       # Standalone all-in-one distribution binary
├── internal/
│   ├── domain/           # 38 Pure business models and port interfaces (DDD)
│   │   ├── agent/        # Agent Mesh & Registry
│   │   ├── alert/        # Alert rules, suppression & incident alerts
│   │   ├── audit/        # Enterprise audit trail
│   │   ├── automation/   # Operational workflow automation
│   │   ├── backup/       # Database & system backup policies
│   │   ├── capacity/     # Hardware & K8s cluster capacity forecasting
│   │   ├── catalog/      # Service catalog & template definitions
│   │   ├── changes/      # Infrastructure change timeline
│   │   ├── cloud/        # Cloud account federation (AWS/GCP/Azure)
│   │   ├── compliance/   # Security compliance & CIS framework
│   │   ├── correlation/  # Event & log correlation engine
│   │   ├── cost/         # FinOps cost estimation & attribution
│   │   ├── deployment/   # Workload lifecycle (Rollout/Canary/BlueGreen)
│   │   ├── drift/        # GitOps configuration drift detection
│   │   ├── ecosystem/    # Tool integrations (Vault, Argo, Prometheus)
│   │   ├── explorer/     # K8s resource discovery & kind registry
│   │   ├── fleet/        # Multi-cluster fleet topology
│   │   ├── gitops/       # Git repository synchronization
│   │   ├── healthcenter/ # Unified cluster health inspection
│   │   ├── incident/     # SRE incidents & root cause analysis (RCA)
│   │   ├── loadbalancer/ # Traefik, HAProxy, NGINX provider ports
│   │   ├── nodemetrics/  # Rollup & time-series node metrics
│   │   ├── notification/ # Telegram, Slack, Webhook channels
│   │   ├── observability/# SLO, SLI error budget tracking
│   │   ├── plugin/       # Dynamic extension plugin system
│   │   ├── ports/        # TransactionManager & database ports
│   │   ├── promotion/    # Staged deployment promotion
│   │   ├── provider/     # Infrastructure cluster providers
│   │   ├── report/       # Executive reporting generator
│   │   ├── reporting/    # Custom operational report builder
│   │   ├── runbook/      # Operational disaster runbooks
│   │   ├── scaffold/     # Code & manifest scaffolder
│   │   ├── search/       # Universal cross-resource search
│   │   ├── settings/     # Platform configuration & feature flags
│   │   ├── tagging/      # Resource metadata tagging
│   │   ├── tenancy/      # Multi-tenant isolation & workspace scoping
│   │   ├── timeline/     # Global platform event timeline
│   │   └── user/         # Identity, RBAC roles & MFA credentials
│   ├── usecase/          # 17 Application business logic orchestrators
│   ├── adapter/
│   │   ├── http/         # 39 HTTP REST handlers, Chi router, JWT/TOTP middleware
│   │   └── event/        # Real-time event listeners & node watchers
│   └── infrastructure/
│       ├── backup/       # DualSync streaming engine & 8 DB drivers
│       ├── cluster/      # Multi-cluster dynamic ClientManager
│       ├── config/       # Configuration loader
│       ├── helm/         # Helm v3 SDK release manager
│       ├── kubernetes/   # K8s client-go dynamic client & ResourceRepo
│       ├── loadbalancer/ # Traefik & HTTP load balancer providers
│       ├── notifier/     # Telegram, Slack, Email delivery
│       ├── postgres/     # PostgreSQL pgxpool, TxManager, tenant query builder
│       └── security/     # Trivy CVE scanner, Checkov IaC, HashiCorp Vault
├── frontend-vue/         # Vue 3 + Vite + TypeScript + Pinia Portal
│   ├── src/
│   │   ├── assets/styles/# Segregated view CSS (100% isolated stylesheets)
│   │   ├── components/   # Modular sub-components, dialogs & drawers
│   │   ├── composables/  # Reactive business logic & API composables
│   │   ├── domain/       # Client-side domain models & formatters
│   │   ├── stores/       # Pinia reactive state stores
│   │   └── views/        # 34 Views (100% compliant: <= 350 lines each)
├── migrations/           # 54 PostgreSQL schema migrations (Up & Down SQL)
├── deploy/               # Deployment manifests for K8s, Docker & Logging
│   ├── k8s/              # k8s-agent DaemonSet & K8s manifests
│   ├── docker/           # Docker Compose agent definitions
│   └── logging/          # Vector & log collection DaemonSets
└── .agents/              # Agent constitutions, memory & task tracking
    ├── context/          # System architecture & reference context
    ├── memory/           # Decision logs (decision_log.jsonl) & project state
    ├── rules/            # Architectural & verification rules
    └── tasks/            # Actionable implementation plans (inprocess / success)
```

---

## Communication Patterns

| Pattern | Technology | Usage |
| :--- | :--- | :--- |
| **Sync Request/Response** | HTTP REST (Chi) | Frontend $\leftrightarrow$ Backend API (JSON v1) |
| **Real-time Pod Logs** | Server-Sent Events (SSE) | Live streaming of Kubernetes Pod logs |
| **Interactive Terminal** | WebSocket (xterm.js) | Full bi-directional web shell into running Pods |
| **Live Telemetry & Alerts**| WebSocket Hub | Saturation curves, request flow animation, alert toast |
| **Host Scrape Telemetry** | HTTP Pull / Native /proc | Central collector $\rightarrow$ `k8s-agent` on port 9100 |
| **Edge Reverse Tunnel** | WebSocket / gRPC | Agents behind NAT/Firewall $\rightarrow$ Central Server |
| **Disaster Dual-Sync** | Streaming Pipeline | zstd stream $\rightarrow$ AES-256 encryption $\rightarrow$ S3 & Local NVMe |

---

## Key Design Principles & Guarantees

1. **Clean Architecture Boundary Enforcement**: The `internal/domain/` layer has zero external third-party imports. All infrastructure bindings pass through port interfaces.
2. **Strict Multi-Tenancy & Data Isolation**: Every request is scoped by `tenant_id` from validated JWT claims. SQL queries use parameterized `$1, $2` variables via pgx with zero string concatenation.
3. **Zero Fake Data & Zero Stubs Invariant**: Prohibit `Math.random()` and fake latency mocks. Offline nodes report `latency_ms: 0` and display em-dash (`--`).
4. **Tri-Runtime Fleet Adaptability**: `k8s-agent` runs identically across Bare-Metal/DB servers (via systemd), Docker hosts (via Compose), and K8s clusters (via DaemonSet).
5. **Disaster Recovery by Design**: Critical control-plane state (etcd) and databases are protected by scheduled, encrypted snapshots with tested failover SLAs (< 30s).

---

## Architectural Refactoring Standards (Quality Gate)

### 1. Frontend Modularity Standard (< 350 Lines per View)
- **Status**: **34 / 34 Views (100% COMPLIANT)**.
- **Tri-Partition Separation**:
  1. **CSS**: Segregated into `src/assets/styles/views/<view>.css` (0 `<style scoped>` blocks in views).
  2. **Composables**: Business logic in `src/composables/use<View>.ts`.
  3. **Sub-Components**: Modals and drawers in `src/components/<feature>/` ($\le$ 300 lines).
- **Active Refactoring Backlog**: Break down legacy monolithic dialogs (`NodeLiveDiagnostics.vue`, `DeepDiveTrafficModal.vue`, `App.vue`) to achieve 100% compliance across all sub-components.

### 2. Backend SOLID & Single Responsibility (< 500 Lines Standard)
- **Active Refactoring Backlog**: Decompose large infrastructure files (`resource_repo.go`, `docker_handler.go`, `tps_collector.go`) into focused sub-packages following the established DDD port pattern.
