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
TradingView REST → Yahoo quote → Yahoo chart → stale real snapshot
```

`change`（%）与 `change_abs`（$）列不可对调（历史 bug）。

最后一档 snapshot 只能使用真实外部源曾经拉取并落盘的数据，必须带来源和时间；禁止 mock、手写占位、随机数据、演示数据进入生产接口。
`/api/quote` 的顶层 `dataTime`、`ts` 和 `X-Data-Time` 必须来自本次返回对象的供应商观测时间；混合来源取最早观测。任一返回对象缺少供应商时间时，聚合时间保持空值，绝不能改用快照批次时间或响应完成时间。供应商只给交易日时，对象和顶层均返回 `timeGranularity=date`、`dataTime=YYYY-MM-DD`、`ts=0`，不补零点伪造秒级精度。

## 财务披露与 evidence-only 研究

- 美股财务优先使用 SEC companyfacts；Nasdaq financials 回退必须再与 SEC submissions 或 Nasdaq SEC filings 中同 symbol、同 10-Q/10-K、同 period 的记录匹配。
- SEC companyfacts 成功响应会原子写入 `TRADINGAGENTS_CACHE_DIR/sec-companyfacts/`，记录原始 URL、抓取时间和 SHA-256。最多 24 小时内可作为 `verified-cache` 复用；URL/CIK/hash 不匹配、过期或内容损坏时必须拒绝。`sourceFetchedAt/sourceContentHash/sourceTransport` 与本次完成时间分开披露。
- `filingDate`、`accession` 与 `sourceURL` 取不到时，财务接口必须标 stale/unavailable，且研究 PIT preflight 不通过。`nasdaq-ref:*` 是明确命名的 Nasdaq/QuoteMedia filing reference，不伪装为 SEC accession number。
- `POST /api/analysis/evidence-only` 是受管理员写权限保护的无 LLM 路径。它只在 dated Nasdaq OHLCV 历史、多期 filing-bound 财务和逐条带时间新闻通过 PIT 门禁后落盘。最新已完成交易日的 historical close 是唯一主研究价；quote 不是必需证据，若取得只作可选 cross-check，缺失或冲突记入 gap 但不阻断 dossier。quote 与 historical 可能共享 Nasdaq 数据链，不声称为独立验证。
- v5 dossier 为每个可用 SEC/XBRL 字段保留 `periodStart/periodEnd/form/filed/accession/sourceURL/unit/periodKind`；10-Q 只有 70-110 日的离散 duration 才可作为季度流量值，10-K 只有 330-400 日 duration 才可作为年度值。YTD 不得冒充单季，缺字段级绑定的 fallback 数值保持 `null/unknown`。
- dossier 目标深度是最多 5 个离散季度和 3 个年度，并公开毛利率、营业利润率、净利率、负债率、环比/同比、TTM、现金变化以及 1/5/20 日收益、年化波动、最大回撤、20 日 ADV 的公式、精确输入和 evidence ID。QoQ/YoY 只比较同频且期间长度可比的事实；TTM 只汇总四个连续离散季度。不足 21 个收盘观测时相关值为 `unknown`；21 个观测可直接产生 20 个日收益并计算年化波动率。
- 请求可附带 `research_context`（`mandate/holdings/liquidity/tax/risk_budget`）；原值进入 dossier 和 `input_hash`。每项都可省略，但省略项必须成为 gap，不允许推测组合约束。
- 新闻按 ticker/公司名相关性过滤，再做标题/链接去重、来源等级、事实/观点和主题分类；保留过滤规则和排除数，不从情绪生成买卖结论。
- 产物状态为 `evidence_only`，结论只能是 `OBSERVE` 或 `ABSTAIN`；它不是投资建议，也不是 10-Agent 分析。原始工具响应仍以 hash 校验的私有 evidence artifact 保存。
- 五因子 `local-heuristic-v2-deterministic-scaling` 只是固定缩放，不是历史校准；无样本外报告时始终为 `unvalidated`。重新生成命令：`cd app/backend && go run ./cmd/genscores -industry=false -seed=false`。

### 每次刷新要最新价

- 后端启动后后台循环刷美股 live 缓存（`market.StartLiveRefresh`），`/api/market` 的 `X-Data-Source` 形如 `live-tv@…`
- 主动刷新：`POST /api/market/refresh?top=500`（或 `?full=1`）
- 前端列表/热力图：种子宇宙仍可读 `us-stocks.json`（名称/板块），**价格一律合并 `/api/market` + `/api/quote`**
- 桌面 Wails embed 的 `/data/us-stocks.json` 不会自动变新；必须走 API live 路径

## 品牌文案

报告 JSON 与 UI 统一使用 **「我不是神」**。批量替换时覆盖：

- `app/backend/data/reports-live.json`
- `app/frontend/public/data/reports.json`

## 不要做

- 默认打开 `STOCKGOD_LIVE_MIRROR`
- 把 `llm-panel-cache.json` / `notes-cache/` / `trades.db` 当产品源数据提交
- 在 handler 里写死美股 mock 列表充当 `/api/stocks`
- 让 `source=mock`、`source=fallback` 或没有数据时间的 pseudo-live 响应进入产品页面

## Freshness 合同

后端统一裁决 freshness，前端只展示结果。产品数据接口逐步统一这些响应头：

| Header | 含义 |
|---|---|
| `X-Data-Source` | 当前数据来源，例如 `tradingview`、`yahoo`、`stale-snapshot`、`reports-live` |
| `X-Data-Time` | 数据本身的生成/抓取时间 |
| `X-Data-Refreshed-At` | 本次接口响应或刷新完成时间 |
| `X-Data-Stale` | `true`/`false` |
| `X-Data-Stale-Reason` | stale 原因；新鲜数据为空 |
| `X-Data-Refreshable` | 是否可由手动刷新按钮更新 |
| `X-Data-Partial-Errors` | 局部刷新失败项，逗号分隔 |

旧 API body 保持兼容；只有对象型响应可以额外带 `meta` 字段。页面核心数据区遇到无真实数据源、无 `dataTime`、或生产路由返回 mock/fallback 时应展示不可用状态，而不是继续渲染默认 0 值。

## Stale 阈值

| 数据域 | 阈值 |
|---|---|
| 美股/A 股行情 | 交易时段超过 60 秒 stale；盘前盘后可放宽但必须显示数据时间 |
| Macro | 24 小时 |
| Reports | 1 个美股交易日；live-generated 盘报按当前 slot 判断 |
| ETF analyses | 7 天 |
| 人工维护策略种子（CN picks/routines） | 7 天 |
| Whales / 13F | 按披露期，不按分钟级行情判断 |
| Notes / Portfolio | 本地用户或作者数据，不按实时行情判断 |

## 新鲜度审计

```bash
node misc/recovery-tools/audit-data-freshness.mjs
BASE_URL=http://127.0.0.1:8765 node misc/recovery-tools/audit-data-freshness.mjs --api
```

审计会检查核心 JSON 的 `generated_at` / `ts` / `updated` 或文件 mtime，并在连接本地服务时确认 `/api/*`、`/data/*` 的 `Cache-Control: no-store` 与 `/api/quote` 的数据源统计。
