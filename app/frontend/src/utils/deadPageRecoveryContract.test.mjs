import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const read = (path) => readFileSync(new URL(path, import.meta.url), 'utf8');

test('shared API requests terminate within the dead-page threshold', () => {
  const api = read('./api.ts');
  assert.match(api, /REQUEST_TIMEOUT_MS = 5_000/);
  assert.match(api, /authorizedFetchWithTimeout/);
  assert.match(api, /controller\.abort\(new Error\('请求超时，请重试'\)\)/);
});

test('report details unwrap the API contract and retry the current route', () => {
  const store = read('../stores/reportsStore.ts');
  const page = read('../pages/Reports.tsx');
  const controller = read('../components/SystemDataSyncController.tsx');
  assert.match(store, /result\.data\.report/);
  assert.match(store, /reports: \[\],[\s\S]{0,100}?reportsMeta: null/);
  assert.match(page, /routeId \? fetchReportById\(routeId\) : fetchReports\(0\)/);
  assert.match(store, /requestSequence !== reportsRequestSequence/);
  assert.match(controller, /if \(pathname === '\/reports'\) void useReportsStore\.getState\(\)\.fetchReports\(0\)/);
});

test('notes and whale detail failures provide retry and navigation recovery', () => {
  const notes = read('../pages/Notes.tsx');
  const whale = read('../pages/WhaleDetail.tsx');
  assert.match(notes, /setTocRetryNonce/);
  assert.match(notes, /返回研究入口/);
  assert.match(whale, /setRetryNonce/);
  assert.match(whale, /暂无可核验的持仓披露/);
  assert.match(whale, /打开数据修复入口/);
});

test('market anomaly failures identify their source beside the retry action', () => {
	const radar = read('../components/MarketAnomaliesRadar.tsx');
	assert.match(radar, /来源：大盘异动服务 \/api\/market\/anomalies/);
	assert.match(radar, /重试异动数据/);
});
