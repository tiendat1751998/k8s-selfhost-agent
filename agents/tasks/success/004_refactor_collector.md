# TASK: Refactor internal/adapter/event/collector.go
## Target File Path: internal/adapter/event/collector.go
## Current Status: SUCCESS

## Critical Issues Found
- **Sequential blocking I/O**: `Collect()` fetches logs, events, describe, owner resources, service YAML, ingress YAML, and node metrics sequentially. This creates a massive latency bottleneck for incident analysis.
- **Potential unbounded memory allocation**: `io.ReadAll(stream)` reads the entire log stream into memory at once without a size limit, which can cause OOM if the logs are exceptionally large despite `TailLines`.

## Action Plan & Refactoring Rules
1. Implement an `errgroup.Group` or concurrent Goroutines to fetch logs, events, metrics, and YAMLs in parallel. Wait for all to complete with a timeout context.
2. Replace `io.ReadAll` with `io.LimitReader` when reading from the log stream to bound memory usage securely.
