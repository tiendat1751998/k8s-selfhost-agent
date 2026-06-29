# TASK: Refactor internal/adapter/http/websocket.go
## Target File Path: internal/adapter/http/websocket.go
## Current Status: SUCCESS

## Critical Issues Found
- **Unbounded channel blocking**: `h.register` and `h.unregister` channels are unbuffered. If the hub's `Run` loop is busy or blocked, `ServeWS` or `readPump` defers will block indefinitely, causing goroutine leaks.

## Action Plan & Refactoring Rules
1. Buffer `register` and `unregister` channels slightly to prevent `ServeWS` and read/write pumps from blocking immediately on connect/disconnect bursts.
2. Consider using an `errgroup` or tracking active pumps to ensure clean shutdown when the hub is destroyed.
