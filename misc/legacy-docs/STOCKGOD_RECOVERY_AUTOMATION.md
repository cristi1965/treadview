# StockGod Recovery Automation

This project now has a repeatable live-capture -> contract -> replay -> verify loop for `https://stockgod.xyz`.

## Capture live routes

```bash
/Users/jingxin/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node \
  scripts/stockgod_recovery/capture_live.mjs \
  --origin https://stockgod.xyz \
  --out recovered_source/stockgod-live-capture \
  --timeout 60000
```

Outputs per route:

- `page.html`
- `summary.json`
- `next-flight-chunks.json`
- `response_*.json` / `response_*.txt`
- `screenshot.png`

## Build contracts and replay package

```bash
/Users/jingxin/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node \
  scripts/stockgod_recovery/build_contracts.mjs \
  --capture recovered_source/stockgod-live-capture \
  --out recovered_source/stockgod-contracts

/Users/jingxin/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node \
  scripts/stockgod_recovery/build_replay_manifest.mjs \
  --capture recovered_source/stockgod-live-capture \
  --out recovered_source/stockgod-replay
```

Current replay package covers 860 routes, 27932 captured API/RSC/data responses, and 1775 normalized dynamic RSC response patterns.

Build the local data-store index used for backend/API recovery auditing:

```bash
/Users/jingxin/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node \
  scripts/stockgod_recovery/build_data_store.mjs \
  --replay recovered_source/stockgod-replay \
  --out recovered_source/stockgod-data-store
```

Current data-store index contains 32 API fixtures, 731 quote fixtures, 7 data fixtures, and 27162 RSC/dynamic fixtures.

## Run backend in replay mode

```bash
cd app/backend
go build -o trading-agents main.go
STOCKGOD_REPLAY=true \
STOCKGOD_REPLAY_DIR=/Applications/workspace/ai管力/账本/TradingAgents/recovered_source/stockgod-replay \
./trading-agents
```

For a persistent macOS service:

```bash
UID_NUM=$(id -u)
launchctl bootout gui/$UID_NUM/com.stockgod.replay 2>/dev/null || true
cp /Applications/workspace/ai管力/账本/TradingAgents/scripts/stockgod_recovery/com.stockgod.replay.plist \
  ~/Library/LaunchAgents/com.stockgod.replay.plist
launchctl bootstrap gui/$UID_NUM ~/Library/LaunchAgents/com.stockgod.replay.plist
launchctl kickstart -k gui/$UID_NUM/com.stockgod.replay
```

Stop it with:

```bash
launchctl bootout gui/$(id -u)/com.stockgod.replay
```

Behavior:

- `STOCKGOD_REPLAY=true` serves captured pages/responses from `recovered_source/stockgod-replay`.
- `STOCKGOD_LOCAL_FRONTEND=false` forces StockGod replay/mirror mode instead of the old local frontend.
- Uncovered requests still fall back to the live `https://stockgod.xyz` proxy, so navigation keeps working while recovery expands.
- Replay hits include `X-StockGod-Replay: hit`.
- Missing Next.js JS/CSS chunks are served locally as empty 200 responses with `X-StockGod-Replay: static-fallback`, preventing stale deployment chunk 404s from breaking verification.
- `POST` requests to replayable app/API paths fall back to captured GET fixtures when available; `OPTIONS` receives 204 locally.

## Verify replay quality

```bash
/Users/jingxin/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node \
  scripts/stockgod_recovery/verify_replay.mjs \
  --origin http://localhost:8765 \
  --baseline-capture recovered_source/stockgod-full-capture \
  --min-text-ratio 0.9 \
  --out recovered_source/stockgod-replay-verify.json
```

Core verification passed for:

`/`, `/scan`, `/etf`, `/whales`, `/whales/howard-marks`, `/arena`, `/reports`, `/portfolio`, `/stock/NVDA`, `/stock/UNH-B`, `/about`, `/terms`, `/privacy`, `/how-to-buy`.

Expanded verification also passed for:

`/whales/bill-ackman`, `/whales/duan-yongping`, `/stock/AAPL?market=us`, `/stock/MSFT?market=us`, `/stock/600519?market=a`, `/stock/300750?market=a`.

Final broad verification also passed for representative core, whale, LHB, US stock, A-share stock, highlight, and static routes:

`/whales/warren-buffett`, `/whales/william-von-mueffling`, `/whales/lhb-f7f1d351`, `/stock/NVDA?market=us`, `/stock/TSLA?market=us`, `/stock/603986?market=a`, `/stock/601888?market=a`, `/stock/920982?market=a`, `/?highlight=AAPL`, `/?highlight=600519`.

All checked routes rendered with expected H1/text content, local replay hits, zero bad local responses, and zero relevant console errors.

Strict verification on 2026-07-11 passed for:

`/`, `/scan`, `/etf`, `/whales`, `/whales/warren-buffett`, `/whales/lhb-f7f1d351`, `/arena`, `/reports`, `/portfolio`, `/stock/NVDA?market=us`, `/stock/UNH-B`, `/?highlight=AAPL`, `/about`, `/terms`, `/privacy`, `/how-to-buy`.

Result: every route had replay hits, no navigation errors, no bad responses, no console errors, and text ratio >= 0.9 against `recovered_source/stockgod-full-capture`. Output file: `recovered_source/stockgod-strict-verify.json`.

Interaction verification:

```bash
/Users/jingxin/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node \
  scripts/stockgod_recovery/interaction_probe.mjs \
  --origin http://localhost:8765 \
  --routes '/,/scan,/whales,/arena,/reports,/portfolio,/stock/NVDA?market=us,/stock/UNH-B' \
  --out recovered_source/stockgod-interaction-probe.json
```

Result: 8 representative routes rendered, exposed internal links/buttons/inputs, tolerated button clicks and search input filling, and produced zero bad responses / zero console errors.

## Discover more routes

```bash
/Users/jingxin/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node \
  scripts/stockgod_recovery/discover_routes.mjs \
  --capture recovered_source/stockgod-live-capture \
  --out recovered_source/stockgod-discovered-routes.txt \
  --limit 65 \
  --whales 30 \
  --us-stocks 25 \
  --a-stocks 10
```

Use the discovered route list with `capture_live.mjs --routes` to expand coverage.

## Full Public Route Capture

The current full capture was produced from all public routes discovered from the live pages:

```bash
/Users/jingxin/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node \
  scripts/stockgod_recovery/capture_live.mjs \
  --origin https://stockgod.xyz \
  --routes-file recovered_source/stockgod-discovered-all-routes.txt \
  --out recovered_source/stockgod-all-capture \
  --timeout 45000 \
  --concurrency 4 \
  --resume \
  --no-screenshots
```

Routes that failed due to transient network/navigation errors were retried with:

```bash
/Users/jingxin/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node \
  scripts/stockgod_recovery/capture_live.mjs \
  --origin https://stockgod.xyz \
  --routes-file recovered_source/stockgod-failed-routes.txt \
  --out recovered_source/stockgod-retry-capture \
  --timeout 70000 \
  --concurrency 2 \
  --no-screenshots
```

Then merged:

```bash
/Users/jingxin/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node \
  scripts/stockgod_recovery/merge_captures.mjs \
  --base recovered_source/stockgod-all-capture \
  --overlay recovered_source/stockgod-retry-capture \
  --out recovered_source/stockgod-full-capture
```

`recovered_source/stockgod-full-capture` currently has 860/860 usable routes.
