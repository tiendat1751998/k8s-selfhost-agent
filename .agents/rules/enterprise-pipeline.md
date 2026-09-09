# ENTERPRISE PIPELINE — DYNAMIC ROUTING PROTOCOL

> Binding framework for the Main Agent (Orchestrator).
> Principle: Identify the scenario first -> Select ONLY required agents -> Enforce decoupled I/O (RECEIVE -> DO -> RETURN).

---

## 🗺️ THE 6 DYNAMIC ROUTING SCENARIOS

### SCENARIO 1: Micro / Cosmetic Fix (Typo, Minor CSS, Button Style)
- **Pipeline**: `frontend-coder` (or `backend-coder`) -> `qa-test-engineer` (MCP) -> `reviewer` -> MERGE
- **Excluded**: BA, Architect, Planner, SRE, DB, Security, PO (Prevents token bloat and over-engineering).

### SCENARIO 2: Logic Bugfix / API Defect (Fix 404, Nil Pointer, Validation)
- **Pipeline**: `research` -> `planner` -> `backend-coder` -> `reviewer` -> `qa-test-engineer` -> MERGE
- **Excluded**: UX Designer, Product Owner, Business Analyst, SRE.

### SCENARIO 3: Security Vulnerability Patching (RBAC, SQLi, Auth, SSRF)
- **Pipeline**: `security-engineer` (PoC) -> `planner` -> `backend-coder` / `database-engineer` -> `reviewer` -> `security-engineer` (Re-test) -> `qa-test-engineer` -> MERGE
- **Excluded**: UX Designer, Product Owner, SRE.

### SCENARIO 4: Performance & Production Incident (Slow System, High CPU/RAM, N+1 Queries)
- **Pipeline**: `sre` (Telemetry analysis) -> `performance-engineer` (Benchmark/Isolation) -> `database-engineer` / `backend-coder` -> `qa-test-engineer` -> `reviewer` -> MERGE
- **Excluded**: UX Designer, Product Owner, Business Analyst.

### SCENARIO 5: Database Schema Evolution / Migration (New Tables, Migrations, Indexing)
- **Pipeline**: `architect` (Data model) -> `database-engineer` (DDL/DML Up/Down) -> `backend-coder` (Repository/Entity) -> `reviewer` -> `qa-test-engineer` -> MERGE
- **Excluded**: UX Designer, SRE, Product Owner.

### SCENARIO 6: Complex New Feature / Major Module (Payment, Multi-Tenant Core, New Views)
- **Pipeline (Full Corporate Structure)**:
  1. `business-analyst` (User stories & Gherkin AC)
  2. `product-owner` (Scope boundaries & prioritization)
  3. `planner` (Topological WBS task tree)
  4. Parallel Design: [`architect` (ADR) + `ux-designer` (Tokens/Layout) + `database-engineer` (Schema)]
  5. Parallel Implementation: [`backend-coder` (APIs) + `frontend-coder` (UI)]
  6. Quality Gates: `qa-test-engineer` (MCP) -> [`reviewer` (Code quality) + `security-engineer` (Vulnerability audit)]
  7. Governance & Rollout: `release-manager` -> `governor` (Anti-collusion) -> RELEASE

---

## 📦 DECOUPLED I/O PROTOCOL: I RECEIVE -> I DO -> I RETURN

Subagents NEVER chat directly with each other. All communication flows through structured task artifacts routed by the Orchestrator:

```text
[ORCHESTRATOR]
      │  Dispatches task payload
      ▼
┌──────────────┐
│  I RECEIVE   │  - Surgical scope (file:line whitelist)
│              │  - Acceptance criteria
│              │  - Upstream design artifacts
└──────┬───────┘
       │
       ▼
┌──────────────┐
│     I DO     │  - Execute specialized role in isolated worktree
│              │  - Self-verify with native test/build commands
└──────┬───────┘
       │
       ▼
┌──────────────┐
│   I RETURN   │  - Structured summary (< 50 lines)
│              │  - Verbatim verification evidence
│              │  - Artifact / Screenshot URI
└──────┬───────┘
       │  Emits structured handoff
       ▼
[ORCHESTRATOR] ──► Inspects evidence, gates quality, routes to next agent
```
