import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const page = readFileSync(new URL('../pages/ETF.tsx', import.meta.url), 'utf8');
const store = readFileSync(new URL('../stores/etfStore.ts', import.meta.url), 'utf8');

test('ETF current and historical modes are explicit and never silently substituted', () => {
  assert.match(store, /mode=\$\{dataMode\}/);
  assert.match(store, /setDataMode: \(dataMode\)/);
  assert.doesNotMatch(store, /const snapshot = await apiGet/);
  assert.match(page, /setDataMode\('current'\)/);
  assert.match(page, /setDataMode\('historical'\)/);
  assert.match(page, /历史快照模式/);
  assert.match(page, /不代表实时价格或当前收益/);
});

test('ETF mode displays freshness age and scheduler diagnostics', () => {
  assert.match(store, /ageSeconds: number/);
  assert.match(store, /nextScheduledAt\?: string/);
  assert.match(page, /快照年龄/);
  assert.match(page, /最近尝试/);
  assert.match(page, /下次计划/);
  assert.match(page, /scopeCompleted/);
  assert.match(store, /error instanceof APIError/);
  assert.match(store, /unavailable\?\.diagnostics/);
});

test('ETF AUM has an explicit source unit and is normalized to USD', () => {
  const data = readFileSync(new URL('../../public/data/etf-analyses.json', import.meta.url), 'utf8');
  const backend = readFileSync(new URL('../../../backend/internal/api/etf_handlers.go', import.meta.url), 'utf8');
  assert.match(data.slice(0, 160), /"aumUnit": "usd_thousands"/);
  assert.match(backend, /normalizeETFAnalysisAUM/);
  assert.match(backend, /data\.ETFs\[index\]\.AUM \*= 1000/);
  assert.match(backend, /unsupported or missing aumUnit/);
});

test('an ETF symbol detail has an H1, static holdings and recovery actions', () => {
  assert.match(page, /\/api\/etf\/\$\{encodeURIComponent\(id\)\}\/holdings/);
  assert.match(page, /<h1[^>]*>\{detail\.name\} \(\{detail\.sym\}\)<\/h1>/);
  assert.match(page, /静态基础资料/);
  assert.match(page, /本页不提供实时价格、涨跌幅或当前净值/);
  assert.match(page, /查看数据健康与修复入口/);
  assert.match(page, /holdings\.dataMode === 'historical'/);
  assert.match(page, /holdings\.degradedReason/);
});

test('ETF compare exposes loading, failure and retry states', () => {
  assert.match(page, /const handleCompare = async/);
  assert.match(page, /compareLoading \? '对比中\.\.\.' : '对比'/);
  assert.match(page, /role="alert"/);
  assert.match(page, />重试<\/button>/);
});
