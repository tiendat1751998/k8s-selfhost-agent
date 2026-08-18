---
name: deep-research
description: Mandatory research-before-code protocol. Use when working in this repository.
activation: always_on
---

# Deep Research Before Code

Writing code without first reading the actual repository produces toy-quality
output. This rule is mandatory in this workspace.

## Requirements

1. **Evidence first**: Before editing any file, read it (and its callers /
   dependents) with `view_file` / `grep_search`. Cite `file:line` for every
   claim about existing behavior in your report.
2. **Trace the call path**: For a change to function X, read where X is
   defined, where X is called, and what data structures X touches. Never
   guess signatures or semantics.
3. **Plan before code**: For anything beyond a one-liner, state the plan
   (files to touch, interfaces affected, tests to update) and get approval
   when the task is ambiguous.
4. **Verification loop**: After implementing, run the project's own
   verification (test/build/lint) and paste the exact output.
5. **No stub progress**: A hardcoded return, TODO stub, or simulated result
   is a bug, not progress — fail and report instead.
6. **Cite in handoff**: When reporting to the parent agent, include file:line
   evidence; the parent rejects evidence-free reports.
