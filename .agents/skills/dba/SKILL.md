---
name: DBA
description: Instructions for database schema management, query optimization, connection pooling, and data integrity.
---

# DBA Playbook

## Session Startup (MANDATORY)

Before running queries or migrations:
1. Read `.agents/context/business-rules.md` — understand business constraints.
2. Read `.agents/context/database-schema.md` — understand the current schema.
3. Read `.agents/context/deployment-topology.md` — understand database host/port settings.

**NEVER run migrations or optimize queries without knowing the active schema and topologies.**

---

## Workflow

### 1. Analyze
- Read the current database schema from `database-schema.md`.
- Understand the query patterns, indexes, and access paths.
- Check slow query logs and PostgreSQL activity.
- Use `EXPLAIN ANALYZE` on problematic queries.

### 2. Design
- Write migration SQL scripts (forward + rollback).
- Design indexes with left-prefix rule in mind.
- Plan data backfill strategies for column type changes.
- Estimate downtime, table locks, and system impact.

### 3. Validate
- Test migration SQL on a staging or test database.
- Verify using `EXPLAIN` that queries utilize the new indexes.
- Check for lock contention.
- Validate that the rollback script works successfully.

### 4. Execute (with Orchestrator approval)
- Perform a backup of the target database before applying migrations.
- Run migration in a PostgreSQL transaction block (`BEGIN; ... COMMIT;`).
- Verify data integrity.
- Update `.agents/context/database-schema.md` with new schema definitions.

### 5. Monitor
- Watch the slow query log for regressions.
- Check PostgreSQL connection pool utilization.
- Monitor Redis cache hit rates.

---

## Indexing Rules

- Foreign keys: must always be indexed.
- WHERE clause columns: index candidate columns based on selectivity.
- ORDER BY columns: index to avoid file sorts.
- Composite indexes: follow the left-prefix rule.
- Max 5 indexes per table (review carefully if more are proposed).

---

## Migration Conventions

- Naming convention: timestamped migrations in `migrations/` directory (e.g. `000001_init.up.sql` or similar timestamp).
- Always backward-compatible (no dropping fields without prior notice and code migration).
- Add nullable columns first, backfill, then alter to `NOT NULL` if required.
- Test both up/down migration directions.

---

## Safety Checklist
- [ ] Backup completed
- [ ] Staging/Local test passed
- [ ] Rollback script verified
- [ ] Orchestrator approved
- [ ] Monitoring alert configured

---

## Terminal Commands Reference
- PostgreSQL CLI: `psql -h localhost -U postgres -d k8sselfhost`
- Redis CLI: `redis-cli`
- NATS CLI: `nats stream info INCIDENTS`

---

## Quality Gates
- EXPLAIN shows index usage (no full table scans on large tables).
- No queries exceeding connection timeouts.
- Cache hit ratio >= 95%.
- Migration contains rollback/down SQL script.

---

## 🚫 ZERO-TOLERANCE ANTI-FAKE RULES (HIGHEST LEVEL)

**YOU MUST COMPLY WITH THE FOLLOWING RULES. VIOLATION = IMMEDIATE TERMINATION.**

### 1. NEVER fabricate data
- **DO NOT** invent query results, `EXPLAIN` plans, or performance metrics.
- **DO NOT** state "index added" unless you have run the schema migration and verified.
- **DO NOT** state "query optimized" unless you have run `EXPLAIN ANALYZE` and compared timings.
- **DO NOT** fabricate slow query logs, cache stats, or database stats.

### 2. ALWAYS verify using the actual tool output
- Every claim must be backed by **real tool output** (psql CLI output, EXPLAIN plan, etc.).
- If you state "index created" → you **MUST** run the schema query and paste the result.
- If you state "query is fast" → you **MUST** run `EXPLAIN ANALYZE` and paste the execution plan.
- If you state "migration applied" → you **MUST** run `\dt` or `SELECT` and verify.

### 3. DO NOT use "should be fast" as proof
- "Should use index" **IS NOT** proof that the index is utilized by the query planner.
- "Query should be optimized" **IS NOT** proof that latency is low.
- **Always run EXPLAIN**: paste the plan, show rows scanned, and show the index used.

### 4. If you cannot verify → state "CANNOT VERIFY"
- If database access is unavailable → report it; do not pretend.
- If a query fails or times out → report the exact error.

### 5. Database = Real queries, not assumptions
- Optimization = run EXPLAIN → show query plan → compare execution times.
- Index verification = query pg_indexes → show active indexes.

### 6. Every "SUCCESS" claim must include 3 things:
1. **Command you ran** (exact SQL / CLI command)
2. **Actual output** (pasted from psql or terminal)
3. **Relevant evidence** (EXPLAIN plan, timing comparison, index list)

**YOU WILL BE REJECTED IF YOU CANNOT PROVE.**

---

## 📤 ORCHESTRATOR OUTPUT CONTRACT (MANDATORY)

When completing a task, you **MUST** end the output with this section.
This is the standard format for the orchestrator to parse and aggregate results.

### Format (copy and fill):

```markdown
## ORCHESTRATOR SUMMARY

**Status**: SUCCESS | PARTIAL | FAILED
**Report**: .agents/reports/<filename.md>

### What was done:
- [bullet point 1]
- [bullet point 2]

### Verification evidence:
```
[paste tool output proof here]
```

### Issues/Blockers:
- [if any, otherwise write "None"]

### Recommended next steps:
- [if any]
```

### Rules:
1. **ALWAYS** include the ORCHESTRATOR SUMMARY section at the end of the output — this is critical.
2. **Status** must be clear: SUCCESS (all passed), PARTIAL (completed with minor issues), FAILED (not completed).
3. **Report path** must be the path to the report file.
4. **Verification evidence** must include actual tool output (terminal, curl, build log) — DO NOT use "should work".
5. If the task failed → specify the cause + suggest a fix.
6. The orchestrator will use this SUMMARY to aggregate all agent results — if missing, the results may be ignored.