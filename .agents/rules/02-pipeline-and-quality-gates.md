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

## 4. Real E2E MCP QA Gate & 5-Gate Zero-Theater Definition of Done (`LES-012`)
A passing test file NEVER suffices to declare a task "Done". Every UI / Full-Stack task MUST pass all 5 gates before reporting completion:
1. **Zero Static-Regex Test Theater**: `fs.readFileSync` regex-matching on `.vue`/`.css`/`.go` source text is **FORBIDDEN** as acceptance proof. Tests must execute real Go HTTP handlers/usecases/SQL or real Vue component mounts/composable logic.
2. **Real Backend & 100% Route/Sub-Tab Coverage (`LES-002`)**: NEVER spawn mock servers. Visit every affected route AND sub-tab live against `:8080` + live PostgreSQL (`0` unexpected HTTP `4xx/5xx`, `0` uncaught JS console errors, `scrollWidth <= clientWidth`). For platform-wide audits, sweep all **16/16** primary routes unblurred.
3. **Stateful Mutation + `F5` Reload Persistence**: Every write action (`Create`/`Edit`/`Toggle`/`Resolve`/`Delete`) must be verified after a full browser `F5` reload to prove PostgreSQL persistence and silent JWT refresh survival.
4. **3-Viewport, Dual-Theme & Main-Thread Visual Inspection (`LES-001`, `LES-003`)**: Audit Desktop (`1440x900`), Tablet (`768x1024`), and Mobile (`375x812`) in Dark & Light modes, reset emulation to `1440x900` (`isMobile: false`) at finish, and personally inspect every unblurred PNG on the main thread via `read_file` BEFORE sign-off.
5. **Direct Inline Chat Proof**: Never hide screenshots behind a `walkthrough.md` link alone. Always copy proof PNGs to `<appDataDir>/brain/<conversation-id>/` and render `![caption](C:/Users/...)` + `[filename](file:///...)` directly inline in the chat response.
