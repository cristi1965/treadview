import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const settings = fs.readFileSync(new URL('../pages/Settings.tsx', import.meta.url), 'utf8')
const dashboard = fs.readFileSync(new URL('../pages/Dashboard.tsx', import.meta.url), 'utf8')
const researchLab = fs.readFileSync(new URL('../pages/ResearchLab.tsx', import.meta.url), 'utf8')
const routes = fs.readFileSync(new URL('../AppRoutes.tsx', import.meta.url), 'utf8')
const syncController = fs.readFileSync(new URL('../components/SystemDataSyncController.tsx', import.meta.url), 'utf8')
const header = fs.readFileSync(new URL('../components/layout/Header.tsx', import.meta.url), 'utf8')
const layout = fs.readFileSync(new URL('../components/layout/Layout.tsx', import.meta.url), 'utf8')
const sidebar = fs.readFileSync(new URL('../components/layout/Sidebar.tsx', import.meta.url), 'utf8')
const i18n = fs.readFileSync(new URL('../i18n.ts', import.meta.url), 'utf8')
const apiSource = fs.readFileSync(new URL('./api.ts', import.meta.url), 'utf8')
const adminSessionHook = fs.readFileSync(new URL('../hooks/useAdminSession.ts', import.meta.url), 'utf8')

test('settings renders separate readiness profiles without equating them to full or live readiness', () => {
  for (const profile of ['historical-research', 'paper-engine', 'live-trading-data']) {
    assert.ok(settings.includes(profile), `missing readiness profile ${profile}`)
    assert.ok(settings.includes(`/api/readiness?profile=${'${profile}'}`))
  }
  assert.ok(settings.includes('当前产品承诺'))
  assert.ok(settings.includes('未纳入当前产品承诺'))
  assert.match(settings, /HTTP \{profile\.httpStatus\}/)
  assert.match(settings, /localizeReadinessText/)
  assert.match(settings, /ReadinessRaw/)
  assert.match(settings, /statusLoading && !systemStatus \? '检查中…'/)
  assert.match(settings, /statusLoading \? '正在检查数据健康…' : '暂无数据健康信息'/)
  for (const token of ['历史时点研究', '来自带日期 Nasdaq 历史收盘价的时点主研究价格', '带日期的 Nasdaq OHLCV 历史', '从权威提供方刷新', '展开原始值']) assert.ok(settings.includes(token), `missing localized readiness token ${token}`)
  assert.ok(settings.indexOf('id="product-readiness"') < settings.indexOf('管理操作授权'))
})

test('dashboard creates an evidence-only dossier while preserving permission gates', () => {
  for (const token of ['/api/readiness?profile=historical-research', '/api/analysis/evidence-only', 'authorizedFetch', 'useAdminSession', '仅证据研究工作台', '生成仅证据研究包', '当前门禁未通过', '仅证据研究包生成失败']) {
    assert.ok(dashboard.includes(token), `missing evidence-only dashboard contract ${token}`)
  }
  assert.ok(!dashboard.includes("apiUrl('/api/analysis/start')"), 'historical dashboard must not invoke the LLM agent endpoint')
  assert.match(dashboard, /navigate\(`\/history\?run_id=\$\{encodeURIComponent\(runID\)\}`\)/)
  assert.match(apiSource, /response\.status !== 401[\s\S]*establishLocalAdminSession\(\)[\s\S]*return fetch\(retryInput/)
  assert.match(apiSource, /verifyOrEstablishLocalAdminSession[\s\S]*await establishLocalAdminSession\(\)[\s\S]*Boolean\(getAdminToken\(\)\)/)
  assert.doesNotMatch(apiSource, /verifyOrEstablishLocalAdminSession[\s\S]{0,240}method: 'GET'/)
  assert.match(adminSessionHook, /verifyOrEstablishLocalAdminSession/)
  assert.match(dashboard, /Bearer token required\|local admin session expired/)
  assert.ok(dashboard.includes('本机管理会话已失效，自动恢复失败，请刷新页面后重试'))
  assert.match(dashboard, /disabled=\{!hasAdminAccess \|\| evidenceBusy\}/)
  assert.doesNotMatch(dashboard, /if \(!historicalReadiness\?\.ready\)/)
  assert.match(dashboard, /生成后会重新检查/)
  for (const field of ['mandate', 'holdings', 'liquidity', 'tax', 'risk_budget', 'issuer_cap_pct', 'revenue_growth_pct', 'ps_multiple']) assert.ok(dashboard.includes(field), `missing research context field ${field}`)
  assert.match(dashboard, /research_context:/)
  assert.match(dashboard, /interface HoldingDraft/)
  assert.match(dashboard, /normalizedHoldings = holdings/)
  assert.match(dashboard, /nextHoldingID\.current\+\+/)
  assert.match(dashboard, /新增持仓/)
  assert.doesNotMatch(dashboard, /JSON\.parse\(researchContext\.holdings/)
  assert.match(dashboard, /已有证据摘要/)
  assert.match(dashboard, /to="\/lab"/)
  for (const forbidden of ['AgentFlowGraph', 'DecisionGauge', 'MarketAnomaliesRadar', 'QDIIPremiumRadar', 'AshareMarketRadar', 'StockChart', 'startAnalysis']) {
    assert.ok(!dashboard.includes(forbidden), `dashboard must not mount ${forbidden}`)
    assert.ok(researchLab.includes(forbidden), `lab must own ${forbidden}`)
  }
  assert.match(routes, /path="\/lab"/)
  assert.match(routes, /path="\/" element=\{<Navigate to="\/dashboard" replace \/>\}/)
  assert.match(routes, /path="\/market" element=\{<Home \/>\}/)
  assert.match(routes, /useWebSocket\(location\.pathname === '\/lab'\)/)
  assert.match(syncController, /researchOnlyRoute/)
  assert.match(syncController, /pathname === '\/market'/)
});

test('research routes replace live chrome with historical and Paper boundaries', () => {
  for (const route of ['/dashboard', '/history', '/lab', '/copilot', '/tactical', '/journal', '/settings']) {
    assert.ok(layout.includes(`'${route}'`), `research chrome missing ${route}`)
  }
  assert.match(layout, /useGlobalDataFreshness\(!researchSurface\)/)
  assert.match(header, /surface === 'research'/)
  assert.match(header, /历史研究 \+ Paper 模拟 · 非实时/)
  assert.match(header, /surface === 'default' && <GlobalDataFreshnessControl/)
  assert.match(header, /surface === 'default' && <a/)
  assert.match(sidebar, /labelKey: 'nav\.copilot'/)
  assert.match(sidebar, /labelKey: 'nav\.tactical'/)
  assert.match(sidebar, /path: '\/market', labelKey: 'nav\.heatmap'/)
  assert.match(i18n, /'nav\.copilot': '研究助手'/)
  assert.match(i18n, /'nav\.tactical': 'Paper 工作台'/)
  assert.match(i18n, /'nav\.heatmap': '实验行情'/)
})
