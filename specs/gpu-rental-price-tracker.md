# GPU 租金价格跟踪规格

## 目标

在 StockGod 实验行情线新增 `/gpu-prices`，以可追溯、失败关闭的方式跟踪 RunPod、Modal、Lambda Cloud、Vast.ai 的 GPU 算力价格。页面不提供下单，不把历史快照冒充实时价格，也不混淆 serverless、云实例和市场 offer 的计费口径。

## 来源与认证

- 仅使用厂商官方结构化价格通道；来源核验见 `docs/research/GPU_PRICE_SOURCES.md`。
- RunPod、Lambda、Vast 按官方文档携带后端环境变量中的 API key。
- Modal 使用 workspace token id/secret；密钥不得进入前端、日志、错误正文或配置响应。
- 未配置凭证时 provider 状态为 `unconfigured`，不调用匿名但未承诺的接口，不以静态常量填充动态表格。

## 数据契约

每条报价至少包含：

- `provider`、`gpuModel`、`product`、`billingMode`
- `gpuCount`、可选 `memoryGiB`、`region`、`offerId`、`availability`
- `currency`、`rawPrice`、`rawUnit`
- 统一比较列 `priceUsdPerGpuHour`
- 可选 `instanceTotalUsdPerHour`、`storageUsdPerHour`、`bandwidthUpUsdPerTb`、`bandwidthDownUsdPerTb`
- `sourceUrl`、`observedAt`

禁止把缺失价格转为 `0`。Modal 的秒价可乘 3600 生成小时比较列，但必须保留原始秒价。Vast 多卡 offer 必须同时保留实例总价和每 GPU 换算价。

## 后端行为

- `GET /api/gpu-prices`：立即读取最近一次内存/数据库快照，返回各 provider 状态和报价；无任何快照时仍返回结构化空结果，HTTP 200，状态明确为 unavailable/unconfigured。
- `GET /api/gpu-prices/history`：按 provider、GPU 型号和天数过滤真实观测记录，默认 30 天、结果有上限。
- `POST /api/gpu-prices/refresh`：管理令牌保护并写审计日志；并发刷新返回冲突；各 provider 独立成功或失败。
- 自动刷新低频运行；首次启动异步刷新，不阻塞页面请求。
- 上游失败可保留最后成功快照，但响应必须通过 provider 状态和 `X-Data-Stale` 明确标记过期，不得伪装 live。
- 历史记录落 SQLite；同一 provider/source identity/observedAt 不重复写入。

## 前端行为

- `/gpu-prices` 使用 `StockGodShell`，桌面侧栏、移动菜单和命令面板均可进入。
- 顶部展示全局数据状态、最近观测时间、刷新入口和认证提示。
- 当前报价支持 provider、GPU 型号和计费模式过滤；不同计费模式有明确标签。
- 历史图只消费后端真实 dated history，不用浏览器轮询结果拼出伪历史。
- loading、部分失败、未配置、无历史、全部不可用均有可操作状态；不可信时隐藏比较结论，但保留来源与诊断。
- 390px 与 1440px 无横向溢出，表格在窄屏改为逐条列表。

## 验收

1. 后端 provider 解析、鉴权头、401/429、局部失败、stale fallback、迁移、过滤和管理鉴权均有测试。
2. 前端 adapter 对缺字段、非法价格、未知时间、stale 和 count 不一致失败关闭。
3. Go 测试、前端测试、TypeScript/Vite 构建通过。
4. 8831 运行态接口与桌面/移动浏览器验证通过；未配 key 时页面可用但明确说明没有动态报价。
5. 全系统审计分数不能因“新增一个未配置页面”虚增；必须按实际可用 provider、历史积累和运行态证据计分。
