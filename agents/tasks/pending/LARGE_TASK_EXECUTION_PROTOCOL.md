# LARGE TASK EXECUTION PROTOCOL

## PURPOSE

Large tasks must NEVER be implemented directly.

Every large task must first be analyzed and decomposed into smaller executable units.

Implementation begins ONLY AFTER the decomposition is complete.

---

# WHAT IS A LARGE TASK

A task is considered LARGE if it meets ANY of the following:

- Estimated implementation > 1 day
- Affects multiple modules
- Affects frontend and backend
- Introduces a new architecture
- Requires database changes
- Requires infrastructure changes
- Requires UI changes across multiple pages
- Creates more than 10 files
- Modifies more than 20 files
- Contains multiple business capabilities
- Has more than one primary objective

Examples:

- Audit entire repository
- Build Deployment Center
- Build GitOps Platform
- Build AI Copilot
- Build Multi-cluster Management

These are EPICS.

Never implement an Epic directly.

---

# MANDATORY EXECUTION FLOW

For every large task:

STEP 1

Read the task.

Do NOT write code.

---

STEP 2

Analyze the objective.

Identify:

- business goals
- technical goals
- affected modules
- affected services
- affected APIs
- affected pages
- affected database objects
- dependencies
- risks

---

STEP 3

Create an implementation plan.

The plan must include:

Objectives

Architecture

Execution order

Dependencies

Validation strategy

Rollback strategy

---

STEP 4

Break the Epic into Features.

Example

Repository Audit

↓

Architecture Audit

↓

Duplicate Code Audit

↓

Dead Code Audit

↓

Dependency Audit

↓

Security Audit

↓

Performance Audit

---

STEP 5

Break every Feature into Tasks.

Example

Architecture Audit

↓

Analyze folders

↓

Analyze module boundaries

↓

Analyze dependencies

↓

Analyze layering

---

STEP 6

Break every Task into Subtasks.

Subtasks should be executable in one focused implementation session.

Examples:

Analyze Components

Analyze Hooks

Analyze Services

Analyze APIs

Generate Report

---

STEP 7

Estimate complexity.

Each subtask must be:

Small

Independent

Testable

Reversible

If a subtask is still large:

Split it again.

---

STEP 8

Create dependency graph.

Identify:

Must finish before

Can run in parallel

Blocked by

Optional

---

STEP 9

Generate execution order.

Only now may implementation begin.

---

# IMPLEMENTATION RULE

Implement ONLY ONE subtask at a time.

Never implement multiple Features simultaneously.

Never jump ahead.

Complete:

Subtask

↓

Validate

↓

Mark complete

↓

Proceed to next Subtask

---

# VALIDATION

Every completed Subtask must be verified.

Check:

Compilation

Tests

Architecture

Code Quality

Performance

Security

No duplicate code

No dead code

---

# REPLANNING

If new information appears:

STOP.

Update the plan.

Regenerate dependencies.

Continue.

Do NOT continue with an outdated plan.

---

# OUTPUT

For every large task, generate:

1. Executive Summary

2. Objectives

3. Risks

4. Architecture Impact

5. Feature Breakdown

6. Task Breakdown

7. Subtask Breakdown

8. Dependency Graph

9. Execution Order

10. Validation Plan

Only after all of the above are complete may implementation begin.

---

# STOP CONDITIONS

Implementation is FORBIDDEN until:

- Analysis complete
- Plan complete
- Task breakdown complete
- Subtask breakdown complete
- Dependency graph complete
- Validation strategy complete

---

# QUALITY RULE

Planning quality is more important than coding speed.

A perfect plan prevents poor implementation.

Never skip planning for large tasks.

Never trade planning quality for implementation speed.