# Over-Engineering Inventory (Ponytail Audit)

> Generated: 2026-09-09T15:49:46+07:00 | Branch: fix/comprehensive-audit

## Findings (ranked by biggest cut first)

| # | Tag | What to Cut | Replacement | Location |
|---|-----|-------------|-------------|----------|
| OE1 | `delete:` | 30+ single-implementation domain interfaces (repository.go) | Remove abstractions, use concrete structs directly | internal/domain/*/repository.go |
| OE2 | `yagni:` | 30+ single-product factory funcs (New*Repo) | Direct struct initialization | internal/infrastructure/postgres/ |
| OE3 | `stdlib:` | github.com/stretchr/testify dependency | Standard library `testing` package | go.mod:19 |
| OE4 | `native:` | concurrency.Go wrapper | Native `go func() + recover` | internal/pkg/concurrency/concurrency.go:9 |
| OE5 | `delete:` | stringutil.go (11-line Truncate func) | Inline at call sites | internal/pkg/stringutil/stringutil.go:5 |
| OE6 | `delete:` | useExplorerOperations.ts (<20 lines re-export) | Import directly | frontend-vue/src/composables/useExplorerOperations.ts:1 |
| OE7 | `delete:` | useExplorerColumns.ts (<20 lines re-export) | Import directly | frontend-vue/src/composables/useExplorerColumns.ts:1 |

## Ponytail Debt
No `ponytail:` comments found. **Clean ledger.**

## Summary
**net: -250 lines, -1 deps possible.**
