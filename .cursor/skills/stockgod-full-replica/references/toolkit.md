# StockGod Replica Toolkit

Use local scripts first. External tools are optional accelerators, not substitutes for project-specific parity checks.

## Recommended External Tools

| Tool | Best Use | Why |
| --- | --- | --- |
| Playwright | Browser automation, screenshots, click/input probes | Best fit for React/Vite SPA behavior and local Wails verification |
| Browsertrix Crawler | Large-scale crawl and WARC/archive capture | Useful when route discovery grows beyond hand-curated probes |
| Crawlee | Custom JS crawler with Playwright/Puppeteer adapters | Useful for scripted route expansion and data/API harvesting |
| ArchiveBox | Save public pages/assets for offline reference | Good archival layer, weaker for interactive parity |
| HTTrack / wget | Static asset mirroring | Only use for static assets; not enough for SPA behavior |
| Percy / Chromatic / Playwright screenshot diff | Visual regression | Useful after baseline screenshots are stable |
| mitmproxy / HAR capture | Network/API inspection | Useful for endpoint contract analysis when Playwright response capture is insufficient |

## Project Tool Mapping

| Need | Use |
| --- | --- |
| Find original routes | `capture_live.mjs` then `discover_routes.mjs` |
| Capture original DOM/buttons/data | `capture_live.mjs` |
| Compare page snapshots | `site-recovery-probe.mjs` |
| Compare endpoint/data shapes | `build_contracts.mjs` |
| Test clicks and inputs | `interaction_probe.mjs` |
| Build local replay fixtures | `build_replay_manifest.mjs` |
| Verify replay coverage | `verify_replay.mjs` |
| Track remaining gaps | `docs/SITE_RECOVERY_GAPS.md` |

## Full-Parity Checklist

- Routes: public pages, detail pages, gated pages, 404 pages, query variants.
- Shell: header, nav, mobile menu, search, watchlist rail, theme/language toggles.
- Data: `/data/*.json`, `/api/*`, request cadence, empty/error/loading states.
- Interactions: search, filters, sorting, tabs, expansion, forms, detail links, back paths.
- Visuals: desktop and mobile screenshots at representative breakpoints.
- Desktop: Wails packaged app, deep links, refresh behavior, local DB path.
- Docs: gap matrix updated with exact artifact path and verification command.
