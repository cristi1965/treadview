# 本地启动（短版）

详细约定见 [AGENTS.md](../AGENTS.md)。

```bash
# 后端
cd app/backend
go build -o /tmp/tradingagents-backend .
STOCKGOD_LOCAL_FRONTEND=true STOCKGOD_REPLAY=false /tmp/tradingagents-backend

# 前端（Node 20+）
cd app/frontend && npm run build   # 由 :8765 托管 dist
# 或热更新：npm run dev            # :5173

# 验收
./scripts/verify-local.sh
```

环境变量写在仓库根 `.env`（勿提交）。LLM 示例：

```bash
TRADINGAGENTS_LLM_PROVIDER=gemini
GOOGLE_API_KEY=...
TRADINGAGENTS_DEEP_THINK_LLM=gemini-3.6-flash
TRADINGAGENTS_QUICK_THINK_LLM=gemini-3.5-flash-lite
```
