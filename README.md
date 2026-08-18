# K8S Self-Healing Agent

**AI-Powered Multi-Agent Kubernetes Control Plane with GitOps Auto-Remediation**

[![CI](https://github.com/tiendat1751998/k8s-selfhost-agent/actions/workflows/ci.yml/badge.svg)](https://github.com/tiendat1751998/k8s-selfhost-agent/actions)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go)](https://go.dev/)
[![ADK](https://img.shields.io/badge/Google_ADK-Agent_Dev_Kit-4285F4?logo=google)](https://adk.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791?logo=postgresql)](https://postgresql.org/)
[![License: CC BY 4.0](https://img.shields.io/badge/License-CC_BY_4.0-lightgrey.svg)](https://creativecommons.org/licenses/by/4.0/)

---

## Overview

An enterprise-grade, multi-tenant platform that continuously monitors Kubernetes and Docker Swarm clusters, performs AI-powered root cause analysis (RCA) on incidents, detects configuration drift against Git baselines, and auto-remediates failures using a **10-agent multi-agent system** built on Google's Agent Development Kit (ADK) (Note: The Go domain model defines 15 agent types).

### Key Features

| Feature | Description |
|---------|-------------|
| **Multi-Agent Orchestrator** | 10 specialist AI agents (Architect, Backend, Frontend, DBA, DevOps, QA, Security, Reviewer, K8s, GitOps) coordinated by a central routing agent |
| **AI-Powered RCA** | Gemini-powered root cause analysis with confidence scoring and risk assessment |
| **GitOps Drift Detection** | Real-time comparison between live cluster state and Git repository baseline |
| **Self-Healing** | Automated remediation via Git commits, scaling actions, and config rollbacks |
| **Real-Time Dashboard** | Premium dark-mode UI with WebSocket live updates, glassmorphism design |
| **Encrypted Credentials** | AES-256-GCM encrypted cluster credentials with on-the-fly decryption |
| **Multi-Tenant RBAC** | Role-based access control with tenant isolation |
| **26 Database Migrations** | Production-grade PostgreSQL schema covering all platform features |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    ADK Orchestrator Agent                     │
│  10 Specialist Agents with Real K8S API Tools                │
│  (Architect│Backend│Frontend│DBA│DevOps│QA│Security│         │
│   Reviewer│Kubernetes│GitOps)                                │
└──────────────────┬──────────────────────────────────────────┘
                   │ HTTP API
┌──────────────────▼──────────────────────────────────────────┐
│              Go REST API Backend (Clean Architecture)        │
│  Adapter → Usecase → Domain ← Infrastructure                │
│  (Handlers) (Services) (Entities) (PostgreSQL/Redis/NATS)   │
└──────────────────┬──────────────────────────────────────────┘
                   │ WebSocket + REST
┌──────────────────▼──────────────────────────────────────────┐
│              Frontend Dashboard (HTML5/CSS3/JS)              │
│  Real-time Monitoring │ Drift Viewer │ Audit Logs            │
└──────────────────────────────────────────────────────────────┘
```

### Clean Architecture Layers

```
├── cmd/                    # Application entrypoints
│   ├── server/             # Main API server
│   ├── agent-runner/       # Autonomous agent runner
│   └── standalone/         # Combined server + agent runner
├── internal/
│   ├── domain/             # Pure business models, port interfaces (DDD)
│   ├── usecase/            # Application business logic
│   ├── adapter/            # HTTP handlers, routers, event listeners
│   └── infrastructure/     # PostgreSQL, Redis, NATS, K8s/Swarm clients
├── orchestrator-agent/     # ADK multi-agent system (Python)
│   ├── app/agent.py        # 10-agent orchestrator with real API tools
│   ├── app/mcp_server.py   # MCP server for K8S platform tools
│   └── tests/              # Unit, integration, and eval tests
├── frontend/               # Premium dashboard UI
├── migrations/             # 26 timestamped SQL migration scripts
├── deployments/            # Docker, Helm, K8s manifests
└── .agents/skills/         # 17 agent skill definitions
```

---

## Tech Stack

| Component | Technology |
|-----------|-----------|
| **Backend** | Go 1.26, Clean Architecture, DDD |
| **AI/Agent** | Google ADK (Agent Development Kit), Gemini 2.0 Flash |
| **Database** | PostgreSQL 16 (26 migrations) |
| **Cache** | Redis 7 |
| **Messaging** | NATS JetStream |
| **Frontend** | HTML5, CSS3, Vanilla JavaScript, WebSocket |
| **Security** | AES-256-GCM, JWT, RBAC, Parameter-bound SQL (with custom tenant isolation SQL parser) |
| **CI/CD** | GitHub Actions (lint → test → build) |
| **Container** | Docker, Helm Charts |
| **Orchestration** | Kubernetes, Docker Swarm |

---

## Quick Start

### Prerequisites

- Go 1.26+
- Python 3.12+ with [uv](https://docs.astral.sh/uv/)
- Docker & Docker Compose
- PostgreSQL 16, Redis 7 (or use Docker Compose)
- [agents-cli](https://pypi.org/project/google-agents-cli/): `uv tool install google-agents-cli`

### 1. Start Infrastructure

```bash
# Start PostgreSQL, Redis, NATS via Docker Compose
make docker-up

# Run database migrations (26 scripts)
make migrate
```

### 2. Start the Go Backend

```bash
# Build and run the API server
make run
# Server starts at http://localhost:8080
```

### 3. Start the ADK Agent

```bash
cd orchestrator-agent

# Install dependencies
agents-cli install

# Launch the agent playground (interactive UI)
agents-cli playground
# Agent playground starts at http://localhost:8000
```

### 4. Run Tests

```bash
# Go backend tests
make test

# Python agent tests
cd orchestrator-agent
uv run pytest tests/unit tests/integration -v
```

---

## Agent System

The orchestrator agent routes user requests to 10 specialist sub-agents (Note: The Go domain model defines 15 agent types):

| Agent | Responsibility | Tools |
|-------|---------------|-------|
| `architect_agent` | Architecture validation, DDD enforcement | — |
| `backend_agent` | Go services, API handlers, business logic | — |
| `frontend_agent` | Dashboard UI, CSS, responsive layouts | — |
| `dba_agent` | Schema migrations, query optimization | — |
| `devops_agent` | Container orchestration, CI/CD, health monitoring | `check_system_health`, `list_cluster_resources`, `get_capacity_forecast` |
| `qa_agent` | Test suites, build verification, quality gates | — |
| `security_agent` | Security audits, encryption, RBAC | — |
| `reviewer_agent` | Code review, quality gate enforcement | — |
| `kubernetes_agent` | K8s manifests, Helm charts, cluster config | `list_cluster_resources`, `get_capacity_forecast` |
| `gitops_agent` | Git workflows, drift detection, CD reconciliation | `get_drift_status` |

---

## Platform Features

- **Resource Explorer**: Browse Pods, Services, Deployments, Nodes across clusters
- **Incident Center**: AI-powered root cause analysis with severity classification
- **Drift Detection**: Visual diff between Git baseline and live cluster state
- **Capacity Planning**: CPU/memory utilization forecasts and scaling recommendations
- **Change Management**: Deployment promotion pipelines across environments
- **Runbook Automation**: Executable operational procedures
- **Fleet View**: Multi-cluster overview with aggregated health metrics
- **Audit Trail**: Complete action logging (who, what, when, result)
- **Notification Center**: Alert routing and escalation policies
- **SLO Monitoring**: Service level objective tracking and burn rate analysis

---

## Course Concepts Demonstrated

| # | Concept | Evidence |
|---|---------|----------|
| 1 | **Agent / Multi-Agent (ADK)** | `orchestrator-agent/app/agent.py` — 10 agents, routing table, real API tools |
| 2 | **MCP Server** | `orchestrator-agent/app/mcp_server.py` + `google-developer-knowledge` + `gopls-mcp-server` |
| 3 | **Antigravity IDE** | Entire 36,000+ LOC codebase vibe-coded with Antigravity |
| 4 | **Security** | AES-256-GCM encryption, JWT, RBAC, parameter-bound SQL |
| 5 | **Deployability** | Dockerfile, Helm charts, GitHub Actions CI/CD |
| 6 | **Agent Skills** | `agents-cli playground/eval/lint`, 17 skill definitions, eval datasets |

---

## License

This project is licensed under the [Creative Commons Attribution 4.0 International License](https://creativecommons.org/licenses/by/4.0/).
