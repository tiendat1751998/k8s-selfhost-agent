# TASK-OPTIMIZATION-05-WEBSOCKET: WEBSOCKET OPTIMIZATION

## Goal
Optimize websocket subscription handling, add heartbeat/reconnection checks in `ws-client.js`, and prevent subscription leaks.

## Success Criteria
- Offline/online state transitions reliably.
- WebSocket reconnects with jittered backoff without flooding connection storms.
