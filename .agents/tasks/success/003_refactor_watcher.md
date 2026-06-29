# TASK: Refactor internal/adapter/event/watcher.go
## Target File Path: internal/adapter/event/watcher.go
## Current Status: SUCCESS

## Critical Issues Found
- **Resource Leak (Timers)**: Inside `watchNamespace`'s infinite loop, `time.After(5 * time.Second)` is used in a select block for retry delays. This creates a new timer on every retry that is not garbage collected until it fires, which can cause memory leaks in tight retry loops.
- **Unbounded Goroutines**: `w.processPod` sequentially processes container statuses, but could potentially block the watcher if the handler takes too long.

## Action Plan & Refactoring Rules
1. Replace `time.After` in loops with `time.NewTimer` and properly defer `timer.Stop()`, or use a `time.Ticker` for fixed backoffs.
2. Ensure `watchPodEvents` cleanly propagates context cancellation without leaking the underlying watcher stream if the context expires unexpectedly.
