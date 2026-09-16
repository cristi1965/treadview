# 行情 API 与 TradingView 方案（2026-09-11）

## 结论

本地自用、仅模拟交易的推荐组合：

- 图表：保留 TradingView 免费 Widget。
- 美股报价：先用 Alpaca Basic 验证；需要全市场实时行情时升级 Algo Trader Plus（当前公开价 99 USD/月）。
- A 股报价：优先接富途 OpenAPI/OpenD，并在账户内确认 A 股 LV1 权限；无法取得权限时评估掘金量化。
- 历史与基本面：Tushare；AKShare 只作研究和备用验证，不作为成交价权威源。
- Paper 成交：后端提交时重新询价，记录 source、data time、bid/ask/trade 和 freshness；过期或来源不可验证时拒绝成交。

## 方案比较

| 方案 | 当前公开价格/门槛 | 适合 | 不适合 |
|---|---:|---|---|
| TradingView Widget | 免费 | 内嵌图表、指标和人工观察 | 后端报价、模拟成交价、数据再分发 |
| Alpaca Basic | 0 USD/月 | 美股原型、IEX 实时、有限订阅 | 全市场 NBBO/SIP 口径 |
| Alpaca Algo Trader Plus | 99 USD/月 | 本地个人项目的美股全市场实时行情 | 未经授权的公开再分发 |
| Massive Stocks Advanced | 199 USD/月 | 更完整的美股实时、历史和 WebSocket | 当前本地项目性价比较低 |
| 富途 OpenAPI/OpenD | API 通常不另收费；行情取决于账户权限/行情卡 | A/H/美股行情、本地网关、Paper | 未取得行情权限或公开再分发 |
| 掘金量化 | 价格需按账户/销售确认 | A 股 tick、盘口、bar | 无明确订阅合同前直接承诺成本 |
| Tushare | 积分/付费分层 | A 股历史、基本面、ETF 元数据 | 订单时点实时成交价 |
| AKShare | 开源免费 | 研究、回填、交叉验证 | SLA、授权明确的实时成交依据 |

## 三档落地

1. 零成本验证：TradingView Widget + Alpaca Basic(IEX) + 富途已有行情权限。界面必须标出 IEX-only 和来源覆盖范围。
2. 推荐：Alpaca Plus 99 USD/月 + 富途 A 股 LV1。对本地 Paper 系统的价格、稳定性和开发量最平衡。
3. 对外商业：单独询价企业行情与展示/再分发许可。个人订阅价格不能推定覆盖公开产品。

## 工程接入原则

- TradingView 仅负责图表，不向订单服务提供可信报价。
- Go 后端增加 US/CN provider adapter；前端不持有供应商密钥。
- WebSocket 更新内存行情，REST snapshot 定期校准；断线后自动降级但不伪装实时。
- 下单和成交时由后端重新询价，保留来源、交易所、行情时间、接收时间和新鲜度。
- Yahoo、东方财富抓取及 AKShare 只作 fallback；来源不可验证或超时则 fail closed。

## 来源

- TradingView Widget Docs: https://www.tradingview.com/widget-docs/
- TradingView Getting Started: https://www.tradingview.com/widget-docs/getting-started/
- Alpaca Market Data Pricing: https://alpaca.markets/data
- Alpaca Market Data FAQ: https://docs.alpaca.markets/us/docs/market-data-faq
- Massive Stocks Pricing: https://massive.com/pricing?product=stocks
- Futu OpenAPI Quote Authority: https://openapi.futunn.com/futu-api-doc/intro/authority.html
- Futu OpenAPI Fees: https://openapi.futunn.com/futu-api-doc/intro/fee.html
- MyQuant Quote API: https://www.myquant.cn/docs/l3333/923
- Tushare Docs: https://tushare.pro/document/1?doc_id=108
- AKShare GitHub: https://github.com/akfamily/akshare

知乎相关内容只作为使用体验线索，没有用于价格、授权或 SLA 结论；这些结论均以供应商官方文档为准。
