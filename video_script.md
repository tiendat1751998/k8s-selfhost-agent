# YouTube Demo Video Script (≤ 5 minutes)

## Video Title
**K8S Self-Healing Agent: AI-Powered Multi-Agent Kubernetes Control Plane**

## Video Description
Capstone project for the AI Agents Intensive Vibe Coding course. An autonomous multi-agent system built on Google ADK that monitors Kubernetes clusters, performs AI-powered root cause analysis, detects GitOps configuration drift, and auto-remediates failures.

---

## Script & Timing

### Intro (0:00 - 0:30) — Problem Statement
**Show**: Title card with cover image

**Say**: "Managing Kubernetes clusters at scale is one of the hardest problems in DevOps. When pods crash with CrashLoopBackOff or OOMKilled, engineers spend 15 to 45 minutes correlating logs, checking config diffs, and identifying root causes. Meanwhile, configuration drift accumulates silently after emergency hotfixes. The K8S Self-Healing Agent solves this with AI."

---

### Why Agents? (0:30 - 1:00)
**Show**: Architecture diagram from README

**Say**: "Instead of static monitoring dashboards, we built an autonomous multi-agent system on Google's Agent Development Kit. Ten specialist agents — each with domain expertise — collaborate under a central orchestrator. The orchestrator analyzes user intent and routes requests to the right specialist. The agents have real tools that query the live cluster API — not mock data."

---

### Architecture Deep Dive (1:00 - 2:00)
**Show**: Code editor with `agent.py` open, scroll through the 10 agents

**Say**: "The system has three layers. First, the ADK Orchestrator — 10 Python agents with a routing table. The DevOps agent has tools like check_system_health and list_cluster_resources. The GitOps agent has get_drift_status. Second, a Go backend with 166 source files following Clean Architecture — handlers, usecases, domain entities, and infrastructure adapters for PostgreSQL, Redis, and NATS. Third, a premium dark-mode dashboard with real-time WebSocket updates."

**Show**: Scroll through migrations folder (23 files)

**Say**: "23 database migrations cover everything from incidents and drift detection to capacity planning and RBAC."

---

### Live Demo (2:00 - 4:00)
**Show**: Start the Go backend server, then the ADK playground

1. **Dashboard Tour** (30s): Navigate through the dashboard — show Health Center, Resource Explorer, Drift Detection, Incident Center, Audit Logs
2. **Agent Playground** (30s): Open `agents-cli playground`, type "Check the health of all platform components", show the orchestrator routing to DevOps agent
3. **Drift Detection** (30s): Show the Drift Center with visual diffs
4. **Capacity Planning** (30s): Show capacity forecasts and resource utilization charts

---

### The Build & Course Concepts (4:00 - 4:45)
**Show**: Antigravity IDE session, .agents/skills/ folder, test results

**Say**: "The entire 36,000-line codebase was vibe-coded with Antigravity IDE. We demonstrate all six course concepts: ADK multi-agent system with 10 specialist agents, MCP server integration for developer knowledge, Antigravity for AI-assisted development, AES-256-GCM encryption and JWT security, Docker and Helm deployability with GitHub Actions CI/CD, and agents-cli for playground testing and evaluation."

**Show**: Run unit tests (14 passing)

---

### Closing (4:45 - 5:00)
**Show**: GitHub repository page

**Say**: "The K8S Self-Healing Agent transforms DevOps from reactive monitoring to proactive self-healing. Check out the full source code on GitHub. Thanks for watching!"

---

## Recording Checklist

Before recording, make sure:

- [ ] Go backend server is running at `localhost:8080`
- [ ] PostgreSQL, Redis are running (via Docker Compose)
- [ ] ADK agent playground is running (`agents-cli playground`)
- [ ] Frontend dashboard is accessible
- [ ] Screen recording software is set to 1080p
- [ ] Microphone is tested
- [ ] Browser zoom is at 100% for clear text
- [ ] Terminal font size is increased for readability

## Upload Settings

- **YouTube Privacy**: Public (or Unlisted if preferred)
- **Length**: ≤ 5 minutes
- **Resolution**: 1080p recommended
- **Tags**: `kubernetes`, `ai-agents`, `google-adk`, `devops`, `self-healing`, `kaggle`
