# ANTI-STUPIDITY & ANTI-LAZY GUARDRAILS (ENTERPRISE DEFENSE SYSTEM)

> Strictly binding for ALL agents (Orchestrator, Coder, Reviewer, QA, Architect, Planner).
> Any violation is classified as a CRITICAL DEFECT requiring immediate rejection and rollback.

---

## 🚫 THE 6 CARDINAL SINS OF AI AGENTS

### 1. BLIND GUESSWORK
- **Violation**: Inventing struct field names, function signatures, file paths, or API schemas without reading the actual source code first.
- **Enforcement**: Step 0 for ANY agent MUST be calling `view_file` on target source definitions. Every proposed change MUST cite exact `file:line` evidence. Guessing is strictly penalized.

### 2. TOY CODE & FAKE PROGRESS
- **Violation**: 
  - Discarding errors via `_ := func()`, `_ = err`, or empty `catch (e) {}`.
  - Injecting synthetic mocks, `Math.random()`, fake latency, or hardcoded dummy JSON in production pathways.
  - Writing empty unit tests that assert nothing or only test trivialities (`assert.True(true)`).
- **Enforcement**: Production-grade implementation only. Proper context propagation, typed domain errors, structured slog/zap logging, and deterministic rollback handlers.

### 3. FABRICATED TEST AUDITS
- **Violation**: Text-only claims of "tests pass" without raw command output, or QA claiming UI is functional without running browser tools.
- **Enforcement**:
  - Backend: Verbatim terminal execution required (command line + exit code 0 + stdout/stderr).
  - QA / Frontend: MUST call `call_mcp_tool` with `chrome-devtools-mcp` (`navigate_page`, `list_network_requests`, `take_screenshot`). Actual on-disk screenshot path and full HTTP request status table required.

### 4. LONE-WOLF EXECUTION
- **Violation**: Main agent bypassing the Graph layer to dump unrefined user prompts directly to coders.
- **Enforcement**: Graph Layer precedence:
  1. `research`: Comprehensive codebase reconnaissance.
  2. `planner`: Granular, topological WBS breakdown with file-level contracts.
  3. `coder`: Surgical implementation inside Git worktree isolation (`Workspace: "branch"`).
  4. `reviewer`: Line-by-line adversarial code audit.
  5. `qa-test-engineer`: Real MCP browser audit across mobile, tablet, desktop viewports.

### 5. MONOLITHIC CODE BLOAT (STRICT < 500 LINES / FILE)
- **Violation**: Appending logic to existing files pushing line counts past 500 lines; coupling CSS, business logic, and UI templates in one file.
- **Enforcement**: Clean Architecture & SOLID segregation:
  - Frontend: Composables in `src/composables/`, styles in `src/assets/styles/`, sub-components < 250 lines in `src/components/`, views < 350 lines.
  - Backend: Strict 4-tier separation (Domain -> Usecase -> Adapter -> Infrastructure).

### 6. SKILL AMNESIA & IGNORING INSTRUCTIONS
- **Violation**: Executing tasks without loading assigned skills from system prompt.
- **Enforcement**: Mandatory execution of Step 0 skill loading. Subagents must state the techniques they are applying from their assigned skills.

### 7. THE "HIDE INSTEAD OF ADAPT" LAZY SHORTCUT & BUTTON CLUTTER SPAM
- **Violation**:
  - **Hiding complex UI features**: Hiding charts, graphs, data visualizations, or telemetry curves on mobile (`display: none !important;`) instead of adapting them into a responsive mobile-first component (e.g. hiding the live saturation line chart on mobile overview!).
  - **Button Spam**: Cramming 5-6 chunky text buttons onto every row of a table (`Logs`, `Scale`, `Restart`, `YAML`, `Details`, `Delete`) instead of designing clean enterprise actions (2 primary actions + `[ ⋯ ]` dropdown / action drawer).
  - **Superficial "Check-the-Box" Fixes**: Making a test pass by deleting or commenting out code, or doing the bare minimum to get 0 overflow while ruining the user experience.
- **Enforcement**:
  - **Mandatory Mobile Data Parity**: Any chart or visualization on desktop MUST have an adapted mobile-first equivalent (e.g. compact SVG sparkline, 120-140px mobile trend card). Completely hiding a chart is an AUTOMATIC REJECTION.
  - **Max 2 Inline Table Actions**: Every table row must have at most 2 primary actions inline; all secondary operations must be inside a `[ ⋯ ]` menu.
  - **Reviewer & QA Failure Gate**: Reviewer and QA MUST explicitly reject any PR that suppresses charts or spams buttons.

---

## 🛡️ 4 HARD ENFORCEMENT GATES

### Gate 1: Mandatory Dispatch Contract (English Only)
Every dispatch prompt from Orchestrator MUST contain:
1. `REQUIRED SKILLS`: Absolute paths of `SKILL.md` files to read in Step 0.
2. `CONTEXT ANCHOR`: Active branch, commit SHA, and mandatory reading of `.agents/memory/project_state.md`.
3. `SCOPE WHITELIST`: Explicit list of allowed files. Touching unlisted files = automatic rejection.
4. `ACCEPTANCE CRITERIA`: Verifiable Gherkin (Given-When-Then) or boolean checklist.
5. `NEGATIVE CONSTRAINTS`: Zero stubs, zero mocks, zero `_ :=`, strict < 500 lines/file.
6. `REQUIRED EVIDENCE`: Exact command to run and expected output format.

### Gate 2: The 8-Point Adversarial Reviewer Checklist
`reviewer` must evaluate every coder diff against:
1. **Concurrency Safety**: Data race prevention, sync primitives, channel deadlocks.
2. **Resource Leaks**: Unclosed response bodies, leaked goroutines, uncancelled contexts.
3. **Error Handling**: Wrapped typed errors (`%w`), contextual logging.
4. **Architectural Fitness**: Layer integrity, dependency direction, < 500 line limit.
5. **Performance & Allocations**: N+1 query elimination, indexing, memory allocations.
6. **Security & RBAC**: Input sanitization, authorization middleware, least privilege.
7. **Simplicity (`ponytail-review`)**: Elimination of dead code, speculative abstractions, and bloat.
8. **Anti-Lazy & Feature Parity**: Zero lazy hiding of charts/data on mobile (`display: none` on charts is an instant reject), max 2 inline table actions (zero button spam).

### Gate 3: Exhaustive QA Matrix (Happy + Unhappy + Edge Cases + Charts + 3 Viewports)
`qa-test-engineer` MUST audit:
1. **Happy Path**: Expected valid workflows return 200 OK.
2. **Unhappy Path**: Invalid inputs return 400 Bad Request, unauthorized calls return 401/403.
3. **Boundary / Edge Cases**: Empty arrays, nil pointers, malformed UUIDs, maximum payload sizes.
4. **Mandatory 3-Tier Viewport Audit (ZERO TOLERANCE FOR TABLET REGRESSION)**:
   - **Desktop (1440x900)**: 100% full-width table, all action buttons visible, zero horizontal scrollbar, interactive drawers/modals functional.
   - **Tablet (768x1024 - iPad Standard)**: HUD cards form a clean 2x2 grid (`repeat(2, 1fr)`), zero horizontal table blowout, off-canvas sidebar triggers properly, filters wrap cleanly without clipping.
   - **Mobile (375x812 - iPhone Standard)**: 44px command bar + live SVG chart/telemetry + 60-75px high density card stream + zero horizontal overflow (`scrollWidth === 375px`).
5. **Feature & Interaction Verification**:
   - Test real clicks on action buttons (`Logs`, `Details`, `Scale`, `Restart`, `YAML`, `Delete`).
   - Confirm modals open with populated data, form inputs work, and API calls succeed with zero console errors.
   - Confirm live charts (splines, bars, SVG trends) actually render on mobile and desktop without clipping, horizontal overflow, or being hidden.

### Gate 4: Fail-Closed Governor Handoff Audit
Before any merge, Orchestrator or `governor` inspects the transcript:
- Did coder call `view_file` before editing?
- Did reviewer actively critique or merely rubber-stamp?
- Did QA execute actual `chrome-devtools-mcp` tools?
- Any failure = REJECT, ROLLBACK worktree, RE-DISPATCH with penalty instructions.
