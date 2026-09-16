# System-wide quality repair plan - 2026-09-15

## Contract

This plan covers the entire StockGod and TradingAgents system: both frontend shells, every route, every backend route group, data generation and freshness, authentication, WebSocket events, research, Paper, Journal, GPU, build output, startup, and runtime operations.

Completion requires more than a renderable page:

1. No static, stale, incomplete, or fallback observation may be presented as live.
2. Research inputs, progress events, history, Paper state, and Journal data must respect one explicit local-admin boundary.
3. Retried or concurrent requests must not duplicate writes or let an older response replace current state.
4. Persisted research and audit artifacts must be private, atomic, and corruption-visible.
5. Every primary user journey must have distinct loading, empty, unavailable, stale, success, and recovery behavior.
6. Both shells must preserve navigation after local failures and provide usable keyboard/mobile semantics.
7. Readiness and startup scripts must fail closed on incomplete data or the wrong runtime.
8. Production delivery must have bounded request/resource costs and observable lifecycle behavior.

## Workstreams

- [x] Data truth: CN overview, Chat, Copilot, Arena, source/time/stale propagation.
- [x] Security: read authorization matrix, CORS/origin rules, authenticated WebSocket, private artifacts.
- [x] Concurrency and durability: WS writer ownership, request sequencing, idempotent Journal writes, atomic result persistence, workflow cancellation.
- [x] Product readiness: full-profile gate, scripts, unique runtime provenance, honest startup result.
- [x] Whole-frontend usability: route-scoped recovery, modal/drawer focus, labels, lang/title/404 semantics.
- [x] Performance: route chunking, compressed static delivery, bounded quote fan-out and external widgets.
- [x] Runtime quality: server timeouts, background job reporting, log lifecycle, failure injection.
- [x] Final acceptance: unit/race/build, full API matrix, all routes at three viewports, desktop/mobile actions, isolated write lifecycles, Quark/Chromium third-party rendering.

## Final status

Implementation acceptance is complete. Strict full-system readiness is HTTP 200 with `status: ok`: all 16 publishable research artifacts pass the evidence-chain validator, while seven explicit unavailable runs and five legacy records remain visible as archives. The independent historical-research, Paper-engine, and live-transport profiles pass. Stale and unconfigured external domains remain visible and fail closed.

## Priority

1. P0 fabricated or mislabeled market output.
2. P1 private-data exposure, unauthenticated events, false readiness, duplicate writes, lost/corrupt audit records, stale response races.
3. P2 accessibility, first-load cost, offline/restart guidance, timeouts, logs, and lower-risk semantics.

## Approval

The user explicitly requested repair of the entire system and authorized parallel agents. P0/P1 and contained P2 fixes are approved, provided existing unrelated dirty-worktree changes are preserved and external-data unavailability is reported rather than fabricated.
