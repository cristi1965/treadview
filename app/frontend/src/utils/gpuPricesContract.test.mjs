import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const srcRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const read = (...parts) => fs.readFileSync(path.join(srcRoot, ...parts), 'utf8');

test('GPU prices is reachable from route, desktop/mobile shared navigation, command palette, and title', () => {
  const routes = read('AppRoutes.tsx');
  const pages = read('pages', 'index.ts');
  const shell = read('components', 'layout', 'StockGodShell.tsx');
  assert.match(pages, /export \{ GPUPrices \} from '\.\/GPUPrices'/);
  assert.match(routes, /path="\/gpu-prices" element=\{<GPUPrices \/>\}/);
  assert.match(shell, /path: '\/gpu-prices', label: 'GPU 租金'/);
  assert.match(shell, /label: 'GPU 算力租金'.*path: '\/gpu-prices'/);
  assert.match(shell, /path === '\/gpu-prices'/);
  assert.match(shell, /\{navItems\.map\(\(item\) =>/);
  assert.match(shell, /navItems\.filter\(\(i\) => i\.group !== 'cockpit'\)/);
});

test('GPU API endpoints are centralized in one hook and history is backend dated data', () => {
  const hook = read('hooks', 'useGPUPrices.ts');
  const page = read('pages', 'GPUPrices.tsx');
  assert.match(hook, /getWithMeta<GPUPricesResponse>\('\/api\/gpu-prices'\)/);
  assert.match(hook, /getWithMeta<GPUPriceReferenceResponse>\('\/api\/gpu-prices\/reference'\)/);
  assert.match(hook, /getWithMeta<GPUPriceHistoryResponse>\(`\/api\/gpu-prices\/history\?/);
  assert.match(hook, /authorizedFetch\(apiUrl\('\/api\/gpu-prices\/refresh'\)/);
  assert.match(page, /仅消费后端 SQLite 中已落库的 dated history/);
  assert.doesNotMatch(page, /setInterval|startQuotePolling|fetchQuoteResult/);
});

test('GPU historical reference remains separate from dynamic comparisons and excludes a fixed Vast price', () => {
  const page = read('pages', 'GPUPrices.tsx');
  const adapter = read('utils', 'gpuPrices.ts');
  assert.match(page, /官方核验参考价/);
  assert.match(page, /非实时/);
  assert.match(page, /不参与动态最低价、价差或历史图计算/);
  assert.match(page, /Vast\.ai 不含固定参考价/);
  assert.match(page, /<ReferencePrices reference=\{reference\} \/>/);
  assert.ok(page.indexOf('<ReferencePrices reference={reference} />') < page.indexOf("current.trust.state !== 'live'"));
  assert.match(adapter, /provider === 'vast'/);
  assert.match(adapter, /modeIsReference/);
});

test('GPU page is fail closed and switches desktop table to narrow-screen records', () => {
  const page = read('pages', 'GPUPrices.tsx');
  assert.match(page, /current\.trust\.state !== 'live'/);
  assert.match(page, /不显示最低价、价差和历史曲线/);
  assert.match(page, /history\.trust\.state === 'live'/);
  assert.match(page, /hidden overflow-hidden border border-line md:block/);
  assert.match(page, /overflow-hidden border border-line md:hidden/);
  for (const label of ['筛选厂商', '筛选 GPU 型号', '筛选计费模式', '历史天数']) assert.ok(page.includes(label));
});

test('GPU acceptance fixture is labeled test-only instead of verified provider data', () => {
  const page = read('pages', 'GPUPrices.tsx');
	const audit = fs.readFileSync(path.join(srcRoot, '..', 'scripts', 'gpu-runtime-audit.mjs'), 'utf8');
  assert.match(page, /current\.testOnly/);
  assert.match(page, /验收夹具/);
  assert.match(page, /测试专用数据/);
	assert.match(audit, /initial\.testOnly === true/);
	assert.match(audit, /initial\.dataMode === 'acceptance-fixture'/);
	assert.match(audit, /测试专用数据/);
});

test('GPU upstream refresh is gated by provider configuration and response metadata, not only admin session', () => {
  const page = read('pages', 'GPUPrices.tsx');
  const adapter = read('utils', 'gpuPrices.ts');
  assert.match(page, /getGPUUpstreamRefreshAvailability\(/);
  assert.match(page, /disabled=\{!upstreamRefresh\.enabled \|\| current\.upstreamRefreshing\}/);
  assert.match(page, /aria-describedby="gpu-upstream-refresh-reason"/);
  assert.match(adapter, /allProvidersUnconfigured/);
  assert.match(adapter, /meta\?\.refreshable === false/);
  assert.match(adapter, /先在后端配置 provider 凭证/);
  assert.match(adapter, /官方核验参考价仍可访问/);
});

test('GPU history has distinct empty and stale states while both remain fail closed', () => {
  const page = read('pages', 'GPUPrices.tsx');
  const adapter = read('utils', 'gpuPrices.ts');
  assert.match(adapter, /authenticatedHistoryIsEmpty/);
  assert.match(page, /暂无历史观测/);
  assert.match(page, /所选条件暂无已落库历史观测/);
  assert.match(page, /历史数据已过期，曲线未展示/);
  assert.match(page, /history\.trust\.state === 'live' && historyChartData\.length/);
});
