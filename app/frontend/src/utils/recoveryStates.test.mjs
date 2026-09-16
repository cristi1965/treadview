import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const read = (path) => readFileSync(new URL(path, import.meta.url), 'utf8');

test('protected portfolio refetches when the local admin session becomes authorized', () => {
  const page = read('../pages/Portfolio.tsx');
  assert.match(page, /setPaperPortfolio\(null\)/);
  assert.match(page, /\}, \[hasAdminAccess\]\);/);
});

test('core market failures expose local retry actions', () => {
  const home = read('../pages/Home.tsx');
  const anomalies = read('../components/MarketAnomaliesRadar.tsx');
  assert.match(home, /retryHomeData/);
  assert.match(home, /重试脉冲数据/);
  assert.match(anomalies, /retryVersion/);
  assert.match(anomalies, /重试异动数据/);
});

test('ETF list and detail failures can retry without leaving the page', () => {
  const page = read('../pages/ETF.tsx');
  assert.match(page, /detailRetryVersion/);
  assert.match(page, /重试 ETF 详情/);
  assert.match(page, /fetchSectors\(true\)/);
  assert.match(page, /重试 ETF 总表/);
});

test('command search distinguishes transport failure from empty results', () => {
  const shell = read('../components/layout/StockGodShell.tsx');
  assert.match(shell, /searchError/);
  assert.match(shell, /股票搜索服务暂不可用/);
  assert.match(shell, /重试搜索/);
  assert.match(shell, /!searchError && results\.length === 0/);
});

test('whales sync status uses the backend route and response contract', () => {
  const store = read('../stores/whalesStore.ts');
  assert.match(store, /\/api\/whales\/status/);
  assert.match(store, /syncStatus: status\.syncStatus \|\| 'Idle'/);
  assert.doesNotMatch(store, /\/api\/whales\/sync-status/);
});
