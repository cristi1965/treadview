import React from 'react'
import type { EvidenceOnlyDossier, ResearchDecision } from '../../stores/analysisStore'

type Language = 'zh' | 'en'

const FIXED_ZH: Record<string, string> = {
  'Evidence-only research dossier; no LLM and not a 10-Agent analysis': '仅证据研究包；未调用 LLM，也不是 10-Agent 分析',
  'For auditable research review only; not investment advice and not an executable trade instruction.': '仅用于可审计研究复核；不构成投资建议，也不是可执行的交易指令。',
  'No verified holdings, mandate, liquidity requirement, or tax context was supplied.': '未提供经核验的持仓、投资授权、流动性需求或税务背景。',
  'No LLM debate, forecast, valuation target, trade instruction, or historical signal validation was performed.': '未执行 LLM 辩论、预测、估值目标、交易指令或历史信号验证。',
  'No LLM debate, forecast, valuation target, trade instruction, or validation of the five-factor panel was performed.': '未执行 LLM 辩论、预测、估值目标、交易指令或五因子面板验证。',
  'Optional mandate, holdings, liquidity, tax, and risk-budget context is stored in the audit input hash; omitted context is reported as a gap and never inferred.': '可选的投资授权、持仓、流动性、税务和风险预算会写入审计输入哈希；未提供的上下文会列为缺口，不会推断。',
  'Holdings were not supplied; no position exposure is inferred.': '未提供持仓；不推断任何仓位暴露。',
  'mandate was not supplied': '未提供投资授权',
  'Holdings were not supplied': '未提供持仓',
  'Liquidity requirements were not supplied': '未提供流动性要求',
  'Tax context was not supplied': '未提供税务背景',
  'Risk budget was not supplied': '未提供风险预算',
  'Structured issuer_cap_pct was not supplied; free-text risk budget is not parsed into a limit.': '未提供结构化发行人上限；自由文本风险预算不会被解析为数值限额。',
  'User-supplied sensitivity assumptions only; not a forecast, target price, or investment advice.': '仅为用户提供的敏感性假设；不是预测、目标价或投资建议。',
  'No research context was supplied; no portfolio constraint or scenario is inferred.': '未提供研究上下文；不推断组合约束或情景。',
  'current ticker is absent from supplied holdings': '已提供持仓中没有当前标的',
  'multiple entries for current ticker make weight ambiguous': '当前标的存在多条持仓记录，权重含义不明确',
  'current ticker weight_pct is missing or invalid': '当前标的的 weight_pct 缺失或无效',
  'issuer_cap_pct and ticker weight are required': '需要同时提供 issuer_cap_pct 与当前标的权重',
  'issuer_cap_pct is missing; free-text risk budget is not parsed': 'issuer_cap_pct 缺失；自由文本风险预算不会被解析',
  'optional scenario assumptions not supplied': '未提供可选情景假设',
  'scenario evidence is incomplete': '情景计算证据不完整',
  'Market and news observations can become stale after the stated provider data times.': '市场与新闻观察在所示供应商数据时间之后可能过期。',
  'Filing values may require issuer-specific accounting interpretation; missing fields remain zero/absent and are not estimated.': '披露数值可能需要结合发行人会计口径解读；缺失字段保持缺失，不作估算。',
  'Filing values may require issuer-specific accounting interpretation; missing fields remain null/unknown and are not estimated.': '披露数值可能需要结合发行人会计口径解读；缺失字段保持空值或未知，不作估算。',
  'News source tiers and topic labels are deterministic review aids, not truth or sentiment scores.': '新闻来源等级与主题标签只是确定性复核工具，不是真实性或情绪评分。',
  'Quote, filing-bound financial periods, news timestamps, and independent OHLCV history passed the trade-date PIT gate.': '行情、绑定披露日的财务期、新闻时间戳与独立 OHLCV 历史已通过交易日时点检查。',
  'All displayed transforms use the listed formulas, exact numeric inputs, and linked evidence IDs; unknown values are not extrapolated.': '所有展示转换均使用列明公式、精确数值输入和关联证据 ID；未知值不外推。',
  'The conclusion is limited to evidence review and does not encode a buy, sell, hold, sizing, or execution recommendation.': '结论仅限证据复核，不包含买入、卖出、持有、仓位或执行建议。',
  'The latest completed close in dated Nasdaq historical OHLCV is the sole primary research price and passed the trade-date PIT gate.': '带日期的 Nasdaq 历史 OHLCV 中，最近一个已完成收盘价是唯一主研究价格，并已通过交易日时点门禁。',
  'A quote, when captured, is an optional disclosed cross-check only; it neither blocks the dossier nor establishes independent confirmation.': '如能取得报价，它只作为已披露的可选交叉核对；既不阻断研究包，也不构成独立确认。',
  'The optional quote cross-check may share Nasdaq lineage with the primary historical source and is never represented as independent confirmation.': '可选报价核对可能与主历史数据同属 Nasdaq 来源链，因此不被表述为独立确认。',
  'Filing-bound shares or market capitalization was unavailable; P/S and P/B remain unavailable.': '缺少绑定披露日的股本或市值；市销率和市净率保持不可用。',
  'no comparable period 330-400 days earlier': '未找到 330-400 天前的可比较期间',
  'no same-frequency comparable period 330-400 days earlier': '未找到 330-400 天前同频率的可比较期间',
  'filing-bound shares or market capitalization is unavailable': '缺少绑定披露日的股本或市值',
  'required field is missing or denominator is zero': '必需字段缺失或分母为零',
  'latest quarterly periods are not four consecutive 60-120 day intervals': '最新季度期间不是四个连续的 60-120 天区间',
  'include only if title or description contains the ticker token or a non-generic company-name token': '仅纳入标题或描述包含股票代码或非通用公司名称词元的新闻',
  'include only if the title contains the ticker token or identifies the company as a title subject; description-only and incidental list mentions are excluded': '仅纳入标题包含股票代码或明确以该公司为主题的新闻；仅在描述中出现或清单式顺带提及的内容会被剔除',
  'deduplicate by normalized title and canonical URL without query or fragment': '按标准化标题与去除查询参数和片段的规范 URL 去重',
  'source tier is a deterministic domain class; content type and theme are keyword classifications, not sentiment': '来源等级是确定性域名分类；内容类型和主题是关键词分类，不是情绪判断',
}

const CALCULATION_ZH: Record<string, string> = {
  as_of_research_price: '截至研究日的主研究价格',
  return_1d: '1 日收益率', return_5d: '5 日收益率', return_20d: '20 日收益率',
  annualized_volatility: '年化波动率', maximum_drawdown: '最大回撤', average_daily_volume_20d: '20 日平均成交量',
  gross_margin: '毛利率', operating_margin: '营业利润率', net_margin: '净利率', liability_ratio: '负债率',
  revenue_qoq: '营收环比', revenue_yoy: '营收同比', net_income_qoq: '净利润环比', net_income_yoy: '净利润同比',
  gross_margin_change_qoq_pp: '毛利率环比变化', operating_margin_change_qoq_pp: '营业利润率环比变化', net_margin_change_qoq_pp: '净利率环比变化', liability_ratio_change_qoq_pp: '负债率环比变化',
  cash_change_qoq: '现金及等价物环比变化', ttm_revenue: '过去十二个月营收', ttm_net_income: '过去十二个月净利润',
  price_to_sales: '市销率', price_to_book: '市净率',
}

const META_ZH: Record<string, string> = {
  historical: '历史行情', fundamentals: '基本面披露', news: '新闻', quote_cross_check: '报价交叉核对',
  computed: '已计算', unknown: '未知', complete: '完整', included: '纳入', excluded: '剔除',
  'tier-2-established-media': '二级：成熟媒体', 'tier-3-other': '三级：其他来源',
  'reported-fact': '事实报道', 'opinion-or-forecast': '观点或预测',
  'management-and-governance': '管理与治理', 'products-and-operations': '产品与经营', other: '其他',
  'ticker-title-token': '标题包含股票代码',
}

export const formatEvidenceDecision = (decision: ResearchDecision | string, language: Language) => {
  if (decision === 'OBSERVE') return language === 'zh' ? '仅观察（不是持有/买卖建议）' : 'Observe only (not a hold, buy, or sell recommendation)'
  if (decision === 'research_unavailable') return language === 'zh' ? '研究不可用（不是投资建议）' : 'Research unavailable (not investment advice)'
  return decision
}

const localizeDynamic = (text: string, language: Language) => {
  if (language === 'en' || !text) return text
  if (FIXED_ZH[text]) return FIXED_ZH[text]
  if (META_ZH[text]) return META_ZH[text]

  const calculation = text.match(/^([a-z0-9_]+)(?::(.+))?$/)
  if (calculation && CALCULATION_ZH[calculation[1]]) return `${CALCULATION_ZH[calculation[1]]}${calculation[2] ? `（${calculation[2]}）` : ''}`

  const patterns: Array<[RegExp, (...values: string[]) => string]> = [
    [/^Captured (\d+) dated market observations; conservative data time (.+)\.$/, (count, time) => `已捕获 ${count} 条带日期的市场观察；保守数据时间为 ${time}。`],
    [/^Captured (\d+) independent dated OHLCV observations; conservative data time (.+)\.$/, (count, time) => `已捕获 ${count} 条独立、带日期的 OHLCV 观察；保守数据时间为 ${time}。`],
    [/^Captured (\d+) dated Nasdaq OHLCV observations; latest completed close at (.+) is the primary research price\.$/, (count, time) => `已捕获 ${count} 条带日期的 Nasdaq OHLCV 观察；${time} 的最近已完成收盘价为主研究价格。`],
    [/^Captured (\d+) filing-bound financial periods; latest filing date (.+)\.$/, (count, date) => `已捕获 ${count} 个绑定披露日的财务期；最近披露日为 ${date}。`],
    [/^Captured (\d+) timestamped news items; latest publication (.+)\.$/, (count, time) => `已捕获 ${count} 条带时间戳的新闻；最近发布时间为 ${time}。`],
    [/^Captured an optional quote cross-check at (.+); it is not required and is not presented as independent from the Nasdaq historical source\.$/, (time) => `已捕获 ${time} 的可选报价核对；该项不是必需输入，也不被视为独立于 Nasdaq 历史来源的确认。`],
    [/^Comparable quarterly history is incomplete: available=(\d+) target=(\d+); missing periods are not synthesized\.$/, (available, target) => `可比季度历史不完整：现有 ${available} 期，目标 ${target} 期；不会合成缺失期间。`],
    [/^Comparable annual history is incomplete: available=(\d+) target=(\d+); missing periods are not synthesized\.$/, (available, target) => `可比年度历史不完整：现有 ${available} 期，目标 ${target} 期；不会合成缺失期间。`],
    [/^Optional quote cross-check trading date conflict: quote=(.+) history=(.+); research price remains historical close ([\d.]+)\. Quote provider=(.+) and historical provider=(.+); this cross-check is optional and is not treated as independent primary market evidence\.$/, (quote, history, price, quoteProvider, historyProvider) => `可选报价核对存在交易日冲突：报价日期 ${quote}，历史日期 ${history}；主研究价格仍采用历史收盘价 ${price}。报价来源 ${quoteProvider}，历史来源 ${historyProvider}；该核对不作为独立主市场证据。`],
    [/^requires (\d+) quarterly periods; available=(\d+)$/, (required, available) => `需要 ${required} 个季度，当前仅有 ${available} 个`],
    [/^company-title-subject:(.+)$/, (company) => `标题主题为公司：${company}`],
    [/^Ticker holding headroom is unknown: (.+)$/, (reason) => `当前标的持仓剩余额度未知：${localizeDynamic(reason, language)}`],
  ]
  for (const [pattern, formatter] of patterns) {
    const match = text.match(pattern)
    if (match) return formatter(...match.slice(1))
  }
  return text
}

const number = (value?: number) => value == null || !Number.isFinite(value) ? '—' : new Intl.NumberFormat('en-US', { maximumFractionDigits: 4 }).format(value)

const copy = {
  zh: { summary: '研究摘要', canAnswer: '能回答', cannotAnswer: '不能回答', canAnswerText: '指定时点的历史价格、波动与回撤，已披露财务期的变化，以及新闻的纳入、剔除和来源。', cannotAnswerText: '买入、卖出、持有、目标价、仓位和执行决策；未知输入和报告缺口不会被推断。', context: '用户研究上下文', provided: '已提供', unknownField: '未知', targetHolding: '本标的持仓', issuerLimit: '发行人上限评估', quantity: '数量', weight: '权重', issuerCap: '上限', headroom: '剩余额度', exceeded: '是否超限', scenario: '用户情景输入', facts: '事实', calculations: '可复算计算', computed: '已计算', unknown: '未知', periods: '财务期', sessions: '历史样本', news: '纳入新闻', basis: '结论依据', factSummary: '事实摘要', risks: '风险', gaps: '缺口', formula: '公式', inputs: '输入', value: '值', state: '状态', evidence: '证据 ID', reason: '原因', financial: '财务期对比', period: '期间', filed: '披露日', revenue: '营收', gross: '毛利', operating: '营业利润', net: '净利润', assets: '总资产', liabilities: '总负债', history: '历史统计与样本', range: '范围', minimum: '最低会话数', observations: '样本明细', newsReview: '新闻纳入与剔除', inputCount: '输入', included: '纳入', excluded: '低相关剔除', duplicates: '去重', sourceTier: '来源等级', type: '类型', theme: '主题', rule: '相关性规则', source: '核对来源', disclaimer: '仅证据包 · 未调用 LLM · 非 10-Agent · 非投资建议', raw: '查看关键字段原文' },
  en: { summary: 'Research summary', canAnswer: 'Can answer', cannotAnswer: 'Cannot answer', canAnswerText: 'Dated historical price, volatility and drawdown; changes across filed financial periods; and how news was included, excluded, and sourced.', cannotAnswerText: 'Buy, sell, hold, target-price, sizing, or execution decisions. Unknown inputs and disclosed gaps are never inferred.', context: 'User research context', provided: 'Provided', unknownField: 'Unknown', targetHolding: 'Holding in this security', issuerLimit: 'Issuer limit assessment', quantity: 'Quantity', weight: 'Weight', issuerCap: 'Cap', headroom: 'Headroom', exceeded: 'Limit exceeded', scenario: 'User scenario inputs', facts: 'Facts', calculations: 'Reproducible calculations', computed: 'Computed', unknown: 'Unknown', periods: 'Financial periods', sessions: 'Historical samples', news: 'Included news', basis: 'Conclusion basis', factSummary: 'Fact summary', risks: 'Risks', gaps: 'Gaps', formula: 'Formula', inputs: 'Inputs', value: 'Value', state: 'Status', evidence: 'Evidence IDs', reason: 'Reason', financial: 'Financial period comparison', period: 'Period', filed: 'Filed', revenue: 'Revenue', gross: 'Gross profit', operating: 'Operating income', net: 'Net income', assets: 'Total assets', liabilities: 'Total liabilities', history: 'Historical statistics and samples', range: 'Range', minimum: 'Minimum sessions', observations: 'Observation details', newsReview: 'News inclusion and exclusions', inputCount: 'Input', included: 'Included', excluded: 'Low relevance excluded', duplicates: 'Duplicates removed', sourceTier: 'Source tier', type: 'Type', theme: 'Theme', rule: 'Relevance rule', source: 'Verify source', disclaimer: 'Evidence package only · no LLM called · not a 10-Agent run · not investment advice', raw: 'View original key fields' },
}

const OriginalFields: React.FC<{ dossier: EvidenceOnlyDossier; language: Language }> = ({ dossier, language }) => {
  if (language !== 'zh') return null
  const values = [dossier.label, dossier.disclaimer, JSON.stringify(dossier.research_context || null), JSON.stringify(dossier.context_assessment || null), ...(dossier.conclusion_basis || []), ...(dossier.risks || []), ...(dossier.gaps || []), ...(dossier.calculations || []).flatMap((item) => [item.name, item.status, item.reason || ''])].filter(Boolean)
  return <details className="border-y border-line py-3 text-xs text-muted"><summary className="cursor-pointer font-semibold text-accent">{copy.zh.raw}</summary><ul className="mt-2 list-disc space-y-1 break-words pl-5 font-mono">{values.map((value, index) => <li key={`${value}-${index}`}>{value}</li>)}</ul></details>
}

const hasText = (value?: string) => Boolean(value?.trim())
const finite = (value?: number) => typeof value === 'number' && Number.isFinite(value)

export const EvidenceOnlyDossierView: React.FC<{ dossier: EvidenceOnlyDossier; decision: ResearchDecision | string; language: Language; ticker?: string }> = ({ dossier, decision, language, ticker }) => {
  const c = copy[language]
  const computed = dossier.calculations?.filter((item) => item.status === 'computed').length || 0
  const unknown = (dossier.calculations?.length || 0) - computed
  const context = dossier.research_context
  const assessment = dossier.context_assessment
  const targetSymbol = (assessment?.holding?.symbol || assessment?.symbol || ticker || '').trim().toUpperCase()
  const targetHolding = context?.holdings?.find((holding) => holding.symbol?.trim().toUpperCase() === targetSymbol)
  const quantity = assessment?.holding?.quantity ?? assessment?.holding_quantity ?? targetHolding?.quantity
  const weightPct = assessment?.holding?.weight_pct ?? assessment?.holding_weight_pct ?? targetHolding?.weight_pct
  const issuerCapPct = assessment?.issuer_cap?.cap_pct ?? assessment?.issuer_cap_pct ?? context?.issuer_cap_pct
  const headroomPct = assessment?.issuer_cap?.headroom_pct ?? assessment?.issuer_headroom_pct ?? assessment?.headroom_pct ?? (finite(weightPct) && finite(issuerCapPct) ? issuerCapPct! - weightPct! : undefined)
  const withinCap = assessment?.issuer_cap?.within_cap
  const limitExceeded = withinCap == null ? assessment?.issuer_limit_exceeded ?? assessment?.over_limit ?? (finite(headroomPct) ? headroomPct! < 0 : undefined) : !withinCap
  const assessedInput = (field: string, fallback?: string) => {
    const input = assessment?.inputs?.[field]
    return { provided: input ? input.status === 'provided' : hasText(fallback), value: input?.value || fallback || input?.reason || c.unknownField }
  }
  const contextFields: Array<[string, boolean, React.ReactNode]> = [
    ['mandate', assessedInput('mandate', context?.mandate).provided, assessedInput('mandate', context?.mandate).value],
    ['holdings', assessedInput('holdings', context?.holdings?.length ? `${context.holdings.length}` : '').provided, assessedInput('holdings', context?.holdings?.length ? `${context.holdings.length} ${language === 'zh' ? '条持仓' : 'holdings'}` : '').value],
    ['liquidity', assessedInput('liquidity', context?.liquidity).provided, assessedInput('liquidity', context?.liquidity).value],
    ['tax', assessedInput('tax', context?.tax).provided, assessedInput('tax', context?.tax).value],
    ['risk_budget', assessedInput('risk_budget', context?.risk_budget).provided, assessedInput('risk_budget', context?.risk_budget).value],
  ]
  return <div className="space-y-4 text-sm text-muted">
    <section className="border border-sky-500/35 bg-sky-500/10 p-4 text-sky-100"><div className="font-bold">{c.disclaimer}</div><p className="mt-1 text-xs font-semibold">{localizeDynamic(dossier.label, language)}</p><p className="mt-1 text-xs leading-relaxed">{localizeDynamic(dossier.disclaimer, language)}</p><p className="mt-2 font-semibold">{formatEvidenceDecision(dossier.conclusion || decision, language)}</p></section>
    <section className="grid gap-3 border-y border-line py-3 sm:grid-cols-2" aria-label={language === 'zh' ? '研究能力边界' : 'Research capability boundary'}><div><h2 className="text-sm font-semibold text-emerald-200">{c.canAnswer}</h2><p className="mt-1 text-xs leading-relaxed">{c.canAnswerText}</p></div><div><h2 className="text-sm font-semibold text-amber-200">{c.cannotAnswer}</h2><p className="mt-1 text-xs leading-relaxed">{c.cannotAnswerText}</p></div></section>
    <section className="border border-line p-4" aria-label={c.context}>
      <h2 className="text-base text-white">{c.context}</h2>
      <p className="mt-1 text-xs text-faint">{language === 'zh' ? '以下内容来自用户输入；未提供字段保持未知，不由研究包推断。' : 'These values are user supplied. Missing fields remain unknown and are not inferred by the dossier.'}</p>
      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        {contextFields.map(([label, provided, value]) => <div key={label} className="border-t border-line pt-2"><div className="flex items-center justify-between gap-2"><code className="text-xs text-slate-300">{label}</code><span className={provided ? 'text-xs font-semibold text-emerald-300' : 'text-xs font-semibold text-amber-200'}>{provided ? c.provided : c.unknownField}</span></div><div className="mt-1 break-words text-xs">{value}</div></div>)}
      </div>
      <div className="mt-4 grid gap-4 border-t border-line pt-3 sm:grid-cols-2">
        <div><h3 className="flex items-center justify-between gap-2 font-semibold text-white"><span>{c.targetHolding} · {targetSymbol || c.unknownField}</span><span className={(assessment?.holding?.status === 'provided' || (!assessment?.holding && targetHolding)) ? 'text-xs text-emerald-300' : 'text-xs text-amber-200'}>{(assessment?.holding?.status === 'provided' || (!assessment?.holding && targetHolding)) ? c.provided : c.unknownField}</span></h3><div className="mt-2 grid grid-cols-2 gap-2 text-xs"><span>{c.quantity}<strong className="mt-1 block text-white">{finite(quantity) ? number(quantity) : c.unknownField}</strong></span><span>{c.weight}<strong className="mt-1 block text-white">{finite(weightPct) ? `${number(weightPct)}%` : c.unknownField}</strong></span></div>{assessment?.holding?.reason && <p className="mt-2 text-xs text-faint">{localizeDynamic(assessment.holding.reason, language)}</p>}</div>
        <div><h3 className="flex items-center justify-between gap-2 font-semibold text-white"><span>{c.issuerLimit}</span><span className={(assessment?.issuer_cap?.status === 'provided' || (!assessment?.issuer_cap && finite(headroomPct))) ? 'text-xs text-emerald-300' : 'text-xs text-amber-200'}>{(assessment?.issuer_cap?.status === 'provided' || (!assessment?.issuer_cap && finite(headroomPct))) ? c.provided : c.unknownField}</span></h3><div className="mt-2 grid grid-cols-3 gap-2 text-xs"><span>{c.issuerCap}<strong className="mt-1 block text-white">{finite(issuerCapPct) ? `${number(issuerCapPct)}%` : c.unknownField}</strong></span><span>{c.headroom}<strong className="mt-1 block text-white">{finite(headroomPct) ? `${number(headroomPct)}%` : c.unknownField}</strong></span><span>{c.exceeded}<strong className={`mt-1 block ${limitExceeded === true ? 'text-rose-300' : limitExceeded === false ? 'text-emerald-300' : 'text-amber-200'}`}>{limitExceeded === true ? (language === 'zh' ? '是' : 'Yes') : limitExceeded === false ? (language === 'zh' ? '否' : 'No') : c.unknownField}</strong></span></div>{(assessment?.issuer_cap?.reason || assessment?.reason) && <p className="mt-2 text-xs text-faint">{localizeDynamic(assessment?.issuer_cap?.reason || assessment?.reason || '', language)}</p>}</div>
      </div>
      <div className="mt-4 border-t border-line pt-3"><h3 className="flex items-center justify-between gap-2 font-semibold text-white"><span>{c.scenario}</span><span className={(assessment?.scenario?.status === 'provided' || (!assessment?.scenario && (finite(context?.scenario?.revenue_growth_pct) || finite(context?.scenario?.ps_multiple)))) ? 'text-xs text-emerald-300' : 'text-xs text-amber-200'}>{(assessment?.scenario?.status === 'provided' || (!assessment?.scenario && (finite(context?.scenario?.revenue_growth_pct) || finite(context?.scenario?.ps_multiple)))) ? c.provided : c.unknownField}</span></h3><div className="mt-2 grid grid-cols-2 gap-2 text-xs sm:grid-cols-4"><span>revenue_growth_pct<strong className="mt-1 block text-white">{finite(assessment?.scenario?.revenue_growth_pct ?? context?.scenario?.revenue_growth_pct) ? `${number(assessment?.scenario?.revenue_growth_pct ?? context?.scenario?.revenue_growth_pct)}%` : c.unknownField}</strong></span><span>ps_multiple<strong className="mt-1 block text-white">{finite(assessment?.scenario?.ps_multiple ?? context?.scenario?.ps_multiple) ? number(assessment?.scenario?.ps_multiple ?? context?.scenario?.ps_multiple) : c.unknownField}</strong></span><span>{language === 'zh' ? '隐含价格（敏感性）' : 'Implied price (sensitivity)'}<strong className="mt-1 block text-white">{finite(assessment?.scenario?.implied_price) ? number(assessment?.scenario?.implied_price) : c.unknownField}</strong></span><span>{language === 'zh' ? '相对当前价格变化' : 'Change from current price'}<strong className="mt-1 block text-white">{finite(assessment?.scenario?.change_pct) ? `${number(assessment?.scenario?.change_pct)}%` : c.unknownField}</strong></span></div>{assessment?.scenario?.disclosure && <p className="mt-2 text-xs text-faint">{localizeDynamic(assessment.scenario.disclosure, language)}</p>}{assessment?.scenario?.reason && <p className="mt-1 text-xs text-amber-200">{localizeDynamic(assessment.scenario.reason, language)}</p>}</div>
      {assessment?.limitations?.length ? <div className="mt-4 border-t border-line pt-3"><h3 className="font-semibold text-amber-200">{language === 'zh' ? '上下文限制' : 'Context limitations'}</h3><ul className="mt-1 list-disc space-y-1 pl-5 text-xs">{assessment.limitations.map((item) => <li key={item}>{localizeDynamic(item, language)}</li>)}</ul></div> : null}
    </section>
    <section className="border border-line p-4"><h2 className="text-base text-white">{c.summary}</h2><div className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-5">{[[c.facts, dossier.facts?.length || 0], [c.calculations, `${computed} ${c.computed} / ${unknown} ${c.unknown}`], [c.periods, dossier.financial_periods?.length || 0], [c.sessions, dossier.historical?.sample_count || 0], [c.news, dossier.news_review?.included_count || 0]].map(([label, value]) => <div key={String(label)}><div className="text-xs text-faint">{label}</div><div className="mt-1 font-bold text-white">{value}</div></div>)}</div><h3 className="mt-4 font-semibold text-white">{c.basis}</h3><ul className="mt-1 list-disc space-y-1 pl-5">{(dossier.conclusion_basis || []).map((item) => <li key={item}>{localizeDynamic(item, language)}</li>)}</ul></section>
    <section className="border border-line p-4"><h3 className="font-semibold text-white">{c.factSummary}</h3>{(dossier.facts || []).map((fact) => <div key={`${fact.category}-${fact.data_time}`} className="mt-2 border-t border-line pt-2"><div className="font-semibold text-slate-200">{localizeDynamic(fact.category, language)} · {fact.provider}</div><p className="mt-1">{localizeDynamic(fact.summary, language)}</p><div className="mt-1 text-xs text-faint">{fact.data_time || 'unknown'} · {c.evidence}: {fact.evidence_ids?.join(', ') || 'missing'}</div>{fact.source_url && <a className="text-accent" href={fact.source_url} target="_blank" rel="noreferrer">{c.source}</a>}</div>)}<div className="mt-4 grid gap-4 sm:grid-cols-2"><div><h3 className="font-semibold text-amber-200">{c.risks}</h3><ul className="list-disc pl-5">{(dossier.risks || []).map((item) => <li key={item}>{localizeDynamic(item, language)}</li>)}</ul></div><div><h3 className="font-semibold text-rose-200">{c.gaps}</h3><ul className="list-disc pl-5">{(dossier.gaps || []).map((item) => <li key={item}>{localizeDynamic(item, language)}</li>)}</ul></div></div></section>
    <details className="border border-line p-4"><summary className="cursor-pointer font-semibold text-white">{c.calculations} ({dossier.calculations?.length || 0})</summary><div className="mt-3 space-y-3">{(dossier.calculations || []).map((item) => <div key={item.name} className="border-t border-line pt-3"><div className="flex flex-wrap justify-between gap-2"><strong className="text-white">{localizeDynamic(item.name, language)}</strong><span>{c.state}: {localizeDynamic(item.status, language)}</span></div><div className="mt-1 break-words">{c.formula}: <code>{item.formula}</code></div><div>{c.inputs}: {Object.entries(item.inputs || {}).map(([key, value]) => `${key}=${number(value)}`).join(', ') || '—'}</div><div>{c.value}: <strong className="text-white">{number(item.value)} {item.unit}</strong></div>{item.reason && <div>{c.reason}: {localizeDynamic(item.reason, language)}</div>}<div className="break-words text-xs text-faint">{c.evidence}: {item.evidence_ids?.join(', ') || '—'}</div></div>)}</div></details>
    <details className="border border-line p-4"><summary className="cursor-pointer font-semibold text-white">{c.financial} ({dossier.financial_periods?.length || 0})</summary><div className="mt-3 overflow-x-auto"><table className="min-w-[900px] w-full text-left text-xs"><thead><tr>{[c.period,c.filed,c.revenue,c.gross,c.operating,c.net,c.assets,c.liabilities].map((heading) => <th key={heading} className="border-b border-line p-2">{heading}</th>)}</tr></thead><tbody>{(dossier.financial_periods || []).map((period) => <tr key={`${period.period_end}-${period.accession}`}><td className="p-2">{period.fiscal_period}<br/>{period.period_end}</td><td className="p-2">{period.filing_date}</td>{[period.total_revenue,period.gross_profit,period.operating_income,period.net_income,period.total_assets,period.total_liabilities].map((value, index) => <td key={index} className="p-2 font-mono">{number(value)}</td>)}</tr>)}</tbody></table></div></details>
    <details className="border border-line p-4"><summary className="cursor-pointer font-semibold text-white">{c.history}: {dossier.historical?.sample_count || 0}</summary>{dossier.historical && <><div className="mt-3 text-xs">{c.range}: {dossier.historical.start_date} - {dossier.historical.end_date} · {c.minimum}: {dossier.historical.minimum_sessions} · {c.state}: {localizeDynamic(dossier.historical.status, language)}</div><details className="mt-3"><summary className="cursor-pointer text-accent">{c.observations}</summary><div className="max-h-80 overflow-auto"><table className="mt-2 min-w-[620px] w-full text-xs"><tbody>{(dossier.historical.observations || []).map((observation) => <tr key={observation.date} className="border-t border-line"><td className="p-1">{observation.date}</td><td>O {number(observation.open)}</td><td>H {number(observation.high)}</td><td>L {number(observation.low)}</td><td>C {number(observation.close)}</td><td>V {number(observation.volume)}</td></tr>)}</tbody></table></div></details></>}</details>
    <details className="border border-line p-4"><summary className="cursor-pointer font-semibold text-white">{c.newsReview}</summary>{dossier.news_review && <><div className="mt-3 flex flex-wrap gap-3 text-xs"><span>{c.inputCount}: {dossier.news_review.input_count}</span><span>{c.included}: {dossier.news_review.included_count}</span><span>{c.excluded}: {dossier.news_review.excluded_low_relevance}</span><span>{c.duplicates}: {dossier.news_review.duplicates_removed}</span></div><ul className="mt-2 list-disc space-y-1 pl-5 text-xs">{(dossier.news_review.rules || []).map((rule) => <li key={rule}>{localizeDynamic(rule, language)}</li>)}</ul><div className="mt-2 space-y-2">{(dossier.news_review.items || []).map((item, index) => <div key={`${item.url}-${index}`} className="border-t border-line pt-2"><a href={item.url} target="_blank" rel="noreferrer" className="font-semibold text-accent">{item.title}</a><div className="text-xs">{item.published_at} · {item.source} · {c.sourceTier}: {localizeDynamic(item.source_tier, language)} · {c.type}: {localizeDynamic(item.content_type, language)} · {c.theme}: {localizeDynamic(item.theme, language)} · {c.rule}: {localizeDynamic(item.relevance_rule, language)}</div></div>)}</div></>}</details>
    <OriginalFields dossier={dossier} language={language} />
  </div>
}
