# TASK: Refactor internal/usecase/rca/worker.go
## Target File Path: internal/usecase/rca/worker.go
## Current Status: SUCCESS

## Critical Issues Found
- **Uncoordinated Goroutine Shutdown**: The worker creates goroutines for message consumption (`go func() { w.processMessage(ctx, msg) }`) without providing a timeout context that is bound to the worker's lifecycle. During shutdown, `w.wg.Wait()` might block indefinitely if a NATS call hangs.
- **Error Handling**: `msg.NakWithDelay` and `msg.Ack` errors are ignored (naked returns/ignores).

## Action Plan & Refactoring Rules
1. Use `context.WithTimeout` or `context.WithCancel` specific to message processing, tied to the main worker context.
2. Handle the returned errors from `msg.NakWithDelay` and `msg.Ack` by logging them properly.
