# Go 后端导读（给还不太熟 Go 的同学）

配合源码看：`app/backend/`。本文用流程图说明「请求从哪进、数据从哪来」。

---

## 1. 目录在干什么（先建立地图）

```
app/backend/
├── main.go                 # 程序入口：组装依赖 → 启动 HTTP
├── internal/
│   ├── api/                # HTTP 层（Gin handler，尽量薄）
│   ├── market/             # 实时/快照报价聚合
│   ├── dataflows/          # 外部客户端（TV、Yahoo…）
│   ├── scoring/            # 五方分 panel
│   ├── orchestrator/       # 多智能体分析编排
│   ├── llm/                # Gemini / DeepSeek 等
│   ├── database/           # SQLite、EDGAR/whales 同步
│   ├── agents/             # 各分析师角色
│   ├── config/             # 读 .env
│   └── models/             # 请求/响应结构体
├── data/                   # 本地 JSON 快照（macro、a-market…）
└── cmd/genscores|regenmarket  # 离线工具，不是 Web 服务主路径
```

Go 小知识：

- `package main` + `func main()` = 可执行程序入口。
- `internal/` = **只允许本模块引用**（外部项目 import 不了），用来保护实现细节。
- `c *gin.Context` = 一次 HTTP 请求的上下文（读 query、写 JSON）。

---

## 2. 启动流程

```mermaid
flowchart TD
  A[main.go 启动] --> B[database.InitDB<br/>打开 SQLite]
  B --> C[config.Load<br/>读 .env / 环境变量]
  C --> D[llm.NewClient<br/>按 provider 建客户端]
  D --> E[orchestrator.New<br/>多智能体编排器]
  E --> F[api.SetupRouter<br/>注册路由 + CORS]
  F --> G{环境开关?}
  G -->|STOCKGOD_REPLAY=true| H[只播本地回放<br/>不接真 API]
  G -->|STOCKGOD_LIVE_MIRROR=true| I[反代 stockgod.xyz<br/>默认不要开]
  G -->|默认| J[注册 /api/* 与 /ws]
  J --> K[configureFrontend<br/>托管 frontend/dist]
  K --> L[router.Run :8765]
```

默认模式：自建 API + 可选托管前端静态文件。

---

## 3. 一次 HTTP 请求怎么走

```mermaid
sequenceDiagram
  participant FE as 前端 / curl
  participant Gin as Gin Engine
  participant H as Handler / 包级函数
  participant S as market / scoring / db
  participant Ext as TV / Yahoo / 本地 JSON

  FE->>Gin: GET /api/xxx
  Gin->>Gin: CORS / 中间件
  Gin->>H: 匹配到路由处理函数
  H->>H: 解析 query / 校验参数
  H->>S: 调业务包（不要在 handler 里写死 mock 列表）
  S->>Ext: 拉行情或读快照
  Ext-->>S: 数据
  S-->>H: 结构体 / []byte
  H-->>FE: JSON + 可选 X-Data-Source
```

记忆口诀：**Router 指路 → Handler 接线 → internal 干活 → JSON 回去**。

---

## 4. 行情链路（最重要）

### 4.1 `/api/quote?syms=RKLB,NVDA`（实时）

```mermaid
flowchart LR
  Q["/api/quote"] --> P["market.Provider.Quotes"]
  P --> TV["1 TradingView REST"]
  TV -->|失败/缺票| YF["2 Yahoo quote/chart"]
  YF -->|仍缺| SNAP["3 内存快照<br/>us-stocks→market.json / a-market"]
  TV --> OUT["map symbol → Quote"]
  YF --> OUT
  SNAP --> OUT
```

`Quote` 字段：`price` / `pct` / `prevClose` / `source`（便于排查用的是哪一路）。

### 4.2 `/api/market`（美股宇宙快照）

```mermaid
flowchart TD
  M["GET /api/market"] --> U["读 us-stocks.json 生成 quotes"]
  U -->|成功| W["回写 data/market.json<br/>并返回 JSON"]
  U -->|失败| F["回退读旧 market.json"]
```

与 `/api/stocks?market=us` **同源**，避免「列表新、market 旧」。

### 4.3 `/api/stocks`

```mermaid
flowchart TD
  S["GET /api/stocks"] --> B["绑定 query:<br/>market/sort/page/limit"]
  B --> C{market?}
  C -->|us 默认| US["loadUS：us-stocks.json<br/>+ panel 五方分"]
  C -->|cn / a| CN["loadCN：a-market.json"]
  US --> F["过滤 → 排序 → 分页"]
  CN --> F
  F --> R["JSON + X-Data-Source"]
```

缓存：进程内约 **60 秒**（`stocksCacheTTL`），减少反复解析大 JSON。

---

## 5. 包关系（依赖方向）

```mermaid
flowchart TB
  main --> api
  main --> config
  main --> llm
  main --> orchestrator
  main --> database

  api --> market
  api --> scoring
  api --> models
  api --> orchestrator
  api --> database

  market --> dataflows
  scoring --> llm
  orchestrator --> agents
  orchestrator --> llm
  agents --> llm
```

原则：

- **上层可以依赖下层**（`api` → `market`）
- **下层不要反过来 import `api`**（避免循环依赖）

---

## 6. 两条产品线在后端怎么分

| 前端产品 | 典型 API |
|---|---|
| StockGod UX | `/api/stocks` `/api/quote` `/api/market` `/api/reports` `/api/whales/*` `/api/notes/*` |
| 驾驶舱 | `/api/analysis/*` `/api/trades` `/api/events` `/ws` |

同一 Gin 进程，**按路由分组**，不是两个服务。

---

## 7. 建议你怎么读源码（顺序）

1. `main.go` — 启动拼装（已加中文注释）
2. `internal/api/router.go` — 路由表（已加分区注释）
3. `internal/api/mock_handlers.go` — `/api/quote` `/api/market` 入口
4. `internal/market/provider.go` — 三层降级
5. `internal/api/stocks_handlers.go` — 列表 + cn/us
6. 需要分析功能时再看 `orchestrator` + `agents`

---

## 8. 本地跑起来

```bash
cd app/backend
go test ./internal/market/ ./internal/api/
go build -o /tmp/tradingagents-backend .
STOCKGOD_LOCAL_FRONTEND=true STOCKGOD_REPLAY=false /tmp/tradingagents-backend
```

抽检：

```bash
curl -s 'http://127.0.0.1:8765/api/quote?syms=RKLB' | python3 -m json.tool
curl -sI 'http://127.0.0.1:8765/api/market' | grep -i x-data-source
```

更多验收见 [TESTING.md](./TESTING.md)。
