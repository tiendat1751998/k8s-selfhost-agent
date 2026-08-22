# Git Branching Strategy & Zero Direct Master Push

> **Mandatory Policy**: Applies to all sessions and subagents in `k8sseflhost`.

## 1. Branching Invariant
- **NEVER push directly to `master`**.
- Every task, bugfix, or feature MUST be developed on a dedicated branch:
  - `feat/<short-description>` for new features
  - `fix/<short-description>` for bug fixes
  - `refactor/<short-description>` for refactoring
- Checkout the new branch before writing or committing code.

## 2. Commit & Push Protocol
- Use Conventional Commits format: `type(scope): description`.
- Push ONLY to the remote feature branch: `git push -u origin <branch-name>`.
- The `master` branch is protected and reserved for stable, verified code.

## 3. Merging
- After full automated verification (tests, builds, lints) and live QA verification (via Chrome DevTools MCP where applicable), report the branch name and verification summary to the user.
- Merge into `master` only after confirmation.
