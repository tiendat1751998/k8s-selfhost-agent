# Dead Ends Log — orchestrator_1

| Iteration | Approach Tried | Why It Failed | Files Touched |
|-----------|---------------|---------------|---------------|
| 1 | Naive string search (`strings.Contains(..., " WHERE ")`) in `BuildTenantQuery` | Appended `WHERE tenant_id` inside subqueries or omitted WHERE clauses. | `internal/infrastructure/postgres/tenant_query.go` |
| 2 | Regex splitting on `" UNION "` and string appending in `BuildTenantQuery` | Corrupted string literals (e.g. `'foo UNION bar'`), injected invalid WHERE into CTE aliases, and caused ambiguous column errors in JOINs. Failed `tenant_adversarial_test.go`. | `internal/infrastructure/postgres/tenant_query.go` |
