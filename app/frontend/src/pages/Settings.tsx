import React, { useEffect, useState } from 'react'
import { useAnalysisStore } from '../stores/analysisStore'
import { KeyRound, Save, RefreshCw, Trash2 } from 'lucide-react'
import { useI18n } from '../i18n'
import { apiUrl, authorizedFetch, clearAdminToken, getAdminToken, setAdminToken } from '../utils/api'
import { markAdminAuthorized, useAdminSession } from '../hooks/useAdminSession'

const providerDefaults: Record<string, { deep: string; quick: string; backendUrl: string }> = {
  dual: { deep: 'gemini-3.1-flash-lite', quick: 'deepseek-chat', backendUrl: 'https://api.deepseek.com/v1' },
  google: { deep: 'gemini-3.5-flash', quick: 'gemini-3.1-flash-lite', backendUrl: '' },
  deepseek: { deep: 'deepseek-chat', quick: 'deepseek-chat', backendUrl: 'https://api.deepseek.com/v1' },
  openai: { deep: 'gpt-4o', quick: 'gpt-4o-mini', backendUrl: 'https://api.openai.com/v1' },
  openai_compatible: { deep: '', quick: '', backendUrl: '' },
}

const isProviderCompatibleModel = (provider: string, model: string) => {
  const value = model.trim().toLowerCase()
	if (!value) return false
  if (provider === 'dual') return value.startsWith('deepseek-') || value.startsWith('gemini-')
  if (provider === 'google') return value.startsWith('gemini-')
  if (provider === 'deepseek') return value.startsWith('deepseek-')
  if (provider === 'openai') return value.startsWith('gpt-') || value.startsWith('o')
  return true
}

interface SystemEndpointStatus {
  domain: string
  endpoint: string
  source: string
  dataTime?: string
  stale: boolean
  staleReason?: string
  refreshable: boolean
  status: string
}

interface SystemDataFileStatus {
  id: string
  path?: string
  exists: boolean
  modifiedAt?: string
  generatedAt?: string
  count?: number
  error?: string
}

interface SystemStatus {
  status: string
  checkedAt: string
  runtime: {
    llmProvider: string
    deepThinkLLM?: string
    quickThinkLLM?: string
	alpacaConfigured?: boolean
	usQuoteFeed?: string
	cnQuoteFeed?: string
  }
  endpoints?: SystemEndpointStatus[]
  dataFiles?: SystemDataFileStatus[]
}

interface ReadinessProfileCheck {
  id: string
  status: string
  detail?: string
  remedy?: string
}

interface ReadinessProfile {
  profile: 'historical-research' | 'paper-engine' | 'live-trading-data'
  label: string
  status: string
  ready: boolean
  httpStatus: number
  checkedAt: string
  scope: string[]
  checks: ReadinessProfileCheck[]
  remedies?: string[]
  disclaimer: string
}

const readinessProfileNames: ReadinessProfile['profile'][] = [
  'historical-research',
  'paper-engine',
  'live-trading-data',
]

const formatStatusTime = (value?: string) => {
  if (!value) return '—'
  const normalized = value.startsWith('ts:') ? Number(value.slice(3)) : NaN
  const date = Number.isFinite(normalized) ? new Date(normalized) : new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

const readinessZh: Record<string, string> = {
  'Historical research': '历史时点研究',
  'Paper engine': 'Paper 模拟引擎',
  'Live quote transport': '实时行情链路',
  'Core live quotes': '核心个股实时行情',
  'PIT quote': '时点行情',
  'PIT primary price from dated Nasdaq historical close': '来自带日期 Nasdaq 历史收盘价的时点主研究价格',
  'dated Nasdaq OHLCV history': '带日期的 Nasdaq OHLCV 历史',
  'independent dated OHLCV history': '带日期的独立 OHLCV 历史',
  'filing-bound fundamentals': '绑定披露日的基本面',
  'reviewed ticker news': '已复核的标的新闻',
  'at least one complete evidence-only dossier': '至少一份完整仅证据研究包',
  'SQLite Paper ledger': 'SQLite Paper 账本',
  'OMS schema and CAS persistence': 'OMS 结构与 CAS 持久化',
  'linked audit chain': '关联审计链',
  'local OCO scheduler': '本地 OCO 调度器',
  'versioned risk policy': '带版本的风险策略',
  'provider-timed US market overview': '带供应商时间的美股行情概览',
  'provider-timed direct AAPL/NVDA quotes': '带供应商时间的 AAPL/NVDA 直连报价',
  'provider-timed US stock list': '带供应商时间的美股列表',
  'session-aware CN market quotes': '识别交易时段的 A 股行情',
  'This profile only proves an auditable historical-research path. It does not imply live market readiness or provide a trade signal.': '此门禁仅证明历史研究路径可审计，不代表实时行情就绪，也不提供交易信号。',
  'Paper-engine readiness is simulation-only. It does not imply live trading data readiness, a broker connection, or a real order/fill.': 'Paper 引擎仅用于模拟，不代表实时交易数据就绪、已连接券商或产生真实订单和成交。',
  'This profile covers quote transport only. Macro, reports, ETF/QDII, and disclosure freshness remain visible in system health and are not real-time quote feeds.': '此门禁只覆盖行情链路。宏观、盘报、ETF/QDII 与披露新鲜度仍在系统健康中如实展示，但不冒充实时行情。',
  'Core quote readiness is independent from market-wide coverage. US overview/list and CN session coverage remain visible as warnings.': '核心个股报价门禁与全市场覆盖分开验收；美股概览/列表及 A 股交易时段覆盖仍作为警告如实展示。',
  'Create an authenticated evidence-only dossier with a provider-timed quote at or before the requested trade date.': '使用经认证的入口创建仅证据研究包，并保留请求交易日当日或之前的供应商时间行情。',
  'Capture at least 21 valid dated OHLCV sessions for the dossier trade date.': '针对研究交易日捕获至少 21 个有效、带日期的 OHLCV 会话。',
  'Capture at least two financial periods with filing date, accession, and source URL available by the trade date.': '捕获至少两个财务期，且披露日、accession 和来源 URL 均在交易日前可用。',
  'Run the deterministic ticker-news review and retain its captured source evidence.': '执行确定性标的新闻复核，并保留已捕获的来源证据。',
  'Create and retain at least one publication-gated evidence-only dossier.': '创建并保留至少一份通过发布门禁的仅证据研究包。',
}

const localizeReadinessText = (value: string, language: 'zh' | 'en') => {
  if (language !== 'zh') return value
  if (readinessZh[value]) return readinessZh[value]
  const refresh = value.match(/^Refresh (.+) from its authoritative provider and retain a source data time before relying on it\.$/)
  if (refresh) return `从权威提供方刷新 ${refresh[1]}，并保留源数据时间后再使用。`
  const restore = value.match(/^Restore a current authoritative source for (.+); a readable snapshot alone is not live readiness\.$/)
  if (restore) return `为 ${restore[1]} 恢复当前权威来源；仅快照可读不代表实时数据就绪。`
  return value
}

const localizeReadinessStatus = (value: string, language: 'zh' | 'en') => language === 'zh'
  ? ({ ok: '就绪', degraded: '降级', warning: '警告', fail: '失败', error: '错误', unknown: '未知' }[value] || value)
  : value

const ReadinessRaw: React.FC<{ values: string[]; language: 'zh' | 'en' }> = ({ values, language }) => language === 'zh' && values.some((value) => localizeReadinessText(value, language) !== value) ? (
  <details className="mt-1 text-[11px] text-muted"><summary className="cursor-pointer">展开原始值</summary><div className="mt-1 break-words font-mono">{values.join(' · ')}</div></details>
) : null

const ReadinessProfileRow: React.FC<{ profile: ReadinessProfile; language: 'zh' | 'en'; experimental?: boolean }> = ({ profile, language, experimental = false }) => {
  const scope = profile.scope.map((value) => localizeReadinessText(value, language))
  const remedies = (profile.remedies || []).map((value) => localizeReadinessText(value, language))
  return <details className={`min-w-0 border-y py-3 ${experimental ? 'border-amber-500/25' : 'border-line'}`}>
    <summary className="cursor-pointer list-none">
      <div className="flex min-w-0 items-start justify-between gap-3">
        <span className="min-w-0"><strong className="block text-sm text-ink">{localizeReadinessText(profile.label, language)}</strong><code className="block break-all text-[10px] text-muted">{profile.profile}</code></span>
        <span className={`shrink-0 text-xs font-semibold ${profile.ready ? 'text-emerald-300' : 'text-amber-300'}`}>{localizeReadinessStatus(profile.status, language)} · HTTP {profile.httpStatus}</span>
      </div>
    </summary>
    <div className="mt-3 space-y-3 text-xs leading-relaxed text-muted">
      <p>{localizeReadinessText(profile.disclaimer, language)}</p>
      <div><strong className="text-ink">独立范围</strong><ul className="mt-1 list-disc space-y-1 pl-5">{scope.map((value) => <li key={value}>{value}</li>)}</ul></div>
      <div><strong className="text-ink">修复项</strong>{remedies.length ? <ul className="mt-1 list-disc space-y-1 pl-5 text-amber-200">{remedies.map((value) => <li key={value}>{value}</li>)}</ul> : <p className="mt-1">无</p>}</div>
      <ReadinessRaw values={[profile.disclaimer, ...profile.scope, ...(profile.remedies || [])]} language={language} />
    </div>
  </details>
}

export const Settings: React.FC = () => {
  const { t, language } = useI18n()
  const localAdmin = useAdminSession()
  const { config, fetchConfig, updateConfig } = useAnalysisStore()

  const [provider, setProvider] = useState('google')
  const [deepModel, setDeepModel] = useState('gemini-3.5-flash')
  const [quickModel, setQuickModel] = useState('gemini-3.1-flash-lite')
  const [lang, setLang] = useState('Bilingual')
  const [debateRounds, setDebateRounds] = useState(1)
  const [riskRounds, setRiskRounds] = useState(1)
  const [backendUrl, setBackendUrl] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<{ ok: boolean; message: string } | null>(null)
  const [systemStatus, setSystemStatus] = useState<SystemStatus | null>(null)
  const [readinessProfiles, setReadinessProfiles] = useState<ReadinessProfile[]>([])
  const [statusLoading, setStatusLoading] = useState(false)
  const [statusLoadError, setStatusLoadError] = useState('')
  const [adminToken, setAdminTokenValue] = useState(() => getAdminToken())
  const [adminStatus, setAdminStatus] = useState<'unknown' | 'checking' | 'valid' | 'invalid'>(
    () => getAdminToken() ? 'unknown' : 'invalid'
  )
  const [adminMessage, setAdminMessage] = useState('')
  const [marketRefreshing, setMarketRefreshing] = useState(false)
  const [marketRefreshMessage, setMarketRefreshMessage] = useState('')

  const verifyAdminToken = async () => {
    setAdminToken(adminToken)
    if (!adminToken.trim()) {
      setAdminStatus('invalid')
      setAdminMessage('未设置令牌，所有管理写操作保持关闭')
      return false
    }
    setAdminStatus('checking')
    try {
      const resp = await authorizedFetch(apiUrl('/api/admin/audit/verify'), { cache: 'no-store' })
      const data = await resp.json().catch(() => ({}))
      if (!resp.ok) {
        setAdminStatus('invalid')
        setAdminMessage(data.error || `验证失败（HTTP ${resp.status}）`)
        return false
      }
      setAdminStatus('valid')
      markAdminAuthorized()
      setAdminMessage(`管理操作已解锁；审计链 ${data.valid ? '有效' : '异常'}，共 ${data.count ?? 0} 条记录`)
      return true
    } catch (err) {
      setAdminStatus('invalid')
      setAdminMessage(err instanceof Error ? err.message : '无法连接后端')
      return false
    }
  }

  const enableLocalSession = async () => {
    setAdminStatus('checking')
    const enabled = await localAdmin.retry()
    setAdminStatus(enabled ? 'valid' : 'invalid')
    setAdminMessage(enabled ? '本机管理会话已启用；长期令牌未发送到前端' : '本机管理会话不可用，可展开高级恢复并验证令牌')
    return enabled
  }

  const removeAdminToken = () => {
    clearAdminToken()
    setAdminTokenValue('')
    setAdminStatus('invalid')
    setAdminMessage('令牌已从当前浏览器会话移除，管理写操作已关闭')
  }

  const fetchSystemStatus = async () => {
    setStatusLoading(true)
    setStatusLoadError('')
    try {
      const [statusResult, ...profileResults] = await Promise.allSettled([
        authorizedFetch(apiUrl('/api/system/status'), { cache: 'no-store' }).then(async (response) => ({
          body: await response.json() as SystemStatus,
          httpStatus: response.status,
        })),
        ...readinessProfileNames.map((profile) => fetch(apiUrl(`/api/readiness?profile=${profile}`), { cache: 'no-store' }).then(async (response) => ({
          body: { ...await response.json(), httpStatus: response.status } as ReadinessProfile,
          profile,
        }))),
      ])

      const failures: string[] = []
      if (statusResult.status === 'fulfilled') {
        setSystemStatus(statusResult.value.body)
      } else {
        failures.push(`系统状态：${statusResult.reason instanceof Error ? statusResult.reason.message : '请求失败'}`)
      }

      const successfulProfiles: ReadinessProfile[] = []
      profileResults.forEach((result, index) => {
        if (result.status === 'fulfilled') {
          successfulProfiles.push(result.value.body)
        } else {
          failures.push(`${readinessProfileNames[index]}：${result.reason instanceof Error ? result.reason.message : '请求失败'}`)
        }
      })
      setReadinessProfiles((current) => {
        const merged = new Map(current.map((profile) => [profile.profile, profile]))
        successfulProfiles.forEach((profile) => merged.set(profile.profile, profile))
        return readinessProfileNames.map((profile) => merged.get(profile)).filter((profile): profile is ReadinessProfile => Boolean(profile))
      })
      if (failures.length > 0) {
        setStatusLoadError(`部分状态请求失败，失败项保留上次成功结果：${failures.join('；')}`)
      }
    } finally {
      setStatusLoading(false)
    }
  }

  const refreshMarketData = async () => {
    if (!(adminStatus === 'valid' || await enableLocalSession())) return
    setMarketRefreshing(true)
    setMarketRefreshMessage('')
    try {
      const resp = await authorizedFetch(apiUrl('/api/market/refresh?top=500'), { method: 'POST' })
      const data = await resp.json().catch(() => ({}))
      if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`)
	  const counts = `美股 ${Number(data.updated) || 0} · A 股 ${Number(data.updatedCN) || 0}`
	  const errors = [data.usError, data.cnError].filter(Boolean).join(' · ')
	  if (data.status === 'partial' || errors) {
		setMarketRefreshMessage(`行情仅部分刷新：${counts}${errors ? ` · ${errors}` : ''}`)
	  } else {
		setMarketRefreshMessage(`行情刷新完成：${counts} · 审计 ${resp.headers.get('X-Audit-Outcome') || 'unknown'}`)
	  }
      await fetchSystemStatus()
    } catch (err) {
      setMarketRefreshMessage(`行情刷新失败: ${err instanceof Error ? err.message : String(err)}`)
    } finally {
      setMarketRefreshing(false)
    }
  }

  const buildConfigUpdates = () => {
    const updates: Record<string, any> = {
      llm_provider: provider,
      deep_think_llm: deepModel,
      quick_think_llm: quickModel,
      output_language: lang,
      max_debate_rounds: debateRounds,
      max_risk_rounds: riskRounds,
      llm_backend_url: backendUrl,
    }
    if (apiKey) {
      if (provider === 'deepseek') {
        updates.deepseek_api_key = apiKey
      } else if (provider === 'google') {
        updates.google_api_key = apiKey
      } else if (provider === 'openai' || provider === 'openai_compatible') {
        updates.openai_api_key = apiKey
      }
    }
    return updates
  }

  const handleTestConnection = async () => {
    setTesting(true)
    setTestResult(null)
    const startTime = Date.now()
    try {
      const resp = await authorizedFetch(apiUrl('/api/config/test'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(buildConfigUpdates()),
      })
      const latency = Date.now() - startTime
      const data = await resp.json().catch(() => ({}))
      if (resp.ok) {
        setTestResult({ ok: true, message: `连接成功 · ${data.llm_provider || provider} / ${data.quick_think_llm || quickModel} · 延迟 ${data.latency_ms ?? latency}ms` })
      } else {
        setTestResult({ ok: false, message: `连接异常 · HTTP ${resp.status} · ${data.error || '请检查 Key 和模型名'}` })
      }
    } catch (err: any) {
      setTestResult({ ok: false, message: `网络请求失败: ${err.message || err}` })
    } finally {
      setTesting(false)
    }
  }

  // Provider-specific hints
  const providerKeyHint = (p: string) => {
    switch(p) {
      case 'google': return t('set.keyGoogle')
      case 'dual': return 'DeepSeek 负责多角色研究，Gemini 负责最终综合；两组 Key 均须在后端已配置'
      case 'deepseek': return t('set.keyDeepseek')
      case 'openai': return t('set.keyOpenai')
      case 'openai_compatible': return t('set.keyCompat')
      default: return t('set.keyGeneric')
    }
  }

  const providerModelHint = (p: string) => {
    switch(p) {
      case 'google': return t('set.hintGoogle')
      case 'dual': return '深度模型填 Gemini，快速模型填 DeepSeek'
      case 'deepseek': return t('set.hintDeepseek')
      case 'openai': return t('set.hintOpenai')
      case 'openai_compatible': return t('set.hintCompat')
      default: return ''
    }
  }

  useEffect(() => {
    fetchConfig()
    fetchSystemStatus()
  }, [])

  useEffect(() => {
    if (localAdmin.state === 'authorized') {
      setAdminStatus('valid')
      setAdminMessage('本机管理会话已启用；长期令牌未发送到前端')
    } else if (localAdmin.state === 'unavailable' && !adminToken) {
      setAdminStatus('invalid')
    }
  }, [localAdmin.state, adminToken])

  useEffect(() => {
    if (config) {
      const nextProvider = config.llm_provider === 'gemini' ? 'google' : config.llm_provider
      const defaults = providerDefaults[nextProvider] || providerDefaults.google
      setProvider(nextProvider)
      setDeepModel(isProviderCompatibleModel(nextProvider, config.deep_think_llm) ? config.deep_think_llm : defaults.deep)
      setQuickModel(isProviderCompatibleModel(nextProvider, config.quick_think_llm) ? config.quick_think_llm : defaults.quick)
      setLang(config.output_language || 'Bilingual')
      setDebateRounds(config.max_debate_rounds)
      setRiskRounds(config.max_risk_rounds)
      setBackendUrl(nextProvider === 'google' ? '' : ((config as any).llm_backend_url || defaults.backendUrl))
    }
  }, [config])

  const handleProviderChange = (nextProvider: string) => {
    const defaults = providerDefaults[nextProvider] || providerDefaults.google
    setProvider(nextProvider)
    setTestResult(null)
    if (!isProviderCompatibleModel(nextProvider, deepModel)) {
      setDeepModel(defaults.deep)
    }
    if (!isProviderCompatibleModel(nextProvider, quickModel)) {
      setQuickModel(defaults.quick)
    }
    setBackendUrl(defaults.backendUrl)
  }

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      setAdminToken(adminToken)
      const result = await updateConfig(buildConfigUpdates())
      if (result.status === 'updated_with_unavailable_llm' || result.runtime_warning) {
        setTestResult({ ok: false, message: `设置已保存，但 LLM 当前不可用：${result.runtime_warning || '请检查 Provider、模型与 API Key'}` })
      } else {
        setTestResult({ ok: true, message: t('set.saved') })
      }
    } catch (err: any) {
      setTestResult({ ok: false, message: `设置保存失败: ${err.message || err}` })
    }
  }

  const productProfiles = readinessProfiles.filter((profile) => profile.profile === 'historical-research' || profile.profile === 'paper-engine')
  const liveProfile = readinessProfiles.find((profile) => profile.profile === 'live-trading-data')

  return (
    <div style={{ maxWidth: '1080px', margin: '0 auto', display: 'flex', flexDirection: 'column', gap: '2rem' }}>
      <section id="product-readiness" className="border border-line bg-surface p-4 sm:p-6">
        <div className="flex flex-col gap-2 border-b border-line pb-3 sm:flex-row sm:items-end sm:justify-between">
          <div><p className="text-xs font-semibold text-accent">当前产品承诺</p><h1 className="mt-1 text-lg font-bold text-ink">研究与 Paper readiness</h1><p className="mt-1 text-xs leading-relaxed text-muted">历史研究与 Paper 模拟引擎分别验收；两者均不代表实时行情、券商连接或真实成交。</p></div>
          <button className="btn btn-secondary self-start" type="button" onClick={fetchSystemStatus} disabled={statusLoading}><RefreshCw size={15} className={statusLoading ? 'animate-spin' : ''} />刷新门禁</button>
        </div>
        {statusLoadError && <div role="alert" className="mt-3 border-l-2 border-rose-500 bg-rose-500/5 px-3 py-2 text-xs text-rose-200">{statusLoadError}</div>}
        <div className="mt-2 grid min-w-0 grid-cols-1 gap-x-5 sm:grid-cols-2">
          {productProfiles.map((profile) => <ReadinessProfileRow key={profile.profile} profile={profile} language={language} />)}
          {!statusLoading && productProfiles.length === 0 && <p className="py-4 text-xs text-muted">产品门禁状态不可用</p>}
        </div>
        {liveProfile && <div className="mt-4"><p className="text-xs font-semibold text-amber-200">实验状态 · 未纳入当前产品承诺</p><ReadinessProfileRow profile={liveProfile} language={language} experimental /></div>}
      </section>

      <div className="card" style={{ padding: '1.5rem' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.65rem', marginBottom: '1rem' }}>
          <KeyRound size={18} color="var(--accent)" />
          <div>
            <h3 style={{ margin: 0, fontSize: '1rem', color: 'var(--text-primary)' }}>管理操作授权</h3>
            <p style={{ margin: '0.25rem 0 0', fontSize: '0.78rem', color: 'var(--text-secondary)' }}>
              本机回环访问会建立 HttpOnly 短期会话，用于刷新数据、运行分析和提交 Paper 模拟订单；长期令牌不会发送到页面。
            </p>
          </div>
        </div>
        <div className="settings-auth-controls" style={{ gap: '0.75rem', alignItems: 'center' }}>
          <button type="button" className="btn btn-primary" onClick={() => void enableLocalSession()} disabled={adminStatus === 'checking'}>
            <KeyRound size={16} /> {adminStatus === 'checking' ? '启用中' : adminStatus === 'valid' ? '本机会话已启用' : '启用本机会话'}
          </button>
          <button type="button" className="btn btn-secondary" onClick={() => { void localAdmin.end(); removeAdminToken() }} title="结束当前管理会话">
            <Trash2 size={16} /> 结束会话
          </button>
        </div>
        <details className="mt-3 border-t border-line pt-3">
          <summary className="cursor-pointer text-xs text-muted">高级恢复：手动 Bearer 令牌</summary>
          <div className="settings-auth-controls mt-3" style={{ gap: '0.75rem', alignItems: 'center' }}>
            <input type="password" className="text-input" value={adminToken} autoComplete="off" placeholder="输入管理令牌" onChange={(event) => { setAdminTokenValue(event.target.value); setAdminStatus('unknown'); setAdminMessage('尚未验证') }} />
            <button type="button" className="btn btn-secondary" onClick={() => void verifyAdminToken()} disabled={adminStatus === 'checking'}>验证令牌</button>
          </div>
        </details>
        <div style={{ marginTop: '0.75rem', fontSize: '0.78rem', color: adminStatus === 'valid' ? '#6ee7b7' : adminStatus === 'invalid' ? '#fca5a5' : 'var(--text-secondary)' }}>
          {adminMessage || (adminToken ? '令牌仅在当前浏览器会话中，验证后启用管理操作' : '管理写操作已关闭')}
        </div>
      </div>
      <div className="card" style={{ padding: '2rem' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '2rem', borderBottom: '1px solid var(--border-color)', paddingBottom: '1rem' }}>
          <div>
            <h2 style={{ fontSize: '1.5rem', color: 'white' }}>{t('set.title')}</h2>
            <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>{t('set.sub')}</p>
          </div>
          <button className="btn btn-secondary" onClick={() => { fetchConfig(); fetchSystemStatus(); }}>
            <RefreshCw size={16} /> {t('set.reload')}
          </button>
        </div>

        <form onSubmit={handleSave} style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
          <div className="settings-two-col" style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1.5rem' }}>
            <div className="input-group">
              <label htmlFor="settings-provider" className="input-label">{t('set.provider')}</label>
              <select id="settings-provider" className="text-input" value={provider} onChange={(e) => handleProviderChange(e.target.value)}>
                <option value="google">Google Gemini</option>
                <option value="deepseek">DeepSeek</option>
                <option value="dual">DeepSeek + Gemini</option>
                <option value="openai">OpenAI</option>
                <option value="openai_compatible">OpenAI-Compatible</option>
              </select>
            </div>

            <div className="input-group">
              <label htmlFor="settings-output-language" className="input-label">{t('set.outLang')}</label>
              <select id="settings-output-language" className="text-input" value={lang} onChange={(e) => setLang(e.target.value)}>
                <option value="Bilingual">{t('set.bilingual')}</option>
                <option value="English">{t('set.en')}</option>
                <option value="Chinese">{t('set.zh')}</option>
              </select>
            </div>
          </div>

          <div className="settings-two-col" style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1.5rem' }}>
            <div className="input-group">
              <label htmlFor="settings-deep-model" className="input-label">{t('set.deep')}</label>
              <input id="settings-deep-model" type="text" className="text-input" value={deepModel} onChange={(e) => setDeepModel(e.target.value)} placeholder={providerModelHint(provider)} />
              <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{providerModelHint(provider)}</span>
            </div>

            <div className="input-group">
              <label htmlFor="settings-quick-model" className="input-label">{t('set.quick')}</label>
              <input id="settings-quick-model" type="text" className="text-input" value={quickModel} onChange={(e) => setQuickModel(e.target.value)} placeholder={providerModelHint(provider)} />
              <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{providerModelHint(provider)}</span>
            </div>
          </div>

          {provider === 'deepseek' || provider === 'openai_compatible' ? (
            <div className="input-group">
              <label htmlFor="settings-base-url" className="input-label">{t('set.baseUrl')}</label>
              <input id="settings-base-url" type="text" className="text-input" value={backendUrl} onChange={(e) => setBackendUrl(e.target.value)} placeholder={
                provider === 'deepseek' ? 'https://api.deepseek.com/v1' : 'http://localhost:8000/v1'
              } />
              <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                {provider === 'deepseek' ? t('set.dsDefault') : t('set.compatHint')}
              </span>
            </div>
          ) : null}

          <div className="settings-two-col" style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1.5rem' }}>
            <div className="input-group">
              <label htmlFor="settings-debate-rounds" className="input-label">{t('set.debate')}</label>
              <input id="settings-debate-rounds" type="number" min="1" max="5" className="text-input" value={debateRounds} onChange={(e) => setDebateRounds(parseInt(e.target.value) || 1)} />
            </div>

            <div className="input-group">
              <label htmlFor="settings-risk-rounds" className="input-label">{t('set.risk')}</label>
              <input id="settings-risk-rounds" type="number" min="1" max="5" className="text-input" value={riskRounds} onChange={(e) => setRiskRounds(parseInt(e.target.value) || 1)} />
            </div>
          </div>

          {provider === 'dual' ? (
            <div className="input-group">
              <span className="input-label">双模型密钥</span>
              <span style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
                DeepSeek 与 Gemini 密钥均从后端环境读取，不在浏览器中输入或保存。
              </span>
            </div>
          ) : (
            <div className="input-group">
              <label htmlFor="settings-api-key" className="input-label">{providerKeyHint(provider)} {t('set.keyOverride')}</label>
              <input
                id="settings-api-key"
                type="password"
                className="text-input"
                placeholder="••••••••••••••••••••••••••••••••"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
              />
              <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                {t('set.keyHint')}
              </span>
            </div>
          )}

          {testResult && (
            <div style={{
              padding: '0.85rem 1.25rem',
              borderRadius: '0.5rem',
              fontSize: '0.85rem',
              fontWeight: 600,
              background: testResult.ok ? 'rgba(16, 185, 129, 0.15)' : 'rgba(239, 68, 68, 0.15)',
              border: `1px solid ${testResult.ok ? 'rgba(16, 185, 129, 0.3)' : 'rgba(239, 68, 68, 0.3)'}`,
              color: testResult.ok ? '#6ee7b7' : '#fca5a5'
            }}>
              {testResult.message}
            </div>
          )}

          <div style={{ display: 'flex', gap: '1rem', marginTop: '1rem', alignItems: 'center' }}>
            <button type="submit" className="btn btn-primary">
              <Save size={18} /> {t('set.save')}
            </button>
            <button
              type="button"
              className="btn btn-secondary"
              onClick={handleTestConnection}
              disabled={testing}
            >
              <RefreshCw size={16} className={testing ? 'animate-spin' : ''} />
              {testing ? '正在测试连接…' : '测试 API 连通性'}
            </button>
          </div>
        </form>
      </div>

      <div className="card" style={{ padding: '1.5rem' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: '1rem', alignItems: 'center', marginBottom: '1rem' }}>
          <div>
            <h3 style={{ fontSize: '1.05rem', color: 'white', margin: 0 }}>核心能力与外部数据</h3>
			<p style={{ fontSize: '0.78rem', color: systemStatus?.status === 'ok' ? '#6ee7b7' : '#fbbf24', margin: '0.35rem 0 0' }}>
			  核心能力: {statusLoading && !systemStatus ? '检查中…' : systemStatus?.status || '未检查'}；外部数据状态见下表
			</p>
            <p style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', margin: '0.35rem 0 0' }}>
              运行时: {systemStatus?.runtime?.llmProvider || provider} / {systemStatus?.runtime?.quickThinkLLM || quickModel}
              {systemStatus?.checkedAt ? ` · 检查 ${formatStatusTime(systemStatus.checkedAt)}` : ''}
            </p>
            <p style={{ fontSize: '0.75rem', color: 'var(--text-muted)', margin: '0.35rem 0 0', maxWidth: '720px' }}>
              /api/health 仅表示进程存活；ready 只表示本地文件或快照可读，不等于 server live。只有接口同时具备来源、数据时间且未标记 stale，页面才会展示为当前可用数据。
            </p>
			<div style={{ marginTop: '0.65rem', display: 'grid', gap: '0.3rem', fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
			  <span>美股报价：{systemStatus?.runtime?.usQuoteFeed || '检查中'}{systemStatus?.runtime?.alpacaConfigured === false ? ' · 在后端配置免费 Alpaca key 后启用 IEX' : ''}</span>
			  <span>A股报价：{systemStatus?.runtime?.cnQuoteFeed || '检查中'} · 富途 OpenD 尚未配置，当前只适合 Paper 研究</span>
			  <span style={{ color: 'var(--text-muted)' }}>密钥通过后端环境变量或仓库外 `0600` 凭证文件配置，页面不会读取或保存。</span>
			</div>
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem', justifyContent: 'flex-end' }}>
            <button className="btn btn-primary" type="button" onClick={() => void refreshMarketData()} disabled={marketRefreshing}>
              <RefreshCw size={16} className={marketRefreshing ? 'animate-spin' : ''} /> {marketRefreshing ? '刷新中' : '管理员刷新行情'}
            </button>
            <button className="btn btn-secondary" type="button" onClick={fetchSystemStatus} disabled={statusLoading}>
              <RefreshCw size={16} className={statusLoading ? 'animate-spin' : ''} /> 刷新状态
            </button>
          </div>
        </div>

        {marketRefreshMessage && (
          <div style={{ marginBottom: '0.75rem', fontSize: '0.78rem', color: marketRefreshMessage.includes('失败') ? '#fca5a5' : '#6ee7b7' }}>
            {marketRefreshMessage}
          </div>
        )}

        <div className="settings-health-table" style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', minWidth: '720px', borderCollapse: 'collapse', fontSize: '0.78rem' }}>
            <thead>
              <tr style={{ color: 'var(--text-muted)', borderBottom: '1px solid var(--border-color)' }}>
                <th style={{ textAlign: 'left', padding: '0.65rem 0.5rem' }}>域</th>
                <th style={{ textAlign: 'left', padding: '0.65rem 0.5rem' }}>接口</th>
                <th style={{ textAlign: 'left', padding: '0.65rem 0.5rem' }}>来源</th>
                <th style={{ textAlign: 'left', padding: '0.65rem 0.5rem' }}>数据时间</th>
                <th style={{ textAlign: 'left', padding: '0.65rem 0.5rem' }}>状态</th>
              </tr>
            </thead>
            <tbody>
              {(systemStatus?.endpoints || []).map((item) => (
                <tr key={`${item.domain}-${item.endpoint}`} style={{ borderBottom: '1px solid rgba(255,255,255,0.06)' }} title={item.staleReason || item.source}>
                  <td style={{ padding: '0.7rem 0.5rem', color: 'var(--text-primary)', whiteSpace: 'nowrap' }}>{item.domain}</td>
                  <td style={{ padding: '0.7rem 0.5rem', color: 'var(--text-secondary)', fontFamily: 'monospace' }}>{item.endpoint}</td>
                  <td style={{ padding: '0.7rem 0.5rem', color: 'var(--text-secondary)' }}>{item.source}</td>
                  <td style={{ padding: '0.7rem 0.5rem', color: 'var(--text-secondary)', whiteSpace: 'nowrap' }}>{formatStatusTime(item.dataTime)}</td>
                  <td style={{ padding: '0.7rem 0.5rem' }}>
                    <span style={{
                      borderRadius: '999px',
                      padding: '0.2rem 0.55rem',
                      border: `1px solid ${item.stale || item.status === 'unavailable' ? 'rgba(251, 191, 36, 0.35)' : 'rgba(16, 185, 129, 0.35)'}`,
                      color: item.stale || item.status === 'unavailable' ? '#fbbf24' : '#6ee7b7',
                      background: item.stale || item.status === 'unavailable' ? 'rgba(251, 191, 36, 0.12)' : 'rgba(16, 185, 129, 0.12)',
                      whiteSpace: 'nowrap',
                    }}>
                      {item.stale ? 'stale' : item.status || 'ok'}
                    </span>
                  </td>
                </tr>
              ))}
              {(!systemStatus?.endpoints || systemStatus.endpoints.length === 0) && (
                <tr>
                  <td colSpan={5} style={{ padding: '1rem 0.5rem', color: 'var(--text-muted)' }}>{statusLoading ? '正在检查数据健康…' : '暂无数据健康信息'}</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        <div className="settings-health-cards" aria-label="数据健康状态">
          {(systemStatus?.endpoints || []).map((item) => {
            const degraded = item.stale || item.status === 'unavailable'
            return (
              <article className="settings-health-card" key={`mobile-${item.domain}-${item.endpoint}`}>
                <div className="settings-health-card__head">
                  <strong>{item.domain}</strong>
                  <span className={degraded ? 'settings-health-badge settings-health-badge--stale' : 'settings-health-badge'}>
                    {item.stale ? 'stale' : item.status || 'ok'}
                  </span>
                </div>
                <code>{item.endpoint}</code>
                <dl>
                  <div><dt>数据时间</dt><dd>{formatStatusTime(item.dataTime)}</dd></div>
                  <div><dt>来源</dt><dd>{item.source || '未知'}</dd></div>
                </dl>
                {item.staleReason && <p>{item.staleReason}</p>}
              </article>
            )
          })}
          {(!systemStatus?.endpoints || systemStatus.endpoints.length === 0) && (
            <div className="settings-health-empty">{statusLoading ? '正在检查数据健康…' : '暂无数据健康信息'}</div>
          )}
        </div>

        <details style={{ marginTop: '1rem', color: 'var(--text-secondary)' }}>
          <summary style={{ cursor: 'pointer', fontSize: '0.8rem' }}>文件明细</summary>
          <div style={{ marginTop: '0.75rem', display: 'grid', gap: '0.5rem' }}>
            {(systemStatus?.dataFiles || []).map((file) => (
              <div key={file.id} style={{ display: 'grid', gridTemplateColumns: '140px 110px 1fr', gap: '0.75rem', fontSize: '0.75rem' }}>
                <span style={{ color: 'var(--text-primary)', fontFamily: 'monospace' }}>{file.id}</span>
                <span>{file.exists ? `${file.count || 0} 条` : 'missing'}</span>
                <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {formatStatusTime(file.generatedAt || file.modifiedAt)} {file.error ? ` · ${file.error}` : ''}
                </span>
              </div>
            ))}
          </div>
        </details>
      </div>
    </div>
  )
}
