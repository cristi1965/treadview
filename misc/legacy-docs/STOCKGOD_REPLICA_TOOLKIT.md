# StockGod Full Replica Toolkit

更新时间: 2026-08-01 CST

## 结论

要“完完全全所有功能原站复刻”，不要指望一个网页下载器自动搞定。StockGod 是 SPA，需要组合工具链：

1. Playwright / site recovery 抓原站 DOM、截图、按钮、输入框、网络请求。
2. route discovery 扩全路由和详情页。
3. contracts 抽 `/data/*`、`/api/*`、RSC/JSON 结构。
4. interaction probe 自动点按钮、填输入、查坏链路。
5. Wails 本地壳验证桌面环境。
6. `docs/SITE_RECOVERY_GAPS.md` 做 P0/P1/P2 缺口矩阵。

## 已新增项目 Skill

项目技能:

```text
.cursor/skills/stockgod-full-replica/SKILL.md
```

触发场景:

- “原站复刻”
- “StockGod 完整恢复”
- “live/local 对齐”
- “找没复刻的功能”
- “按钮/输入/路由/接口差异检查”

## 本项目优先使用的工具

| 工具 | 位置 | 用途 |
| --- | --- | --- |
| Live capture | `scripts/stockgod_recovery/capture_live.mjs` | 抓 HTML、DOM、控件、截图、API/JSON/RSC 响应 |
| Route discovery | `scripts/stockgod_recovery/discover_routes.mjs` | 从 live 捕获里发现更多详情页 |
| Contracts | `scripts/stockgod_recovery/build_contracts.mjs` | 生成路由/接口/数据契约 |
| Fast probe | `scripts/site-recovery-probe.mjs` | 快速对比 live/local 页面快照 |
| Interaction probe | `scripts/stockgod_recovery/interaction_probe.mjs` | 自动点按钮、测输入、查坏响应 |
| Replay manifest | `scripts/stockgod_recovery/build_replay_manifest.mjs` | 构建本地 fixture/replay |
| Replay verify | `scripts/stockgod_recovery/verify_replay.mjs` | 验证 replay 覆盖和文本比例 |

## 外部可参考工具

| 工具 | 适合做什么 | 注意 |
| --- | --- | --- |
| Playwright | SPA 自动化、截图、点击、输入、网络监听 | 本项目主力 |
| Browsertrix Crawler | 大范围 crawl / WARC 存档 | 适合补全路由面 |
| Crawlee | 自定义 JS 爬虫、Playwright/Puppeteer crawler | 适合写定制 route/data harvest |
| ArchiveBox | 页面和资产归档 | 更偏归档，不保证交互 |
| HTTrack / wget | 静态资源镜像 | 不适合单独复刻 React SPA |
| Percy / Chromatic / Playwright screenshot diff | 视觉回归 | 适合后期防回归 |
| mitmproxy / HAR | 网络接口分析 | 可补 Playwright response capture 盲区 |

## 推荐下一步

先跑一次 expanded capture + contracts，把“所有原站功能”变成可执行清单：

```bash
PATH=/Users/jingxin/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH \
node scripts/stockgod_recovery/capture_live.mjs \
  --origin https://stockgod.xyz \
  --out recovered_source/stockgod-live-$(date +%Y%m%d) \
  --routes /,/scan,/etf,/whales,/arena,/reports,/notes,/portfolio,/macro,/stock/NVDA,/about,/terms,/privacy,/how-to-buy
```

然后用 `discover_routes.mjs` 扩全详情页，再按 P0/P1/P2 补实现。

## 2026-08-01 试跑记录

已用项目 skill 跑过一轮小样本:

- Live 快照: `recovered_source/site-recovery/live-skill-trial-20260801/site-probe.json`
- Local 快照: `recovered_source/site-recovery/local-skill-trial-backend-after-tool-fix-20260801/site-probe.json`
- Local 交互: `recovered_source/stockgod-interactions-backend-after-tool-fix-20260801.json`
- `/stock/NVDA` 首屏修复后复测: `recovered_source/site-recovery/local-stock-detail-after-progressive-render-20260801/site-probe.json`

工具优化:

- `site-recovery-probe.mjs` 默认改为 `domcontentloaded + --settle-ms`，避免被长行情请求拖成假超时。
- `interaction_probe.mjs` 同步支持 `--wait-until` / `--settle-ms`。
- 两个本地探针都会在 localhost/127.0.0.1 路由前检查 `/api/health`，服务不可用时输出明确 health 失败，不再产出“空页面”误报。
