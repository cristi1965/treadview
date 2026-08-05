# StockGod Full Restore Plan

目标：尽量恢复 `stockgod.xyz` 的全部公开可见页面、交互、数据结构和本地等价 API。

## 恢复边界

可恢复：
- 公开页面 UI、导航壳、响应式布局、主题、文案和交互。
- 已抓取的页面结构、截图、RSC 文本、公开 JSON 和静态资源。
- 可由公开数据源重建的等价数据流，例如股票列表、ETF 板块、13F、国会交易、盘报 JSON。

不可直接恢复：
- 原站未公开的私有数据库、服务端业务逻辑、内部任务队列、真实账号数据。
- 需要付费或权限的数据源的完整历史数据，除非后续提供凭据或导出。

## 页面与功能总账

| 路径 | 原站功能 | 当前状态 | 数据状态 | 下一步 |
|---|---|---|---|---|
| `/` | AI 产业链脉冲热力图、986 标的、镜头/市场/分类/热度/层级筛选、Macro ticker、Top 榜 | 有 canvas 基础热力图 | mock 986 节点 | 补 StockGod app shell、筛选栏、Macro、Top 榜、热度/层级字段 |
| `/scan` | 全市场扫描、6151 股票、11 列、五方雷达、排序、风险/判读/市值/行业筛选、观察列表 | 有表格和基础排序 | mock 股票 | 补筛选器、判读后/分歧列、观察列表联动、更多 mock/恢复数据 |
| `/etf` | 4533 ETF、47 板块、8 类别、搜索、规模/近1年/近5年/抗跌排序、折叠板块 | 有卡片页 | mock 4 sectors，恢复文本含更多板块 | 将恢复文本解析为完整 sector seed，补原站文案/布局 |
| `/whales` | 机构 13F、国会交易、分类筛选、共识榜、详情页 | 已按原站壳重建 | SQLite/seed + consensus | 继续补国会详情、真实同步状态 |
| `/whales/:slug` | 投资者持仓详情、统计、持仓列表、股票链接 | 已有基础详情 | SQLite holdings | 对齐原站 shell 与行样式 |
| `/arena` | 股票对决、选择器、五方/价格/财务/评级对比、预设对决 | 有基础页待核对 | mock/API 待补 | 根据分析文档重建控件和数据 |
| `/reports` | 市场日历、盘前/收盘报告、Markdown 展开折叠、reports-latest.json | 有页面 | mock reports + 已抓公开 JSON 元信息 | 补原站布局和公开 JSON fallback |
| `/portfolio` | 观察列表、持仓、本地存储、空状态、删除 | 有页面待核对 | localStorage | 对齐原站，联动 Scan/Heatmap 收藏 |
| `/stock/:symbol` | 股票详情、五方分析、持有人、市场信息 | 有基础页 | stocks + whales holders | 补原站详情结构、持有人数据 |

## 公共系统

- StockGod theme: `#08090b`, `#111317`, `#181b21`, `#21242c`, `#d98a6a`。
- App shell: 204px desktop sidebar, 56px sticky header, mobile top bar, mobile bottom nav, watchlist rail.
- Global search: `⌘K` placeholder, ticker/name/sector search.
- Watchlist: localStorage, add/remove from scan/heatmap/stock, portfolio display.
- Market status pill: 美股交易/休市状态.

## 数据源计划

1. `recovered_source/*.json` 作为 seed/reference。
2. 后端 `/api/*` 提供本地统一 JSON。
3. 对缺失真实数据的区域先提供 deterministic mock，字段和排序与原站一致。
4. 后续可接 Yahoo/Nasdaq/SEC EDGAR/CapitolTrades 替换 mock。

## 当前优先级

1. 抽 StockGod app shell 为共享组件，替换 Whales 私有壳并用于其它页面。
2. ETF：从恢复文本解析 seed，补齐更多 sector 数据。
3. Scan：补筛选器和原站列结构。
4. Home：补镜头/市场/热度/层级控制栏。
5. Reports/Portfolio/Arena：按分析文档逐页对齐。
