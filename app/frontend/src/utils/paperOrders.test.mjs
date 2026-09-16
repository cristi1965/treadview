import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

const source = fs.readFileSync(new URL('./paperOrders.ts', import.meta.url), 'utf8');
const tacticalSource = fs.readFileSync(new URL('../pages/TacticalCommand.tsx', import.meta.url), 'utf8');
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
}).outputText;
const module = { exports: {} };
const requireStub = (id) => {
  if (id === './api') return { get: (...args) => args, post: (...args) => args };
  throw new Error(`Unexpected test dependency: ${id}`);
};
new Function('module', 'exports', 'require', compiled)(module, module.exports, requireStub);
const { buildPaperOrderRequest, establishPaperDailyEquityBaseline, paperSchedulerPollDelay, simulatePaperFill } = module.exports;

const plan = {
  side: 'BUY', market: 'US', currency: 'USD', orderType: 'LIMIT', tif: 'GTC',
  entry: 98.2, triggerPrice: null, protectiveStop: 96.24, takeProfit: 104.09, quantity: 15,
  maxLoss: 29.4, maxRiskAmount: 150, plannedNotional: 1473,
  maxNotionalAmount: 1500, riskLimitPassed: true,
};

test('builds an explicitly PAPER request with quote provenance and risk snapshot', () => {
  const request = buildPaperOrderRequest({
    clientOrderId: 'paper-1', symbol: ' nvda ', plan, referencePrice: 100, investableCapital: 10_000,
    quoteSource: 'tradingview', quoteTime: '2026-09-06T10:00:00Z',
  });
  assert.equal(request.environment, 'PAPER');
  assert.equal(request.symbol, 'NVDA');
  assert.equal(request.riskSnapshot.plannedNotional, 1473);
  assert.equal(request.protectiveStop, 96.24);
  assert.equal(request.triggerPrice, null);
  assert.equal(request.quoteSource, 'tradingview');
});

test('Paper request preserves optional research provenance without inferring trade intent', () => {
  const request = buildPaperOrderRequest({
    clientOrderId: 'paper-research-1', symbol: 'NVDA', plan, referencePrice: 100, investableCapital: 10_000,
    quoteSource: 'tradingview', quoteTime: '2026-09-06T10:00:00Z', researchRunId: 'run-1', researchTicker: 'NVDA',
  });
  assert.equal(request.researchRunId, 'run-1');
  assert.equal(request.researchTicker, 'NVDA');
  assert.equal(request.side, plan.side);
});

test('refuses to build a request when risk did not pass', () => {
  assert.throws(() => buildPaperOrderRequest({
    clientOrderId: 'paper-2', symbol: 'NVDA', plan: { ...plan, riskLimitPassed: false }, referencePrice: 100,
    investableCapital: 10_000, quoteSource: 'tradingview', quoteTime: '2026-09-06T10:00:00Z',
  }), /风险校验未通过/);
});

test('scheduler polling follows the scanner interval with bounded fallback', () => {
  assert.equal(paperSchedulerPollDelay('5s'), 5_000);
  assert.equal(paperSchedulerPollDelay('250ms'), 1_000);
  assert.equal(paperSchedulerPollDelay('2m'), 60_000);
  assert.equal(paperSchedulerPollDelay('unavailable'), 5_000);
});

test('daily baseline command selects currency without accepting client equity', () => {
  assert.deepEqual(establishPaperDailyEquityBaseline('USD'), ['/api/paper-orders/daily-baseline?currency=USD']);
});

test('partial Paper fill sends a stable fill id and explicit quantity', () => {
  assert.deepEqual(simulatePaperFill('paper/order 1', { fillId: 'browser-fill-1', quantity: 2 }), [
    '/api/paper-orders/paper%2Forder%201/simulated-fill',
    { fillId: 'browser-fill-1', quantity: 2 },
  ]);
});

test('tactical scheduler polling reschedules from server interval and stops on unmount', () => {
  assert.match(tacticalSource, /paperSchedulerPollDelay\(scheduler\.interval\)/);
  assert.match(tacticalSource, /window\.setTimeout\(\(\) => void pollScheduler\(\), delay\)/);
  assert.match(tacticalSource, /window\.clearTimeout\(timeout\)/);
  assert.match(tacticalSource, /setPaperScheduler\(scheduler\);\s*setPaperSchedulerError\(''\)/);
  assert.doesNotMatch(tacticalSource, /setPaperScheduler\(scheduler\);\s*setPaperError\(''\)/);
  assert.match(tacticalSource, /paperScheduler\?\.auditRetention/);
  assert.match(tacticalSource, /if \(response\.warning\) setPaperWarning\(response\.warning\)/);
});

test('Paper refresh and rejection paths preserve operation evidence', () => {
  const refreshStart = tacticalSource.indexOf('const refreshPaperState = async () =>');
  const refreshEnd = tacticalSource.indexOf('\n  useEffect(() =>', refreshStart);
  const refreshSource = tacticalSource.slice(refreshStart, refreshEnd);
  assert.match(tacticalSource, /const generation = \+\+paperRefreshGeneration\.current/);
  assert.match(tacticalSource, /if \(generation !== paperRefreshGeneration\.current\) return/);
  assert.match(tacticalSource, /setPaperLoadError\(error instanceof Error \? error\.message : '无法读取 paper 订单记录'\)/);
  assert.doesNotMatch(refreshSource, /setPaperError\(/);
  assert.match(tacticalSource, /if \(!validation\.valid\) \{[\s\S]*?可审计的 REJECTED Paper 记录[\s\S]*?\}\s*const response = await submitPaperOrder\(request\)/);
  assert.match(tacticalSource, /error instanceof APIError && error\.data\?\.order/);
  assert.match(tacticalSource, /setActivePaperOrder\(rejected\)/);
});

test('paper workbench exposes policy thresholds and both audit phases', () => {
  assert.match(source, /submissionPolicyCheck\?: PaperPolicyCheck/);
  assert.match(source, /fillPolicyCheck\?: PaperPolicyCheck/);
  assert.match(tacticalSource, /策略 \{paperPortfolio\.limits\.policyVersion\}/);
  assert.match(tacticalSource, /paperPortfolio\.limits\.minStopCoveragePct/);
  assert.match(tacticalSource, /paperPortfolio\.limits\.maxDailyLossPct/);
  assert.match(tacticalSource, /paperPortfolio\.limits\.maxDrawdownPct/);
  assert.match(tacticalSource, /paperPortfolio\.limits\.maxStressLossPct/);
  assert.match(tacticalSource, /activePaperOrder\.fills\.map/);
  assert.match(tacticalSource, /paperPortfolio\.baseCurrency\.status/);
  assert.match(tacticalSource, /group\.dailyLossPct/);
  assert.match(tacticalSource, /riskSnapshot\.submissionPolicyCheck/);
  assert.match(tacticalSource, /riskSnapshot\.fillPolicyCheck/);
});

test('paper workbench leads with currency-isolated accounts and demotes QDII to optional observation', () => {
  for (const token of ['Paper 工作台独立于 QDII 实时数据', 'CNY 与 USD 是彼此隔离的 Paper 子账户', '不做隐式汇兑或跨币种抵扣', 'QDII 溢价数据（可选实时观察）', '不影响 Paper 子账户']) {
    assert.ok(tacticalSource.includes(token), `missing Paper/QDII boundary ${token}`);
  }
  assert.ok(tacticalSource.indexOf('人工条件单计划生成器') < tacticalSource.indexOf('QDII 溢价数据（可选实时观察）'));
  assert.match(tacticalSource, /<details className="border-y border-line bg-surface\/40/);
  assert.match(tacticalSource, /useQDIIPremiums\(\{ enabled: qdiiOpen, pollMs: qdiiOpen \? 30_000 : 0 \}\)/);
  assert.match(tacticalSource, /onToggle=\{\(event\) => setQDIIopen\(event\.currentTarget\.open\)\}/);
});

test('Paper accepts research provenance without inheriting direction or quantity', () => {
  for (const token of ['useSearchParams', 'research_ticker', 'research_run_id', "sourceResearchTicker ? '' : 'BUY_LOW'", '来源研究记录', '没有传入方向、数量、价格或止损', '来源研究记录未提供方向或数量', '请选择独立 Paper 策略', '重新取得新鲜权威报价', '服务端提交与成交风控']) {
    assert.ok(tacticalSource.includes(token), `missing research-to-Paper boundary ${token}`);
  }
});

test('Paper persists research provenance and hands terminal orders to Journal', () => {
  for (const token of ['researchRunId: sourceResearchRunID', 'researchTicker: sourceResearchRunID', 'activePaperOrder.researchRunId', 'paper_order_id=', 'research_run_id=']) {
    assert.ok(tacticalSource.includes(token), `missing persisted Paper provenance ${token}`);
  }
  assert.ok(tacticalSource.includes('记录到交易复盘'));
});

test('dynamic Paper disclosures have Chinese summaries and preserve server originals', () => {
  for (const token of ['交易日历：', '审计记录：', '查看服务端日历、会话、审计与免责声明原文', 'calendar_coverage:', 'audit_retention:', '查看汇率隔离原文与证据', '各币种仍按独立子账户与独立限额管理']) {
    assert.ok(tacticalSource.includes(token), `missing localized Paper disclosure ${token}`);
  }
});

test('Paper uses symbol-level quote time and distinguishes unauthorized scanner state', () => {
  assert.match(tacticalSource, /currentQuote\?\.dataTime \|\| quoteResult\.meta\.dataTime/);
  assert.match(tacticalSource, /quoteTime: quoteDataTime/);
  assert.ok(tacticalSource.includes('未授权，状态不可见'));
  assert.ok(tacticalSource.includes('STOPPED / 读取失败'));
  assert.doesNotMatch(tacticalSource, /STOPPED \/ 不可见/);
});

test('Paper quote failure exposes recovery while closed quotes remain draft-only', () => {
  assert.ok(tacticalSource.includes('重新获取报价'));
  assert.match(tacticalSource, /dailyBaselineMissing \? refreshPaperState\(\) : setQuoteRefreshVersion\(\(value\) => value \+ 1\)/);
  assert.match(tacticalSource, /disabled=\{loading \|\| paperBusy\}/);
  assert.match(tacticalSource, /planningBlockReason/);
  assert.ok(tacticalSource.includes('待开市复核'));
  assert.match(tacticalSource, /disabled=\{!hasAdminAccess \|\| !orderPlan \|\| !orderPlan\.riskLimitPassed \|\| Boolean\(quoteBlockReason\) \|\| paperBusy\}/);
});

test('Paper requests only the selected symbol so unrelated markets cannot contaminate its quote', () => {
  assert.match(tacticalSource, /fetchQuoteResult\(\[selectedTicker\.trim\(\)\.toUpperCase\(\)\]\)/);
  assert.doesNotMatch(tacticalSource, /fetchQuoteResult\(\['513100', 'NVDA'/);
});

test('Paper blocks BUY before submit when the scanner daily baseline is unavailable', () => {
  assert.match(tacticalSource, /selectedPortfolioGroup != null && !selectedPortfolioGroup\.dailyRiskComplete/);
  assert.match(tacticalSource, /dailyBaselineMissing && orderPlan\?\.side === 'BUY'/);
  assert.ok(tacticalSource.includes('等待本地扫描器在常规交易时段建立后再提交 BUY'));
  assert.ok(tacticalSource.includes('重新检查日初基线'));
  assert.match(tacticalSource, /dailyBaselineMissing \? refreshPaperState\(\) : setQuoteRefreshVersion\(\(value\) => value \+ 1\)/);
});

test('Paper lifecycle exposes explicit baseline, partial fill, OCO and audit states', () => {
  for (const token of ['建立 ${selectedCurrency} 日初基线', '模拟成交数量', '复核模拟成交', '确认标记成交', '写入后进入审计链', 'OCO 保护', 'Paper 审计验证', 'verifyPaperAuditChain', 'pendingFillId']) {
    assert.ok(tacticalSource.includes(token), `missing Paper lifecycle token ${token}`);
  }
  assert.match(tacticalSource, /simulatePaperFill\(activePaperOrder\.clientOrderId, \{ fillId, quantity: requestedQuantity \}\)/);
  assert.match(tacticalSource, /setFillConfirmOpen\(true\)/);
  assert.match(tacticalSource, /paperAuditVerification\?\.valid === true && paperAuditVerification\.pendingCount === 0/);
  assert.match(tacticalSource, /activeLifecycleOrders\.some\(\(order\) => \(order\.fills\?\.length \?\? 0\) > 0\)/);
  assert.match(tacticalSource, /activeLifecycleOrders\.some\(\(order\) => Boolean\(order\.parentOrderId && order\.ocoGroupId\)\)/);
  assert.match(tacticalSource, /orderedPaperOrders\.map\(\(order\) =>/);
  assert.doesNotMatch(tacticalSource, /orderedPaperOrders\.slice\(/);
});
