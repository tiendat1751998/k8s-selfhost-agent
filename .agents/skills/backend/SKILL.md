---
name: Backend Engineer
description: Instructions for developing production-grade Go services, database structures, and business logic.
---

# Backend Engineer Playbook

## Workflow Overview

All tasks follow the 4-step workflow:

```
1. Read Context → 2. Implement Feature → 3. Write Tests → 4. Run Quality Gates
```

---

## Step 1: Read Context

Before writing any code:

1. Read `.agents/context/coding-standards.md` — mandatory coding standards.
2. Understand the project structure: `go.mod`, `main.go`, `internal/`, `pkg/`.
3. Read files related to the feature being implemented.
4. Understand existing patterns: error handling, logging (zap), routing (chi), DB access (pgx).
5. Check existing test files to follow the same style.

**NEVER implement without reading the context first.**

---

## Step 2: Implement Feature

### Coding Rules (MANDATORY)

### Implementation Guidelines

- Follow existing project patterns — do not invent new approaches.
- Every I/O-bound function must accept `context.Context` as the first parameter.
- Use dependency injection via constructors — avoid global state.
- Clear error handling — never swallow errors.
- Use `zap.Logger` for structured logging.
- Use `chi/v5` for HTTP routing, `pgx/v5` for DB connection pooling, and `encoding/json` for JSON serialization.
- Use `sync.Pool` for allocation optimization.
- Implement goroutine panic recovery.

### Go-Specific Notes

- After every code modification → **run `go build ./...`** immediately.
- Verify compilation after each edit.
- If `go build` fails → fix the compilation issues before continuing.

---

## Step 3: Write Tests

### Test Style

- Use **table-driven tests** — idiomatic Go.
- Use `testify/assert` or manual assertions.
- Mock external dependencies (DB, HTTP, NATS) — never test against real services in unit tests.
- Test both the happy path and error cases.
- Name tests clearly: `Test_<Function>_<Scenario>_<Expected>`.

### Test Structure

```go
func Test_CreateOrder_InvalidInput_ReturnsDomainError(t *testing.T) {
    // Arrange
    // Act
    // Assert
}
```

### Coverage

- Business logic: target 80%+ coverage.
- Error handling: must test ALL error paths.
- Edge cases: nil input, empty slice, zero values, max int64.

---

## Step 4: Run Quality Gates

Before completing a task, run **ALL** quality gates:

```bash
go fmt ./...
go vet ./...
staticcheck ./...
go test ./...
go test -race ./...
```

### Quality Gate Rules

- **All 5 gates MUST pass** — if any gate fails, fix before reporting done.
- If `staticcheck` is not available: `go install honnef.co/go/tools/cmd/staticcheck@latest`.
- If `go test -race` is too slow: `go test -race ./internal/...`.
- NEVER skip quality gates because "it's probably correct".

---

## 🔥 MASTER-LEVEL GOROUTINE & CONCURRENCY RULES (PRODUCTION-GRADE)

### 1. Centralized Safe Goroutine Spawning & Panic Recovery
NEVER manually write `go func() { defer recover() }` blocks inline across the codebase. It creates massive duplication, error-prone boilerplate, and violates DRY. Use a centralized concurrency utility package (e.g., `pkg/concurrency`) to guarantee recovery.

```go
// CORRECT: pkg/concurrency/goroutine.go
package concurrency

import (
	"context"
	"go.uber.org/zap"
)

// Go spawns a goroutine with recovery to prevent server crashes
func Go(log *zap.Logger, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("recovered panic in background goroutine",
					zap.Any("panic", r),
					zap.Stack("stack"),
				)
			}
		}()
		fn()
	}()
}
```

### 2. Prevent Channel Spin Locks & Leakages
When reading from channels inside select loops, ALWAYS verify if the channel was closed. Reading from a closed channel returns the zero value instantly without blocking, which will cause your select loop to spin at 100% CPU.

```go
// CORRECT: Handle channel closure by nil-ing the channel to disable the select case
go func(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case msg, ok := <-ch:
            if !ok {
                ch = nil // Set to nil so this select case is ignored, preventing infinite spins
                continue
            }
            process(msg)
        }
    }
}(ctx)
```

### 3. Thread-Safe Encapsulation (Mutex Ownership)
NEVER anonymously embed `sync.Mutex` or `sync.RWMutex` in your structs. Embedding them exposes the `.Lock()` and `.Unlock()` methods on the struct's public API surface, allowing external callers to lock or unlock the struct, leading to deadlocks.

```go
// CORRECT: Mutex encapsulated as a private unexported field
type SafeCache struct {
	mu   sync.RWMutex
	data map[string]*Item
}

func (c *SafeCache) Load(key string) (*Item, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.data[key]
	return val, ok
}

func (c *SafeCache) Store(key string, val *Item) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = val
}
```

### 4. Bounded Structured Concurrency (Resource Limits)
NEVER spawn unbounded goroutines inside a loop (e.g., calling `eg.Go` without limits on a dynamic slice of items). This can trigger out-of-memory (OOM) failures or exhaust file descriptors. Always bound concurrency using Go 1.20's `SetLimit` or a buffered channel semaphore.

```go
// CORRECT: Structured concurrency with explicit resource limits
func ProcessBatch(ctx context.Context, items []*Item) error {
	eg, egCtx := errgroup.WithContext(ctx)
	
	// Limit maximum concurrent active workers (e.g., max 10 concurrent database queries)
	eg.SetLimit(10) 
	
	for _, item := range items {
		item := item // Pin range variable to prevent loop variable capture bugs
		eg.Go(func() error {
			return processItem(egCtx, item)
		})
	}
	return eg.Wait()
}
```

### 5. Proper Context Propagation in Concurrency
Any concurrent worker must respect the parent context deadline. Always pass the derived context (e.g., `egCtx` from `errgroup`) to down-stream requests to abort database transactions and connection requests immediately on cancellation.

### 6. Always Run the Race Detector
No code involving concurrency or shared state is ready for production unless verified by the race detector:
```bash
go test -race ./...
```

---

## 🔥 REDIS OPTIMIZATION RULES (CRITICAL)

### Redis Connection Pool Configuration

```go
// CORRECT: connection pool with proper settings
redisClient := redis.NewClient(&redis.Options{
    Addr:         os.Getenv("REDIS_ADDR"),
    Password:     os.Getenv("REDIS_PASSWORD"),
    DB:           0,
    PoolSize:     300,        // Match concurrent request volume
    MinIdleConns: 50,         // Keep warm connections ready
    ReadTimeout:  150 * time.Millisecond,
    WriteTimeout: 150 * time.Millisecond,
    PoolTimeout:  300 * time.Millisecond,
    IdleTimeout:  5 * time.Minute,
    MaxRetries:   2,
    MinRetryBackoff: 8 * time.Millisecond,
    MaxRetryBackoff: 512 * time.Millisecond,
})
```

### Redis Key Patterns

| Pattern | TTL | Use Case |
|---------|-----|----------|
| `incident:{id}:detail` | 10 min | Single incident detail |
| `incident:list:{hash}` | 3 min | Incident list cache |
| `session:{user_id}` | 7 days | User session |
| `rate_limit:{ip}:{action}` | 1 min | Rate limiting |

### Redis Anti-Patterns (MUST NOT)

- ❌ **NO `KEYS *` command** in production — use SCAN.
- ❌ **NO big keys** (>10KB) — split into smaller chunks.
- ❌ **NO N+1 Redis calls** — use Pipeline or MGet.
- ❌ **NO cache without TTL** — always set expiration.
- ❌ **NO silent failure** — log ALL Redis errors with zap.
- ❌ **NO single connection** — always use connection pool.
- ❌ **NO `SELECT` in code** — configure DB in the connection string.

### Redis Pipeline Usage (Batch Operations)

```go
// CORRECT: Pipeline for batch operations
pipe := redisClient.Pipeline()
for _, id := range ids {
    pipe.Get(ctx, "incident:"+id)
}
cmds, err := pipe.Exec(ctx)
if err != nil {
    logger.Error("redis pipeline failed", zap.Error(err))
}

// CORRECT: MGet for batch reads
vals, err := redisClient.MGet(ctx, keys...).Result()
```

### Redis Error Handling

```go
// CORRECT: Log error, continue without cache (graceful degradation)
val, err := redisClient.Get(ctx, key).Result()
if err == redis.Nil {
    metrics.CacheMisses.Inc()
    return nil, nil // cache miss, fetch from DB
}
if err != nil {
    logger.Error("redis GET failed",
        zap.String("key", key),
        zap.Error(err),
    )
    metrics.CacheErrors.Inc()
    return nil, nil // fallback to DB
}
```

---

## 🔥 NATS JETSTREAM OPTIMIZATION RULES (CRITICAL)

### NATS JetStream Publisher Config

```go
// CORRECT: Publish with context and structured payload
data, err := json.Marshal(event)
if err != nil {
    return err
}

_, err = js.Publish(ctx, "incidents.created", data)
if err != nil {
    logger.Error("NATS JetStream publish failed", zap.Error(err))
    return err
}
```

### NATS JetStream Consumer Config

```go
// Subscription configuration
sub, err := js.PullSubscribe("incidents.>", "incident-processor", nats.BindStream("INCIDENTS"))
if err != nil {
    return err
}

// Processing loop
for {
    msgs, err := sub.Fetch(10, nats.MaxWait(1*time.Second))
    if err != nil {
        if err == nats.ErrTimeout {
            continue
        }
        logger.Error("NATS fetch failed", zap.Error(err))
        time.Sleep(1 * time.Second)
        continue
    }
    
    for _, msg := range msgs {
        // Process message...
        if err := msg.Ack(); err != nil {
            logger.Error("NATS ack failed", zap.Error(err))
        }
    }
}
```

### NATS Anti-Patterns (MUST NOT)

- ❌ **NO synchronous waiting** on publish without context deadlines.
- ❌ **NO connection recreation** per request — reuse a single connection.
- ❌ **NO auto-acknowledgement** for critical message flows — always use explicit Ack.
- ❌ **NO ignoring consumer errors** — log all failures and monitor consumer lag.

---

## 🐛 BUG FIX WORKFLOW

### Step 1: Reproduce the Bug
1. Read the error message / log output carefully.
2. Identify the exact file and line where the bug occurs.
3. Write a test case that reproduces the bug (RED).
4. Run the test to confirm it fails: `go test ./... -run TestBugName -v`.

### Step 2: Root Cause Analysis
1. Read the code around the bug location.
2. Check for common Go bugs:
   - Nil pointer dereference.
   - Race condition (needs `-race` flag).
   - Incorrect error handling (swallowed errors).
   - Off-by-one in slices.
   - Goroutine leak (missing context cancellation).
   - Connection leak (not closing connection pool clients/rows).
3. Use `go vet` and `staticcheck` to find static issues.

### Step 3: Fix the Bug
1. Make minimal changes — do not refactor unrelated code.
2. Add/update tests to cover the fix (GREEN).
3. Run: `go build ./...`
4. Run: `go test ./... -v`
5. Run: `go test -race ./...`

### Step 4: Verify the Fix
1. Confirm the test passes.
2. Check that no regressions occur: `go test ./... -race`.
3. Check that no goroutine leaks exist.

---

## Reference Files

- `.agents/context/coding-standards.md` — project coding standards.
- `go.mod` — dependencies and Go version.
- `internal/` — private Go application code.
- `pkg/` — public Go library code.

---

## 🚫 ZERO-TOLERANCE ANTI-FAKE RULES (HIGHEST LEVEL)

**YOU MUST COMPLY WITH THE FOLLOWING RULES. VIOLATION = IMMEDIATE TERMINATION.**

### 1. NEVER fabricate data
- **DO NOT** invent test results, metrics, benchmarks, or reports.
- **DO NOT** state "feature implemented" unless you have run `go build` and verified compilation.
- **DO NOT** state "tests passed" unless you have run the actual test command.
- **DO NOT** fabricate API responses, curl output, or database query results.
- **DO NOT** write "optimized" unless you have run a load test and compared actual metrics.

### 2. ALWAYS verify using the actual tool output
- Every claim must be backed by **real tool output** (terminal output, curl response, build log).
- If you state "build pass" → you **MUST** run the build command and paste the output.
- If you state "test pass" → you **MUST** run the test command and paste the output.
- If you state "API returns 200" → you **MUST** run curl and paste the response.
- If you state "deploy OK" → you **MUST** run `kubectl get pods` and paste the output.

### 3. DO NOT use health checks alone as proof of correctness
- A simple `/healthz` return code 200 **IS NOT** proof that your business feature works.
- Health checks only prove that the process is running, not that the business logic is correct.
- **Always test actual behavior**: make real API calls, run real DB queries, and verify real outputs.

### 4. If you cannot verify → state "CANNOT VERIFY"
- If a tool fails → report the failure; do not pretend it succeeded.
- If you lack access → report the lack of access; do not fabricate data.
- If a task is too complex for one session → state that you need more time.

### 5. Code = Real code, not pseudocode
- If you fix code → show the actual diff, the actual file, and verify compilation.
- If you optimize → show the before/after metrics from a load test tool.

### 6. Every "SUCCESS" claim must include 3 things:
1. **Command you ran** (exact CLI command)
2. **Actual output** (pasted from the terminal)
3. **Relevant evidence** (file diff, metric comparison, log output)

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
