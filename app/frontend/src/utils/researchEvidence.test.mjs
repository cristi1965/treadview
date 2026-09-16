import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';

const stockDetail = fs.readFileSync(new URL('../pages/StockDetail.tsx', import.meta.url), 'utf8');

test('stock detail preserves ETF owners historical provenance', () => {
  assert.match(stockDetail, /etfOwnersResult\.dataMode === 'historical'/);
  assert.match(stockDetail, /etfOwnersResult\.updated/);
  assert.match(stockDetail, /etfOwnersResult\.degradedReason/);
});
const history = fs.readFileSync(new URL('../pages/History.tsx', import.meta.url), 'utf8');
const i18n = fs.readFileSync(new URL('../i18n.ts', import.meta.url), 'utf8');
const analysisStore = fs.readFileSync(new URL('../stores/analysisStore.ts', import.meta.url), 'utf8');
const dossierView = fs.readFileSync(new URL('../components/research/EvidenceOnlyDossierView.tsx', import.meta.url), 'utf8');
const tactical = fs.readFileSync(new URL('../pages/TacticalCommand.tsx', import.meta.url), 'utf8');

test('stock detail exposes fundamental dates, field sources, and reproducible score inputs', () => {
  for (const token of ['fiscalPeriod', 'asOf', 'fetchedAt', 'sourceLinks', 'fieldSources', 'scoreDetail.reproduces_panel', 'scoreDetail?.components']) {
    assert.ok(stockDetail.includes(token), `missing ${token}`);
  }
});

test('history exposes structured run, model, hash, claim, and evidence metadata', () => {
  for (const token of ['audit.run_id', 'audit.method_version', 'audit.model_provider', 'input_hash', 'content_hash', 'evidence_ids', 'payload_ref', 'research_health', 'source_status']) {
    assert.ok(history.includes(token), `missing ${token}`);
  }
  assert.ok(history.includes('Legacy 结果'));
  assert.ok(history.includes("return 'unknown'"));
  assert.ok(history.includes('完整性未知'));
  assert.match(history, /entry\.kind === 'user-input'/);
  assert.match(history, /entry\.payload_excerpt/);
  assert.match(history, /entry\.payload_ref && entry\.payload_size/);
});

test('history separates evidence-only dossiers from agent runs and discloses their limits', () => {
  assert.ok(i18n.includes("'hist.title': '研究证据记录'"));
  assert.ok(!i18n.includes('过往多智能体股票评估的审计轨迹'));
  for (const token of [
    '仅证据包', '未调用 LLM', '非 10-Agent', '非投资建议',
    '事实摘要', '结论依据', '风险', '缺口', 'dossier.facts', 'dossier.risks', 'dossier.gaps', 'dossier.conclusion_basis',
    'research_mode', '多智能体 Agent Run',
  ]) {
    assert.ok(`${history}\n${dossierView}`.includes(token), `missing history disclosure ${token}`);
  }
});

test('evidence-only dossier models and renders every v2 reproducibility surface', () => {
  for (const token of ['calculations', 'financial_periods', 'historical', 'news_review', 'formula', 'inputs', 'value', 'status', 'evidence_ids', 'source_tier', 'excluded_low_relevance', 'duplicates_removed']) {
    assert.ok(analysisStore.includes(token), `store missing dossier v2 field ${token}`);
    assert.ok(dossierView.includes(token), `view missing dossier v2 field ${token}`);
  }
  assert.match(dossierView, /仅观察（不是持有\/买卖建议）/);
  assert.match(dossierView, /Observe only \(not a hold, buy, or sell recommendation\)/);
  assert.match(dossierView, /localizeDynamic/);
  assert.match(dossierView, /查看关键字段原文/);
  for (const token of ['最近一个已完成收盘价是唯一主研究价格', '可比季度历史不完整', '市销率和市净率保持不可用', '年化波动率', '状态']) {
    assert.ok(dossierView.includes(token), `missing dynamic Chinese research mapping ${token}`);
  }
  for (const token of ['未执行 LLM 辩论', '可选的投资授权', '未提供持仓', '必需字段缺失或分母为零', '最新季度期间不是四个连续', '能回答', '不能回答']) {
    assert.ok(dossierView.includes(token), `missing R14 Chinese boundary ${token}`);
  }
});

test('research context is structured, assessed, and remains unknown when absent', () => {
  for (const token of ['EvidenceResearchContext', 'EvidenceContextAssessment', 'research_context', 'context_assessment', 'issuer_cap_pct', 'scenario', 'holding', 'issuer_cap', 'limitations']) {
    assert.ok(analysisStore.includes(token), `store missing context contract ${token}`);
  }
  for (const token of ['用户研究上下文', '已提供', '未知', '本标的持仓', '发行人上限评估', '剩余额度', '是否超限', 'assessment?.holding', 'assessment?.issuer_cap', 'context?.holdings?.find']) {
    assert.ok(dossierView.includes(token), `view missing context disclosure ${token}`);
  }
  assert.match(dossierView, /issuerCapPct! - weightPct!/);
  assert.match(history, /ticker=\{selectedItem\.ticker\}/);
});

test('history hides legacy agent sections for evidence-only and derives duration until backend supplies it', () => {
  assert.match(history, /selectedKind === 'agent-run' \|\| selectedKind === 'legacy'/);
  for (const token of ['generation_duration_secs', 'elapsed_secs', 'generation_duration_ms', 'duration_ms', '按证据时间推算', '耗时待后端补充']) {
    assert.ok(`${history}\n${analysisStore}`.includes(token), `missing duration compatibility ${token}`);
  }
});

test('analysis state preserves observe and unavailable without coercing unknown to hold', () => {
  for (const token of ["'OBSERVE'", "'research_unavailable'", 'ResearchHealth', 'payload_ref', "case 'analysis_unavailable'"]) {
    assert.ok(analysisStore.includes(token), `missing ${token}`);
  }
  assert.ok(history.includes("case 'HOLD'"));
  assert.ok(history.includes("case 'OBSERVE'"));
});

test('empty history exposes one auditable research recovery path', () => {
  assert.match(history, /to="\/dashboard#research-entry"/);
  assert.match(history, /前往可审计研究入口/);
  assert.match(history, /historical-research 门禁/);
});

test('mobile history opens detail ahead of the list and Paper prefill carries no trade intent', () => {
  assert.match(history, /mobileDetailOpen/);
  assert.match(history, /返回研究记录列表/);
  assert.match(history, /hidden lg:flex/);
  assert.match(history, /research_ticker/);
  assert.match(history, /research_run_id/);
  assert.match(history, /不预填方向、数量、价格或止损/);
  assert.doesNotMatch(history, /params\.set\(['"](?:side|quantity|price|stop)/);
});

test('history keeps failed and legacy records in a collapsed archive', () => {
  assert.match(history, /失败归档/);
  assert.match(history, /<details/);
  assert.match(history, /group\.archived/);
  assert.match(history, /historyGroups\.find\(\(group\) => !group\.archived/);
});

test('Paper quote gating uses the selected symbol instead of mixed-batch stale state', () => {
  assert.match(tactical, /currentQuote\?\.session === 'closed'/);
  assert.match(tactical, /quoteAgeMs > 90_000/);
  assert.doesNotMatch(tactical, /quoteResult\.meta\.stale\s*\?/);
});
