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

## 2. 本地 API 烟雾

后端需在 `:8765` 运行（见 `AGENTS.md`），然后：

```bash
./scripts/verify-local.sh
# 或指定端口
BASE_URL=http://127.0.0.1:8877 ./scripts/verify-local.sh
```

脚本检查：

- `/api/health`
- `/api/market` 的 `X-Data-Source` 含 `us-stocks`，且 RKLB 价与 `us-stocks.json` 一致量级
- `/api/stocks?market=cn` 返回 A 股代码（首位为数字）
- `/api/macro`、`/api/premarket-movers` 200
- `/api/reports` 正文不含「我不是股神」

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

1. `/` 热力图有数据  
2. `/scan` 列表非空  
3. `/stock/RKLB` 价格非明显陈旧  
4. `/notes` 无 StockGod 主导航壳  
5. `/reports` 品牌为「我不是神」

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
