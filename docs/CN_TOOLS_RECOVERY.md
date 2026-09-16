# A 股工具页恢复

## Spec

- `/etf#cn-tools` 是 A 股工具独立视图，不展示美股 ETF 筛选器和其过期错误。
- `cn-routines.json` / `cn-picks.json` 的 `updated` 是规则版本日期，不是行情时间。
- 价格、涨跌、动作、买卖点和提醒只能使用完整且未过期的实时报价。
- 报价缺失、时间未知或过期时继续 fail-closed，不用规则版本日期伪装新鲜行情。

## Plan

1. 为组合 freshness 决策增加纯函数回归测试。
2. 修正 `/api/cn/routines` 和 `/api/cn/picks` 的数据时间与缺失报价门禁。
3. 将 `#cn-tools` 分支为独立的 A 股页面内容。
4. 运行 Go / 前端测试、构建、API 和桌面/手机页面验收。

## Acceptance

- 报价完整且最新刷新在阈值内时，A 股工具显示内容且 `X-Data-Time` 来自行情。
- 无实时刷新或关键报价不完整时，响应标记 stale，前端不显示动态数字。
- `/etf#cn-tools` 首标题为“A 股工具箱”，页面不出现“ETF analyses stale”或“全部 0”。
