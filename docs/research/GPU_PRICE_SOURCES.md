# GPU 租用价格一手来源核验

> 核验日期：2026-09-09（Asia/Shanghai）  
> 范围：RunPod、Modal、Lambda Cloud、Vast.ai。只使用厂商官方价格页、文档、API/OpenAPI 和官方客户端源码。  
> 注意：下文是技术与条款风险检查，不是法律意见。“当前可无认证访问”不等于“厂商承诺匿名 API”。

## 结论

| 厂商 | 当前公开价格 | 计费口径 | 官方支持的结构化价格通道 | 无认证结构化数据 | 建议的长期跟踪源 |
|---|---|---|---|---|---|
| RunPod | 公开价格页，USD/GPU-hour 及 per-second 切换 | Pod 计算和大部分存储按秒 | GraphQL `gpuTypes` / `lowestPrice`，文档要求 API key | **当前实测可用，但非承诺契约** | 认证的 GraphQL；人工对照 pricing 页 |
| Modal | 公开价格页，USD/second | 按实际资源时间，无最低使用时长增量 | `modal billing rates --json` / `Workspace.billing.rates()`，需 workspace 认证 | **未找到官方匿名价格 API** | 认证的 billing rates；人工对照 pricing 页 |
| Lambda | 公开价格页，USD/GPU-hour | ODC 按小时定价、每分钟计费 | `GET /api/v1/instance-types`，需 API key | OpenAPI 规范可匿名下载，**实时价格端点不可** | 公开 pricing 页，或认证的 instance-types API |
| Vast.ai | 市场实时浮动，无静态统一价 | 计算按秒；存储、带宽另计 | `POST /api/v0/bundles/`，文档要求 Bearer key | **当前实测可用，但非承诺契约** | 认证的 search offers API / CLI |

## RunPod

### 价格与计费

- 官方 [GPU Cloud Pricing](https://www.runpod.io/pricing) 在核验日展示：Secure Cloud B300 `$7.89/hr`、B200 `$6.79/hr`、H200 `$4.59/hr`、H100 SXM `$3.49/hr`、A100 PCIe/SXM `$1.59/hr`、RTX 4090 `$0.74/hr`。页面可切换 Community/Secure 和 per-hour/per-second。置信度：高。
- [Pods pricing](https://docs.runpod.io/pods/pricing) 明确 Pod 计算与存储按秒计费，无 ingress/egress 费。On-demand 部署前需有足以支付 1 小时的余额，但这不是 1 小时最低计费。置信度：高。
- 同页公开存储价：container disk 运行时 `$0.10/GB/month`，停止后不收费且数据清除；volume disk 运行时 `$0.10/GB/month`、停止时 `$0.20/GB/month`；network volume 低于 1 TB 为 `$0.07/GB/month`、高于 1 TB 为 `$0.05/GB/month`，且按小时计费。置信度：高。
- [Manage Pods](https://docs.runpod.io/pods/manage-pods) 说明 stop 释放 GPU，保留 `/workspace` volume，清空 container disk；停止期间 volume 继续收费。置信度：高。

### API、认证与抓取边界

- [GraphQL Manage Pods](https://docs.runpod.io/sdks/graphql/manage-pods) 官方文档提供 `gpuTypes` 以及 `lowestPrice(input: {gpuCount: 1}) { uninterruptablePrice }` 的结构化查价方式，示例 URL 包含 `api_key`。[GraphQL overview](https://docs.runpod.io/sdks/graphql/configurations) 也称所有请求应包含 API key。置信度：高。
- 2026-09-09 现场对 `https://api.runpod.io/graphql` 无 key POST `gpuTypes` 价格查询，收到 HTTP 200 JSON；RTX 4090 当时 `lowestPrice.uninterruptablePrice=0.34`、`stockStatus=Medium`。另一次实测 `securePrice/communityPrice` 包含 B200 `6.79/5.98`、H100 SXM `3.49/2.69`、RTX 4090 `0.74/0.34`。这是当前后端行为，不是官方匿名 API 承诺。置信度：中高。
- [REST API overview](https://docs.runpod.io/api-reference/overview) 明确称所有 REST 请求要求 Bearer API key；[API key 管理](https://docs.runpod.io/get-started/api-keys) 支持 Read Only/Restricted 权限。置信度：高。
- RunPod [Terms of Service](https://www.runpod.io/legal/terms-of-service) 禁止未经许可的自动化访问、数据挖掘和系统性数据抓取。因此不应自动抓公开 pricing HTML；自动化跟踪应使用厂商明确文档化的 API，配置最小权限 key，并接受价格/库存随时变动。置信度：高。

## Modal

### 价格与计费

- 官方 [Pricing](https://modal.com/pricing) 在核验日展示：B300 `$0.001972/sec`、B200 `$0.001736/sec`、H200 SXM `$0.001261/sec`、H100 SXM `$0.001097/sec`、A100 80 GB `$0.000694/sec`、A100 40 GB `$0.000583/sec`、L40S `$0.000542/sec`。换算后 H100 约 `$3.9492/hour`、A100 80 GB 约 `$2.4984/hour`、B200 约 `$6.2496/hour`。置信度：高。
- [Billing](https://modal.com/docs/guide/billing) 明确无 reservations 要求、无最低使用时长增量，只为使用或请求的 compute 付费。工作区按月结算，不代表计算时长按月取整。置信度：高。
- [Scale](https://modal.com/docs/guide/scale) 说明 Function 无输入时默认 scale-to-zero；`min_containers`、`buffer_containers` 或更长 `scaledown_window` 会保留空闲资源并产生费用。置信度：高。
- 公开 pricing 页的 Volume 价格是 `$0.09/GiB/month`，含 1 TiB/month free。[Volumes](https://modal.com/docs/guide/volumes) 说明用量每日快照，删除数据后最多仍可能计费 4 天。置信度：高。

### API、认证与抓取边界

- [modal billing CLI](https://modal.com/docs/cli/latest/billing) 提供 `modal billing rates --json`，返回当前价格的机器可读 JSON。[Python SDK `Workspace.billing.rates()`](https://modal.com/docs/sdk/py/latest/Workspace#billingrates) 返回当前 workspace 的价格 mapping。置信度：高。
- 这两个通道都有 workspace 上下文。[JavaScript SDK reference](https://modal.com/docs/sdk/js/latest) 明确要求 API token/secret pair；凭证可来自客户端参数、`MODAL_TOKEN_ID`/`MODAL_TOKEN_SECRET` 或 `~/.modal.toml`。官方 [`modal-client` billing CLI 源码](https://github.com/modal-labs/modal-client/blob/main/py/modal/cli/billing.py) 也显示 rates 命令走 `_Workspace.from_context().billing.rates()`。置信度：高。
- 未找到 Modal 官方承诺的无认证结构化价格 API。`modal.com/pricing` 的初始 HTML 目前包含价格，但 DOM 结构不是 API 契约。长期自动化应优先用认证的 `billing rates --json`，以获取 workspace 实际价格结构。置信度：中高。

## Lambda Cloud

### 价格与计费

- 官方 [On-Demand GPU instances](https://lambda.ai/instances) 以 USD/GPU/hour 展示价格，并注明另加适用的 sales tax/VAT/GST。核验日示例：8-GPU B200 `$6.69/GPU/hr`、H100 SXM `$3.99/GPU/hr`、A100 80 GB `$2.79/GPU/hr`；1-GPU B200 `$6.99/GPU/hr`、GH200 `$2.29/GPU/hr`、H100 SXM `$4.29/GPU/hr`、H100 PCIe `$3.29/GPU/hr`、A100 40 GB `$1.99/GPU/hr`。价格会随实例的 GPU 数量而不同。置信度：高。
- [Billing overview](https://docs.lambda.ai/public-cloud/billing/) 明确 On-Demand Cloud 按小时用量定价、每分钟计费。实例启动且通过 health checks 后开始计费，到 terminate 结束；只要仍在运行，即使未主动使用也收费。置信度：高。
- 同页说明 filesystems 按 GiB/month 定价、每小时计费；只要 filesystem 存在就继续收费，即使未挂载到实例，无最低存储期，不收 ingress/egress 费。文档中 `$0.20/GiB/month` 仅是示例，明确警告可能不是当前价。置信度：高。

### API、认证与抓取边界

- 官方 [Cloud API](https://docs.lambda.ai/api/cloud) 的 `GET /api/v1/instance-types` 返回实例规格、价格和区域容量。官方 [OpenAPI 1.10.0](https://cloud.lambda.ai/api/v1/openapi.json) 定义 `price_cents_per_hour` 为“US cents per hour”。置信度：高。
- Cloud API 全局安全要求是 Bearer 或 Basic API key；新集成应优先 Bearer。现场无 key 请求 `https://cloud.lambda.ai/api/v1/instance-types` 返回 HTTP 401，`code=global/invalid-api-key`。[Access and security](https://docs.lambda.ai/public-cloud/access-security/) 还说明 Cloud API keys 对所有 Lambda API 操作有完整权限，因此价格跟踪用 key 需当作高敏感凭证保管。置信度：高。
- `openapi.json` 本身现场无 key 返回 HTTP 200，可用于跟踪 schema 变化，但其价格只是字段示例，不是当前报价。置信度：高。
- Lambda [Terms/AUP](https://lambda.ai/legal/terms-of-service) 要求 web crawling 限速，不得损害服务；Cloud Platform Guidelines 还限制为构建竞争产品或竞争性 benchmark 而监控服务。非竞争性的内部低频跟踪可以公开 pricing 页为人工基准；需稳定自动化时优先官方 API，并限速、保护 key。置信度：中高。

## Vast.ai

### 价格与计费

- [Pricing guide](https://docs.vast.ai/guides/instances/pricing) 明确 Vast.ai 是市场，host 自行设价，价格随供需实时浮动，没有可代表整个平台的静态 GPU 价目表。总成本由 GPU compute、storage 和 bandwidth 组成。置信度：高。
- 同页与 [Billing guide](https://docs.vast.ai/guides/reference/billing) 明确 GPU 计算按活跃时间每秒收费；存储在实例存在时持续收费，stop 不停止存储费，delete 才停止；带宽按实际字节数收费。每台机器的三项价格都可不同。置信度：高。
- 2026-09-09 现场无 key 请求官方 `POST https://console.vast.ai/api/v0/bundles/`，限制 verified/rentable/on-demand，返回 64 个 offers、`truncated=false`。当时每个 GPU 类型最低 `dph_total` 样例：RTX 4090 1 GPU `$0.3222/hour`；A100 PCIe 1 GPU `$1.0022/hour`；H100 NVL 1 GPU `$2.7756/hour`；H100 PCIe 4 GPU 实例 `$8.5356/hour`；H100 SXM 2 GPU 实例 `$4.5348/hour`。价格包含当前 offer 的计算与存储部分，是瞬时快照，不应作为固定报价。置信度：高。

### API、认证与抓取边界

- [Search offers API](https://docs.vast.ai/api-reference/search/search-offers) 的正式请求是 `POST /api/v0/bundles/`，返回 offer 级结构化报价，可分 on-demand、bid/interruptible、reserved；文档标记 `Authorization: Bearer <token>` 为 required。[Authentication](https://docs.vast.ai/api-reference/authentication) 更明确称每个 API 请求都必须包含 API key，并建议 CI/共享工具使用 scoped keys。置信度：高。
- 虽然上述无 key 实测成功，但它与官方认证文档相冲突，只能视为当前宽松实现，不能当作无认证 SLA。生产跟踪应按文档使用最小权限 Bearer key，并保留 401/403/429、schema 和行情波动处理。置信度：高。
- Vast.ai [Terms of Use](https://console.vast.ai/terms/) 禁止未经书面许可的系统性网页数据获取以创建 collection/database。因此不应用 HTML scraping 建长期价格库；应使用官方明确提供的认证 search offers API/CLI，仍需遵守账号条款、权限和限频。置信度：高。

## 实施建议

1. 价格快照必须存 `observed_at`、币种、原始单位、GPU 数量、产品线/云类型、region/offer ID，不要只存“GPU 名 + 每小时价”。
2. 保留厂商原始价格和换算价两列；Modal 从每秒换算每小时时乘 3600，不改写原值。
3. Vast.ai 的 `dph_base` 与 `dph_total` 要分开，并额外存储、上/下行带宽价；多 GPU offer 同时保留实例总价和每 GPU 换算价。
4. RunPod 和 Vast.ai 的匿名结构化响应只能做监测/降级信号，不能作生产契约；生产必须使用官方文档所要求的凭证。
5. 对 API 做条件请求/合理限频，保留最后成功快照但显式标记 stale；不应在 API 失败时静默把历史价当成当前价。
6. 上线前由责任人再复核四家的当期 Terms/AUP 和 API 限频；对 RunPod/Vast.ai 禁止网页系统化抓取的条款，不用技术可达性替代授权。

## 核验限制

- 价格是时点快照，本文不保证阅读时仍相同。RunPod/Vast.ai 还叠加云类型、库存、offer、地区和存储等因素。
- 本轮没有 Modal 凭证，因此没有现场执行 `modal billing rates --json`；该命令的结构化输出与认证链由官方文档及官方客户端源码确认。
- RunPod 与 Vast.ai 的无 key 响应是本轮现场观测；因为官方文档要求 key，不应将其外推为未来可用性。
- 价格页目前的初始 HTML 均含默认视图报价，但 DOM/CSS 结构不属于版本化 API，不应被视为稳定 schema。
