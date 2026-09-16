# RunPod / Vast.ai 免凭据价格端点复核

> 核验日期：2026-09-10（Asia/Shanghai）  
> 范围：只使用供应商官方文档、API 参考、官方开源客户端与条款。  
> 判定口径：“当前无 key 能返回”是现场行为；“官方允许/承诺无 key”必须有公开文档或契约支持。本文不构成法律意见。

## 结论

| 供应商 | 官方承诺的免 key 结构化价格 feed | 现场无 key 行为 | 生产判定 |
|---|---|---|---|
| RunPod | **未找到** | v2 Catalog `401`；旧 GraphQL `gpuTypes` `200` | 使用认证的 v2 Catalog；旧 GraphQL 匿名响应不是契约，且 GraphQL 已宣布将退役 |
| Vast.ai | **未找到** | Search Offers `200`；GPU Metrics current/history `403` | 使用认证的 Search Offers 或 GPU Metrics；匿名 Offers 响应不是契约 |

**最终答案：截至本次核验，RunPod 和 Vast.ai 都没有可证实为“无需凭据、官方承诺、结构化”的 GPU 价格 feed。** 两家都有公开网页价格信息，但 HTML 展示页不是版本化的 JSON/CSV/API 契约。

## RunPod findings

| Claim | 官方证据 | 置信度 |
|---|---|---|
| RunPod 最新的结构化 GPU 目录是 `GET /v2/catalog/gpus`，响应包含 `price.secure` / `price.community` / `price.serverless`，但 `Authorization` 明确标记为 `required`。 | [List GPU types](https://docs.runpod.io/api-reference-v2/catalog/list-gpu-types) 的官方请求示例携带 `Authorization: Bearer <token>`，并定义了缺 token 的 `401 missing bearer token`。 | 高 |
| RunPod REST API 的全局契约是所有请求需要 API key，不支持将 Catalog 解释为匿名公共 feed。 | [REST API overview](https://docs.runpod.io/api-reference/overview) 明确写明所有请求都需要 RunPod API key。 | 高 |
| 旧 GraphQL 可查 `gpuTypes` 与 `lowestPrice`，但官方用法仍要求 key，而且该 API 已弃用、计划在 2027 年初退役。 | [GraphQL overview](https://docs.runpod.io/sdks/graphql/configurations) 说明请求 URL 包含 API key 并标注弃用时间；[Manage Pods: Get GPU type details](https://docs.runpod.io/sdks/graphql/manage-pods#list-all-gpu-types) 的官方 `gpuTypes` / `lowestPrice` 示例使用 `?api_key=${YOUR_API_KEY}`。 | 高 |
| 官方 CLI 也未把价格查询定义为匿名能力。 | 官方 `runpodctl` 的 [`gpu list`](https://github.com/runpod/runpodctl/blob/main/cmd/gpu/list.go) 输出 GPU 价格，但 [`NewClient`](https://github.com/runpod/runpodctl/blob/main/internal/api/client.go) 在 key 缺失时返回 `ErrNoCredentials`。 | 高 |
| 2026-09-10 17:01 CST 的无 key 实测：v2 Catalog 返回 HTTP `401` / `missing bearer token`；旧 GraphQL `gpuTypes` 返回 HTTP `200` 且有 46 个 GPU 记录。后者只是当前实现宽松，不能作为持续匿名可用的生产契约。 | 官方文档对 REST 和 GraphQL 都规定 key，且 [RunPod Terms of Service](https://www.runpod.io/legal/terms-of-service) 限制未授权的自动化访问和系统化数据获取。实测仅证明当时技术可达。 | 高 |
| 未找到 RunPod 官方承诺的免凭据结构化价格 feed。 | 官方 [pricing page](https://www.runpod.io/pricing) 是可匿名浏览的 HTML 展示页；官方 [documentation index](https://docs.runpod.io/llms.txt) 列出的结构化 GPU 价格通道是需 key 的 v2 Catalog 和 GraphQL。“未找到”是本次官方资料检索结论，不是对未公开服务的绝对断言。 | 高 |

## Vast.ai findings

| Claim | 官方证据 | 置信度 |
|---|---|---|
| Vast.ai 没有全平台固定 GPU 价目表；价格由 host 设定并随供需实时波动。官方指定的程序化查价通道是 CLI 或 Search Offers API。 | [Pricing guide](https://docs.vast.ai/guides/instances/pricing) 明确说明 marketplace 模式、无 static price quotes，并指向 `vastai search offers` 和 Search Offers API。 | 高 |
| Search Offers `POST /api/v0/bundles/` 是包含 offer 级实时价格的官方结构化通道，但 Bearer `Authorization` 明确标记为 `required`。 | [Search offers API](https://docs.vast.ai/api-reference/search/search-offers) 的请求示例携带 Bearer token，授权字段标记必需。[Authentication](https://docs.vast.ai/api-reference/authentication) 进一步明确每个 Vast.ai API 请求都必须包含 API key。 | 高 |
| Vast.ai 现在还有更接近“价格 feed”的 GPU current/history metrics，但两者也都要求 key。 | [Show GPU metrics](https://docs.vast.ai/api-reference/machines/show-gpu-metrics) 和 [Show GPU trends](https://docs.vast.ai/api-reference/machines/show-gpu-trends) 均在 `Authorization` 字段标记 `required`；对应返回当前供需/价格快照和历史时序。 | 高 |
| 2026-09-10 17:01 CST 的无 key 实测：Search Offers 返回 HTTP `200` 和 offer JSON；GPU Metrics current/history 均返回 HTTP `403` / `This action requires login.`。Offers 的匿名成功与全局认证文档相冲突，只能归类为未文档化实现行为。 | [Authentication](https://docs.vast.ai/api-reference/authentication) 和 [Search offers API](https://docs.vast.ai/api-reference/search/search-offers) 是生产集成应遵循的公开契约，不能由一次匿名 `200` 反向推导出免认证 SLA。 | 高 |
| 即使不考虑 API 认证，Vast.ai 条款也不支持通过系统化网页获取来替代正式 API。 | [Terms of Use](https://console.vast.ai/terms/) 的 Prohibited Activities 禁止未经书面许可系统化获取网站数据以建立 collection/compilation/database，并保留随时更改价格的权利。 | 高 |
| 未找到 Vast.ai 官方承诺的免凭据结构化价格 feed。 | 官方 [documentation index](https://docs.vast.ai/llms.txt) 列出 Search Offers、GPU Metrics 和 GPU Trends，但它们均归属需 key 的 API；Pricing guide 仅另外指向面向人的 Dashboard。“未找到”是本次官方资料检索结论。 | 高 |

## 生产约束

1. RunPod 优先使用认证的 v2 `GET /v2/catalog/gpus`；不应新建依赖旧 GraphQL 的集成。
2. Vast.ai 如需 offer 级报价，使用认证的 Search Offers；如需聚合当前/历史价格，使用认证的 GPU Metrics/Trends。
3. 两家都应使用最小权限 key，并显式处理 `401` / `403` / `429`、schema 变化和价格时效。
4. 如业务硬性要求“零凭据”，这两家在本次官方证据下都不符合；不能用当前匿名 `200` 规避这个约束。

## 复核限制

- 无 key 实测是单一时点的只读探测，不是可用性、限频、schema 或授权的压力测试。
- 本次没有使用 RunPod/Vast.ai 账号凭据，因此没有验证认证后的实时响应；认证契约由官方文档确认。
- 价格与库存随时间变动，本文不把现场样例价格写成持续报价。
