---
name: verify-local-quotes
description: >-
  Verify local StockGod/TradingAgents market APIs and data freshness after quote,
  stocks, market snapshot, cache, macro, ETF, or reports changes. Use when
  validating /api/market, /api/quote, /api/stocks, RKLB/NVDA freshness,
  no-store headers, or before claiming data bugs are fixed.
---

# Verify local quotes

## Steps

1. Ensure backend is running from `app/backend` on `:8765` (or note `BASE_URL`).
2. Run:

```bash
./scripts/verify-local.sh
```

3. For data freshness, cache headers, or stale UI claims, also run:

```bash
node misc/recovery-tools/audit-data-freshness.mjs
BASE_URL=http://127.0.0.1:8765 node misc/recovery-tools/audit-data-freshness.mjs --api
```

4. If a script fails, check in order:
   - Process is latest binary (`go build -o /tmp/tradingagents-backend .`)
   - CWD when starting is `app/backend`
   - `app/frontend/public/data/us-stocks.json` `generated_at`
   - TV column mapping / Yahoo chart prev logic in `internal/market` + `tradingview_rest.go`

4. Optional live quote:

```bash
curl -s 'http://127.0.0.1:8765/api/quote?syms=RKLB,NVDA'
```

## Pass criteria

- `/api/market` `X-Data-Source` starts with `us-stocks`
- `/api/stocks?market=cn` symbols are numeric A-share codes
- Reports body has zero `我不是股神`
- `/api/*` and `/data/*` data responses use `Cache-Control: no-store` when freshness/cache was changed
- Known stale source files are reported explicitly instead of hidden by UI or browser cache
