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

1. Read `.agents/skills/backend/coding-standards.md` in project root — mandatory coding standards
2. Understand project structure: `go.mod`, `main.go`, `internal/`, `pkg/`, `api/`
3. Read files related to the feature being implemented
4. Understand existing patterns: error handling, logging (zap), routing (gin), DB access (gorm)
5. Check existing test files to follow the same style

**NEVER implement without reading context first.**

---

## Step 2: Implement Feature

### Coding Rules (MANDATORY)

| Rule | Detail |
|------|--------|
| **Prices** | Always use `int64` — maps to BIGINT in DB. NEVER use float for money |
| **Weight** | Use `int` — unit is grams |
| **Currency** | Always **VND** — no multi-currency support |
| **DB Enums** | Use **UPPERCASE**: `ORDER_STATUS_PENDING`, `PAYMENT_METHOD_COD` |
| **Domain Errors** | Return `*DomainError` pointer — enables wrapping, type assertion, structured response |

### Implementation Guidelines

- Follow existing project patterns — do not invent new approaches
- Every I/O-bound function must accept `context.Context` as first parameter
- Use dependency injection via constructors — avoid global state
- Clear error handling — never swallow errors
- Use `zap.Logger` for structured logging
- Use `gin` for HTTP routing, `gorm` for DB, `sonic v1.15.1` for JSON serialization

### Go-Specific Notes

- After every `patch` tool use → **run `go build ./...`** immediately
- `patch` tool can corrupt Go files — verify with `go build` after each edit
- If `go build` fails → fix before continuing

---

## Step 3: Write Tests

### Test Style

- Use **table-driven tests** — idiomatic Go
- Use `testify/assert` or manual assertions
- Mock external dependencies (DB, HTTP, Kafka) — never test against real services
- Test both happy path and error cases
- Name tests clearly: `Test_<Function>_<Scenario>_<Expected>`

### Test Structure

```go
func Test_CreateOrder_InvalidInput_ReturnsDomainError(t *testing.T) {
    // Arrange
    // Act
    // Assert
}
```

### Coverage

- Business logic: target 80%+ coverage
- Error handling: must test ALL error paths
- Edge cases: nil input, empty slice, zero values, max int64

---

## Step 4: Run Quality Gates

Before completing task, run **ALL** quality gates:

```bash
go fmt ./...
go vet ./...
staticcheck ./...
go test ./...
go test -race ./...
```

### Quality Gate Rules

- **All 5 gates MUST pass** — if any gate fails, fix before reporting done
- If `staticcheck` not available: `go install honnef.co/go/tools/cmd/staticcheck@latest`
- If `go test -race` too slow: `go test -race ./internal/...`
- NEVER skip quality gates because "it's probably correct"

---

## 🔥 REDIS OPTIMIZATION RULES (CRITICAL)

### Redis Connection Pool (MUST configure correctly)
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

### Redis Key Patterns (MUST follow)
| Pattern | TTL | Use Case |
|---------|-----|----------|
| `product:{spu_id}:detail` | 5 min | Single product detail |
| `product:{spu_id}:stock` | 30 sec | Real-time stock (short TTL) |
| `products:list:{hash}` | 3 min | Product list cache |
| `categories:tree` | 30 min | Category tree (rarely changes) |
| `categories:all` | 30 min | All categories |
| `category:{id}` | 10 min | Single category |
| `search:{query_hash}` | 15 min | Search results |
| `session:{user_id}` | 7 days | User session |
| `cart:{user_id}` | 7 days | User cart |
| `rate_limit:{ip}:{action}` | 1 min | Rate limiting |

### Redis Anti-Patterns (MUST NOT)
- ❌ **NO `KEYS *` command** in production — use SCAN
- ❌ **NO big keys** (>10KB) — split into smaller chunks
- ❌ **NO N+1 Redis calls** — use Pipeline/MGet
- ❌ **NO cache without TTL** — always set expiration
- ❌ **NO silent failure** — log ALL Redis errors with zap
- ❌ **NO single connection** — always use connection pool
- ❌ **NO `SELECT` in code** — configure DB in connection string

### Redis Pipeline Usage (MUST for batch operations)
```go
// CORRECT: Pipeline for batch operations
pipe := redisClient.Pipeline()
for _, id := range ids {
    pipe.Get(ctx, "product:"+id)
}
cmds, err := pipe.Exec(ctx)
if err != nil {
    logger.Error("redis pipeline failed", zap.Error(err))
}

// CORRECT: MGet for batch reads
vals, err := redisClient.MGet(ctx, keys...).Result()
```

### Redis Error Handling (MUST)
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

### Redis Health Check (MUST)
```go
// In main.go, add Redis health check:
if redisClient != nil {
    healthChecker.AddCheck("redis", func(ctx context.Context) error {
        return redisClient.Ping(ctx).Err()
    })
}
```

---

## 🔥 KAFKA OPTIMIZATION RULES (CRITICAL)

### Kafka Producer Config (MUST use optimized config)
```go
// HIGH THROUGHPUT (for async events like analytics, logs)
producer := kafka.NewWriter(kafka.WriterConfig{
    Addr:         kafka.TCP(brokers...),
    Balancer:     &kafka.LeastBytes{},
    BatchTimeout: 5 * time.Millisecond,
    WriteTimeout: 5 * time.Second,
    Async:        true,
    RequiredAcks: kafka.RequireOne,
    MaxAttempts:  3,
    BatchSize:    100,
    Compression:  kafka.Lz4,
})

// HIGH RELIABILITY (for critical events like orders, payments)
producer := kafka.NewWriter(kafka.WriterConfig{
    Addr:         kafka.TCP(brokers...),
    Balancer:     &kafka.LeastBytes{},
    BatchTimeout: 10 * time.Millisecond,
    WriteTimeout: 10 * time.Second,
    Async:        false,
    RequiredAcks: kafka.RequireAll,
    MaxAttempts:  3,
    BatchSize:    1,
    Compression:  kafka.Lz4,
})
```

### Kafka Consumer Config (MUST)
```go
reader := kafka.NewReader(kafka.ReaderConfig{
    Brokers:         brokers,
    Topic:           topic,
    GroupID:         groupID,
    MinBytes:        1e3,    // 1KB
    MaxBytes:        10e6,   // 10MB
    MaxWait:         500 * time.Millisecond,
    ReadLagInterval: -1,
    CommitInterval:  1 * time.Second,
    StartOffset:     kafka.FirstOffset,
})
```

### Kafka Anti-Patterns (MUST NOT)
- ❌ **NO synchronous produce in hot path** — use Async for non-critical
- ❌ **NO producer per message** — reuse single writer per service
- ❌ **NO consumer per message** — use reader.Read in loop with commit
- ❌ **NO nil producer check missing** — always check producer != nil
- ❌ **NO ignoring write errors** — log and track failed publishes
- ❌ **NO no-comsumer group** — always set GroupID for consumers
- ❌ **NO manual offset commit in auto-commit** — pick one strategy

### Kafka Error Handling (MUST)
```go
// Producer error handling
err := producer.WriteMessages(ctx, msg)
if err != nil {
    logger.Error("kafka publish failed",
        zap.String("topic", topic),
        zap.Error(err),
    )
    metrics.KafkaPublishErrors.Inc()
    // For critical events: retry or write to outbox
}

// Consumer error handling
for {
    msg, err := reader.ReadMessage(ctx)
    if err != nil {
        logger.Error("kafka read failed",
            zap.String("topic", topic),
            zap.Error(err),
        )
        continue // Don't crash on transient errors
    }
    // Process message...
    if err := reader.CommitMessages(ctx, msg); err != nil {
        logger.Error("kafka commit failed", zap.Error(err))
    }
}
```

### Kafka Message Format (MUST)
```go
// Standard message envelope
type Event struct {
    ID        string    `json:"id"`
    Type      string    `json:"type"`
    Timestamp time.Time `json:"timestamp"`
    Payload   []byte    `json:"payload"`
}

// Serialize
data, err := json.Marshal(event)
msg := kafka.Message{
    Key:   []byte(event.ID),   // For partitioning
    Value: data,
    Topic: topic,
}
```

---

## 🐛 BUG FIX WORKFLOW (When fixing bugs)

### Step 1: Reproduce the Bug
1. Read the error message / log output carefully
2. Identify the exact file and line where the bug occurs
3. Write a test case that reproduces the bug (RED)
4. Run the test to confirm it fails: `go test ./... -run TestBugName -v`

### Step 2: Root Cause Analysis
1. Read the code around the bug location
2. Check for common Go bugs:
   - Nil pointer dereference
   - Race condition (needs -race flag)
   - Incorrect error handling (swallowed errors)
   - Off-by-one in slices
   - Goroutine leak (missing context cancellation)
   - Connection leak (not closing response bodies)
3. Use `go vet` and `staticcheck` to find static issues

### Step 3: Fix the Bug
1. Make minimal change — don't refactor
2. Add/update test to cover the fix (GREEN)
3. Run: `go build ./...`
4. Run: `go test ./... -v`
5. Run: `go test -race ./...`

### Step 4: Verify Fix
1. Confirm test passes
2. Check no regression: `go test ./... -race`
3. Check no goroutine leak
4. Update TASK_LOG.md with fix details

---

## Reference Files

- `coding-standards.md` — project coding standards (MUST follow)
- `go.mod` — dependencies and Go version
- `internal/` — private application code
- `pkg/` — public library code
- `api/` — API definitions (protobuf, OpenAPI)
- `packages/go-shared/pkg/redis/` — shared Redis client (read before modifying)
- `packages/go-shared/pkg/kafka/` — shared Kafka config (read before modifying)

## Session Memory

- Record architecture decisions that have been approved
- Record bugs that have been fixed to avoid recurrence
- Record patterns that the team has agreed on

## 🚫 ZERO-TOLERANCE ANTI-FAKE RULES (MỨC CAO NHẤT)

**BẠN TUÂN THỦ CÁC QUY TẮC SAU. VI PHẠM = LOẠI HOÀN TOÀN.**

### 1. KHÔNG BAO GIỜ bịa/fabricate data
- **ĐỪNG** bịa kết quả test, metric, benchmark, hay báo cáo
- **ĐỪNG** viết code rồi nói "đã implement" mà chưa chạy `go build` / `npm run build`
- **ĐỪNG** nói "test passed" nếu chưa thực sự chạy test command
- **ĐỪNG** bịa API response, curl output, hay database query result
- **ĐỪNG** tạo file rồi nói "đã tạo" mà chưa verify file tồn tại
- **ĐỪNG** viết "đã optimize" nếu chưa chạy load test và so sánh metric thực tế

### 2. LUôn verify bằng tool output thực tế
- Mọi claim phải có **tool output** (terminal output, curl response, build log) để chứng minh
- Nếu bạn nói "build pass" → bạn **PHẢI** chạy build command và paste output
- Nếu bạn nói "test pass" → bạn **PHẢI** chạy test command và paste output
- Nếu bạn nói "API return 200" → bạn **PHẢI** chạy curl và paste response
- Nếu bạn nói "deploy OK" → bạn **PHẢI** chạy `docker stack ps` và paste output

### 3. ĐỪNG dùng health check làm proof
- Health check (`curl /health` → 200) **KHÔNG PHẢI** là proof rằng feature hoạt động
- Health check chỉ chứng process sống, không chứng business logic đúng
- **Luôn test actual behavior**: gọi API thật, query DB thật, load test thật

### 4. Nếu không thể verify → nói "KHÔNG THỂ VERIFY"
- Nếu tool fail → report failure, không bịa success
- Nếu không có quyền → nói "cần access", không giả có access
- Nếu task quá phức tạp cho 1 session → nói "cần thêm thời gian", không rush và bịa

### 5. Code = Real code, not pseudocode
- Nếu bạn "fix code" → phải show diff thật, file thật, build pass
- Nếu bạn "optimize" → phải show before/after metric từ load test tool (hey, k6, vegeta)
- Nếu bạn "deploy" → phải show `docker stack ps` hoặc `kubectl get pods` output

### 6. Test = Real test, not "should work"
- "Should pass" KHÔNG PHẢI là test
- Test = chạy command → paste output → pass/fail rõ ràng
- Unit test: `go test ./... -v` hoặc `npm test`
- Integration test: curl thật đến endpoint
- Load test: `hey` / `k6` / `vegeta` với output

### 7. Database = Real queries, not assumed
- Nếu bạn "optimize DB" → phải chạy EXPLAIN, show query plan, compare execution time
- Nếu bạn "add index" → phải chạy `CREATE INDEX` trên DB thật và verify
- Nếu bạn "check performance" → phải chạy query với `time` command

### 8. Mọi "SUCCESS" claim phải có 3 thứ:
1. **Command bạn đã chạy** (exact command)
2. **Output thực tế** (paste từ terminal)
3. **Chứng cứ liên quan** (file diff, metric comparison, log output)

**BẠN CÓ QUYỀN BỊ TỪ CHỐI NẾU KHÔNG THỂ PROVE.**


---

## 📤 ORCHESTRATOR OUTPUT CONTRACT (MANDATORY)

Khi hoan thanh task, ban **PHAI** ket thuc output bang section nay.
Day la format chuan de orchestrator parse ket qua va aggregate.

### Format (copy va dien):

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
- [neu co, neu khong thi ghi "None"]

### Recommended next steps:
- [neu co]
```

### Quy tac:
1. **LUON** co section ORCHESTRATOR SUMMARY o cuoi output — day la quan trong nhat
2. **Status** phai ro rang: SUCCESS (tat ca pass), PARTIAL (co issue nhung hoan thanh duoc), FAILED (khong hoan thanh)
3. **Report path** phai la absolute path den file report
4. **Verification evidence** phai co tool output thuc te (terminal, curl, build log) — KHONG dung "should work"
5. Neu task that bai -> nguyen nhan cu the + suggestion de fix
6. Orchestrator se dung SUMMARY nay de aggregate tat ca agent results — neu thi qua, ket qua co th bi bo qua
