---
name: verify-local-quotes
description: >-
  Verify local StockGod/TradingAgents market APIs after quote or stocks changes.
  Use when validating /api/market, /api/quote, /api/stocks, freshness of RKLB/NVDA,
  or before claiming data bugs are fixed.
---

# Verify local quotes

## Steps

1. Ensure backend is running from `app/backend` on `:8765` (or note `BASE_URL`).
2. Run:

```bash
./scripts/verify-local.sh
```

3. If script fails, check in order:
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
