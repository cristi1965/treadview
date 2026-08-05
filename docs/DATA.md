# Data Sources

本地自建数据约定。改行情/盘报前先读本页。

## 权威文件

| 文件 | 用途 | 谁读 |
|---|---|---|
| `app/frontend/public/data/us-stocks.json` | 美股宇宙（价/涨跌/市值） | `/api/stocks?market=us`、`/api/market`、panel |
| `app/frontend/public/data/us-panel-summary.json` | 五方分 | Scan / 热力图 / stocks API |
| `app/backend/data/a-market.json` | A 股 quotes | `/api/a-market`、`/api/stocks?market=cn` |
| `app/backend/data/market.json` | US quote 兜底（由 us-stocks 回写） | `market.Provider` 缓存 |
| `app/backend/data/macro.json` | 首页宏观条 | `/api/macro` |
| `app/backend/data/premarket-movers.json` | 涨跌异动 | `/api/premarket-movers` |
| `app/backend/data/reports-live.json` | 盘报正文 | `/api/reports` |
| `app/frontend/public/data/reports.json` | 前端静态盘报副本 | 构建静态资源 |

## 刷新原则

1. **美股宇宙**：用 TradingView scanner 批量刷新 `us-stocks.json`，再 `npm run build` 或拷到 `dist/data`
2. **market.json**：后端启动/`/api/market` 会从 us-stocks 回写；勿手写陈旧价长期当源
3. **macro**：Yahoo chart 批量更新 `series[].price/pct` 与 `ts`
4. **premarket-movers**：可由 us-stocks 按涨跌幅重建（非真盘前也可标 `session: regular`）
5. **A 股**：当前 `a-market.json` 偏快照；刷新前保持 `quotes` schema：`price/pct/vol/mcapYi`

## 报价优先级

```
TradingView REST → Yahoo quote → Yahoo chart → market.json snapshot
```

`change`（%）与 `change_abs`（$）列不可对调（历史 bug）。

## 品牌文案

报告 JSON 与 UI 统一使用 **「我不是神」**。批量替换时覆盖：

- `app/backend/data/reports-live.json`
- `app/frontend/public/data/reports.json`

## 不要做

- 默认打开 `STOCKGOD_LIVE_MIRROR`
- 把 `llm-panel-cache.json` / `notes-cache/` / `trades.db` 当产品源数据提交
- 在 handler 里写死美股 mock 列表充当 `/api/stocks`

## 新鲜度审计

```bash
node scripts/audit-data-freshness.mjs
BASE_URL=http://127.0.0.1:8765 node scripts/audit-data-freshness.mjs --api
```

审计会检查核心 JSON 的 `generated_at` / `ts` / `updated` 或文件 mtime，并在连接本地服务时确认 `/api/*`、`/data/*` 的 `Cache-Control: no-store` 与 `/api/quote` 的数据源统计。
