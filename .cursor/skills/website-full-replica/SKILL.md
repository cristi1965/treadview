---
name: website-full-replica
description: End-to-end public website recovery, cloning, and parity workflow. Use when the user asks to fully clone, recover, replicate, compare, scrape reference behavior, or verify any website's routes, UI, interactions, data contracts, network calls, screenshots, desktop shell behavior, live/local gaps, or source-site parity. Also use for StockGod / TradingAgents site parity work.
---

# Website Full Replica

Use this skill to drive complete parity work between a public reference website and a local implementation. The goal is not a blind proxy: implement local, maintainable routes, data, and interactions unless the user explicitly asks for a proxy or replay mode.

## Guardrails

- Do not bypass auth, paywalls, private APIs, bot protections, or hidden credentials.
- Treat original-site captures as reference artifacts for authorized compatibility work.
- Keep `recovered_source/` out of commits unless the user explicitly asks to preserve an artifact.
- For this repository, preserve the split: StockGod UX routes use `StockGodShell`; TradingAgents cockpit routes use `Layout`.
- Reply in Chinese unless the user requests English.

## Tool Stack

Prefer the local project tools first:

1. `misc/recovery-tools/stockgod_recovery/capture_live.mjs` for HTML, DOM, controls, screenshots, RSC/JSON/API bodies.
2. `misc/recovery-tools/stockgod_recovery/discover_routes.mjs` for expanding route coverage from captured links.
3. `misc/recovery-tools/stockgod_recovery/build_contracts.mjs` for endpoint/data/route contracts.
4. `misc/recovery-tools/site-recovery-probe.mjs` for fast live/local page snapshots.
5. `misc/recovery-tools/stockgod_recovery/interaction_probe.mjs` for click/input/link smoke coverage.
6. `misc/recovery-tools/stockgod_recovery/build_replay_manifest.mjs` and `verify_replay.mjs` only when replay fixtures are needed.
7. `misc/recovery-tools/audit-data-freshness.mjs` for data freshness and API cache/header checks.

Read `references/toolkit.md` when choosing external tools or explaining the stack to the user.

## Inputs To Establish

- `REFERENCE_ORIGIN`: the public source site, for example `https://stockgod.xyz`.
- `LOCAL_ORIGIN`: the local implementation, for example `http://127.0.0.1:8765`.
- `ROUTES`: the initial route list. Prefer explicit product-critical routes from the user; otherwise start with `/`, top nav links, sitemap/robots links, and a few detail pages.
- `OUT_PREFIX`: artifact prefix under `recovered_source/`, for example `stockgod` or a normalized site name.

For this repo's StockGod work, default to:

```bash
REFERENCE_ORIGIN=https://stockgod.xyz
LOCAL_ORIGIN=http://127.0.0.1:8765
OUT_PREFIX=stockgod
ROUTES="/,/scan,/etf,/whales,/arena,/reports,/notes,/portfolio,/macro,/stock/NVDA,/about,/terms,/privacy,/how-to-buy"
```

## Standard Workflow

1. Establish the route list.
   - Include public pages, top nav routes, detail pages, query variants, gated/empty states, and not-found behavior.
   - For unknown sites, run a small first capture before expanding. Do not assume route names.

2. Capture the live original.

```bash
PATH=/Users/jingxin/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH \
node misc/recovery-tools/stockgod_recovery/capture_live.mjs \
  --origin "$REFERENCE_ORIGIN" \
  --out "recovered_source/${OUT_PREFIX}-live-$(date +%Y%m%d)" \
  --routes "$ROUTES"
```

3. Discover more routes and recapture.

```bash
node misc/recovery-tools/stockgod_recovery/discover_routes.mjs \
  --capture "recovered_source/${OUT_PREFIX}-live-$(date +%Y%m%d)" \
  --out "recovered_source/${OUT_PREFIX}-routes-$(date +%Y%m%d).txt" \
  --limit 120

node misc/recovery-tools/stockgod_recovery/capture_live.mjs \
  --origin "$REFERENCE_ORIGIN" \
  --out "recovered_source/${OUT_PREFIX}-live-expanded-$(date +%Y%m%d)" \
  --routes-file "recovered_source/${OUT_PREFIX}-routes-$(date +%Y%m%d).txt" \
  --resume
```

4. Build contracts from the expanded capture.

```bash
node misc/recovery-tools/stockgod_recovery/build_contracts.mjs \
  --capture "recovered_source/${OUT_PREFIX}-live-expanded-$(date +%Y%m%d)" \
  --out "recovered_source/${OUT_PREFIX}-contracts-$(date +%Y%m%d)"
```

5. Probe live and local for visible parity.

```bash
node misc/recovery-tools/site-recovery-probe.mjs \
  --origin "$REFERENCE_ORIGIN" \
  --routes "$ROUTES" \
  --out "recovered_source/site-recovery/${OUT_PREFIX}-live-$(date +%Y%m%d)"

node misc/recovery-tools/site-recovery-probe.mjs \
  --origin "$LOCAL_ORIGIN" \
  --routes "$ROUTES" \
  --out "recovered_source/site-recovery/${OUT_PREFIX}-local-$(date +%Y%m%d)"
```

6. Probe interactions for clicks, inputs, links, and bad responses.

```bash
node misc/recovery-tools/stockgod_recovery/interaction_probe.mjs \
  --origin "$LOCAL_ORIGIN" \
  --routes "$ROUTES" \
  --out "recovered_source/${OUT_PREFIX}-interactions-local-$(date +%Y%m%d).json"
```

7. Audit freshness when a parity issue involves changing data, API cache headers, stale UI, or market-like values. This audit is project-specific; skip it for unrelated websites unless the local implementation uses the same data files.

```bash
node misc/recovery-tools/audit-data-freshness.mjs
BASE_URL=http://127.0.0.1:8765 node misc/recovery-tools/audit-data-freshness.mjs --api
```

8. Implement by priority.
   - P0: route exists, shell identity, primary search/nav, critical data, major forms/buttons, no console errors.
   - P1: filters/sorting/tabs/expand-collapse/detail navigation, endpoint shape parity, empty/loading/error states.
   - P2: copy polish, count exactness, mobile breakpoints, microinteractions, title/meta parity.

9. Verify before claiming done.
   - Run focused probes for changed routes.
   - Run project-specific smoke checks after API/data changes. In this repo, use `./scripts/verify-local.sh` or `audit-data-freshness.mjs --api` for market/data changes.
   - Run `cd app/frontend && npm run build` only for frontend compile-risk changes.
   - Run `cd app/backend && go test ./...` only for backend/data/API changes.
   - Run desktop/Wails validation only for Wails, deep-link, packaging, or static-serving changes.
   - If using a running app, confirm the process is the latest binary before trusting `http://127.0.0.1:8765`.
   - Update `docs/SITE_RECOVERY_GAPS.md` with status and artifact paths.

## What Counts As Complete

For a route or feature to be marked complete:

- Live/local h1/title/main text/controls/links are intentionally matched or the difference is documented.
- All visible buttons either work or are intentionally disabled with matching state.
- Inputs have matching placeholders, type, and submit behavior.
- Network requests are locally served, replayed from an approved fixture, or intentionally replaced by self-built data.
- Desktop Wails build serves the same route without refresh/deep-link failures.
- A probe artifact, build/test log, screenshot, or interaction result proves the claim.
