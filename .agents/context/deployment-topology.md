# Deployment Topology

## Overview

The K8S Self-Healing Agent system consists of 3 primary services backed by 3 infrastructure components.

---

## Service Map

```
┌────────────────────────────────────────────────────┐
│                 External Clients                    │
│    Browser Dashboard / ADK Playground / curl        │
└──────────────┬────────────────┬─────────────────────┘
               │ :8080          │ :8200
               ▼                ▼
┌──────────────────┐  ┌──────────────────────────────┐
│  Go Backend      │  │  ADK Playground (Python)     │
│  (standalone)    │  │  uvicorn :8000 → proxy :8200 │
│  Port: 8080      │  │  10 Specialist Agents        │
│                  │  │  MCP Server (K8s tools)       │
│  Serves:         │  └──────────┬───────────────────┘
│  - REST API      │             │ HTTP calls
│  - WebSocket     │◄────────────┘
│  - Frontend SPA  │
│  - /metrics      │
│  - /healthz      │
└──────┬───────────┘
       │
       ▼
┌──────────────────────────────────────────────────┐
│               Infrastructure Layer                │
│                                                   │
│  ┌────────────┐ ┌──────────┐ ┌────────────────┐  │
│  │ PostgreSQL │ │  Redis   │ │ NATS JetStream │  │
│  │ Port: 5432 │ │ Port:6379│ │ Port: 4222     │  │
│  │ 24 tables  │ │ Cache DB0│ │ Stream:        │  │
│  │ 24 migrat. │ │          │ │ INCIDENTS      │  │
│  └────────────┘ └──────────┘ └────────────────┘  │
│                                                   │
│  ┌────────────────────┐ ┌──────────────────────┐  │
│  │ Kubernetes Cluster │ │ Docker Swarm Host    │  │
│  │ (client-go)        │ │ Port: tcp://...:2375 │  │
│  └────────────────────┘ └──────────────────────┘  │
└──────────────────────────────────────────────────┘
```

---

## Services

### 1. Go Backend (Standalone Mode)

| Property | Value |
|----------|-------|
| **Binary** | `cmd/standalone/main.go` |
| **Port** | `8080` |
| **Framework** | chi/v5 router |
| **Config** | `config.yaml` + environment variables |
| **Env Vars** | `ENCRYPTION_KEY` (32-byte AES key), `JWT_SECRET` |
| **Health** | `/healthz`, `/readyz`, `/livez` |
| **Metrics** | `/metrics` (Prometheus) |
| **Profiling** | `/debug/pprof/*` (gated by `K8S_ENABLE_PPROF=true`) |

### 2. ADK Playground (Python)

| Property | Value |
|----------|-------|
| **Entrypoint** | `orchestrator-agent/app/agent.py` |
| **Server** | uvicorn (port 8000 internal, proxied to 8200) |
| **Command** | `uv run agents-cli playground --port 8200` |
| **Config** | `orchestrator-agent/.env` (`GOOGLE_API_KEY`, `BACKEND_API_URL`) |
| **Dependencies** | `google-adk`, `httpx`, `fastapi` |

### 3. Frontend Dashboard (Static SPA)

| Property | Value |
|----------|-------|
| **Location** | `frontend/` directory |
| **Tech** | HTML5, CSS3, Vanilla JavaScript |
| **Served by** | Go backend static file server at `/*` |
| **Auth** | JWT token in `Authorization: Bearer <token>` header |

---

## Infrastructure Components

### PostgreSQL 16

| Property | Value |
|----------|-------|
| **Host** | Configured in `config.yaml` (`postgres.host`) |
| **Port** | `5432` |
| **Database** | `mydatabase` (configurable) |
| **Driver** | `pgx/v5` (connection pool) |
| **Pool** | max_conns: 25, min_conns: 5 |
| **SSL** | `disable` (configurable) |
| **Migrations** | 24 timestamped SQL files in `migrations/` |

### Redis 7

| Property | Value |
|----------|-------|
| **Host** | Configured in `config.yaml` (`redis.host`) |
| **Port** | `6379` |
| **DB** | `0` |
| **Driver** | `go-redis/v9` |
| **Auth** | Optional password |

### NATS JetStream

| Property | Value |
|----------|-------|
| **URL** | Configured in `config.yaml` (`nats.url`) |
| **Port** | `4222` |
| **Stream** | `INCIDENTS` |
| **Subjects** | `incidents.>` |
| **Driver** | `nats-io/nats.go` |

---

## Kubernetes Deployment (Helm)

| Property | Value |
|----------|-------|
| **Chart** | `deployments/helm/k8sselfhost/` |
| **Image** | `k8sselfhost:0.1.0` |
| **Replicas** | 1 (HPA up to 5) |
| **Resources** | CPU: 100m-500m, Memory: 128Mi-256Mi |
| **Security** | `runAsNonRoot: true`, `readOnlyRootFilesystem: true`, drop ALL capabilities |
| **Probes** | Liveness: `/livez`, Readiness: `/readyz` |
| **Service** | ClusterIP on port 8080 |

---

## Network Topology

```
Internet → Ingress Controller → Service (ClusterIP:8080)
                                       │
                                       ▼
                              ┌─────────────────┐
                              │  Go Backend Pod  │
                              │  Port: 8080      │
                              └────┬────┬────┬───┘
                                   │    │    │
                          ┌────────┘    │    └────────┐
                          ▼             ▼             ▼
                   PostgreSQL       Redis         NATS
                   :5432            :6379         :4222
```
