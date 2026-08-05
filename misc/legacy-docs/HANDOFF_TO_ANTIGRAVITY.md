# Handoff To Antigravity

请接手这个项目，目标是把丢失源码的 `https://stockgod.xyz` 尽量完整复刻到本地 `TradingAgents` 项目里。

## 工作区

Repo:

```text
/Applications/workspace/ai管力/账本/TradingAgents
```

当前核心方向不是重写一个新站，而是按 live 站逐项恢复：

- 路由
- UI
- 数据接口
- 页面交互
- 详情页
- 观察列表/watchlist
- 单端口部署 `http://localhost:8765`

## 已经做过的事

1. 已经恢复了一套 StockGod 风格前端：
   - `/`
   - `/scan`
   - `/etf`
   - `/etf/:id`
   - `/whales`
   - `/whales/:slug`
   - `/arena`
   - `/reports`
   - `/portfolio`
   - `/stock/:symbol`
   - `/about`
   - `/terms`
   - `/privacy`
   - `/how-to-buy`

2. 后端新增/恢复了多类 API：
   - stocks
   - heatmap
   - whales
   - ETF
   - reports
   - arena

3. Go 后端已经改成可以托管前端 dist：
   - `app/backend/internal/api/router.go`
   - 目标是直接打开 `http://localhost:8765/` 就能访问完整前端。

4. Vite 构建路径已改：
   - `app/frontend/vite.config.ts`
   - `base: '/'`
   - 这是为了修复 `/stock/NVDA` 等深层路由刷新空白。

5. 已创建本地专用 skill：
   - `/Users/jingxin/.codex/skills/site-recovery`
   - 脚本：`/Users/jingxin/.codex/skills/site-recovery/scripts/site_probe.mjs`
   - 用来抓 live 站路由、按钮、链接、API/JSON、截图、console error。

6. 已经跑过 live 站核心探测，结果在：
   - `recovered_source/live-core-probe/site-probe.json`
   - `SITE_RECOVERY_GAPS.md`

## 重要发现

Live 站大量功能不是硬写在页面里，而是依赖这些数据/API：

```text
/data/reports-latest.json
/data/judgment-p0-us.json
/data/dilution-flags.json
/data/us-panel-summary.json
/data/us-stocks.json
/data/etf-analyses.json
/api/premarket-movers
/api/macro
/api/market
/api/quote?syms=...
```

优先补这些数据契约，别继续盲目堆 UI。

## 当前最重要的交接注意

上一轮被中断时，`app/frontend/src/components/layout/StockGodShell.tsx` 已经被部分修改：

- 移动端菜单按钮开始补真实展开
- 左侧观察浮条改成点击 `/portfolio`
- 观察浮条数量改成读取 `usePortfolioStore`

但还没有完成后续补丁，也没有在这之后跑 build。

请先检查并修好这个文件：

```text
app/frontend/src/components/layout/StockGodShell.tsx
```

然后继续完成这些未完成交互：

- `/scan` 页顶部“我的观察列表”按钮应跳到 `/portfolio`
- `/whales` 里的机构卡片不要用 `window.location.href`，改成 React Router `navigate`
- 移动端菜单点击后，点菜单项应关闭菜单
- 观察列表最好做成 live 站那种 drawer，而不只是跳转

## 推荐下一步

1. 先跑构建，确认当前中断后的代码是否还能编译：

```bash
cd /Applications/workspace/ai管力/账本/TradingAgents/app/frontend
/Users/jingxin/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node ./node_modules/vite/bin/vite.js build
```

2. 跑 Go 测试：

```bash
cd /Applications/workspace/ai管力/账本/TradingAgents/app/backend
go test ./...
```

3. 启动单端口服务：

```bash
cd /Applications/workspace/ai管力/账本/TradingAgents/app/backend
go run .
```

访问：

```text
http://localhost:8765/
```

4. 用 site recovery probe 对比 local 和 live：

```bash
cd /Applications/workspace/ai管力/账本/TradingAgents

node ~/.codex/skills/site-recovery/scripts/site_probe.mjs \
  --origin http://localhost:8765 \
  --routes /,/scan,/etf,/whales,/whales/howard-marks,/arena,/reports,/portfolio,/stock/NVDA,/about,/terms,/privacy,/how-to-buy \
  --out recovered_source/local-core-probe \
  --timeout 60000
```

Live 结果已经在：

```text
recovered_source/live-core-probe/site-probe.json
```

## 高优先级缺口

参考：

```text
SITE_RECOVERY_GAPS.md
```

优先级：

1. 补 `/data/*.json` 静态数据契约或对应后端路由
2. 补 `/api/quote`, `/api/macro`, `/api/market`, `/api/premarket-movers`
3. 对齐 `/scan` 的密集筛选按钮和 live 数据字段
4. 对齐 `/stock/:symbol` 文案/结构：live 有 `5 方独立评分`, `谁在持仓`
5. Arena 使用 quote 数据刷新，而不是纯静态价格
6. Whales 确保列表中所有 slug 都能打开详情
7. 做 live 站同款观察列表 drawer

## 注意事项

- 当前 git worktree 很脏，有很多未跟踪文件，不要随便 reset 或删除。
- 不要把“seed/static replica data”说成已经恢复了原始数据库。
- 原站源码没有真正拿回，只是通过 live site probe 反推功能和数据契约。
- 如果要继续安装或修改依赖，优先用项目里已存在的 Node runtime，不要混用系统 Node 14。

