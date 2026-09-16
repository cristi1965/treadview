# 死页面审计闭环

## 目标

循环检查并修复当前 WebUI 中的死页面，直到所有注册路由在桌面和移动端均有明确、
可操作的终态。外部实时数据不可用不等于页面通过：页面必须在 5 秒内给出可理解的
降级状态，并提供仍然有效的内容、重试或修复入口。

## 死页面判定

以下任一情况判为 `FAIL_PRODUCT`：

- 路由空白、React 崩溃、404 或错误壳；
- 页面持续加载，5 秒内没有成功、空、过期或失败终态；
- 页面主操作完全不可用，且没有原因、恢复入口或可用替代内容；
- 导航、按钮或详情链接指向不存在的路由；
- 陈旧、未知或失败数据仍被当作实时可信数字展示；
- 移动端关键内容或操作因溢出、遮挡而不可达。

明确标注来源/时间/失败原因，且有可用历史内容、规则内容、重试或修复入口的
fail-closed 页面，不判为死页面。

## 路由清单

### 历史研究 + Paper 主产品

- `/`（应重定向 `/dashboard`）
- `/dashboard`
- `/lab`
- `/dashboard/macro`
- `/journal`
- `/history`
- `/settings`
- `/tactical`
- `/copilot`

### StockGod 实验行情

- `/market`
- `/scan`
- `/stock/NVDA`
- `/multichart`
- `/etf`
- `/etf#cn-tools`
- `/etf/510300`
- `/reports`
- `/reports/:id`（从列表取真实 ID）
- `/macro`
- `/portfolio`
- `/arena`
- `/whales`
- `/whales/:slug`（从列表取真实 slug）
- `/whales/congress/:slug`（从列表取真实 slug）
- `/notes`
- `/notes/:id`（从列表取真实 ID）
- `/about`、`/terms`、`/privacy`、`/how-to-buy`
- `/__dead_page_probe__`（应显示可用 404 页面和返回入口）

## 用例模型

每条路由分别在 `1440x1000` 和 `390x844` 执行：

1. 直接访问并等待最多 5 秒；
2. 记录 URL、页面标题、一级标题/主状态和可见主操作；
3. 检查错误边界、永久加载、横向溢出、遮挡和控制台错误；
4. 对主要导航和详情入口执行一次进入/返回；
5. 数据失败时验证来源、时间、stale/unavailable 和恢复入口；
6. 写操作只验证门禁、校验、取消与失败状态，不产生真实订单或外部副作用。

状态只使用：`PASS`、`FAIL_PRODUCT`、`FAIL_CASE`、`BLOCKED_ENV`、`NOT_RUN`。

## 循环与停止条件

每轮固定执行：路由清点 -> 浏览器审计 -> 缺陷报告 -> 修复 -> 聚焦回归 -> 全路由回归。
停止条件：

- 所有清单项均有终态；
- `FAIL_PRODUCT = 0`；
- `BLOCKED_ENV` 只允许外部数据源阻断，且对应页面本身已通过 fail-closed 验收；
- 桌面和 390px 无关键横向溢出或不可达操作；
- 前端契约测试、TypeScript 检查和构建通过；
- 后端相关改动通过聚焦测试与 `verify-local.sh` 或等价抽检；
- 最后一轮是无新增缺陷的完整回归轮。

## 证据

本轮报告写入 `docs/DEAD_PAGE_AUDIT_REPORT.md`，保留首轮失败和每次复测结果，不能用
最终通过覆盖原始缺陷。
