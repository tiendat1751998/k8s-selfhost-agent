# TASK: Refactor cmd/server/main.go
## Target File Path: cmd/server/main.go
## Current Status: SUCCESS

## Critical Issues Found
- **Context propagation gaps**: Database, redis, and NATS connections are established using the root context or background context, but nested lifecycle operations (e.g., graceful shutdown) do not cleanly propagate execution context to allow downstream cancel tracking.
- **Naked database connection closers**: Several connections are deferred without checking for return error results or handling panics during the synchronization sync loop shutdown phases.
- **Goroutine Leak in server listener**: If initialization fails after the HTTP server goroutine has been started, the HTTP listener does not clean up its resources, leading to potential background listener leaks in test suites.

## Action Plan & Refactoring Rules
1. Unify and pass down context values properly into `postgres.NewClient`, `redis.NewClient`, and `nats.NewClient`.
2. Wrap deferred calls (`pgClient.Close()`, `redisClient.Close()`, `natsClient.Close()`) with proper error logging and panic recoveries.
3. Establish a coordinated lifecycle controller (using context cancel or errgroup) that ensures if any dependency component fails to start or crashes, all other background components and HTTP listeners are cleanly and automatically shut down.
