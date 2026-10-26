# 02 — Multi-Agent Pipeline & Quality Gates

> Binding execution loop, routing matrix, and quality verification gates.

## 1. Anti-Bureaucracy Routing (No Heavy Pipelines for Small Tasks)
Classify requests into the minimum necessary tier:
- **Tier 0 — Q&A / Discovery / Plan / Rules**: Direct answer or 1 read-only specialist (`research`, `architect`).
- **Tier 1 — Backend / DB / Agent / CLI / Docs**:
  - `backend-coder` / `database-engineer` / `devops` (`Workspace: "branch"`, `Model: "pro"`) → `reviewer` → Merge + Post-merge test.
  - *Fast-Track*: Changes `< 50 lines` of docs, configuration, or pure file splits skip extra hops.
- **Tier 2 — Frontend / Full-Stack UI**:
  - `frontend-coder` / `backend-coder` (`Workspace: "branch"`, `Model: "pro"`) → `reviewer` → `qa-test-engineer` (real `chrome-devtools-mcp`) → Merge.

## 2. Git Branching & Worktree Hygiene
- **Protected Master**: NEVER push directly to `master`. Work on `feat/<name>` or `fix/<name>`.
- **Atomic Worktree Cleanup**: Immediately after merge into `master`:
  ```powershell
  git branch -d <branch>
  git worktree prune
  ```
- Never merge rewritten history (`fetch.prune=true`).

## 3. Adversarial Review Gate (`reviewer`)
Before merge, `reviewer` audits diffs against:
1. Concurrency safety (data races, channel leaks, mutex contention).
2. Resource leaks (unclosed responses, goroutine leaks, uncancelled contexts).
3. Error wrapping (`%w`) and typed domain errors.
4. Modularity: strictly `< 500 lines/file` (views `<= 350`, components `< 300`).
5. Performance & allocations (N+1 queries, indexing, zero-alloc hot paths).
6. Security/RBAC (tenant isolation, input validation, no secrets).
7. Simplicity (`ponytail-review`): prune speculative bloat and redundant abstractions.
8. Feature & mobile parity (no `display: none` on charts, max 2 inline table actions).

## 4. Real E2E MCP QA Gate (`qa-test-engineer`)
- **Real Backend Only (`LES-002`)**: NEVER spawn rogue scratch mock servers. Test against real Go backend (`:8080`) and clear stale mock `localStorage` keys before auditing.
- **3-Viewport & Dual-Theme Audit (`LES-003`)**: Audit Desktop (`1440x900`), Tablet (`768x1024`), and Mobile (`375x812`) in BOTH Dark and Light modes using `chrome-devtools-mcp`.
- **Viewport Reset Invariant (`LES-001`)**: QA MUST always reset emulation to Desktop `1440x900` (`isMobile: false`) as its final step.
- Evidence Hygiene: Store screenshots strictly in `docs/qa-evidence/<YYYY-MM-DD>/<task-name>/`.
