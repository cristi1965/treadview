# 实时数据与使用闭环修复

## 用户可见目标

1. Settings 不再只提供一个只覆盖行情的“刷新门禁”操作，而是能说明並执行实际可刷新的数据域。
2. `live-trading-data` 只在真实、带来源时间且未过期的必需数据均通过时返回 HTTP 200。
3. History 默认优先展示可用证据包，旧的 `research_unavailable` 记录收起，但不删除审计历史。
4. 本机单用户模式的管理授权有可理解的启用路径；未授权仍 fail-closed，不在页面回显令牌。

## 不允许的做法

- 不通过放宽 stale 时间、修改系统时钟、写死新日期或手工伪造快照让门禁变绿。
- 不把响应时间当作交易所时间、NAV 日期、财报期或 SEC 披露日。
- 不删除旧失败研究记录来美化页面；只改信息层级。
- 不移除管理写操作的鉴权和审计。

## 验收信号

- 快速红绿回路：`GET /api/readiness?profile=live-trading-data`。
- 分项证据：每个业务端点的 `X-Data-Source` / `X-Data-Time` / `X-Data-Stale` 与 readiness 一致。
- 回归：Go 全量测试、前端统一测试、TypeScript、Vite build、`verify-local.sh`。
- 浏览器：Dashboard、History、Settings、Tactical 及实时数据页在桌面/390px 无死页、永久 loading、错误边界或横向溢出。

## 2026-09-09 最终验收记录

- `live-trading-data`：HTTP 200，AAPL/NVDA 直接报价、US overview、US stock list、session-aware CN 四项通过；header/body 均为 `stale=false / ready=true`。
- US 核心池：30 只每 45 秒刷新，报价请求对同一标的集合共享在途 flight；非核心标的仍可按需逐票取价。
- A 股：使用东财 `f124` 作为供应商观测时间；`/api/a-market` 实测 508/508 条均有 `eastmoney-cn + dataTime`，旧全量静态行不再混入实时集合。
- 管理会话：服务仅监听 `127.0.0.1:8831`；仅同源浏览器 POST 可建立 `HttpOnly + SameSite=Strict` 1 小时会话；Bearer 仅保留为高级恢复；Paper 账户受保护读取 HTTP 200。
- Paper：浏览器读取 CNY/USD 子账户各 100000；PLTR 的 Yahoo 报价带供应商时间，条件单可计算且提交按钮启用；未提交订单。
- History：当前聚合头为 `evidence_only`，另返回 `X-Research-Unavailable-Count: 5`；AAPL 默认展示，旧失败归档默认折叠。
- 自动化：Go `./...` 全通过；前端合同 `66 + 7` 全通过；TypeScript 和 Vite 生产构建通过；`verify-local.sh` 通过。
- 浏览器：32 路由 x 桌面/390px，共 `64/64 PASS`；当前构建新增 console error 为 0。
- 报告详情：请求序列拒绝旧响应覆盖，09-04 深链连续 10/10 正确；ETF `mode=best` 单请求 HTTP 200 返回明确标日期的历史降级数据。

## 10 角色最终复评

> 均为模拟审计视角，不代表真实从业者背书或投资建议。

| 角色 | 最终分 |
|---|---:|
| 公募基金经理 | 92 |
| 量化基金经理 | 91 |
| 基本面研究员 | 91 |
| 合规负责人 | 91 |
| 美股日内交易员 | 90 |
| A 股波段交易员 | 93 |
| 机构交易风控员 | 91 |
| 资深牛散 | 92 |
| 移动端高频用户 | 90 |
| 产品可用性审计员 | 94 |

最低分 **90/100**，10 个角色全部严格 `>88`；算术平均分 **91.5/100**。

完整系统健康仍会把 ETF sectors、QDII premium、Whales 披露等非实时数据标为 stale/unknown；这些状态没有被实时行情门禁隐藏或改写。
