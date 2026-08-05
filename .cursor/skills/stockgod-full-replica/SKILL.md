---
name: stockgod-full-replica
description: End-to-end StockGod original-site recovery and parity workflow. Use when the user asks to fully clone, recover, replicate, compare, or verify stockgod.xyz features, routes, UI, interactions, data contracts, network calls, screenshots, Wails desktop behavior, or live/local gaps for TradingAgents / StockGod.
---

# StockGod Full Replica

Use this skill to drive complete StockGod parity work inside this repository. The goal is not a blind proxy of `stockgod.xyz`; keep the project rule: self-built frontend and self-built/local data unless the user explicitly enables live mirror mode.

## Guardrails

- Do not bypass auth, paywalls, private APIs, bot protections, or hidden credentials.
- Treat original-site captures as reference artifacts for authorized compatibility work.
- Keep `recovered_source/` out of commits unless the user explicitly asks to preserve an artifact.
- Preserve the split: StockGod UX routes use `StockGodShell`; TradingAgents cockpit routes use `Layout`.
- Reply in Chinese unless the user requests English.

## Tool Stack

Prefer the local project tools first:

1. `scripts/stockgod_recovery/capture_live.mjs` for HTML, DOM, controls, screenshots, RSC/JSON/API bodies.
2. `scripts/stockgod_recovery/discover_routes.mjs` for expanding route coverage from captured links.
3. `scripts/stockgod_recovery/build_contracts.mjs` for endpoint/data/route contracts.
4. `scripts/site-recovery-probe.mjs` for fast live/local page snapshots.
5. `scripts/stockgod_recovery/interaction_probe.mjs` for click/input/link smoke coverage.
6. `scripts/stockgod_recovery/build_replay_manifest.mjs` and `verify_replay.mjs` only when replay fixtures are needed.

Read `references/toolkit.md` when choosing external tools or explaining the stack to the user.

## Standard Workflow

1. Establish the route list.
   - Start with `/`, `/scan`, `/etf`, `/whales`, `/arena`, `/reports`, `/notes`, `/portfolio`, `/macro`, `/stock/NVDA`, `/about`, `/terms`, `/privacy`, `/how-to-buy`.
   - Include discovered detail routes for `/stock/*`, `/whales/*`, ETF details, gated pages, and not-found behavior.

2. Capture the live original.

```bash
PATH=/Users/jingxin/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH \
node scripts/stockgod_recovery/capture_live.mjs \
  --origin https://stockgod.xyz \
  --out recovered_source/stockgod-live-$(date +%Y%m%d) \
  --routes /,/scan,/etf,/whales,/arena,/reports,/notes,/portfolio,/macro,/stock/NVDA,/about,/terms,/privacy,/how-to-buy
```

3. Discover more routes and recapture.

```bash
node scripts/stockgod_recovery/discover_routes.mjs \
  --capture recovered_source/stockgod-live-$(date +%Y%m%d) \
  --out recovered_source/stockgod-routes-$(date +%Y%m%d).txt \
  --limit 120

node scripts/stockgod_recovery/capture_live.mjs \
  --origin https://stockgod.xyz \
  --out recovered_source/stockgod-live-expanded-$(date +%Y%m%d) \
  --routes-file recovered_source/stockgod-routes-$(date +%Y%m%d).txt \
  --resume
```

4. Build contracts from the expanded capture.

```bash
node scripts/stockgod_recovery/build_contracts.mjs \
  --capture recovered_source/stockgod-live-expanded-$(date +%Y%m%d) \
  --out recovered_source/stockgod-contracts-$(date +%Y%m%d)
```

5. Probe live and local for visible parity.

```bash
node scripts/site-recovery-probe.mjs \
  --origin https://stockgod.xyz \
  --routes /,/scan,/etf,/whales,/arena,/reports,/notes,/portfolio,/macro,/stock/NVDA \
  --out recovered_source/site-recovery/live-full-$(date +%Y%m%d)

node scripts/site-recovery-probe.mjs \
  --origin http://127.0.0.1:8765 \
  --routes /,/scan,/etf,/whales,/arena,/reports,/notes,/portfolio,/macro,/stock/NVDA \
  --out recovered_source/site-recovery/local-full-$(date +%Y%m%d)
```

6. Probe interactions for clicks, inputs, links, and bad responses.

```bash
node scripts/stockgod_recovery/interaction_probe.mjs \
  --origin http://127.0.0.1:8765 \
  --routes /,/scan,/etf,/whales,/arena,/reports,/notes,/portfolio,/macro,/stock/NVDA \
  --out recovered_source/stockgod-interactions-local-$(date +%Y%m%d).json
```

7. Implement by priority.
   - P0: route exists, shell identity, primary search/nav, critical data, major forms/buttons, no console errors.
   - P1: filters/sorting/tabs/expand-collapse/detail navigation, endpoint shape parity, empty/loading/error states.
   - P2: copy polish, count exactness, mobile breakpoints, microinteractions, title/meta parity.

8. Verify before claiming done.
   - Run `cd app/frontend && npm run build`.
   - Run `cd app/backend && go test ./...`.
   - Run `make build-desktop` for Wails changes or route/static changes.
   - Start Wails app and confirm `curl -fsS http://127.0.0.1:8765/api/health`.
   - Re-run focused probes for changed routes.
   - Update `docs/SITE_RECOVERY_GAPS.md` with status and artifact paths.

## What Counts As Complete

For a route or feature to be marked complete:

- Live/local h1/title/main text/controls/links are intentionally matched or the difference is documented.
- All visible buttons either work or are intentionally disabled with matching state.
- Inputs have matching placeholders, type, and submit behavior.
- Network requests are locally served, replayed from an approved fixture, or intentionally replaced by self-built data.
- Desktop Wails build serves the same route without refresh/deep-link failures.
- A probe artifact, build/test log, screenshot, or interaction result proves the claim.
