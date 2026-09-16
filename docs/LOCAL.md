# 本地启动（短版）

详细约定见 [AGENTS.md](../AGENTS.md)。

推荐使用可持久运行且能核对 PID/二进制/cwd 的服务脚本：

```bash
./scripts/trading-cockpit-service.sh start
./scripts/trading-cockpit-service.sh status
./scripts/check-main-product-readiness.sh
./scripts/trading-cockpit-service.sh stop
./scripts/trading-cockpit-service.sh uninstall  # 完整移除自动启动
```

默认地址为 `http://127.0.0.1:8765`；可用 `BASE_URL` 覆盖验收脚本地址。
首次启用本机写操作时，可创建 `app/backend/.stockgod-admin-token`（权限 `0600`）；
后端只读取且不回显该文件，前端仍需在设置页为当前标签页建立会话授权。

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

只使用历史研究工作台、不需要后台全市场实时刷新时，可显式隔离该网络负载（API 仍正常可用，不会进入 replay）：

```bash
STOCKGOD_DISABLE_LIVE_REFRESH=true \
STOCKGOD_LOCAL_FRONTEND=true STOCKGOD_REPLAY=false /tmp/tradingagents-backend
```

环境变量写在仓库根 `.env`（勿提交）。LLM 示例：

```bash
TRADINGAGENTS_LLM_PROVIDER=gemini
GOOGLE_API_KEY=...
TRADINGAGENTS_DEEP_THINK_LLM=gemini-3.6-flash
TRADINGAGENTS_QUICK_THINK_LLM=gemini-3.5-flash-lite
```

GPU 动态租金默认每 6 小时采集一次、保留 90 天历史。推荐把凭证放在仓库外的
`~/Library/Application Support/TradingAgents/gpu-provider-credentials.json`，并设置权限 `0600`：

```json
{
  "runpod_api_key": "",
  "modal_token_id": "",
  "modal_token_secret": "",
  "lambda_api_key": "",
  "vast_api_key": ""
}
```

页面只能要求后端重读该文件，不会接收或回显密钥。也可通过
`STOCKGOD_GPU_CREDENTIALS_FILE`、`STOCKGOD_GPU_REFRESH_INTERVAL` 和
`STOCKGOD_GPU_HISTORY_RETENTION` 覆盖路径与调度。

ETF 当前快照由后端每日尝试刷新；也可手工运行：

```bash
cd app/backend && go run ./cmd/regenetf -limit 24
```

仅当有限范围全部标的完成历史校验时才替换 current；失败时保留 last-good 并在历史模式显示。
# 免费行情配置

美股可选用 Alpaca Basic 的免费 IEX 行情。它不是 SIP 全市场合并行情，系统会把来源标记为 `alpaca-iex`、模式标记为 `iex-only`。推荐运行安全配置向导；它会隐藏输入 Secret，并写入仓库外、权限为 `0600` 的本机文件：

```bash
./scripts/setup-alpaca-basic.sh
```

默认文件为 macOS 的 `~/Library/Application Support/TradingAgents/alpaca.env`，可用
`STOCKGOD_ALPACA_CREDENTIALS_FILE` 覆盖。后端进程环境中的
`ALPACA_API_KEY_ID` / `ALPACA_API_SECRET_KEY` 优先于文件。

未配置时后端会立即跳过 Alpaca，继续使用现有公开网页来源和本地快照；这些降级来源不得被理解为专业交易级实时行情。TradingView Widget 仅用于图表展示。
