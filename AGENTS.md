# AGENTS.md — TradingAgents / StockGod（本地自建）

> **真实仓库**：`/Applications/workspace/ai管力/账本/pxx-trading-Agents`  
> （旁路 `TradingAgents` 为指向本仓的符号链接）  
> 文档：`docs/README.md` · 非业务：`misc/README.md`

给 AI coding agent 的入口。先读本文件，再按需打开 `docs/`。

## 这是什么

| 线 | 路由 | 说明 |
|---|---|---|
| **历史研究 + Paper** | `/` → `/dashboard`，以及 `/history` `/tactical` `/settings` | 当前主产品 |
| **StockGod 实验行情** | `/market` `/scan` `/stock/:sym` `/reports` `/notes` … | 实时门禁通过前仅作实验 |

**硬原则**：自建前端 + 自建数据；不要默认反代 `stockgod.xyz`。

## 仓库结构（规整后）

```
├── AGENTS.md / docs/       # 入口与稳定文档
├── app/backend/            # Go API :8765（cmd: genscores / regenmarket）
├── app/frontend/           # React + Vite
├── scripts/verify-local.sh
├── misc/                   # 非业务：旧文档、恢复工具、Wails/移动草稿
└── .cursor/rules|skills
```

## 技术栈

- 后端：Go + Gin，`app/backend/main.go`
- 前端：React + Vite + TypeScript
- 行情：TradingView → Yahoo → 本地快照
- LLM：Gemini / DeepSeek（密钥只在 `.env`）

## 必读文档

| 文档 | 何时 |
|---|---|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | 路由 / 模块 |
| [docs/BACKEND_GO.md](docs/BACKEND_GO.md) | Go 流程图与新手导读 |
| [docs/DATA.md](docs/DATA.md) | 行情 / 快照 |
| [docs/TESTING.md](docs/TESTING.md) | 验收 |
| [docs/AI_NATIVE.md](docs/AI_NATIVE.md) | 省 token |
| [docs/LOCAL.md](docs/LOCAL.md) | 启动 |
| [HANDOFF.md](HANDOFF.md) | 近期缺口 |

## 常用命令

```bash
cd app/backend && go test ./... && go build -o /tmp/tradingagents-backend .
STOCKGOD_LOCAL_FRONTEND=true STOCKGOD_REPLAY=false /tmp/tradingagents-backend

cd app/frontend && npm run build   # 或 npm run dev
./scripts/verify-local.sh
```

## Agent 约定

1. 改行情后必须跑 `verify-local.sh` 或等价抽检。
2. 中文回复；勿提交 `.env` / Key / `*.db` / 二进制。
3. 品牌：「我不是神」。
4. 默认不要读 `misc/`。
5. 双壳：StockGod=`StockGodShell`；驾驶舱=`Layout`；`/notes` 独立壳。

## 关键路径

```
app/backend/internal/api/router.go
app/backend/internal/market/provider.go
app/backend/internal/api/stocks_handlers.go
app/backend/internal/scoring/
app/frontend/src/AppRoutes.tsx
app/frontend/public/data/us-stocks.json
```
