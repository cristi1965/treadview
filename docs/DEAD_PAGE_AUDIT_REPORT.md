# 死页面审计报告

日期：2026-09-10  
范围：`specs/dead-page-audit-loop.md` 固定的 31 类路由，加 `/gpu-prices`，共 32 类路由  
视口：桌面 `1440x1000`、手机 `390x844`

## 最终结论

- 32 类路由、两个视口共 64 次完整渲染：`PASS 64 / FAIL_PRODUCT 0 / FAIL_CASE 0 / BLOCKED_ENV 0`。
- 真实详情 URL 使用列表或数据文件中的实际 ID，不用虚构详情 ID 代替验收。
- 64 次渲染均在最多 5 秒后有主内容或明确恢复状态；React/page error 0、关键运行时 console error 0、导航错误 0、文档级横向溢出 0。
- 未知路由显示可返回的 404；外部数据不可用的页面在 5 秒内进入 stale/unavailable/error 终态，并保留重试、历史内容或规则内容。
- 写操作只验收权限、校验和失败关闭，没有提交真实订单、触发 Whales 外部同步或改写生产数据。

## 固定路由矩阵

以下每个 URL 均在两个视口通过：

| 产品线 | 通过的实际 URL |
|---|---|
| 历史研究 + Paper | `/`、`/dashboard`、`/lab`、`/dashboard/macro`、`/journal`、`/history`、`/settings`、`/tactical`、`/copilot` |
| 行情与研究工具 | `/market`、`/scan`、`/stock/NVDA`、`/multichart`、`/etf`、`/etf#cn-tools`、`/etf/510300` |
| GPU 算力价格 | `/gpu-prices` |
| 盘报 | `/reports`、`/reports/live-postmarket-2026-09-08` |
| 私密工具与组合 | `/macro`、`/portfolio`、`/arena` |
| 披露 | `/whales`、`/whales/warren-buffett`、`/whales/congress/christian-d-menefee` |
| 笔记与静态页 | `/notes`、`/notes/start-why`、`/about`、`/terms`、`/privacy`、`/how-to-buy` |
| 错误恢复 | `/__dead_page_probe__` |

真实详情来源：本轮盘报 ID `live-postmarket-2026-09-08` 来自 `/api/reports?limit=5`，并从盘报列表实际进入；笔记 ID 来自 `data/notes-toc.json`；国会 slug 由 `/whales` 可见列表项进入。

## 首轮缺陷与闭环

| 编号 | 首轮证据 | 修复 | 最终证据 |
|---|---|---|---|
| D1 | `/reports` 的快讯请求在前端 5 秒超时；后端同样等待 5 秒才返回历史快照，形成竞态，页面显示“加载失败” | 后端实时抓取预算改为 4 秒，保证在前端截止前返回可验证快照 | 独立接口 `HTTP 200`，总耗时 `4.026738s`；浏览器 4.6 秒内显示 25 条内容 |
| D2 | 快讯状态仓库只读取 JSON，丢弃 `X-Data-Stale/Source/Time`，旧快照可能被标成“实时” | 改用带元数据的请求，传播 stale、source、dataTime、reason | 页面显示“过期快照”、`2026-08-14 23:20:00`、`stale-snapshot:flash-live` 与降级原因 |
| D3 | Copilot、MultiChart、A 股嵌入图表使用“实盘毫秒级”“实时盯盘”等未经门禁验证的承诺 | 改为条件注入或中性图表观察说明 | 三页在 390px 复测中旧承诺命中数为 0，均无横向溢出 |
| D4 | 初始恢复探针使用 `/reports/1`、`/whales/congress/test` 等不存在 ID，只能证明错误态 | 从实际列表提取 ID 并重跑 | 三类真实详情在两个视口均渲染成功；无崩溃、无溢出 |

## 数据降级不是死页

以下能力当前仍受外部数据或存量证据限制，但页面均按 fail-closed 通过产品验收：

- 美股/A 股总实时门禁未就绪：页面不把旧价、旧评分或未知时间显示成当前决策依据。
- ETF 总表为明确日期的历史研究快照；QDII 溢价只有完整来源、净值日和报价时间时才显示数字。
- Whales 当前没有完整 SEC 13F 成功记录，机构页明确显示来源未知/暂无可核验持仓；国会旧样本明确排除出信号榜单。
- 报告、日历和快讯显示各自的数据时间、来源和 stale 原因，不以刷新时间冒充数据时间。

这些是数据就绪度缺口，不是 UI 死页面。它们继续计入专业可用度评分，不能据此声称实时交易系统已就绪。

## R18 新增 GPU 页面证据

- `/gpu-prices` 在 `1440x1000` 和 `390x844` 均于 5 秒内显示“官方 GPU 租金跟踪”，页面标题为 `GPU 算力租金跟踪 | 我不是神`，文档宽度分别未超过视口。
- 四家 provider 均未配置时，动态报价、最低价、价差和历史曲线保持隐藏；“刷新上游”在两个视口均为禁用态，并明确提示先在后端配置 provider 凭证。
- 同一页面仍显示独立的“官方核验参考价”、核验日期和“非实时”边界；Vast.ai 明确不含固定参考价，参考价不进入动态比较。
- 桌面侧栏、移动菜单和命令面板均实际进入 `/gpu-prices` 后返回；没有点击刷新上游或外部同步入口。

## 自动化与运行证据

- 前端：89 个 Node 契约测试 + 7 个 TypeScript 行为测试通过；TypeScript 检查和 Vite 生产构建通过。
- 后端：`go test ./...` 全部通过。
- 生产前端：Vite 构建通过并由本地 Go 服务加载。
- 浏览器：32 类路由 x 2 视口，共 64 次；真实详情使用 `/reports/live-postmarket-2026-09-08`、`/notes/start-why`、`/whales/warren-buffett` 和 `/whales/congress/christian-d-menefee`。
- 浏览器审计拦截全部非 GET/HEAD/OPTIONS 请求；16 次自动本机会话 POST 均在浏览器层拦截，未产生写操作。控制台仅记录 14 次该主动拦截造成的 `ERR_BLOCKED_BY_CLIENT` 和 6 次已被页面 fail-closed 的 HTTP 503；关键运行时错误为 0。
- 原始逐路由证据保存在 `/tmp/dead-page-r18.json`；GPU 目检截图为 `/tmp/dead-page-r18-gpu-desktop.png` 和 `/tmp/dead-page-r18-gpu-mobile.png`。
- 快讯接口响应头：`X-Data-Stale: true`、`X-Data-Time: 2026-08-14T15:20:00Z`、`X-Data-Source: stale-snapshot:flash-live`。
- 五方分已于 `2026-09-10T07:14:59Z` 从当天真实上游刷新后的 6,151 标的宇宙重新计算；接口返回 `X-Data-Stale: false`，模型仍明确标记 `validation.status=unvalidated`，不作为已验证交易信号。
- R19 全市场刷新已从串行长等待改为 8-worker 有限并发，并通过全量与 race 测试。真实 6,151 标的刷新在 123.87 秒内正常结束，但本轮美股上游返回 0 条有效报价，接口保留 `usError`，未推进完整快照时间；A 股同轮更新 5,191 条。
- AAPL/NVDA 直连报价已改为主链与 Yahoo/Nasdaq 后备链受控竞速；部署后连续 3 次门禁均由 Nasdaq 在 4 秒截止前返回完整报价和 `2026-09-09` provider 日期，收盘状态明确为 `closed`。
- R21 在明确指向当前服务 `BASE_URL=http://127.0.0.1:8831` 后，`verify-local.sh` 全部通过；盘报最新日期为 `2026-09-09`，符合当前交易日目标。本机 `8765` 仍有另一个旧版进程，默认地址的验收会命中旧版并误报 GPU 404；运行验收必须显式绑定待测实例。
- R21 市场刷新任一地域失败时改为 `status=partial` 且 `X-Data-Stale: true`，设置页同时显示美股/A 股更新数与具体错误，不再将 HTTP 200 等同于完整成功。
- Congress、CN/local guru 与东方财富龙虎榜 overlay 的替换已改为先校验非空数据、再事务提交；空输入与强制写失败回归测试证明旧快照保留。

## 后续循环入口

若路由、导航、详情链接或数据消费者发生变更，必须重新运行完整 32 类路由矩阵。新增页面只有在两个视口都有成功、空、过期或失败终态，并且动态数字通过来源/时间门禁后，才能加入 `PASS`。
