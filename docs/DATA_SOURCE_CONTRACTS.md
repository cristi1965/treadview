# 数据源契约

本页记录无需密钥的数据源及其 fail-closed 边界。抓取完成时间只能写入
`refreshedAt`，不得冒充来源数据时间。

## Nasdaq 行情 fallback

- 使用 Nasdaq.com 的 quote info JSON：<https://api.nasdaq.com/api/quote/AAPL/info?assetclass=stocks>。
- 来源时间取 `data.primaryData.lastTradeTimestamp`，交易状态取 `data.marketStatus`。
- 币种优先取 `data.primaryData.currency`；字段为空时，仅对版本化美股 instrument master 中存在且 `assetclass=stocks`
  的 symbol 补 `USD`。响应用独立 `currencySource=instrument-master:stockgod-us-symbol-universe-v1@<generated_at>`
  记录来源，不声称该币种由 Nasdaq 返回。
- symbol、价格、来源成交时间或市场状态缺失时放弃该来源，继续走 stale snapshot，不用请求完成时间补齐。
- 该接口是 Nasdaq.com 页面内部接口，不是版本化开发者 API；fixture 测试覆盖当前契约，运行时仍需 fail closed。

## SEC EDGAR 基本面

- API 契约：<https://www.sec.gov/search-filings/edgar-application-programming-interfaces>。
- CIK 映射优先使用 SEC company browse Atom（按 ticker 查询，响应较小），失败后回退
  <https://www.sec.gov/files/company_tickers.json>；两者都必须返回可解析的官方 CIK，不使用本地猜测映射。
- Company Facts：`https://data.sec.gov/api/xbrl/companyfacts/CIK##########.json`。
- 每个字段只取同一 10-K/10-Q accession、form、filed、period end 的事实；流量指标还必须匹配 period start。
  每个可用字段保留 `periodStart/periodEnd/form/filed/accession/sourceURL/unit/periodKind`。季度 duration 必须为 70-110 日，年度 duration 必须为 330-400 日；YTD 累计值不得进入离散季度序列。缺失或语义不可证明的字段输出 `null`，不填 `0`。
- `SEC_EDGAR_USER_AGENT` 可配置符合 SEC 自动访问政策的联系信息。

## Nasdaq 财务期次 fallback

- SEC EDGAR 仍为美股基本面第一来源；SEC 网络或报表数据不可用时，才尝试 Nasdaq.com financials。
- 年度端点为 `https://api.nasdaq.com/api/company/{symbol}/financials?frequency=1`，季度为 `frequency=2`；
  期次必须从 `Period Ending:` / `Quarterly Ending:` 表头解析，不用本地时间猜测。
- [Nasdaq 官方财务页](https://www.nasdaq.com/market-activity/stocks/aapl/financials)标明数值单位为 `USD Thousands`；
  解析后按字段保留 source URL、period、frequency 和 unit，缺失值保持未知，不填 `0`。

## Nasdaq 20 日 ADV

- 来源为 `https://api.nasdaq.com/api/quote/{symbol}/historical?assetclass=stocks&fromdate=YYYY-MM-DD&todate=YYYY-MM-DD`。
  [Nasdaq Historical Quotes](https://www.nasdaq.com/market-activity/quotes/historical) 说明日级历史数据包含价格和成交量。
- 查询参数用 ISO 日期；每条观测必须有可解析交易日和正成交量，周末、重复日期、空值均不计入。
- 只接受完整 20 个交易日窗口，返回 window/start/end/source/sourceURL/refreshedAt 及原始日观测；
  任一 symbol 不足 20 日时整批 fail closed，不用单日 volume 冒充 ADV。

## Evidence-only 历史研究价契约

- Nasdaq dated historical OHLCV 的最新已完成交易日收盘是唯一主研究价证据，必须保留日期、
  provider URL、原始 payload hash，并以 `as_of_research_price:<date>` 公式和 `tool-historical` evidence ID 披露。
- evidence-only PIT preflight 必需 historical、filing-bound fundamentals 和 reviewed news；quote info 不是必需源。
  普通 10-Agent `RunResearchPreflight` 仍必须包含 `get_stock_data`。
- quote 只是可选 cross-check；缺失、日期不同或价格不同都记入 gap，不阻断 historical-primary dossier。
  quote 与 historical 可能同属 Nasdaq 数据链，不得表述为独立交叉验证。
- OHLCV 行需要正数 open/high/low/close/volume，且 high/low 必须包含 open 和 close；非法行不进入统计或子模型。
- 21 个有效收盘观测对应 20 个日收盘收益，足以计算20日收益和年化波动率；不额外要求第22个观测。
- filing-bound 期间保留 `frequency/form/periodStart/periodEnd/filingDate/accession/sourceURL`。目标为至少
  5 个离散季度和 3 个年度；每个可用字段另保留严格 filing 绑定。真实源不足时明确披露 available/target，不补造期间，`historical-research` readiness 保持 503。
- QoQ/YoY 要求同频、同字段期间语义且 duration 长度相差不超过 14 日；TTM 只允许四个连续、各自通过 70-110 日门禁的季度值。无法证明则结果为 `unknown`。
- evidence-only 请求可选 `research_context.mandate/holdings/liquidity/tax/risk_budget`。上下文原样参与审计输入 hash；缺项仅记录 gap，不推断、不用于生成交易动作。
- 顶层与期间财务缺失值序列化为 `null`，真实报告的 0 通过 `availability/availableFields` 保留；
  unknown 计算不携带伪造的 0 输入。
- 新闻只保留 ticker 出现在标题，或公司在标题中是明确主体的项；描述中的单纯提及和列表式旁带提及会被排除。

## OHLCV 市场子模型

- `ohlcv-5d-momentum-next-session-v1` 只使用 as-of 时可见的 5 日收盘动量，标签为下一交易日收盘收益。
- 最新 30 个样本固定作样本外集，披露数据集 SHA-256、Spearman IC/95% CI、方向准确率/95% CI 和多数类基线。
- 该报告独立命名为 market-price-only submodel，`validates_five_factor_panel=false`。五方基本面分整体继续
  `unvalidated`，即使输入历史五方样本也只输出评估指标，不自动改成 validated。
- 从 evidence-only 结果重算并可选附加到 panel：

  ```bash
  cd app/backend
  go run ./cmd/validate-price-model -input /path/to/evidence-only-result.json
  go run ./cmd/validate-price-model -input /path/to/evidence-only-result.json -panel data/us-panel-summary.json
  ```

## 快照时间与评分验证

- 当单 symbol 快照没有自身观测时间时，`X-Data-Source` 标记
  `stale-snapshot:snapshot-batch-time`，`X-Data-Time` 只表示快照批次时间，不表示成交时间。
- 当前没有可审计的点时历史五因子、历史 market cap 和前瞻基准收益联合样本，
  因此不生成伪历史样本，验证状态保持 `unvalidated`。

## 2026 A/H 交易日历

- A 股休市以[上交所公告](https://www.sse.com.cn/disclosure/announcement/general/c/c_20251222_10802507.shtml)和[深交所公告](https://investor.szse.cn/disclosure/notice/general/t20251222_618087.html)为准。
- 港股全日休市及半日市以[港交所 2026 通告](https://www.hkex.com.hk/-/media/HKEX-Market/Services/Circulars-and-Notices/Participant-and-Members-Circulars/SEHK/2025/ce_SEHK_CT_075_2025.pdf)为准。
- 当前内置日历只覆盖 2026；超出维护年份时 session fail closed 为 `closed`。
