# Coding Standards

## Language & Framework

| Component | Technology | Version |
|-----------|-----------|---------|
| Backend | Go | 1.26 |
| Router | chi/v5 | v5.2.1 |
| Database | pgx/v5 | v5.7.4 |
| Cache | go-redis/v9 | v9.7.3 |
| Messaging | nats.go | v1.42.0 |
| Logging | zap | v1.27.0 |
| K8s Client | client-go | v0.36.2 |
| Telemetry | OpenTelemetry | v1.44.0 |
| WebSocket | gorilla/websocket | v1.5.4 |
| Config | viper | v1.20.1 |
| Container | docker/docker | v28.5.2 |
| AI Agents | google-adk (Python) | latest |
| Frontend | HTML5/CSS3/Vanilla JS | N/A |

---

## Go Coding Rules

### Error Handling
- Handle all errors explicitly. Never ignore errors with `_`.
- Return errors with context: `fmt.Errorf("failed to %s: %w", action, err)`.
- Use structured logging for error reporting: `logger.Get().Error("msg", zap.Error(err))`.

### Context Propagation
- Propagate `context.Context` through all calls to database pools, HTTP clients, and usecases.
- Set context deadlines on all external API queries.

### Struct Initialization
- Use explicit struct field names. No positional struct initialization.
```go
// Correct
incident := Incident{
    ID:     id,
    Status: "detected",
}
// Incorrect
incident := Incident{id, "detected"}
```

### Imports
- Use Go standard import ordering: stdlib, external, internal.
- Never import from `internal/domain/` into `internal/infrastructure/` or vice versa without port interfaces.

### Naming
- Use descriptive variable names over comments.
- Package names: lowercase, single word.
- Interface names: verb-based (`Repository`, `Publisher`, `Handler`).

---

## Architecture Boundaries

```
Presentation (HTTP/WS) → Usecase → Domain ← Infrastructure (Adapter)
```

- **Domain layer**: Zero external imports. Only pure Go types and port interfaces.
- **Usecase layer**: May import Domain. Coordinates business logic.
- **Adapter layer**: May import Usecase and Domain. HTTP handlers, event listeners.
- **Infrastructure layer**: Implements Domain port interfaces. Database, cache, messaging clients.

### Forbidden Cross-Layer Imports
- Presentation MUST NOT import Infrastructure directly.
- Domain MUST NOT import any other layer.
- Infrastructure MUST NOT import Adapter or Presentation.

---

## Database Query Rules

- All queries use parameter-bound placeholders: `$1, $2, $3`.
- Never use `fmt.Sprintf` or string concatenation for SQL.
- Always `defer rows.Close()` after query execution.
- Use connection pool via pgx pool (max 25 connections).

---

## File Size Limits

| File Type | Max Lines |
|-----------|-----------|
| Go files | 1000 lines |
| JavaScript files | 500 lines |

---

## Forbidden Patterns

- No `// TODO`, `// FIXME`, or placeholder comments.
- No stub methods returning dummy values.
- No unused imports or dead variables.
- No versioned files (`_v2.go`, `_final.js`, `_new.js`).
- No mocks in production business paths (only in `_test.go`).
- No `setTimeout` to simulate async behavior.

---

## Testing Standards

- Table-driven tests for Go.
- Test files co-located with source: `*_test.go`.
- Test both happy path and error cases.
- Use `go test -race ./...` for race condition detection.
- Python tests: `uv run pytest` with 80%+ coverage target.
