# TASK: Refactor cmd/standalone/main.go
## Target File Path: cmd/standalone/main.go
## Current Status: SUCCESS

## Critical Issues Found
- **Resource leaks in mock generators**: Background goroutines `generateOperationalData` and `generateEnterpriseData` run forever in an infinite loop without listening to the parent execution context context cancellation cleanly.
- **Unseeded or uncoordinated random usage**: The use of global `rand.Intn` calls inside concurrent goroutines can lead to lock contention on the global math/rand source.
- **Hardcoded UI payloads**: Large mock data arrays are declared in place in handlers, causing unnecessary allocation overhead on every initialization.

## Action Plan & Refactoring Rules
1. Ensure all background loop generators (e.g., `generateOperationalData`, `generateEnterpriseData`) monitor the passed context `ctx` in all select blocks (including tickers and sends).
2. Utilize local, concurrent-safe random number generation (`rand.New(rand.NewSource(...))`) or use Go 1.22+ `math/rand/v2` equivalent structures to avoid global lock contention.
3. Extract static mock arrays to global read-only variables or separate mock-data files, separating the generation logic from setup code.
