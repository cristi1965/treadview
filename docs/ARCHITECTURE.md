# Architecture

## 运行拓扑

```
浏览器
  ├─ Vite :5173 (dev) ──proxy /api──► Go :8765
  └─ 或直接访问 Go :8765（托管 app/frontend/dist）
```

## 双产品线

| 线 | 壳 | 后端重点 |
|---|---|---|
| StockGod UX | `StockGodShell`（`/notes` 独立壳） | market / stocks / reports / whales / etf / notes |
| TradingAgents 驾驶舱 | `Layout` | analysis / trades / events / WS |

路由入口：

- 前端：`app/frontend/src/AppRoutes.tsx`
- 后端：`app/backend/internal/api/router.go`

## 后端分层

```
main.go
  → config + llm + orchestrator + Gin router
internal/api/          HTTP handlers（薄）
internal/market/       报价聚合（TV → Yahoo → snapshot）
internal/dataflows/    外部数据客户端
internal/scoring/      五方分与 panel
internal/orchestrator/ 多智能体分析编排
internal/database/     SQLite + EDGAR/whales 同步
internal/agents/       分析师 prompt/角色
```

## 行情数据流

1. **`GET /api/quote?syms=`**  
   TradingView → Yahoo quote/chart → `market.json`（由 `us-stocks.json` 回写）

2. **`GET /api/market`**  
   优先从 `us-stocks.json` 生成 quotes；失败才读旧 `market.json`

3. **`GET /api/stocks?market=us|cn`**  
   - `us`：`us-stocks.json` + `us-panel-summary.json`  
   - `cn`：`a-market.json`

4. **`GET /api/macro` / `/api/premarket-movers`**  
   本地快照文件（可用脚本刷新）

## 前端数据

- 静态宇宙：`app/frontend/public/data/*`（build 进 `dist/data`）
- 运行时 API：`src/utils/api.ts`、各 store

## 环境开关

| 变量 | 作用 |
|---|---|
| （默认） | 自建 API + 本地 dist |
| `STOCKGOD_REPLAY=true` | 只播本地回放 |
| `STOCKGOD_LIVE_MIRROR=true` | 反代源站（默认关） |
| `PORT` | 后端端口（默认 8765） |

## 改动建议

- 新 HTTP 接口：只加在 `internal/api`，业务逻辑下沉到 `market` / `scoring` / `dataflows`
- 新行情源：实现客户端放 `dataflows`，接入 `market.Provider`
- 勿在 handler 里硬编码 mock 列表（历史坑）
