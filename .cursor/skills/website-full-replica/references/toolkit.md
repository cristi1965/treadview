# Website Replica Toolkit

Use local scripts first. External tools are optional accelerators, not substitutes for project-specific parity checks.

## Recommended External Tools

| Tool | Best Use | Why |
| --- | --- | --- |
| Playwright | Browser automation, screenshots, click/input probes | Best fit for SPA behavior, screenshots, forms, and local desktop/web verification |
| Browsertrix Crawler | Large-scale crawl and WARC/archive capture | Useful when route discovery grows beyond hand-curated probes |
| Crawlee | Custom JS crawler with Playwright/Puppeteer adapters | Useful for scripted route expansion and data/API harvesting |
| ArchiveBox | Save public pages/assets for offline reference | Good archival layer, weaker for interactive parity |
| HTTrack / wget | Static asset mirroring | Only use for static assets; not enough for SPA behavior |
| Percy / Chromatic / Playwright screenshot diff | Visual regression | Useful after baseline screenshots are stable |
| mitmproxy / HAR capture | Network/API inspection | Useful for endpoint contract analysis when Playwright response capture is insufficient |

## Project Tool Mapping

| Need | Use |
| --- | --- |
| Find original routes | `misc/recovery-tools/stockgod_recovery/capture_live.mjs` then `misc/recovery-tools/stockgod_recovery/discover_routes.mjs` |
| Capture original DOM/buttons/data | `misc/recovery-tools/stockgod_recovery/capture_live.mjs` |
| Compare page snapshots | `misc/recovery-tools/site-recovery-probe.mjs` |
| Compare endpoint/data shapes | `misc/recovery-tools/stockgod_recovery/build_contracts.mjs` |
| Test clicks and inputs | `misc/recovery-tools/stockgod_recovery/interaction_probe.mjs` |
| Audit freshness/cache headers | `misc/recovery-tools/audit-data-freshness.mjs` |
| Build local replay fixtures | `misc/recovery-tools/stockgod_recovery/build_replay_manifest.mjs` |
| Verify replay coverage | `misc/recovery-tools/stockgod_recovery/verify_replay.mjs` |
| Track remaining gaps | `docs/SITE_RECOVERY_GAPS.md` |

## Full-Parity Checklist

- Routes: public pages, detail pages, gated pages, 404 pages, query variants.
- Shell: header, nav, mobile menu, search, account/watchlist/sidebar rails, theme/language toggles.
- Data: local data files, API endpoints, request cadence, empty/error/loading states.
- Interactions: search, filters, sorting, tabs, expansion, forms, detail links, back paths.
- Visuals: desktop and mobile screenshots at representative breakpoints.
- Desktop: packaged app, deep links, refresh behavior, local storage/DB path when applicable.
- Docs: gap matrix updated with exact artifact path and verification command.
