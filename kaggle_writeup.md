# K8S Self-Healing Agent: AI-Powered Multi-Agent Control Plane for Kubernetes Operations

**Track: Agents for Business**

---

## 1. Problem Definition

Managing Kubernetes and Docker Swarm clusters at production scale is inherently complex. Site Reliability Engineers (SREs) face three critical operational challenges:

**High Mean Time to Resolution (MTTR)**: When pods crash with `CrashLoopBackOff` or `OOMKilled`, engineers manually correlate logs across distributed namespaces, inspect pod YAML diffs, and check resource quota violations. This reactive process typically takes 15–45 minutes per incident.

**Configuration Drift**: Running cluster configurations silently diverge from the Git repository baseline after manual `kubectl edit` or emergency hotfixes. Without continuous reconciliation, these drifts accumulate until they cause cascading failures during deployments.

**Cognitive Overload**: Modern platform teams manage hundreds of microservices across multiple clusters. Simultaneously monitoring health, analyzing capacity trends, enforcing security policies, and maintaining code quality exceeds human bandwidth — especially during incident response when focus matters most.

These are not theoretical problems. According to the DORA State of DevOps reports, elite teams achieve MTTR under one hour, while low performers average over one week. The gap is tooling and automation.

---

## 2. The Solution: Why Agents?

Static monitoring dashboards and rule-based alerting are insufficient for modern infrastructure. They generate alerts but cannot reason about root causes, correlate signals, or take corrective action autonomously.

The **K8S Self-Healing Agent** replaces passive monitoring with an **autonomous multi-agent system** built on Google's **Agent Development Kit (ADK)**. The system introduces:

- **Autonomous Reasoning**: Instead of hardcoded rules, the agent uses LLM-powered analysis (Gemini) to correlate container logs, resource metrics, and configuration state to determine root causes.
- **Multi-Agent Specialization**: Ten specialist agents — each with domain-specific expertise (Architecture, Backend, Frontend, DBA, DevOps, QA, Security, Code Review, Kubernetes, GitOps) — collaborate under an orchestrator that delegates tasks based on intent classification.
- **Real-Time Remediation**: The agent doesn't just alert — it generates remediation plans, creates Git commits, triggers scaling actions, and continuously reconciles cluster state against the GitOps baseline.

This architecture transforms DevOps from a reactive discipline into a proactive, self-healing system.

---

## 3. Agent Architecture

The system follows **Clean Architecture** and **Domain-Driven Design (DDD)** with strict layer boundaries:

```
┌─────────────────────────────────────────────────────────────┐
│                    ADK Orchestrator Agent                     │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐        │
│  │Architect │ │ Backend  │ │Frontend  │ │   DBA    │        │
│  │  Agent   │ │  Agent   │ │  Agent   │ │  Agent   │        │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘        │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐        │
│  │  DevOps  │ │    QA    │ │Security  │ │ Reviewer │        │
│  │  Agent   │ │  Agent   │ │  Agent   │ │  Agent   │        │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘        │
│  ┌──────────┐ ┌──────────┐                                   │
│  │   K8s    │ │  GitOps  │   ← Real API Tools                │
│  │  Agent   │ │  Agent   │   (health, resources, drift)      │
│  └──────────┘ └──────────┘                                   │
└──────────────────┬──────────────────────────────────────────┘
                   │ HTTP API Calls
┌──────────────────▼──────────────────────────────────────────┐
│              Go REST API Backend Server                       │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────┐      │
│  │  Adapter    │  │   Usecase    │  │    Domain      │      │
│  │ (Handlers)  │→ │ (Services)   │→ │ (Entities)     │      │
│  └─────────────┘  └──────────────┘  └────────────────┘      │
│         │                                    ▲               │
│  ┌──────▼────────────────────────────────────┴────────┐     │
│  │            Infrastructure Layer                      │     │
│  │  PostgreSQL │ Redis │ NATS │ K8s/Swarm API Client   │     │
│  └──────────────────────────────────────────────────────┘     │
└──────────────────────────────────────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────────────────┐
│              Frontend Dashboard (HTML5/CSS3/JS)              │
│  Real-time WebSocket │ Dark Mode │ Drift Viewer │ Audit Log │
└──────────────────────────────────────────────────────────────┘
```

### Key Technical Decisions

- **Go Backend** (166 source files, 23 database migrations): Chosen for performance, strong typing, and native Kubernetes client-go support.
- **ADK Python Agent** (10 specialist sub-agents): Google's Agent Development Kit provides the multi-agent orchestration framework with built-in tool registration and intent routing.
- **PostgreSQL + Redis + NATS JetStream**: Production-grade data layer with encrypted credential storage, caching, and event-driven messaging.
- **AES-256-GCM Encryption**: All cluster credentials are encrypted at rest in PostgreSQL and decrypted on-the-fly during API calls.

---

## 4. Applied Course Concepts

### Concept 1: Agent / Multi-Agent System (ADK)

The orchestrator agent (`app/agent.py`) implements a **10-agent multi-agent system** using Google ADK:

```python
root_agent = Agent(
    name="root_agent",
    model=Gemini(model="gemini-2.0-flash"),
    instruction=ORCHESTRATOR_INSTRUCTION,  # Routing table + keyword rules
    sub_agents=[
        architect_agent, backend_agent, frontend_agent, dba_agent,
        devops_agent, qa_agent, security_agent, reviewer_agent,
        kubernetes_agent, gitops_agent,
    ],
)
```

Each agent loads specialized instructions from `.agents/skills/<name>/SKILL.md`. The DevOps, Kubernetes, and GitOps agents have real API tools that query the backend for live cluster data.

### Concept 2: MCP Server

The project integrates MCP (Model Context Protocol) servers for developer knowledge and Go language intelligence:
- **`google-developer-knowledge`**: Provides semantic search over Google developer documentation.
- **`gopls-mcp-server`**: Exposes Go language server features (diagnostics, symbol search, references, rename) as MCP tools.
- **Custom MCP Server** (`app/mcp_server.py`): Exposes K8S platform tools (health checks, resource listing, drift detection) as MCP resources.

### Concept 3: Antigravity IDE

The entire 50,000+ line codebase was developed interactively using the **Antigravity AI Coding Assistant**. Key workflows leveraged:
- **Planning Protocol**: `implementation_plan.md` artifacts for large tasks requiring architectural review.
- **Task Tracking**: `task.md` checklists updated in real-time during implementation.
- **Browser Verification**: Automated browser subagent for UI testing and visual validation.
- **Skill System**: 17 custom `.agents/skills/` definitions (Architect, Backend, Frontend, DBA, DevOps, QA, Security, etc.) loaded by both Antigravity and the ADK agent.

### Concept 4: Security Features

- **AES-256-GCM Encryption**: Cluster credentials (kubeconfig, Docker socket paths, access tokens) are encrypted inside PostgreSQL using dynamic key management. Decryption occurs in-memory during API calls via `internal/infrastructure/cluster/manager.go`.
- **Parameter-Bound SQL**: All 23 migration files and database queries use strictly `$1, $2` parameter binding — zero string concatenation.
- **JWT Authentication**: API routes are protected with secure JWT validation.
- **Input Sanitization**: All client inputs are validated and sanitized against XSS.
- **RBAC**: Role-based access control enforces tenant isolation.

### Concept 5: Deployability

- **Multi-stage Dockerfile**: Both the Go backend (`deployments/docker/Dockerfile`) and ADK agent (`orchestrator-agent/Dockerfile`) have production-ready container images.
- **Helm Charts**: Full Kubernetes deployment manifests in `deployments/helm/`.
- **GitHub Actions CI/CD**: Automated lint → test → build pipeline (`.github/workflows/ci.yml`) with PostgreSQL and Redis service containers.
- **Docker Compose**: One-command local infrastructure setup with PostgreSQL, Redis, and NATS.

### Concept 6: Agent Skills (Agents CLI)

- **`agents-cli playground`**: Local development server for interactive agent testing.
- **`agents-cli eval`**: Automated evaluation with routing datasets (`tests/eval/datasets/routing-dataset.json`).
- **Unit Test Suite**: 14 unit tests covering all 10 agents, 5 API tools, error handling, and routing verification.
- **Integration Tests**: End-to-end agent streaming tests with `InMemorySessionService`.

---

## 5. The Build Journey

This project was built entirely through **vibe coding** — conversational, AI-assisted development using Antigravity IDE over multiple intensive sessions:

1. **Backend Foundation**: Established Clean Architecture with Go, PostgreSQL connection pools, Redis caching, and NATS messaging.
2. **Feature Development**: Built 20+ production features — Resource Explorer, Drift Detection, Capacity Planning, Incident RCA, Change Management, Fleet View, Runbooks, and Automation Engine.
3. **Frontend Dashboard**: Created a premium dark-mode dashboard with real-time WebSocket updates, glassmorphism design, and responsive layouts.
4. **ADK Integration**: Scaffolded the orchestrator agent with `agents-cli`, expanded from a simple ReAct agent to a 10-agent multi-agent system with real operational tools.
5. **Testing & Verification**: Comprehensive test suites for both Go and Python, automated CI/CD pipeline, and browser-verified UI flows.

---

## 6. User Value & Impact

For **SRE/DevOps teams** managing Kubernetes at scale, this agent delivers:

- **Reduced MTTR**: AI-powered RCA identifies root causes in seconds instead of minutes.
- **Eliminated Drift**: Continuous GitOps reconciliation ensures cluster state matches the repository baseline.
- **Proactive Capacity Planning**: Forecast resource utilization before saturation causes outages.
- **Audit Compliance**: Every action is logged with who, what, when, and result for compliance.
- **Developer Productivity**: Natural language interface replaces manual `kubectl` commands and YAML editing.

---

## Project Links

- **GitHub Repository**: [https://github.com/tiendat1751998/k8s-selfhost-agent](https://github.com/tiendat1751998/k8s-selfhost-agent)
- **Demo Video**: [YouTube Link — Coming Soon]
