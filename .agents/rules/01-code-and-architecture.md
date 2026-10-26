# 01 — Code & Architecture Standards

> Universal Invariants for all code modifications in `k8sseflhost`.

## 1. CodeGraph First (`LES-005`)
- In indexed repos (`.codegraph/`), call `codegraph_explore` (`ServerName: "codegraph"`, `projectPath: "d:\\project\\k8sseflhost"`) or shell `codegraph explore "<symbol>"` **BEFORE** grep/find/reading files.
- Inspect caller/callee trees and definitions first; cite `file:line` evidence for all changes. Never guess signatures or paths.

## 2. File Modularity & Size Limits (Strict `< 500 lines`)
- Absolute hard ceiling: **`< 500 lines per file`** across `.go`, `.vue`, `.ts`, and `.css`.
- Frontend split: Orchestrator views `<= 350 lines` (`src/views/`), Sub-components `< 300 lines` (`src/components/`), Composables (`src/composables/`), CSS segregated in `src/assets/styles/`.

## 3. Go Backend: 4-Layer Clean Architecture & ACID
- Pure unidirectional dependency flow:
  `internal/domain/` (pure entities & errors) → `internal/usecase/` (business flows & ports) → `internal/adapter/http/` (REST handlers & RBAC) → `internal/infrastructure/` (Postgres, client-go, drivers).
- Security & DB: Parameterized SQL only (zero string concatenation); strict tenant isolation (`tenant_id`); context propagation (`r.Context()`).
- Zero Toy/Mock Code: No `Math.random()`, no fake latency, no stubs, no `_ = err` or ignored errors. Offline hosts return `latency_ms: 0` and display `--`.

## 4. Vue 3 Frontend & Dual-Theme Tokens (`LES-003`, `LES-005`)
- Design System: Use `tokens.css` surface ladder (`var(--bg-canvas)`, `var(--bg-card)`, `var(--border-subtle)`).
- Dual-Theme Parity: Support both `[data-theme='dark']` and `[data-theme='light']`. Zero hardcoded neon hex (`#38bdf8`, `#06b6d4`, `#34d399`, `#fb7185`) or `rgba(15, 23, 42, ...)`. Semantic colors restricted to 6px status dots.
- Anti-Lazy Mobile Parity: Never hide charts on mobile (`display: none` forbidden). Provide responsive mobile SVG sparkline or trend card.
- Table Actions: Max 2 primary actions inline per row; secondary operations collapse into `[ ⋯ ]` menu.
- Zero Cookie-Cutter CRUD Clones (`LES-005`): Every screen must implement its domain Job-to-be-Done visual paradigm (Hardware Blades for `/hosts`, Workload Topology for `/deployments`, Burn-Rate Gauges for `/slo`, Compliance Scorecards for `/audit`, Pipeline Cards for `/automation`, Playbook Runner for `/runbooks`).

## 5. Security & Zero Hardcoded Secrets (`LES-004`)
- Zero secrets, passwords, DSNs, or tokens in source code or scratch scripts (`brain/*/scratch/`). Always load configuration from `.env` via `os.Getenv("DATABASE_URL")`.
