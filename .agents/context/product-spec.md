# Product Specification: K8s Self-Host Platform

> **Version:** 1.0 — Draft
> **Author:** datdt + AI
> **Status:** Pending Review

---

## 1. Vision

Xây dựng **all-in-one self-hosted platform** cho dev/devops/devsecops — kết hợp sức mạnh multi-cluster management của Rancher, tính modular của KubeSphere, và tính lightweight dễ deploy của Portainer. Tích hợp **AI-powered** monitoring & auto-remediation mà không platform nào trên thị trường có.

### One-liner
> **"Rancher + KubeSphere + AI Auto-Remediation trong một binary dễ deploy."**

---

## 2. Target Users

| Tier | Persona | Nhu cầu chính |
|---|---|---|
| **Solo Dev** | Freelancer / indie dev tự manage infra | Simple deploy, 1-click app install, DB backup |
| **Small Team** (2-10) | Startup / small company | Multi-user RBAC, shared cluster, CI/CD |
| **Enterprise** (10+) | Multi-team, multi-project | Multi-tenant, compliance, audit trail, fleet management |

Platform phải **scale down** cho solo dev (single binary, 5 phút setup) và **scale up** cho enterprise (HA, multi-cluster, policy enforcement).

---

## 3. Competitive Positioning

| Feature | Rancher | KubeSphere | Portainer | OpenShift | **K8s Self-Host (Ours)** |
|---|---|---|---|---|---|
| Multi-cluster | ✅ Best | ✅ | ⚠️ Basic | ✅ | ✅ |
| Built-in CI/CD | ❌ (Fleet only) | ✅ (Jenkins) | ❌ | ✅ (Tekton) | ✅ **Built-in + External** |
| DB Backup/Restore | ❌ (Velero) | ⚠️ (Velero) | ❌ | ⚠️ (OADP) | ✅ **Native multi-DB** |
| AI Auto-Remediation | ❌ | ❌ | ❌ | ❌ | ✅ **Unique** |
| GitOps | ✅ Fleet | ⚠️ | ⚠️ | ✅ ArgoCD | ✅ **Built-in** |
| DevSecOps | ⚠️ NeuVector | ⚠️ | ❌ | ✅ Best | ✅ |
| Lightweight deploy | ⚠️ Heavy | ❌ Heavy | ✅ Best | ❌ Very heavy | ✅ **Single binary** |
| Open Source | ✅ | ✅ | ⚠️ CE/BE | ❌ Paid | ✅ |

### Differentiators (3 pillars)
1. **AI-Powered** — RCA, auto-remediation, intelligent monitoring (không ai có)
2. **All-in-One** — Dev + DevOps + DevSecOps trong 1 platform
3. **Lightweight Self-Host** — Single binary, 5 phút deploy, không cần 3-node cluster như Rancher

---

## 4. Feature Modules & Priority

### P0 — Core Platform (Must-have, ship first)

#### M01: Cluster Management
- Import cluster via kubeconfig
- Cluster health dashboard (CPU, memory, pods, nodes)
- Resource explorer (Pods, Deployments, Services, ConfigMaps, Secrets)
- Multi-cluster fleet view
- **Đã có:** ✅ Phần lớn

#### M02: AI Incident Center
- Real-time anomaly detection
- AI-powered Root Cause Analysis (Gemini)
- Confidence scoring & risk assessment
- Auto-remediation suggestions + execution
- **Đã có:** ✅ Core implemented

#### M03: GitOps Engine
- Git baseline configuration management
- Live drift detection (cluster state vs Git)
- Auto-reconciliation & rollback
- Multi-cluster GitOps deployment (Fleet-like)
- **Đã có:** ✅ Drift detection, ⚠️ cần mở rộng multi-cluster deploy

#### M04: RBAC & Multi-Tenancy
- User/team/role management
- Tenant isolation (data + namespace level)
- Project-level & namespace-level permissions
- Audit trail (who did what, when)
- **Đã có:** ✅ Core RBAC + tenant isolation

#### M05: Observability Stack
- Metrics dashboard (Prometheus integration)
- Log aggregation & search (centralized)
- Distributed tracing (OTEL)
- SLO tracking & burn rate
- Alert rules + notification channels (Slack, email, webhook)
- **Đã có:** ✅ Metrics + tracing, ⚠️ log aggregation cần thêm, ⚠️ alerting cần mở rộng

---

### P1 — DevOps Essentials (Ship trong Phase 2)

#### M06: DB Backup & Restore 🆕
- **Architecture:** Operator-pattern CRD-driven

```
┌──────────────────────────────────────────────────┐
│              Backup Controller                    │
│  (Go operator, watches BackupPolicy CRDs)        │
├──────────────────────────────────────────────────┤
│                                                   │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐          │
│  │ Postgres │  │  MySQL  │  │ MongoDB │  ...     │
│  │ Adapter  │  │ Adapter │  │ Adapter │          │
│  │(pgdump/  │  │(xtra-   │  │(mongo-  │          │
│  │ WAL)     │  │ backup) │  │ dump)   │          │
│  └────┬─────┘  └────┬────┘  └────┬────┘          │
│       │              │            │               │
│       └──────────┬───┘────────────┘               │
│                  ▼                                 │
│         ┌────────────────┐                        │
│         │ Storage Backend│                        │
│         │ (S3/MinIO/     │                        │
│         │  Local/NFS)    │                        │
│         └────────────────┘                        │
└──────────────────────────────────────────────────┘
```

- **Supported DBs:** PostgreSQL, MySQL, MongoDB, Redis (Phase 1: Postgres first, iteratively add others)
- **Features:**
  - Scheduled backups (CronJob-based, configurable)
  - On-demand manual backup
  - Point-in-Time Recovery (PITR) cho Postgres (WAL streaming)
  - Logical backup (pg_dump, mysqldump, mongodump) + Physical backup
  - Backup to S3-compatible storage (MinIO, AWS S3, GCS)
  - Backup encryption (AES-256)
  - Retention policy (keep last N backups, auto-cleanup)
  - Restore to same cluster or different cluster
  - Backup status dashboard (success/fail, size, duration)
  - Backup verification (auto-restore to sandbox, integrity check)

#### M07: Helm Catalog / App Store 🆕
- Browse curated Helm charts
- 1-click deploy applications
- Custom chart repositories
- App lifecycle management (upgrade, rollback, uninstall)
- Application health monitoring

#### M08: CI/CD Pipeline Engine 🆕
- **Built-in simple pipeline:**
  - Visual pipeline builder (drag & drop or YAML)
  - Build → Test → Scan → Deploy stages
  - Container image build (Kaniko / BuildKit — no Docker daemon required)
  - Pipeline run history & logs
- **External tool integration:**
  - ArgoCD sync status & management
  - Tekton pipeline visualization
  - Jenkins job trigger & status
  - GitHub Actions webhook integration

#### M09: Secret Management 🆕
- Encrypted secret storage (AES-256-GCM — đã có crypto package)
- Secret rotation scheduling
- HashiCorp Vault integration (optional)
- Leaked secret scanning in Git repos
- Secret injection into pods (mutating webhook)

#### M10: Centralized Logging 🆕
- Log collection agent (DaemonSet, tương tự Fluentd/Promtail)
- Log storage (embedded hoặc external Loki)
- Full-text search & filter
- Log-based alerting
- Log retention & rotation policies

---

### P2 — DevSecOps & Compliance (Phase 3)

#### M11: Security Scanner 🆕
- Container image vulnerability scanning (Trivy integration)
- CIS Kubernetes Benchmark scanning
- Runtime security monitoring
- Security audit reports
- CVE tracking & remediation suggestions

#### M12: Policy Engine 🆕
- Policy-as-Code (OPA Gatekeeper / Kyverno integration)
- Pre-built policy library (pod security, resource limits, network)
- Policy violation dashboard
- Admission webhook enforcement
- Compliance reporting (SOC2, PCI-DSS checklists)

#### M13: Network Management 🆕
- Network policy visualization (topology map)
- Network policy management UI
- Service mesh integration (Istio/Linkerd — optional)
- Ingress/Gateway management

#### M14: Container Registry 🆕
- Embedded private registry (Distribution-based)
- Image scanning on push
- Image signing & verification
- Garbage collection & retention
- Registry mirror/proxy for air-gapped environments

---

### P3 — Enterprise & Scale (Phase 4, ongoing)

- Cluster provisioning (create RKE2/K3s clusters from UI)
- Disaster Recovery (cross-region failover)
- Cost optimization & right-sizing recommendations
- Plugin/extension marketplace
- Multi-region federation
- API gateway management

---

## 5. Architecture

### Deployment Model: Hybrid Monolith

```
┌─────────────────────────────────────────────────┐
│                K8s Self-Host Platform             │
│                                                   │
│  ┌─────────────────────────────────────────────┐ │
│  │         Core Monolith (Go binary)           │ │
│  │                                              │ │
│  │  REST API │ WebSocket │ RBAC │ GitOps       │ │
│  │  AI/RCA   │ Cluster Manager │ Observability │ │
│  │  App Store│ CI/CD Controller │ Secrets      │ │
│  └──────────────────┬──────────────────────────┘ │
│                     │                             │
│  ┌──────────────────┼──────────────────────────┐ │
│  │    Optional Microservices (heavy workloads)  │ │
│  │                                              │ │
│  │  ┌──────────┐  ┌───────────┐  ┌──────────┐ │ │
│  │  │ Backup   │  │    Log    │  │ Pipeline │ │ │
│  │  │ Engine   │  │ Collector │  │  Runner  │ │ │
│  │  └──────────┘  └───────────┘  └──────────┘ │ │
│  └──────────────────────────────────────────────┘ │
│                                                   │
│  ┌──────────────────────────────────────────────┐ │
│  │         ADK Agent System (Python)            │ │
│  │  Orchestrator + 10 Specialist Agents         │ │
│  └──────────────────────────────────────────────┘ │
│                                                   │
│  Infrastructure: PostgreSQL │ Redis │ NATS        │
└─────────────────────────────────────────────────┘
```

### Design Principles
1. **Core monolith first** — API, RBAC, cluster management, UI đều trong 1 binary
2. **Optional microservices** — Backup engine, log collector, pipeline runner có thể chạy riêng khi cần scale
3. **Single binary deploy** — Solo dev chỉ cần `./k8sselfhost` + Postgres + Redis
4. **HA deploy** — Enterprise chạy 3 replicas + external Postgres + Redis cluster

---

## 6. Non-Functional Requirements

| Requirement | Target |
|---|---|
| **Startup time** | < 10 giây (cold start) |
| **Memory footprint** | < 512MB (core monolith, idle) |
| **API latency** | P99 < 200ms (CRUD operations) |
| **Concurrent users** | 100+ (single instance) |
| **Cluster support** | 50+ managed clusters |
| **Backup RPO** | Configurable, minimum 1 hour |
| **Backup RTO** | < 30 minutes (single DB restore) |
| **Availability** | 99.9% (HA mode) |
| **Security** | AES-256-GCM encryption, JWT auth, tenant isolation |
| **Compliance** | CIS Benchmark, OWASP Top 10 |

---

## 7. Tech Stack (Confirmed)

| Layer | Technology | Rationale |
|---|---|---|
| Backend | Go 1.26 | Already built, Clean Architecture |
| AI Agents | Python 3.12 + Google ADK | Already built, Gemini integration |
| Database | PostgreSQL 16 | Already built, 26 migrations |
| Cache | Redis 7 | Already built |
| Messaging | NATS JetStream | Already built |
| Frontend | **Vue.js** (migration from Vanilla JS) | Rancher dashboard dùng Vue, scalable cho 14+ modules |
| K8s Client | client-go v0.36 | Already built |
| Container | Docker API v28.5 | Already built |
| Observability | OTEL + Prometheus + Zap | Already built |
| DB Backup | Custom operator (Go) | New — pg_dump/WAL + xtrabackup + mongodump |
| CI/CD | Custom controller + ArgoCD/Tekton integration | New |
| Log Aggregation | Custom agent → Loki compatible storage | New |
| Security Scanning | Trivy (library integration) | New |

---

## 8. Decisions Log (Confirmed)

| Question | Decision |
|---|---|
| **Q1: Tên sản phẩm** | Giữ "K8s Self-Host" tạm thời, đặt tên chính thức sau |
| **Q2: Frontend** | ✅ **Chuyển sang Vue.js** — scalable cho 14+ modules, Rancher dashboard cũng dùng Vue |
| **Q3: Backup storage** | ✅ **Embedded MinIO + external S3 + local filesystem** — zero-config cho solo dev |
| **Q4: Log aggregation** | ✅ **Built-in simple + external Loki/Elasticsearch** — flexible cho mọi tier |
| **Q5: Timeline** | Không deadline cụ thể, iterate từng phase, quality over speed |
