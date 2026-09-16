# System-wide quality audit - 2026-09-15

## Verdict

The implementation is usable and fail-closed across both product shells. All ten system cases pass. Strict full-system readiness returns HTTP 200 with `status: ok`: all 16 publishable research artifacts pass the evidence-chain validator, while seven explicit `research_unavailable` runs and five pre-audit records remain visible as archives without being misclassified as publishable evidence failures.

| Status | Count |
| --- | ---: |
| PASS | 10 |
| BLOCKED_ENV | 0 |
| FAIL_PRODUCT | 0 |
| Total | 10 |

## Case Results

| ID | Priority | Status | Result and evidence |
| --- | --- | --- | --- |
| SYS-DATA | P0 | PASS | CN overview no longer fabricates prices, catalysts, or action advice. Chat, Copilot, Arena and page stores require source/time/stale metadata and preserve unavailable states. Backend and frontend contract suites pass. |
| SYS-SEC | P1 | PASS | Sensitive research, configuration, Journal/Paper, audit and WebSocket reads share the local-admin boundary. Same-origin HttpOnly session bootstrap works before first fetch; unauthenticated read=401, same-origin session + read=200, Lab WebSocket client count=1. |
| SYS-CONC | P1 | PASS | Search, Arena, selectors and stores use latest-request ownership. WebSocket has one writer per client, deadlines, bounded queues and slow-client removal. Race tests pass. |
| SYS-DUR | P1 | PASS | Journal retry is idempotent; research results use private atomic persistence and corrupted records are surfaced. Process-owned analysis cancellation and shutdown are bounded. |
| SYS-READY | P1 | PASS | Runtime and scripts agree on one managed `127.0.0.1:8765` instance. Full readiness is HTTP 200 with `status: ok`; publishable evidence chains are 16/16, seven explicit unavailable runs remain archived, and malformed or unresolved publishable candidates still fail closed. Historical-research, Paper-engine and live-transport profiles independently pass. |
| SYS-UX | P2 | PASS | Route-local recovery, distinct empty/error/stale states, modal/drawer focus containment, labels, language/title metadata, correct 404 recovery and neutral Pine copy are implemented. Usability retest: 8 PASS, 0 FAIL, 4 intentional not-run write cases. |
| SYS-PERF | P2 | PASS | Routes are lazy chunks; initial shared JS is about 351 KB raw/120 KB gzip. Production `/assets/` compression is verified; quote work and server deadlines are bounded; service logs rotate on managed restart. |
| SYS-ROUTES | P0 | PASS | 96/96 assertions across three viewports; six handled server-error routes remained inside their shells. Evidence: `output/playwright/all-routes-system-wide-final/audit.json`. |
| SYS-ACTIONS | P1 | PASS | Desktop 13/13 and mobile 13/13. Evidence: `output/playwright/module-actions-desktop-system-wide-final/audit.json`, `output/playwright/module-actions-mobile-system-wide-final/audit.json`. |
| SYS-WRITES | P0 | PASS | Evidence-only AAPL dossier exists; isolated Paper desktop/mobile each 28/28; GPU desktop 12/12 and mobile 14/14. No live brokerage path is enabled. |

## Runtime Evidence

- Managed service: PID `14856` after the final rebuild, binary under `~/Library/Application Support/TradingAgents/runtime/`, checkout `app/backend`, one listener on `127.0.0.1:8765`.
- Previous 27 MB log retained as `backend.log.20260915T102720Z`; new launches rotate logs above 10 MB.
- `verify-local.sh`: PASS. It accepts old panel data only when the response explicitly carries `X-Data-Stale: true` and a reason.
- Frontend: 146 Node contract tests + 11 TypeScript tests PASS; TypeScript PASS; production build PASS.
- Backend: `go test ./...` PASS; race tests for API, LLM and orchestrator PASS.
- Real Chromium: `/dashboard`, `/lab`, `/multichart` HTTP 200 with no page exceptions. The final full-readiness probe is HTTP 200 with `status: ok`, and `verify-local.sh` passes. Screenshots: `app/frontend/output/playwright/system-wide-browser-final/`.
- TradingView: two first-viewport widgets rendered in Chromium; unavailable widgets remain explicit display-only empty states. External loading is not treated as backend market evidence.

## Residual Boundaries

- Explicit `research_unavailable` records remain retained for audit history and are reported separately from publishable evidence chains. Malformed, unresolved, or falsely publishable records still invalidate strict readiness.
- Current market-wide, macro, reports, A-share breadth, QDII, ETF holdings and GPU provider data include stale or unconfigured domains. The UI now shows their source/time/reason and blocks dynamic conclusions.
- Safari, Firefox, VoiceOver, Wails native shell, OS-level offline startup, disk-full and process-crash recovery still require dedicated environments; they are not inferred from Chromium success.
