# YouTube Demo Video Script (≤ 5 minutes)

## Video Title
**K8S Self-Healing Agent: The Complete Autonomous Control Plane for DevOps & SRE**

## Video Description
Capstone project for the AI Agents Intensive Vibe Coding course. An enterprise-grade, multi-tenant autonomous SRE control plane built on Google ADK and Go Clean Architecture. It solves the entire DevOps lifecycle: continuous monitoring, AI-powered root-cause analysis (RCA), GitOps configuration drift remediation, change management, runbook automation, SLO tracking, and compliance auditing.

---

## Script & Timing

### Intro (0:00 - 0:40) — The Complete DevOps Challenge
**Show**: Title card with cover image

**Say**: "Modern platform engineering teams face a mountain of operational challenges: troubleshooting transient pod failures, resolving configuration drift, coordinating service promotions across dev and production environments, tracking SLO burn rates, executing runbooks, and ensuring security compliance. 

The **K8S Self-Healing Agent** is not just an alerting tool — it is a complete, autonomous SRE Control Plane designed to solve the full lifecycle of DevOps problems."

---

### Why Agents? & Tech Stack (0:40 - 1:20)
**Show**: Architecture diagram from README

**Say**: "Instead of static dashboards, we built a collaborative multi-agent system on Google's Agent Development Kit. 10 specialist agents — including dedicated DevOps, GitOps, DBA, Kubernetes, Security, and Code Reviewer agents — coordinate under a central orchestrator. 

Our agents are backed by a production-grade Go backend designed with Clean Architecture, using PostgreSQL for state, Redis for caching, NATS JetStream for event-driven message queuing, and AES-256-GCM for in-memory decryption of cluster credentials."

---

### Key Capabilities & Live Demo (1:20 - 4:00)
**Show**: Open the Frontend Dashboard, tour the tabs:

**Say**: "Let's tour the control plane in action.

1. **Resource Explorer & Fleet View** (1:20 - 1:50): Here we can inspect live Pods, Services, and Deployments across both Kubernetes and Docker Swarm clusters in a single multi-tenant pane.
2. **AI RCA & Incident Center** (1:50 - 2:30): When an incident occurs — like an OOMKill or scheduling failure — the system retrieves container logs, correlates events, and runs AI-powered root-cause analysis with confidence scoring, suggesting immediate fixes.
3. **GitOps Drift Control** (2:30 - 3:00): The Drift Center continuously compares live cluster configuration against your Git baseline, showing visual diffs and providing auto-reconciliation to bring services back in sync.
4. **Change Management & Promotion** (3:00 - 3:30): Deployments are managed transactionally, allowing automated, secure promotion of configurations from dev to staging and production.
5. **Runbook Center & Playbook Automation** (3:30 - 4:00): Common operations are documented as executable runbooks, which the agent can trigger automatically to resolve recurring issues."

---

### Course Concepts & Engineering Polish (4:00 - 4:45)
**Show**: Terminal running `uv run pytest` (14 unit tests passing) and code files in Antigravity IDE

**Say**: "Developed using the Antigravity IDE, this project demonstrates all six course concepts:
- **ADK Multi-Agent System**: 10 specialist Python agents with dedicated skill files.
- **MCP Server**: Integrating developer knowledge and language server diagnostic tools.
- **Security**: Strict parameter-bound SQL (23 database migrations) and JWT auth.
- **Deployability**: Multi-stage Dockerfiles and Helm charts ready for production GKE or Cloud Run."

---

### Closing (4:45 - 5:00)
**Show**: GitHub repository page: `github.com/tiendat1751998/k8s-selfhost-agent`

**Say**: "By automating the entire SRE loop — from detection to GitOps remediation — this platform drastically reduces MTTR and operational overhead. The full source code is public on GitHub. Thanks for watching!"

---

## Recording Checklist

Before recording, make sure:

- [ ] Go backend server is running at `localhost:8080`
- [ ] PostgreSQL, Redis, NATS are active in Docker
- [ ] ADK agent playground is running (`agents-cli playground`)
- [ ] Frontend dashboard is open on the Resource Explorer tab
- [ ] Screen recording software is set to 1080p
- [ ] Microphone volume and clarity are tested
- [ ] Browser zoom is adjusted for optimal legibility

## Upload Settings

- **YouTube Privacy**: Public (or Unlisted if preferred)
- **Length**: ≤ 5 minutes
- **Resolution**: 1080p recommended
- **Tags**: `kubernetes`, `ai-agents`, `google-adk`, `devops`, `sre`, `self-healing`, `gitops`, `kaggle`
