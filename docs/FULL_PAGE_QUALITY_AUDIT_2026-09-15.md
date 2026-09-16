# Full-page quality audit - 2026-09-15

## Scope

- Feature/release: all StockGod and TradingAgents routes served by the local Go binary
- Case source: `AppRoutes.tsx`, repository contracts, all-routes/module-actions/usability audits
- Target runtime: Chromium automation at 1440x1000, 1024x900, and 390x844; Quark for TradingView
- Environment/build: `http://127.0.0.1:8765`, production frontend embedded in the Go binary
- Side effects: one AAPL evidence-only research run was persisted; Paper and GPU writes used isolated temporary databases

## Summary

All registered routes and core desktop/mobile interactions pass after repair. Historical research, Paper simulation, and GPU workflows have terminal runtime evidence. Live-market readiness remains degraded because the local market summary is older than the current session; the UI and API expose that state instead of presenting it as live.

| Status | Count |
| --- | ---: |
| PASS | 9 |
| FAIL_PRODUCT | 0 |
| FAIL_CASE | 0 |
| BLOCKED_ENV | 1 |
| OUT_OF_SCOPE | 0 |
| NOT_RUN | 0 |
| Total | 10 |

## Case Results

| ID | Priority | Case | Platform | Status | Actual result | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| routes-96 | P0 | All registered routes, three viewports, handled 5xx, overflow and runtime exceptions | Web | PASS | 96/96 assertions passed; six routes displayed handled server errors | `output/playwright/all-routes-quality-final-20260915/audit.json` |
| actions-desktop-13 | P1 | Module navigation and recovery actions | Desktop Web | PASS | 13/13 passed | `output/playwright/module-actions-desktop-quality-final3-20260915/audit.json` |
| actions-mobile-13 | P1 | Module navigation and recovery actions | Mobile Web | PASS | 13/13 passed | `output/playwright/module-actions-mobile-quality-final3-20260915/audit.json` |
| usability-12 | P1 | Primary tasks, recovery, overflow, and explicit write boundaries | Desktop and mobile Web | PASS | No failed task; 9 passed and three intentionally delegated to dedicated lifecycle evidence | `output/playwright/usability-quality-research-write-20260915/audit.json` |
| research-write | P0 | Dashboard evidence-only generation and run-ID history handoff | Desktop Web | PASS | AAPL run `run-20260915T080436.976Z-fad11e56ef62` persisted as publishable `evidence_only` | `output/playwright/usability-quality-research-write-20260915/audit.json` |
| paper-write | P0 | Authenticated submit, partial fills, OCO, cancel, audit chain, and Journal handoff | Desktop and mobile Web | PASS | 28/28 assertions per viewport; temporary databases only | `output/playwright/paper-lifecycle-desktop/audit.json`, `output/playwright/paper-lifecycle-mobile/audit.json` |
| gpu-write | P1 | Provider fixture, persistence, admin refresh, and responsive rendering | Desktop and mobile Web | PASS | Desktop 12/12; mobile 14/14; temporary databases only | `output/playwright/gpu-runtime-desktop/audit.json`, `output/playwright/gpu-runtime-mobile/audit.json` |
| automated-tests | P0 | Frontend contracts/types/build and backend packages | Local | PASS | Frontend 147/147 tests, TypeScript and Vite build; `go test ./...` and backend build passed | command logs for this audit run |
| tradingview-quark | P1 | Third-party iframe creation and visible chart contents | Quark | PASS | NVDA and TSLA rendered K-line, volume, BOLL, RSI, and MACD with delayed-session disclosure | Quark visual acceptance, 2026-09-15 |
| live-freshness | P0 | Current-session market summary | API | BLOCKED_ENV | `verify-local.sh` correctly rejected `us-panel-summary` generated at `2026-09-14T09:54:19Z` | `scripts/verify-local.sh` output, 2026-09-15 |

## Repaired Product Findings

The statements below describe the pre-repair behavior retained for traceability; final acceptance evidence is listed in the case table above.

### P1 - trust and core workflows

1. Dashboard copy describes evidence-only research but submits the LLM agent endpoint; optional research context is not bound into the invoked audit input.
2. Heatmap can mix live and static rows while assigning a current response time to the whole payload.
3. Sentiment, macro calendar, news, and flash surfaces can discard or replace source observation time and freshness metadata.
4. Deep-provider fallback is not represented in model provenance, and config test can pass while the configured deep provider is unavailable.
5. Analysis failures are not persisted as auditable terminal runs; status can return to idle while an older result remains visible.
6. Batch analysis can overwrite degraded/error state with running/done.
7. History merging is count-based rather than run-ID-based; frontend load failures are rendered as an empty archive.
8. Stock detail aggregates independent requests in one `Promise.all`, so one failure can hide successful sections or leave them loading.
9. Scan, Notes, and ETF detail requests lack stale-response protection during rapid selection changes.
10. Settings aggregates independent readiness calls in one `Promise.all`, expanding one failure to the whole page.
11. TradingView reports ready after a host probe even when the actual iframe remains blank.
12. Historical readiness does not verify the referenced evidence artifact; Paper readiness does not include the admin write gate.
13. Paper research provenance is syntactically checked but not bound to an existing publishable run and matching ticker.

### P2 - usability and semantics

1. Arena market selection does not apply consistently to the comparison universe.
2. Research Lab derives the default date from UTC rather than the user's local calendar date.
3. Desktop navigation can highlight both Dashboard and Macro for `/dashboard/macro`.
4. The ET parser assumes UTC-5 year-round and is one hour wrong during daylight-saving time.
5. The macro gate accepts any non-empty client string and is not a security boundary.

All P1 findings above were repaired with contract or runtime coverage. The P2 Arena scope, local Lab date, exact navigation state, DST-aware ET parsing, and non-security macro routing behavior were also corrected.

## Remaining Risk

- External TradingView rendering still depends on the user's proxy and browser. Quark rendered the widgets during final acceptance; blocked environments retain a visible unavailable state.
- Live-data readiness is currently degraded by the stale 2026-09-14 market summary. Historical research and Paper-engine readiness remain separately labeled and passed independently.
- The repository has extensive pre-existing uncommitted work. Fixes must stay within the named files and must not normalize unrelated changes.

## Approval Context

The user explicitly requested that the audited product be repaired for availability, low learning cost, usability, and trustworthiness. P0/P1 findings above are approved for implementation; P2 items are included only when they have a contained, testable fix.
