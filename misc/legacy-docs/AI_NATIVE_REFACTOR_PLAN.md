# AI Native Refactor Plan

本计划用于把 TradingAgents / StockGod 从“功能恢复型仓库”收敛成 AI 原生、可维护、可验证的产品仓库。原则：先降低上下文噪声和验证成本，再做代码分层重构。

## 目标

- 让 AI agent 能快速判断入口、边界、数据权威源和验收命令。
- 让 CI 覆盖当前真实技术栈：Go API、Vite 前端、本地 API smoke。
- 让产品源码、恢复材料、运行产物、数据快照有清晰边界。
- 为后续拆分 `api / app service / domain / infra / ai` 留出稳定路径。

## 非目标

- 不默认反代 `stockgod.xyz`。
- 不在本阶段重写前端页面或后端业务逻辑。
- 不删除用户未确认的大目录、大数据、恢复材料。
- 不提交 `.env`、数据库、二进制、LLM cache。

## 当前状态

| 区域 | 判断 | 风险 |
|---|---|---|
| `AGENTS.md` / `docs/*` | 已具备 AI 协作入口 | 需持续避免文档与代码漂移 |
| `.cursorignore` | 已屏蔽大 JSON / archive | 仅影响 AI 索引，不等于 git 清理 |
| `.github/workflows/ci.yml` | 已改为 Go/Vite/smoke | smoke 依赖本地快照数据完整 |
| `scripts/verify-local.sh` | 已覆盖关键数据契约 | 应随数据 API 变化同步维护 |
| `recovered_source` / `docs/archive` | 仍是主要体积来源 | 不应进入默认上下文或常规 diff |
| `internal/api` | handler 仍偏厚 | 后续逐步抽 service |
| 前端 `pages/stores/utils/types` | 可运行但跨目录找业务 | 后续按 feature 拆分 |

## 阶段 0：冻结基线

验收命令：

```bash
make check
```

运行态验收：

```bash
cd app/backend
go build -o /tmp/tradingagents-backend .
STOCKGOD_LOCAL_FRONTEND=true STOCKGOD_REPLAY=false /tmp/tradingagents-backend

# another shell
./scripts/verify-local.sh
```

完成条件：

- Go 测试通过。
- 前端构建通过。
- `/api/market`、`/api/stocks?market=cn`、`/api/reports` 的 smoke 通过。

## 阶段 1：仓库卫生

建议先生成清单，不直接删除：

```bash
git ls-files | rg 'node_modules|dist/|\.db$|trading-agents$|recovered_source|docs/archive' > /tmp/tracked-noise.txt
du -sh recovered_source docs/archive app/frontend/node_modules 2>/dev/null
```

处理策略：

- `node_modules`、`dist`、二进制、数据库：从 git 追踪中移除，保留 `.gitignore`。
- `recovered_source`：迁移到外部归档、Git LFS 或 artifact storage；主仓只保留 `docs/archive/recovered_source/README.md` 和少量索引。
- `docs/archive`：保留小文档，图片/HTML/response dumps 不进入主仓默认路径。

需要用户确认的操作：

- `git rm --cached` 批量取消追踪。
- 删除或移动任何大目录。
- 清理历史提交中的敏感信息。

## 阶段 2：数据契约

新增 `contracts/`，先用 JSON Schema，不引入复杂生成链：

```text
contracts/
  quote.schema.json
  stocks.schema.json
  market.schema.json
  reports.schema.json
```

落地规则：

- 后端 handler 返回结构必须符合 schema。
- 前端类型从 schema 或 OpenAPI 生成前，手写类型必须引用 schema 字段名。
- `scripts/verify-local.sh` 检查关键字段和代表样本。

优先覆盖：

- `/api/quote?syms=RKLB,NVDA`
- `/api/market`
- `/api/stocks?market=us|cn`
- `/api/reports`

## 阶段 3：后端分层

目标结构：

```text
app/backend/internal/
  transport/http/      # Gin route + request/response binding
  app/                 # use cases: QuoteService, StocksService, ReportsService
  domain/              # Stock, Quote, Report, Score
  infra/marketdata/    # TradingView, Yahoo, snapshot providers
  infra/storage/       # JSON repository, SQLite
  ai/                  # prompts, tool schemas, eval hooks
  jobs/                # refresh/sync commands
```

迁移顺序：

1. 从 `internal/api/stocks_handlers.go` 抽 `StocksService`。
2. 从 `internal/market/provider.go` 抽 provider interface 与 snapshot repository。
3. 从 reports/notes handlers 抽 JSON repository。
4. 将 `dataflows/tools.go` 里的 LLM tool 定义迁入 `internal/ai/tools`。

约束：

- 每次只迁一条 API，不做横向大改。
- 迁移后必须保留旧 endpoint 行为和 JSON shape。
- 每次补或更新对应单测。

## 阶段 4：前端 feature 化

目标结构：

```text
app/frontend/src/
  app/                 # routes, providers, shell wiring
  shared/              # api, format, ui primitives
  features/
    stocks/
      pages/
      components/
      store.ts
      api.ts
      types.ts
    reports/
    portfolio/
    arena/
```

迁移顺序：

1. `features/stocks`：`Scan`、`StockDetail`、`stocksStore`、`stockgodData` 中股票相关逻辑。
2. `features/reports`：reports store/page/components。
3. `features/portfolio`。
4. `features/arena`。

约束：

- 先移动，不重写。
- 每次迁移后跑 `npm run build`。
- 保持 `/notes` 独立壳。

## 阶段 5：AI 原生评测

新增：

```text
evals/
  fixtures/
    quotes.json
    stock-panels.json
    reports.json
  cases/
    scoring.jsonl
    analyst-tools.jsonl
  README.md
```

最低评测：

- LLM tool 调用前必须先取实时 quote。
- 分析输出必须包含数据时间戳和 source。
- 对固定股票样本输出稳定 JSON shape。
- 网络失败时返回可解释的 degraded result。

## 决策记录

后续架构选择写入：

```text
docs/adr/
  0001-data-authority.md
  0002-api-contracts.md
  0003-backend-layering.md
  0004-frontend-feature-slices.md
```

## 当前优先级

1. 保持 `make check` 和 CI 绿。
2. 跑通 `scripts/verify-local.sh`，修掉实际数据契约失败。
3. 清理 git 追踪噪声，但删除/批量取消追踪前必须确认。
4. 建 `contracts/`，把 smoke 从脚本断言升级为契约断言。
