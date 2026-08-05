# Site Recovery Gaps

更新时间: 2026-08-01 03:30 CST

## 目标

- 原站: https://stockgod.xyz
- 本地: http://127.0.0.1:8765
- 桌面壳: Wails, `app/backend/cmd/desktop`

## 使用的复刻工具

1. `site-recovery` skill
   - 用途: 抓 live/local 路由、截图、按钮、链接、输入框、网络请求、console 错误，并输出缺口矩阵。
   - 原始脚本: `/Users/jingxin/.codex/skills/site-recovery/scripts/site_probe.mjs`
   - 当前项目适配脚本: `scripts/site-recovery-probe.mjs`

2. `playwright` skill
   - 用途: 真浏览器点击验证，包括菜单、搜索、Tab、卡片、详情跳转、展开/收起、表单。
   - 运行要求: Node 20+ 或 Codex bundled Node。

3. Wails 本地壳验证
   - 用途: 确认桌面环境里 `/api/*`、`/ws`、静态资源、深链路刷新是否正常。

## 本轮探针产物

- Live JSON: `recovered_source/site-recovery/live-stockgod-20260801/site-probe.json`
- Local JSON: `recovered_source/site-recovery/local-wails-20260801/site-probe.json`
- Live screenshots: `recovered_source/site-recovery/live-stockgod-20260801/*.png`
- Local screenshots: `recovered_source/site-recovery/local-wails-20260801/*.png`

## 已确认的大缺口

## P0 修复进展

2026-08-01:

- 已新增项目内 probe: `scripts/site-recovery-probe.mjs`。
- 已同步原站公开 `data/reports.json` 到 `app/backend/data/reports-live.json`，共 72 篇，并按项目品牌规则把正文里的旧中文品牌替换为「我不是神」。
- 已同步 `app/frontend/public/data/reports.json` 与 `app/frontend/public/data/reports-latest.json`。
- 已恢复 StockGodShell 桌面搜索按钮和左侧搜索 rail。
- `/reports` 已改为初始 5 篇、默认展开最新 1 篇、加载更多显示剩余天数。
- `/notes` 已恢复原站式总目录卡和 10 个章节大卡。
- 验证产物: `recovered_source/site-recovery/local-p0-after-20260801/site-probe.json`。

2026-08-01 P1:

- `/scan` 已收敛 US 行业枚举顺序，去掉混入的中文行业/其他类，搜索框 placeholder 对齐为 `搜代码 / 公司 / 行业…`。
- `/arena` 已同步原站公开 `data/arena.json` 到 `app/backend/data/arena-us.json`。
- `/arena` 已实现近期成交默认折叠和 `展开全部 N 笔` / `收起` 交互。
- 验证产物:
  - `recovered_source/site-recovery/local-p1-after-20260801/site-probe.json`
  - `recovered_source/site-recovery/local-p1-arena-data-20260801/site-probe.json`

### P0: 全站搜索入口不一致

Live 多数页面有 `搜索` 按钮；Local 多数页面变成 `打开菜单` 或缺少同等搜索入口。

影响路由:
- `/`
- `/scan`
- `/reports`
- `/whales`
- `/arena`
- `/macro`

建议:
- 抽一个 `StockGodSearchTrigger`，放回 StockGodShell 顶部/移动端菜单。
- 对齐 live 的 placeholder: `搜代码 / 名称 / 板块…`。
- 搜索结果必须支持股票详情跳转和市场参数。

状态: 已完成第一版。桌面 header 和左侧 rail 已出现 `搜索` 按钮，仍需后续逐页点选验证搜索面板结果排序和移动端细节。

### P0: 首页热力图分类数量与榜单数据不一致

Live 和 Local 都有热力图，但分类数量、榜单股票和评分明显不同。

例子:
- Live `AI 产业链 439`; Local `AI 产业链1768`
- Live 首页榜单含 `NIPG`, `SNOW`, `CNTA`; Local 含 `TSM`, `ASML`, `LLY`

建议:
- 明确是否要完全贴原站快照，还是保留自建实时数据。
- 若目标是完全复刻，应恢复 live 对应的数据快照与分类聚合口径。

### P0: `/reports` 默认报告与列表行为不一致

Live 默认显示:
- `盘报 | 盘前看点 · 07-31(开盘前 1h)`
- 有 `查看更早 · 还有 34 天`

Local 默认显示:
- `盘报 | 收盘复盘 · 07-30(美东收盘)` 起始
- 列表数量和按钮文案不同

建议:
- 补齐 `07-31` 最新报告快照。
- 对齐默认选中逻辑: 原站优先最新盘前。
- 对齐“查看更早”分页/加载更多行为。

状态: 已完成第一版。当前原站公开 latest 为 `20260731-close`，本地 `/reports` 已同步并默认展开该报告。

### P0: `/notes` 左侧目录卡未完整复刻

Live 有完整学习目录卡:
- `雷司令投资笔记 投资常识,随手可查`
- `开始之前`, `市场地图`, `K线语言`, `趋势与形态`, `技术指标`, `看懂公司`, `交易实务`, `流派与大师`, `心态与风险`, `工具与词典`

Local 只剩较简化的笔记页，按钮数量从 Live 21 降到 Local 10。

建议:
- 恢复目录分组、进度 `0/10 已读`、分类图标和卡片点击态。
- 补齐 TOC 数据契约或本地静态 TOC。

状态: 已完成第一版。TOC 数据原本完整，前端已恢复总目录卡和 10 个章节大卡。

### P1: `/scan` 筛选和列头不一致

Live 有:
- `已判读(4653)`
- `高分 均≥65(88)`
- `共识好票(111)`
- `分歧大(707)`
- `Communication Services 3`
- 列头按钮 `代码 / 名称`, `五方`, `判读后`

Local 有:
- `已判读(6151)`
- `高分 均≥65(116)`
- `共识好票(154)`
- `分歧大(1765)`
- 不同分类和观察页入口

建议:
- 对齐筛选按钮、行业枚举、排序按钮和计数口径。
- 明确 `watchlist` 是否是本地增强功能；完全复刻时应放到二级入口。

状态: 已完成第一版可见交互。行业枚举和搜索入口已对齐；评分计数仍保留本地自建数据口径，后续若要求逐数值完全一致，需要同步原站评分快照。

### P1: `/arena` 展开全部行为缺失

Live 有:
- `展开全部 14 笔`
- `展开全部 12 笔`

Local 没有同等按钮，增加了 `五神虚拟盘`、`股票对决`、`完整观察页`。

建议:
- 恢复分组内展开全部/折叠交互。
- 对齐 live 的组合列表数据和 `/api/quote?syms=*` 请求节奏。

状态: 已完成第一版。已同步原站公开 arena 数据，并恢复 `展开全部 N 笔` / `收起`。

### P1: `/macro` 原站含 Token 输入和进入/收起/观察态

Live 有:
- `Token` password 输入
- `0 观察`
- `收起`
- `进入`

Local 是 TradingAgents 宏观页，出现 `新增宏观事件`，与 StockGod 原站行为不同。

建议:
- 区分 `/macro` 在 StockGod UX 与 TradingAgents 驾驶舱之间的归属。
- 若完全复刻原站，`/macro` 应恢复 StockGod 原站的宏观驾驶舱入口态。

状态: 已完成第一版。`/macro` 已切回 StockGodShell 私密入口态，保留 `Token` password 输入、`进入`、观察列表与收起按钮；原 TradingAgents 宏观日历保留到 `/dashboard/macro`。验证产物: `recovered_source/site-recovery/local-p1-macro-after-20260801/site-probe.json`。

### P1: `/stock/:sym` 首屏等待所有接口导致标题延迟

试跑 skill 时发现 `/stock/NVDA` 在短 settle 下容易只显示 loading，导致本地快照 h1 为空；原站会先显示公司名，再补报价/持仓/基本面。

状态: 已完成第一版。`StockDetail` 已改为先用本地 `us-stocks.json` 快照渲染公司名、代码和基础信息，再异步补持仓、实时报价、基本面和新闻。验证产物: `recovered_source/site-recovery/local-stock-detail-after-progressive-render-20260801/site-probe.json`。

### P2: `/settings` 原站 404，本地存在设置页

Live `/settings` 返回 404 `没找到这个页面`。
Local `/settings` 是 TradingAgents 设置页。

建议:
- 若 StockGod 复刻严格优先，`/settings` 不应挂在 StockGod shell 可见导航。
- TradingAgents 设置页可迁到 `/dashboard/settings` 或仅驾驶舱壳展示。

## 下一步执行顺序

1. 先补全 StockGodShell 全站搜索入口和移动端菜单。
2. 补 `/reports` 最新报告、默认选中、加载更多。
3. 补 `/notes` 目录卡和进度态。
4. 补 `/scan` 筛选/排序/行业枚举。
5. 补 `/arena` 展开全部。
6. 重新跑 live/local probe，更新缺口矩阵。
