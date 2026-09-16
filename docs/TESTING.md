# Testing

本项目测试分三层：**单元**、**API 烟雾**、**页面验收**。行情相关改动至少跑前两层。

## 1. 单元测试（Go）

```bash
cd app/backend
go test ./...
go test ./internal/market/ -v
```

约定：

- 纯解析/换算放可测函数（如 `parseUsStocksQuotes`），避免只测 HTTP
- 网络依赖测试用 `t.Skip` 或 build tag，默认 CI 不打外部行情
- 新加关键路径（报价 pct、market=cn、品牌文案）优先补表驱动测试

### 当前覆盖重点

| 包 | 关注点 |
|---|---|
| `internal/market` | us-stocks → quote map；prev/pct 计算 |
| `internal/scoring` | panel 分数映射（按需补） |
| `internal/api` | handler 绑定与过滤（按需补 httptest） |

数据/研究链聚焦回归：

```bash
go test ./internal/dataflows ./internal/api ./internal/agents ./internal/scoring
```

覆盖 `/api/quote` 的最保守对象观测时间和 date-only 粒度、SEC/Nasdaq filing 的 symbol+period 匹配、SEC 字段级 XBRL provenance、YTD 排除、5Q+3A 深度、QoQ/YoY/TTM 期间门禁、缺披露元数据 fail-closed、可选研究上下文 hash、evidence-only 冷启动使用 dated historical close 主研究价、可选 quote cross-check、历史行情/财务/新闻计算和 hash/lineage/持久化，以及 deterministic scaling 仍保持 `unvalidated`。

## 2. 本地 API 烟雾

后端需在 `:8765` 运行（见 `AGENTS.md`），然后：

```bash
./scripts/verify-local.sh
# 或指定端口
BASE_URL=http://127.0.0.1:8877 ./scripts/verify-local.sh
```

脚本检查：

- `/api/health`（仅 liveness，不代表数据可用）
- `/api/readiness`（聚合数据来源、时间与 stale 状态；degraded 返回 HTTP 503）
- `/api/market` 的 `X-Data-Source` 含 `us-stocks`，且 RKLB 价与 `us-stocks.json` 一致量级
- `/api/stocks?market=cn` 返回 A 股代码（首位为数字）
- `/api/macro`、`/api/premarket-movers` 200
- `/api/reports` 正文不含「我不是股神」

### Readiness profiles

`/api/readiness` 不带参数时仍执行严格 full readiness。三个窄 profile 独立返回
`200`（ready）或 `503`（degraded），并携带各自的 `checks`、`remedies` 与边界声明：

```bash
curl -i 'http://127.0.0.1:8765/api/readiness?profile=historical-research'
curl -i 'http://127.0.0.1:8765/api/readiness?profile=paper-engine'
curl -i 'http://127.0.0.1:8765/api/readiness?profile=live-trading-data'
```

- `historical-research`：同一完整 evidence-only v5 dossier 必须以最新已完成 Nasdaq historical close 作为 PIT 主研究价，具备至少 21 个有效日期 OHLCV、披露日在 trade date 之前且字段级期间语义完整的 5 个离散季度 + 3 个年度 SEC filing，以及完成过筛选的新闻证据。quote 只是可选且不视为独立确认；真实外源不足时返回 503。
- `paper-engine`：只检查 Paper SQLite/OMS schema、审计链、OCO scheduler 和版本化风险策略自身健康。
- `live-trading-data`：严格检查当前行情、宏观、异动、报告、ETF/QDII 和 Whales 数据；另两个 profile 就绪不能替代本项。

认证 Paper 生命周期验收使用临时 SQLite 与 `httptest` HTTP server，不访问券商或真实交易接口：

```bash
./scripts/verify-paper-runtime.sh
```

该脚本覆盖 Bearer 认证、提交、BUY STOP 跳空风险拒绝、保留预占、Paper BUY 模拟成交、自动 OCO 子单、保护 SELL、兄弟单取消及审计链校验。

完整 Paper UI 的桌面和手机触控生命周期使用两个隔离的临时数据库与随机本地端口：

```bash
./scripts/verify-paper-browser-runtime.sh
```

GPU 租金跟踪的独立验收也使用随机端口和临时 SQLite。桌面与移动端都会真实经过 Service 刷新、历史落库、router 和管理员重读/刷新端点，不 mock GPU API：

```bash
./scripts/verify-gpu-browser-runtime.sh
```

脚本才会设置精确开关 `STOCKGOD_GPU_RUNTIME_FIXTURE=temporary-local-acceptance-v1`。后端仅在管理员令牌非空、数据库位于系统临时目录之下，且 replay/live-mirror 都关闭时允许启动；任一条件不满足都会直接拒绝。

通用浏览器动作与全路由审计在已启动的本地服务上运行；其中 GPU/ETF 的动态写按钮仍使用浏览器内确定性响应，GPU 的真实后端链证据以上述独立脚本为准：

```bash
cd app/frontend
USABILITY_BASE_URL=http://127.0.0.1:8765 USABILITY_ROUND=release node scripts/module-actions-audit.mjs
USABILITY_BASE_URL=http://127.0.0.1:8765 USABILITY_DEVICE=mobile USABILITY_ROUND=release node scripts/module-actions-audit.mjs
USABILITY_BASE_URL=http://127.0.0.1:8765 USABILITY_ROUND=release node scripts/usability-audit.mjs
USABILITY_BASE_URL=http://127.0.0.1:8765 USABILITY_ROUND=release node scripts/all-routes-audit.mjs
```

## 3. 手动报价抽检

```bash
curl -s 'http://127.0.0.1:8765/api/quote?syms=RKLB,NVDA' | python3 -m json.tool
```

期望（盘中/收盘附近）：`source` 多为 `tradingview`；RKLB 与当日收盘同量级。

## 4. 前端

```bash
cd app/frontend
npm run build
# 可选：npm run dev 后硬刷新 :5173
```

页面冒烟清单（改壳/首页/个股后）：

1. `/` 重定向到 `/dashboard`，历史研究门禁与生成入口可见
2. `/market` 实验行情在实时数据不可用时显示简明降级状态，端点明细仅在点击诊断后展开
3. `/etf#cn-tools` 显示独立“A 股工具箱”；完整新鲜报价显示实时动态，否则只显示无价格的规则模式
4. `/scan` 列表非空
5. `/stock/RKLB` 价格非明显陈旧
6. `/notes` 无 StockGod 主导航壳
7. `/reports` 品牌为「我不是神」

## 5. 回归时「先修谁」

若数据不对，按优先级查：

1. 后端是否最新二进制、cwd 是否在 `app/backend`
2. `us-stocks.json` 的 `generated_at`
3. TV 列映射 / Yahoo chart prev（见 `HANDOFF.md`）
4. 前端是否仍代理到旧端口

## 6. 禁止当作测试通过的信号

- 仅 `npm run build` 成功
- 仅 `go test` 在无 network 用例时全绿，但未跑 `verify-local.sh`
- 源站截图一致但本地 API 仍读 mock
