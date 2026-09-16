# Data Freshness P0

## Goal

StockGod pages must never present mock or pseudo-live data as real market data. Every product data response should expose when the data was produced, where it came from, whether it is stale, and whether a manual refresh can update it.

## Decisions

- Backend is the source of truth for freshness decisions.
- Frontend reads and displays freshness; it does not infer stale state from filenames or local clocks except for display formatting.
- Existing API bodies stay compatible. Freshness is added through response headers first, with body `meta` only where the endpoint already has an object response.
- Header manual refresh is lightweight: current page plus core quotes, macro, and movers.
- Stale real snapshots can be shown only when they came from a real external source and include data time.
- Mock, fixture, placeholder, random, or hand-written demo data cannot be used by production routes.

## Required Headers

Every product data endpoint should converge on:

- `X-Data-Source`
- `X-Data-Time`
- `X-Data-Refreshed-At`
- `X-Data-Stale`
- `X-Data-Stale-Reason`
- `X-Data-Refreshable`
- `X-Data-Partial-Errors`

## P0 Coverage

First pass covers all main navigation data pages through their shared API paths:

- `/`, `/scan`, `/stock/:sym`: `/api/stocks`, `/api/stocks/search`, `/api/market`, `/api/quote`, `/api/panel-summary`, `/api/heatmap`
- `/reports`: `/api/reports`, `/api/reports/:id`, `/api/market/calendar`, `/api/flash`
- `/macro`, `/dashboard/macro`: `/api/macro`
- `/etf`: `/api/etf/*`, `/api/cn/picks`, `/api/cn/routines`
- `/whales`: `/api/whales/*`
- `/arena`: `/api/arena`
- `/portfolio`: watchlist is local user data; market prices still go through quote APIs
- `/notes`: local authored notes, marked as `local-notes`

## Stale Thresholds

- US and CN quotes: stale when trading-session data is older than 60 seconds; extended/off-session data can be older but must keep `dataTime`.
- Macro: stale after 24 hours.
- Reports: stale after one US trading day unless a live-generated report exists for the current slot.
- ETF analyses: stale after 7 days.
- Hand-maintained strategy seeds such as CN picks/routines: stale after 7 days, even when live quotes are overlaid.
- Whales/13F: stale is based on filing/report period, not minute-level quote freshness.
- Notes and user portfolio data are local authored data and not real-time market data.

## Validation

- `scripts/verify-local.sh` must fail if core product endpoints return `source=mock` or `source=fallback`.
- It must also fail for pseudo-live data without `X-Data-Time`.
- Stale responses are allowed only when explicitly marked with `X-Data-Stale: true` and a reason.
