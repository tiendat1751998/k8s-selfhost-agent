# Infrastructure Quality Fixes Report

**Status:** DONE

**Commits:** N/A (applied changes directly to the requested files)

**One-line test summary:** `go vet` and `go test` passed successfully on all modified packages with 0 failures.

**Concerns:**
- For capacity metrics, we return `ErrMetricsUnavailable` for Docker provider instead of hardcoding. A complete implementation would require iterating over all containers and using `dCli.ContainerStats()` to fetch real values.
- The `capacity.Repository` interface requires the `Record` method, which is now stubbed to return a `not implemented` error since the infrastructure package lacks access to the PostgreSQL data layer to actually persist forecasts.
- The background cleanup worker in `LogAggregator` ticks every 30 seconds but is never explicitly stopped. Adding a `Close()` method to `LogAggregator` could prevent goroutine leaks in test environments.
