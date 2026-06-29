# TASK-REFACTOR-08-STANDALONE-WIRING: Wire Mock Handlers in Standalone Mode

## Goal
Wire all missing HTTP platform handlers to their mock repositories in standalone mode to activate all REST routes.

## Scope
- Update `cmd/standalone/main.go` to instantiate all HTTP handlers (such as `NewDriftHandler`, `NewCorrelationHandler`, `NewTaggingHandler`, etc.) using the newly created mock repositories.
- Assign all handlers to the `PlatformHandlers` struct passed to `NewRouterWithWS`.

## Success Criteria
- Standalone server compiles and starts.
- Querying `/api/v1/drift` or `/api/v1/correlation` returns actual mock JSON data rather than 404.
