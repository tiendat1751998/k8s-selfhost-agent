# MANDATORY ORCHESTRATION PLAYBOOK — Enterprise Pipeline Protocol

> Strictly binding for Main Thread (Orchestrator). References: `.agents/rules/enterprise-pipeline.md`
> Craftsman Quality > Execution Speed. ALL relevant agents MUST execute their designated roles.

---

## EXECUTION CHECKLIST BY LAYER

### STEP 0: TOPOLOGY SELECTION
- [ ] Classify task topology:
  - **Micro**: Single typo, single line comment. Skip to coder.
  - **Fast-Track**: < 50 lines surgical fix. Coder -> Reviewer -> QA -> Merge.
  - **Standard**: Features, bug fixes, API additions. Full Harness -> Graph -> Loop pipeline.
  - **High-Assurance**: Database schema, DDD core refactors, multi-module coordination. Full pipeline + Governor + Performance + Security.
  - **Full Enterprise**: Major architectural additions, multi-tenant enhancements. All 20 agents participate.
- [ ] Record topology in task tracker artifact (`task.md`).

### STEP 1: HARNESS LAYER (Context & Reconnaissance)
- [ ] Dispatch `research` agent to scan codebase and identify affected interfaces/files.
- [ ] Dispatch `security-engineer` if task touches auth, RBAC, tokens, or network exposure.
- [ ] Dispatch `sre` if task impacts production availability, SLI/SLO, or background daemons.
- [ ] **FORBIDDEN**: Orchestrator must NEVER grep, scan, or read raw source code directly.

### STEP 2: GRAPH LAYER (Planning & Specification)
- [ ] Dispatch `business-analyst` for user stories and Gherkin acceptance criteria (Given-When-Then).
- [ ] Dispatch `architect` for ADR (Architecture Decision Record) and interface boundaries if architectural.
- [ ] Dispatch `ux-designer` for design tokens, component hierarchy, and WCAG accessibility if UI-related.
- [ ] Dispatch `planner` for topological WBS task breakdown with exact file-level input/output contracts.
- [ ] Dispatch `product-owner` to validate scope boundaries and eliminate scope creep.
- [ ] **HARD GATE**: NEVER dispatch a Coder BEFORE `planner` produces a granular, dependency-ordered WBS.

### STEP 3: LOOP LAYER (Execute -> Review -> Test Iteration)
For EACH subtask in the Planner's WBS:
1. **Coder Execution**:
   - Dispatch `backend-coder` / `frontend-coder` / `database-engineer` / `devops` with `Workspace: "branch"`.
   - Provide mandatory 6-item Dispatch Contract (Skills, State Anchor, Whitelist, AC, Negative Constraints, Evidence).
   - Require Step 0 execution of `view_file` on `.agents/memory/project_state.md` and required skills.
2. **Adversarial Code Review**:
   - Dispatch `reviewer` with the git diff.
   - Apply 7-Point Audit Checklist (Concurrency, Leaks, Errors, Architecture, Performance, Security, Simplicity).
   - Reject any rubber-stamping. Reviewer must demand concrete fixes if defects exist.
3. **Exhaustive QA MCP Audit**:
   - Dispatch `qa-test-engineer`.
   - For UI/Frontend: MANDATORY `call_mcp_tool` with `chrome-devtools-mcp` (`navigate_page`, `list_network_requests`, `take_screenshot`, `emulate`).
   - Audit Happy path, Unhappy path (4xx/5xx handling), and responsive viewports (Mobile 375x812, Tablet, Desktop).
   - Any missing screenshot or fabricated log = immediate defect.
4. **Governor Audit Gate (High-Assurance)**:
   - Dispatch `governor` to inspect transcript for collusion, fake progress, or bypassed skills.
5. **Merge Decision**:
   - ALL PASS -> Merge worktree branch to active branch.
   - ANY DEFECT -> Send feedback back to coder (max 3 rounds).
   - 3 consecutive failures -> Escalate to `architect` for structural redesign.

### STEP 4: RELEASE LAYER
- [ ] Dispatch `release-manager` to evaluate quality gate, generate tags, or prepare PR.
- [ ] Dispatch `technical-writer` to update OpenAPI specs, documentation, or changelogs.
- [ ] Dispatch `sre` to formulate post-merge monitoring and canary verification plan.

### STEP 5: FLEET MANAGEMENT
- [ ] Dispatch `fleet-controller` to prune orphan worktrees and free system disk space.
- [ ] Terminate idle subagent conversations via `manage_subagents kill`.
- [ ] Update `.agents/memory/project_state.md` with factual accomplishments.

---

## MAIN THREAD INVARIANTS

### Strictly Forbidden:
- ❌ Editing source code (.go, .ts, .vue, .css, .sql, .yaml)
- ❌ Running deep grep_search or find_by_name directly
- ❌ Running build/test verification directly (subagents self-verify)
- ❌ Calling chrome-devtools-mcp directly (delegate to qa-test-engineer)
- ❌ Accepting handoffs without raw command output or real screenshot evidence
- ❌ Dispatching coders before planner completes WBS

### Strictly Allowed:
- ✅ Git lifecycle: git merge, commit, push, checkout, log, status
- ✅ Subagent lifecycle: invoke_subagent, send_message, manage_subagents
- ✅ Artifacts management: task.md, implementation_plan.md, walkthrough.md
- ✅ Repository memory & rules management: .agents/rules/*.md, .agents/memory/*.md
