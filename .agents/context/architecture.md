# System Architecture

## Overview

K8S Self-Healing Agent is a production-grade, multi-tenant SRE Control Plane built with **Clean Architecture** and **Domain-Driven Design (DDD)** principles. It combines a Go backend, a Python ADK multi-agent orchestrator, and a vanilla JavaScript dashboard frontend.

---

## Architecture Layers

```
┌─────────────────────────────────────────────────────────┐
│                   Presentation Layer                     │
│  ┌─────────────────┐  ┌──────────────────────────────┐  │
│  │ Frontend (HTML/  │  │ ADK Orchestrator (Python)    │  │
│  │ CSS/JS Dashboard)│  │ 10 Specialist Agents         │  │
│  └────────┬─────────┘  └──────────┬───────────────────┘  │
│           │                       │                      │
├───────────┼───────────────────────┼──────────────────────┤
│           ▼                       ▼                      │
│                   Adapter Layer (HTTP)                    │
│  ┌───────────────────────────────────────────────────┐   │
│  │ chi Router + JWT Auth + RBAC + CORS + Metrics     │   │
│  │ 25+ HTTP Handlers (REST JSON API)                 │   │
│  │ WebSocket Hub (real-time notifications)            │   │
│  └───────────────────────┬───────────────────────────┘   │
│                          │                               │
├──────────────────────────┼───────────────────────────────┤
│                          ▼                               │
│                   Usecase Layer                           │
│  ┌───────────────────────────────────────────────────┐   │
│  │ Business logic services coordinating DB, cache,   │   │
│  │ events, and cluster operations                    │   │
│  └───────────────────────┬───────────────────────────┘   │
│                          │                               │
├──────────────────────────┼───────────────────────────────┤
│                          ▼                               │
│                   Domain Layer                            │
│  ┌───────────────────────────────────────────────────┐   │
│  │ 26 Domain Modules (Pure business models & ports)  │   │
│  │ No external dependencies allowed                  │   │
│  └───────────────────────┬───────────────────────────┘   │
│                          │                               │
├──────────────────────────┼───────────────────────────────┤
│                          ▼                               │
│                Infrastructure Layer                       │
│  ┌──────────┐ ┌───────┐ ┌──────┐ ┌─────────┐ ┌───────┐ │
│  │PostgreSQL│ │ Redis │ │ NATS │ │   K8s   │ │Docker │ │
│  │  (pgx)   │ │(go-red│ │(jets)│ │(client) │ │(API)  │ │
│  └──────────┘ └───────┘ └──────┘ └─────────┘ └───────┘ │
└─────────────────────────────────────────────────────────┘
```

---

## Directory Structure

```
├── cmd/
│   ├── server/           # Full-featured server entrypoint
│   ├── standalone/       # Standalone mode (all-in-one)
│   └── agent-runner/     # ADK agent runner entrypoint
├── internal/
│   ├── domain/           # Pure business models and port interfaces (DDD)
│   │   ├── incident/     # Incident aggregate root
│   │   ├── drift/        # GitOps drift detection
│   │   ├── fleet/        # Multi-cluster fleet management
│   │   ├── deployment/   # Deployment lifecycle
│   │   ├── runbook/      # Runbook automation
│   │   ├── audit/        # Audit logging
│   │   ├── explorer/     # Resource explorer
│   │   ├── promotion/    # Environment promotion
│   │   ├── changes/      # Change management
│   │   ├── correlation/  # Event correlation
│   │   ├── capacity/     # Capacity planning
│   │   ├── compliance/   # Compliance checks
│   │   ├── observability/# SLO/SLI monitoring
│   │   ├── notification/ # Notification channels
│   │   ├── automation/   # Automation rules
│   │   ├── tagging/      # Resource tagging
│   │   ├── reporting/    # Report generation
│   │   ├── healthcenter/ # Health center
│   │   ├── timeline/     # Event timeline
│   │   ├── cost/         # Cost management
│   │   ├── backup/       # Backup management
│   │   ├── catalog/      # Service catalog
│   │   ├── gitops/       # GitOps PR management
│   │   ├── agent/        # Agent framework
│   │   └── provider/     # Provider abstraction
│   ├── usecase/          # Application business logic
│   ├── adapter/
│   │   ├── http/         # HTTP handlers, router, middleware
│   │   └── event/        # NATS event listeners
│   └── infrastructure/
│       ├── postgres/     # PostgreSQL repositories (pgx)
│       ├── redis/        # Redis cache client
│       ├── nats/         # NATS JetStream publisher
│       ├── kubernetes/   # K8s client-go integration
│       ├── cluster/      # Multi-cluster manager
│       ├── config/       # Viper config loader
│       ├── llm/          # LLM client (Gemini/Ollama)
│       ├── gitprovider/  # Git provider (GitHub)
│       └── provider/     # Cluster provider factory
├── orchestrator-agent/   # Python ADK multi-agent system
│   ├── app/
│   │   ├── agent.py      # 10-agent orchestrator definition
│   │   └── mcp_server.py # MCP server for K8s platform tools
│   └── tests/            # Unit and integration tests
├── frontend/             # Vanilla JS dashboard (HTML/CSS/JS)
├── migrations/           # 24 timestamped PostgreSQL migrations
├── deployments/
│   ├── docker/           # Dockerfiles
│   ├── helm/             # Helm chart (k8sselfhost)
│   └── k8s/              # Raw K8s manifests
└── .agents/              # Agent profile definitions
```

---

## Communication Patterns

| Pattern | Technology | Usage |
|---------|-----------|-------|
| Sync Request/Response | HTTP REST (chi) | Frontend <-> Backend API |
| Real-time Push | WebSocket (gorilla) | Live notifications, cluster events |
| Async Event Bus | NATS JetStream | Incident events, audit trail |
| Agent-to-Backend | HTTP REST | ADK agents calling Go API tools |
| Agent-to-Agent | ADK Transfer | Orchestrator routing between 10 agents |

---

## Key Design Decisions

1. **Clean Architecture Boundary Enforcement**: Domain layer has zero external imports. All infrastructure binds through port interfaces declared in Domain.
2. **Multi-Tenancy**: Every request is scoped by `tenant_id` extracted from JWT claims.
3. **AES-256-GCM Encryption**: Cluster credentials stored encrypted in-memory, decrypted only at point of use.
4. **Parameter-Bound SQL**: All database queries use `$1, $2` placeholders via pgx. No string concatenation.
5. **Nil-Safe Platform Handlers**: Standalone mode gracefully skips uninitialized handlers.
