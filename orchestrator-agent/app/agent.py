# ruff: noqa
# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import json
import os
import urllib.request
import urllib.error

from google.adk.agents import Agent
from google.adk.apps import App
from google.adk.models import Gemini
from google.genai import types

import google.auth

if os.getenv("GOOGLE_GENAI_USE_VERTEXAI") == "True":
    try:
        _, project_id = google.auth.default()
        os.environ["GOOGLE_CLOUD_PROJECT"] = project_id
        os.environ["GOOGLE_CLOUD_LOCATION"] = "global"
    except Exception:
        pass
else:
    os.environ["GOOGLE_GENAI_USE_VERTEXAI"] = "False"


BACKEND_BASE_URL = os.getenv("K8S_BACKEND_URL", "http://localhost:8080")


def load_skill_instruction(skill_name: str, fallback_desc: str) -> str:
    """Load agent instruction from the .agents/skills/<name>/SKILL.md file.

    Strips YAML frontmatter (delimited by ---) and returns the markdown body.
    Falls back to fallback_desc if the file is missing or unreadable.
    """
    try:
        base_dir = os.path.dirname(
            os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        )
        skill_path = os.path.join(base_dir, ".agents", "skills", skill_name, "SKILL.md")
        if os.path.exists(skill_path):
            with open(skill_path, "r", encoding="utf-8") as f:
                content = f.read()
                if content.startswith("---"):
                    parts = content.split("---", 2)
                    if len(parts) >= 3:
                        content = parts[2].strip()
                return content
    except Exception:
        pass
    return fallback_desc


def _backend_get(path: str) -> dict:
    """Execute a GET request against the K8S Control Plane backend API."""
    url = f"{BACKEND_BASE_URL}{path}"
    token = os.getenv("BACKEND_API_TOKEN")
    if not token:
        raise ValueError("Missing required environment variable: BACKEND_API_TOKEN")
    req = urllib.request.Request(
        url,
        headers={
            "Accept": "application/json",
            "Authorization": f"Bearer {token}",
        },
    )
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.URLError as exc:
        return {"error": f"Backend unreachable at {url}: {exc.reason}"}
    except Exception as exc:
        return {"error": f"Request to {url} failed: {exc}"}


def check_system_health(query: str) -> str:
    """Check the real-time health status of all platform components.

    Queries the K8S Control Plane backend to retrieve live health statuses
    for PostgreSQL, Docker Swarm, Kubernetes API, Redis, and WebSocket.

    Args:
        query: A descriptive query about which components to check.

    Returns:
        A JSON string with the health status of each component.
    """
    data = _backend_get("/api/v1/health")
    return json.dumps(data, indent=2)


def list_cluster_resources(kind: str) -> str:
    """List Kubernetes or Docker Swarm resources from the active cluster.

    Retrieves live resources (Pods, Services, Deployments, Nodes, ReplicaSets)
    from the connected cluster provider via the backend API.

    Args:
        kind: The resource kind to list. One of: pod, service, deployment, node, replicaset.

    Returns:
        A JSON string with the list of resources and their statuses.
    """
    data = _backend_get(f"/api/v1/explorer?kind={kind}")
    return json.dumps(data, indent=2)


def get_drift_status(query: str) -> str:
    """Check GitOps configuration drift between the Git baseline and live cluster state.

    Compares the running cluster configuration against the Git repository baseline
    to detect services that have drifted from their declared state.

    Args:
        query: A descriptive query about which resources or namespaces to check for drift.

    Returns:
        A JSON string with drift detection results including drifted resources and diff details.
    """
    data = _backend_get("/api/v1/drift")
    return json.dumps(data, indent=2)


def list_incidents(query: str) -> str:
    """List active incidents and their AI-powered root cause analysis.

    Retrieves incident records from the platform, including severity levels,
    affected resources, and automated RCA (Root Cause Analysis) from the AI engine.

    Args:
        query: A descriptive query to filter incidents by severity or resource.

    Returns:
        A JSON string with incident records and RCA analysis results.
    """
    data = _backend_get("/api/v1/incidents")
    return json.dumps(data, indent=2)


def get_capacity_forecast(query: str) -> str:
    """Get cluster capacity utilization and resource forecasting data.

    Retrieves real-time CPU, memory, and storage utilization metrics along
    with predictive forecasts for capacity planning.

    Args:
        query: A descriptive query about which cluster or resource pool to analyze.

    Returns:
        A JSON string with capacity metrics and forecast data.
    """
    data = _backend_get("/api/v1/capacity")
    return json.dumps(data, indent=2)


# ---------------------------------------------------------------------------
# Model Configuration
# ---------------------------------------------------------------------------

model = Gemini(
    model="gemini-3.1-flash-lite",
    retry_options=types.HttpRetryOptions(attempts=3),
)


# ---------------------------------------------------------------------------
# Specialist Sub-Agents (10 agents, each with dedicated skill instructions)
# ---------------------------------------------------------------------------

# 1. Architect Agent — validates Clean Architecture, DDD, module boundaries
architect_agent = Agent(
    name="architect_agent",
    model=model,
    instruction=load_skill_instruction(
        "architect",
        "You are the Architect Agent. Validate system architecture, enforce Clean Architecture boundaries, modularity, and DDD principles. Never write business features.",
    ),
    description="Validates software architecture, enforces Clean Architecture boundaries, modularity, and DDD principles. Use for architecture reviews, dependency analysis, and layer violation detection.",
)

# 2. Backend Agent — Go services, handlers, business logic, API endpoints
backend_agent = Agent(
    name="backend_agent",
    model=model,
    instruction=load_skill_instruction(
        "backend",
        "You are the Backend Engineer. Build production-grade Go services, database repositories, business logic adapters, and REST API handlers. Never modify frontend code.",
    ),
    description="Develops production-grade Go backend services, database structures, business logic, API handlers, and transaction coordinators. Use for any Go/API/backend/endpoint/service/CRUD tasks.",
)

# 3. Frontend Agent — HTML, CSS, JavaScript, UI dashboards
frontend_agent = Agent(
    name="frontend_agent",
    model=model,
    instruction=load_skill_instruction(
        "frontend",
        "You are the Frontend Engineer. Build beautiful, premium, responsive UI dashboards and components using HTML, CSS, and vanilla JavaScript. Never modify Go backend code.",
    ),
    description="Develops premium dashboard views, custom layouts, CSS styling, and interactive frontend elements. Use for any React/Next.js/UI/frontend/CSS/HTML tasks.",
)

# 4. DBA Agent — database schemas, SQL migrations, query optimization
dba_agent = Agent(
    name="dba_agent",
    model=model,
    instruction=load_skill_instruction(
        "dba",
        "You are the DBA Agent. Manage database schemas, write timestamped SQL migration scripts, optimize query performance, and enforce parameter-bound SQL security. Never use fmt.Sprintf for queries.",
    ),
    description="Manages database schema migrations, query performance optimizations, connection pooling, and parameter-bound SQL security. Use for any PostgreSQL/MySQL/SQL/database/schema/migration/query tasks.",
)

# 5. DevOps Agent — containers, K8s/Swarm deployments, CI/CD, health monitoring
devops_agent = Agent(
    name="devops_agent",
    model=model,
    instruction=load_skill_instruction(
        "devops",
        "You are the DevOps Engineer. Manage container orchestration, Kubernetes and Docker Swarm deployments, CI/CD pipelines, and platform health monitoring.",
    ),
    description="Manages container configurations, Kubernetes/Swarm deployment setups, CI/CD integrations, infrastructure scaling, and health metrics monitoring. Use for any Docker/K8s/deploy/infrastructure/container tasks.",
    tools=[check_system_health, list_cluster_resources, get_capacity_forecast, list_incidents],
)

# 6. QA Agent — test suites, build verification, quality gates
qa_agent = Agent(
    name="qa_agent",
    model=model,
    instruction=load_skill_instruction(
        "qa",
        "You are the QA Engineer. Write unit and integration tests, validate compilations, execute test suites, and enforce quality gates. Never implement business logic.",
    ),
    description="Develops test suites, runs unit and integration verifications, and validates builds. Use for any test/unit test/integration/QA/coverage tasks.",
)

# 7. Security Agent — security audits, secrets management, encryption
security_agent = Agent(
    name="security_agent",
    model=model,
    instruction=load_skill_instruction(
        "security",
        "You are the Security Engineer. Audit security controls, review secrets management, enforce input sanitization, validate encryption usage (AES-256), and ensure RBAC permissions.",
    ),
    description="Audits security controls, secrets management, input sanitization, and encryption. Use for any security/vulnerability/scan/OWASP/auth tasks.",
)

# 8. Code Reviewer Agent — code quality audits, quality gate enforcement
reviewer_agent = Agent(
    name="reviewer_agent",
    model=model,
    instruction=load_skill_instruction(
        "reviewer",
        "You are the Code Reviewer. Audit all proposed code changes against quality gate rules. Reject duplicate code, dead code, placeholders, oversized files, and architectural layer violations.",
    ),
    description="Audits code changes against quality gate rules and architectural layers. Use for any PR/code review/pull request tasks.",
)

# 9. Kubernetes Agent — K8s manifests, Helm charts, CRDs, cluster permissions
kubernetes_agent = Agent(
    name="kubernetes_agent",
    model=model,
    instruction=load_skill_instruction(
        "kubernetes",
        "You are the Kubernetes Engineer. Manage Kubernetes resource manifests, Helm charts, CRDs, deployment configs, network policies, and cluster setups. Use typed client-go structures.",
    ),
    description="Manages Kubernetes resources, manifests, Helm charts, CRDs, and cluster permissions. Use for any Kubernetes-specific manifest, scaling, or cluster configuration tasks.",
    tools=[list_cluster_resources, get_capacity_forecast],
)

# 10. GitOps Agent — Git workflows, drift detection, CD reconciliation
gitops_agent = Agent(
    name="gitops_agent",
    model=model,
    instruction=load_skill_instruction(
        "gitops",
        "You are the GitOps Engineer. Manage Git workflows, repository syncing, PR generation, CD reconciliation loops, and configuration drift detection.",
    ),
    description="Manages Git workflows, repo syncing, PR generation, and CD reconciliations. Use for any GitOps/drift/ArgoCD/FluxCD/Git branch tasks.",
    tools=[get_drift_status],
)

# ---------------------------------------------------------------------------
# Root Agent: Orchestrator / Router (delegates all work, never executes tasks)
# ---------------------------------------------------------------------------

ORCHESTRATOR_INSTRUCTION = """You are the **Orchestrator** — the central coordinator of the K8S Self-Healing Agent multi-agent system.

## Your Role
You are the ONLY agent authorized to receive user requests and route them. You NEVER write code, review code, fix bugs, or execute tasks directly. Your sole responsibility is to:
1. **Analyze** the user's request to determine intent and task type.
2. **Delegate** to the correct specialist agent.
3. **Aggregate** and present the specialist's output back to the user.

## Agent Routing Table

| Task Type | Primary Agent | Secondary Agent |
|-----------|--------------|-----------------|
| Architecture / Design / DDD | architect_agent | — |
| Backend / API / Go / Handler | backend_agent | architect_agent (for design review) |
| Frontend / UI / CSS / HTML / JS | frontend_agent | — |
| Database / Schema / Migration / SQL | dba_agent | backend_agent |
| DevOps / Deploy / Docker / Container | devops_agent | kubernetes_agent |
| Kubernetes / Helm / CRD / Manifest | kubernetes_agent | devops_agent |
| GitOps / Drift / Git / CD / ArgoCD | gitops_agent | devops_agent |
| Security / Audit / RBAC / Encryption | security_agent | — |
| QA / Test / Coverage / Verification | qa_agent | — |
| Code Review / PR / Quality Gate | reviewer_agent | — |

## Keyword-Based Routing

| Keywords | Route To |
|----------|----------|
| PostgreSQL, SQL, database, schema, migration, query, index | dba_agent |
| Docker, container, deploy, infrastructure, CI/CD, pipeline | devops_agent |
| Kubernetes, K8s, Helm, CRD, manifest, pod, node, service | kubernetes_agent |
| Go, API, backend, endpoint, handler, CRUD, REST | backend_agent |
| HTML, CSS, JavaScript, UI, frontend, dashboard, layout | frontend_agent |
| test, unit test, integration, QA, coverage, verification | qa_agent |
| PR, code review, pull request, quality gate, audit | reviewer_agent |
| security, vulnerability, OWASP, auth, encryption, RBAC | security_agent |
| architecture, design pattern, DDD, Clean Architecture | architect_agent |
| GitOps, drift, ArgoCD, FluxCD, reconciliation, Git sync | gitops_agent |
| health, status, monitoring, metrics, capacity, forecast | devops_agent |
| incident, alert, failure, issue, outage | devops_agent |

## Rules
1. **NEVER** do work directly — always delegate to the appropriate specialist.
2. **NEVER** ask the user to clarify when the intent is obvious — decide autonomously.
3. If a request spans multiple domains, delegate to the primary agent first.
4. For general greetings or ambiguous requests, introduce yourself and list your capabilities.
"""

root_agent = Agent(
    name="root_agent",
    model=model,
    instruction=ORCHESTRATOR_INSTRUCTION,
    sub_agents=[
        architect_agent,
        backend_agent,
        frontend_agent,
        dba_agent,
        devops_agent,
        qa_agent,
        security_agent,
        reviewer_agent,
        kubernetes_agent,
        gitops_agent,
    ],
)

app = App(
    root_agent=root_agent,
    name="app",
)
