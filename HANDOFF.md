# TradingAgents / StockGod 交接文档（给 Codex）

> **更新时间**: 2026-08-04  
> **真实仓库**: `/Applications/workspace/ai管力/账本/pxx-trading-Agents`（`TradingAgents` 目录为指向本仓的符号链接）  
> **原则**: 本地自建前端 + 自建数据源；**不要**反代/依赖 `stockgod.xyz` 上游（除非显式开 `STOCKGOD_LIVE_MIRROR`）  
> **AI 入口**: 优先读 [AGENTS.md](./AGENTS.md) · [docs/README.md](./docs/README.md)

## 0. 仓库梳理（2026-08-04）

| 问题 | 处理 |
|---|---|
| Cursor 打开的 `TradingAgents` 几乎空 | 改为符号链接 → `pxx-trading-Agents` |
| `docs/archive/recovered_source` ~18GB（含 node_modules） | 已从磁盘删除并从 git 取消跟踪 |
| 文档入口过多 | `docs/` 只留核心 5+LOCAL；其余进 `docs/archive/` |
| db / 二进制进版本库 | 取消跟踪并写入 `.gitignore` |

日常路径：`AGENTS.md` → `docs/README.md` → 按需打开 ARCHITECTURE/DATA/TESTING。

---

## 0. 30 秒上下文

本仓库同时跑两条产品线：

| 线 | 路由 | 壳 |
|---|---|---|
| **StockGod 克隆** | `/` `/scan` `/etf` `/whales` `/arena` `/reports` `/notes` `/portfolio` `/stock/:sym` | `StockGodShell`（笔记页除外） |
| **TradingAgents 驾驶舱** | `/dashboard` `/journal` `/macro` `/history` `/settings` | 原 `Layout` |

品牌中文已改为 **「我不是神」**（去掉「股」）；英文仍多为 `Not a Stock God`。

---

## 1. 怎么跑

### 推荐（生产式本地）

```bash
# 后端
cd app/backend
go build -o /tmp/tradingagents-backend .
STOCKGOD_LOCAL_FRONTEND=true STOCKGOD_REPLAY=false /tmp/tradingagents-backend
# 监听 :8765，托管 app/frontend/dist

# 前端（改代码后）
export NVM_DIR="$HOME/.nvm"; . "$NVM_DIR/nvm.sh"; nvm use 20   # 需要 Node 20+
cd app/frontend && npm run build
# 硬刷新浏览器；8765 直接吃 dist，一般不用重启后端
```

### 开发（Vite）

```bash
cd app/frontend && npm run dev   # :5173，/api 代理到 :8765
```

### 环境开关（`router.go`）

| 变量 | 作用 |
|---|---|
| （默认） | 自建 API + 本地 `dist` |
| `STOCKGOD_REPLAY=true` | 只播本地回放，不打源站 |
| `STOCKGOD_LIVE_MIRROR=true` | 反代 stockgod.xyz（**默认关，勿开**） |

`.env` 在仓库根；含 LLM Key。**交接时不要把 Key 写进文档/提交。**

---

## 2. 本轮已完成（到 2026-07-31）

### UX / 壳

- 首页热力图：改为源站风格 **分数 × L0–L7 layer-scatter**（`heatmap.ts` / `Heatmap.tsx` / `Home.tsx`）
- **笔记页独立壳**：`/notes` 不再包 `StockGodShell`，奶油色三栏（`Notes.tsx` + `AppRoutes.tsx`）
- **个股次要 chrome**：左侧 `搜索` + `打开观察列表`；抽屉关闭文案 `收起`（`StockGodShell.tsx`）
- 品牌文案：`我不是股神` → `我不是神`

### 盘报

- 从源站同步 `app/backend/data/reports-live.json`（约 70 篇，最新 **07-30**）
- `app/frontend/public/data/reports-latest.json` → `report-20260730-close`
- 日历 `IsToday` 按 **America/New_York** 动态算（`reports_handlers.go`）

### 评分 / 基本面

- 自建五方分：`internal/scoring/`（启发式 + DeepSeek batch top-500）
- 面板：`GET/POST /api/panel-summary`；缓存 `app/backend/data/llm-panel-cache.json`
- 基本面/新闻：Yahoo + 东财/Nasdaq fallback；`/api/fundamentals` `/api/news`

### 报价新鲜度（刚修完，重要）

**根因**：

1. TradingView 列映射反了：`change`(%) 与 `change_abs`($) 对调 → pct 错  
2. Yahoo chart 用 `chartPreviousClose` 经常不等于上一交易日收盘 → RKLB 曾显示 **-7.59%**（应为约 **+10.4%**）  
3. `us-stocks.json` 快照价滞后（RKLB 曾 $83.35，实盘收盘 $64.68）

**修复**：

- `tradingview_rest.go`：列顺序 `close, change, change_abs, ... previous_close`；UA 改 Mozilla  
- `market/provider.go`：`yahooChart` 用日线 closes 末两根算 prev/pct  
- `StockDetail.tsx`：实时价覆盖后按价比缩放 `marketCap`  
- 已用 TV scanner **刷新** `public/data/us-stocks.json`（并拷到 `dist/data/`）  
  - `generated_at`: `2026-07-31 03:31 ET`  
  - RKLB: **64.68 / +10.38% / mcapB 38.69**  
  - 约 5901 成功 / 250 miss

**验收**：

```bash
curl -s 'http://127.0.0.1:8765/api/quote?syms=RKLB'
# 期望 source=tradingview, price≈64.68, pct≈10.38, prevClose≈58.6
```

Vite `:5173` 用户请 **硬刷新**；后端需是包含上述 Go 修复的二进制。

---

## 3. 关键路径速查

```
app/backend/
  main.go
  internal/api/router.go              # 路由 / 前端托管 / 禁止默认同源镜像
  internal/api/reports_handlers.go    # 盘报 + 日历 IsToday
  internal/api/stock_data_handlers.go # panel / fundamentals / news
  internal/market/provider.go         # TV → Yahoo → local snapshot
  internal/dataflows/tradingview_rest.go
  internal/dataflows/yfinance.go
  internal/scoring/{panel,llm_batch,llm_industry}.go
  cmd/genscores/                      # 重生 panel：-seed -llm -llm-industry -limit 500
  data/reports-live.json
  data/llm-panel-cache.json

app/frontend/
  src/pages/{Home,Scan,StockDetail,Notes,Reports}.tsx
  src/components/layout/StockGodShell.tsx
  src/utils/{heatmap,stockgodData,api}.ts
  public/data/us-stocks.json          # 行情快照（需定期刷）
  public/data/us-panel-summary.json
  public/data/reports-latest.json
  dist/                               # 8765 实际托管目录

SITE_RECOVERY_GAPS.md                 # 与源站差距矩阵
recovered_source/                     # 探针截图 / live fixtures
```

---

## 4. 数据流（个股页）

```
StockDetail
  ├─ findUsStock(/data/us-stocks.json)     # 名称/板块/旧价/市值
  ├─ GET /api/stocks/search                # us-stocks.json + panel（已接真源）
  ├─ GET /api/quote?syms=                  # TV优先，权威现价/涨跌
  ├─ GET /api/fundamentals?sym=
  ├─ GET /api/news?sym=
  └─ GET /api/whales/stock/:sym
```

报价链路：`market.Provider.Quotes` = TradingView scanner → Yahoo quote API → Yahoo chart → `us-stocks.json` 派生的 `market.json` 快照（`/api/market` 同源）。

---

## 5. 已知缺口 / 下一步建议

1. ~~`/api/stocks` / `search` mock~~ **已修**（2026-07-31）：读 `us-stocks.json` + panel，60s 缓存。

2. ~~`/api/market` 陈旧 `market.json`~~ **已修**：运行时从 `us-stocks.json` 生成并回写 `data/market.json`（RKLB 与 `/api/stocks` 一致）。

3. ~~`/api/stocks?market=cn` 忽略参数~~ **已修**：读 `a-market.json`（约 5504 只）。

4. ~~macro / premarket 过期~~ **已刷**：macro 用 Yahoo chart 刷新；premarket 按 `us-stocks` 日涨跌异动重建。

5. ~~报告正文「我不是股神」~~ **已替** →「我不是神」（`reports-live.json` / `reports.json`）。

6. **行情快照定期刷新**  
   → 把本次 TV 批量刷新脚本固化为 `cmd/regenusstocks` 或 cron；现在是一次性 Python。A 股 `a-market.json` 仍偏旧。

7. **TV 429**  
   → 已换 UA；若再 429，依赖修好的 yahooChart。

8. **search miss ~250 只**  
   → 需 AMEX/OTC/其它前缀或 Yahoo 补洞。

9. **笔记页 `document.title`** 仍可能是默认「我不是神 · Not a Stock God」，可按文章设 title。

10. **Whales/ETF** 比源站更密（自建数据，属预期）。

11. **勿提交** `.env`、API Key；旧 `HANDOFF.md` 曾含明文 Key，**已应轮换/忽略写入 git**。

---

## 6. 常用命令

```bash
# 重生五方分（贵：DeepSeek）
cd app/backend
go run ./cmd/genscores -seed -llm -llm-industry -limit 500

# 仅 API 刷 panel
curl -X POST 'http://127.0.0.1:8765/api/panel-summary/refresh?llm=1&llm_industry=1&limit=500'

# 报价抽检
curl -s 'http://127.0.0.1:8765/api/quote?syms=RKLB,NVDA' | python3 -m json.tool

# 源站对照探针（需 Playwright + Node20）
# 见 SITE_RECOVERY_GAPS.md
```

---

## 7. 给 Codex 的接手指令（可直接贴）

1. 先确认 `:8765` health 与 `GET /api/quote?syms=RKLB` 是否 `tradingview` + pct≈+10。  
2. 前端以 `app/frontend` 为准；改完 `nvm use 20 && npm run build`，用户硬刷新。  
3. **不要**开 `STOCKGOD_LIVE_MIRROR`；页面必须来自本地 `dist`。  
4. 个股「不新鲜」优先查：TV 列映射、`yahooChart` prev、`us-stocks.json` `generated_at`。  
5. 笔记 `/notes` 保持独立壳，不要重新包回 `StockGodShell`。  
6. 品牌中文用「我不是神」；不要批量删「股票/美股/A股」里的「股」。  
7. 未要求则不要 git commit / push。

---

## 8. 当前运行态（写文档时）

- 后端：`http://127.0.0.1:8765`（`STOCKGOD_REPLAY=false`，托管本地 dist）  
- 前端产物：`dist/assets/index-B2Qt2QU1.js`  
- RKLB API 报价：64.68 / +10.38% / tradingview  
- `us-stocks.json` 已同步同价  

用户若仍看 `:5173` 旧 UI：硬刷新或重启 `npm run dev`。
