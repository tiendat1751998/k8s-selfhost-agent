# AGENTS.md — k8s-selfhost-agent Constitution

> Binding project constitution for `k8sseflhost`. Applies to all agents and subagents.

## 1. Operational & Deployment Reality (Physical Anchors)
- **No Automated CI/CD**: Editing Go code locally does NOT auto-deploy to remote nodes.
- **Canonical Agent Rollout (`LES-006`)**: ALWAYS use [`./deploy-agent.sh`](./deploy-agent.sh) (`./deploy-agent.sh <user>@<ip>` or `./deploy-agent.sh` locally on Linux). It builds static `linux/amd64` `./k8s-agent`, transfers via `/tmp`, cleans legacy part floods, preserves remote `~/.k8s-agent/agent.env`, and restarts `systemctl --user restart k8s-agent`. Never invent ad-hoc `scp`/`ssh` scripts.
- **Ports & Services**: Backend on `:8080` (`cmd/server` or `cmd/standalone`), Frontend Vite UI on `:3000`, Node Agent on `:9100` (`~/.k8s-agent/logengine` ClickHouse `MergeTree`).
- **CodeGraph First (`LES-005`)**: Call `codegraph_explore` (`ServerName: "codegraph"`, `projectPath: "d:\\project\\k8sseflhost"`) or shell `codegraph explore` BEFORE grep/find/reading files.

## 2. Orchestrator & Subagent Invariants
- **Main Agent is Permanent Orchestrator**: The main thread plans, dispatches, and aggregates. Never code directly in the main thread.
- **Strict Role Separation**:
  - Implementation ONLY: `backend-coder`, `frontend-coder`, `database-engineer`, `devops` (`Workspace: "branch"`, `Model: "pro"`).
  - Auditing & Verification (Read-Only): `reviewer`, `qa-test-engineer`, `governor`, `architect`.
- **Anti-Bureaucracy Execution**: Use Fast-Track (`coder -> reviewer`) for small changes (< 50 lines). Avoid bloated multi-agent chains.
- **Protected Master**: NEVER push directly to `master`. Always work on `feat/<name>` or `fix/<name>`. On merge, prune worktrees and delete feature branches immediately.

## 3. Hard Architectural & Quality Invariants
- **Strict `< 500 lines/file`**: Across `.go`, `.vue`, `.ts`, and `.css`. Orchestrator views `<= 350 lines`, components `< 300 lines`.
- **Go 4-Layer Clean Architecture**: `domain` → `usecase` → `adapter/http` → `infrastructure`. Parameterized SQL only, tenant isolation, zero stubs/mocks/`_ = err`.
- **Dual-Theme Design System (`tokens.css`)**: Zero hardcoded neon hex or `rgba(15, 23, 42, ...)`. Max 2 inline table actions + `[ ⋯ ]` menu. Never hide charts on mobile (`display: none` forbidden).
- **Real E2E MCP QA (`LES-001`, `LES-002`, `LES-003`)**: `qa-test-engineer` tests against the real Go backend (:8080) in Desktop (1440x900), Tablet (768x1024), Mobile (375x812) across Dark & Light modes, and always resets viewport to 1440x900 (`isMobile: false`). Zero rogue mock servers.

## 4. Scoped Rules & Memory Index
- Full rules reside in `.agents/rules/`:
  - [`01-code-and-architecture.md`](.agents/rules/01-code-and-architecture.md)
  - [`02-pipeline-and-quality-gates.md`](.agents/rules/02-pipeline-and-quality-gates.md)
  - [`03-ops-and-deploy-reality.md`](.agents/rules/03-ops-and-deploy-reality.md)
- Memory: Read active [`project_state.md`](.agents/memory/project_state.md), [`decision_log.jsonl`](.agents/memory/decision_log.jsonl), and [`lessons.jsonl`](.agents/memory/lessons.jsonl).
- **Archive Isolation**: NEVER load `.agents/memory/archive/` during boot or routine tasks; it is reserved strictly for deep historical forensics.
