import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

const loadModule = (url, dependencies = {}) => {
  const source = fs.readFileSync(url, 'utf8');
  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
  }).outputText;
  const module = { exports: {} };
  const localRequire = (id) => {
    if (id in dependencies) return dependencies[id];
    throw new Error(`Unexpected dependency: ${id}`);
  };
  new Function('module', 'exports', 'require', compiled)(module, module.exports, localRequire);
  return module.exports;
};

const dataTrust = loadModule(new URL('./dataTrust.ts', import.meta.url));
const {
  adaptGPUPricesResponse,
  adaptGPUPriceHistoryResponse,
  adaptGPUPriceReferenceResponse,
  getGPUUpstreamRefreshAvailability,
} = loadModule(
  new URL('./gpuPrices.ts', import.meta.url),
  { './dataTrust': dataTrust },
);

const meta = {
  source: 'official-provider-apis', dataTime: '2026-09-09T02:00:00Z', refreshedAt: '2026-09-09T02:01:00Z',
  stale: false, staleReason: '', refreshable: true, partialErrors: [],
};

const quote = {
  provider: 'modal', gpu_model: 'H100', product: 'serverless', billing_mode: 'per_second', gpu_count: 1,
  currency: 'USD', raw_price: 0.001097, raw_unit: 'gpu_second', price_usd_per_gpu_hour: 3.9492,
  source_url: 'https://modal.com/pricing', observed_at: '2026-09-09T02:00:00Z',
};

test('GPU current-price adapter accepts camel or snake case without losing raw billing units', () => {
  const result = adaptGPUPricesResponse({ quotes: [quote], count: 1, status: 'live' }, meta);
  assert.equal(result.trust.state, 'live');
  assert.equal(result.quotes[0].rawUnit, 'gpu_second');
  assert.equal(result.quotes[0].rawPrice, 0.001097);
  assert.equal(result.quotes[0].priceUsdPerGpuHour, 3.9492);
});

test('GPU adapters preserve acceptance fixture provenance for explicit test-only UI', () => {
  const fixtureMeta = { ...meta, source: 'acceptance-fixture' };
  const current = adaptGPUPricesResponse({
    quotes: [quote], count: 1, status: 'live', dataMode: 'acceptance-fixture', testOnly: true,
  }, fixtureMeta);
  assert.equal(current.testOnly, true);
  assert.equal(current.dataMode, 'acceptance-fixture');

  const history = adaptGPUPriceHistoryResponse({
    items: [quote], count: 1, dataMode: 'acceptance-fixture', testOnly: true,
  }, fixtureMeta);
  assert.equal(history.testOnly, true);
  assert.equal(history.dataMode, 'acceptance-fixture');
});

test('GPU current-price adapter fails closed for invalid prices, times, stale data, and count mismatch', () => {
  for (const invalid of [
    { ...quote, raw_price: 0 },
    { ...quote, price_usd_per_gpu_hour: null },
    { ...quote, observed_at: 'unknown' },
    { ...quote, source_url: '' },
    { ...quote, source_url: 'javascript:alert(1)' },
  ]) {
    const result = adaptGPUPricesResponse({ quotes: [invalid], count: 1, status: 'live' }, meta);
    assert.equal(result.trust.state, 'unavailable');
    assert.deepEqual(result.quotes, []);
  }
  assert.deepEqual(adaptGPUPricesResponse({ quotes: [quote], count: 2, status: 'live' }, meta).quotes, []);
  const stale = adaptGPUPricesResponse({ quotes: [quote], count: 1, status: 'live' }, { ...meta, stale: true });
  assert.equal(stale.trust.state, 'stale');
  assert.deepEqual(stale.quotes, []);
});

test('GPU provider diagnostics survive when no trusted quotes are available', () => {
  const result = adaptGPUPricesResponse({
    quotes: [], count: 0, status: 'unavailable',
    providers: { runpod: { status: 'unconfigured', configured: false, message: 'credential missing' } },
  }, meta);
  assert.deepEqual(result.quotes, []);
  assert.equal(result.providers[0].provider, 'runpod');
  assert.equal(result.providers[0].status, 'unconfigured');
  assert.equal(result.providers[0].configured, false);
});

test('GPU upstream refresh requires provider configuration and backend refreshability before admin authorization', () => {
  const unconfigured = ['runpod', 'modal', 'lambda', 'vast'].map((provider) => ({
    provider, status: 'unconfigured', configured: false,
  }));
  const missingCredentials = getGPUUpstreamRefreshAvailability(unconfigured, meta, true, true);
  assert.equal(missingCredentials.enabled, false);
  assert.match(missingCredentials.reason, /先在后端配置 provider 凭证/);
  assert.match(missingCredentials.reason, /官方核验参考价仍可访问/);

  const configured = [{ provider: 'runpod', status: 'live', configured: true }];
  const notRefreshable = getGPUUpstreamRefreshAvailability(configured, { ...meta, refreshable: false }, true);
  assert.equal(notRefreshable.enabled, false);
  assert.match(notRefreshable.reason, /后端标记当前数据不可刷新/);
  assert.equal(getGPUUpstreamRefreshAvailability(configured, meta, false).enabled, false);
  assert.equal(getGPUUpstreamRefreshAvailability(configured, meta, true).enabled, true);
});

test('GPU history adapter only exposes complete dated backend observations', () => {
  const point = {
    provider: 'runpod', gpuModel: 'H100', billingMode: 'instance', priceUsdPerGpuHour: 2.69,
    observedAt: '2026-09-08T02:00:00Z', sourceUrl: 'https://www.runpod.io/pricing',
  };
  const live = adaptGPUPriceHistoryResponse({ points: [point], count: 1, status: 'live' }, meta);
  assert.equal(live.trust.state, 'live');
  assert.equal(live.points.length, 1);
  for (const invalid of [{ ...point, priceUsdPerGpuHour: 0 }, { ...point, observedAt: 'today' }]) {
    assert.deepEqual(adaptGPUPriceHistoryResponse({ points: [invalid], count: 1 }, meta).points, []);
  }
  assert.deepEqual(adaptGPUPriceHistoryResponse({ points: [point], count: 2 }, meta).points, []);

  const empty = adaptGPUPriceHistoryResponse(
    { items: [], count: 0 },
    { ...meta, dataTime: 'unknown', stale: true, staleReason: 'no authenticated observations', refreshable: false },
  );
  assert.equal(empty.empty, true);
  assert.equal(empty.trust.state, 'unavailable');
  assert.deepEqual(empty.points, []);

  const stale = adaptGPUPriceHistoryResponse(
    { points: [point], count: 1, status: 'live' },
    { ...meta, stale: true, staleReason: 'authenticated observations exceed the freshness window', refreshable: false },
  );
  assert.equal(stale.empty, false);
  assert.equal(stale.trust.state, 'stale');
  assert.deepEqual(stale.points, []);
});

const referenceProvider = (provider, hasFixedReference = true) => ({
  provider, status: hasFixedReference ? 'verified' : 'market-variable',
  sourceUrl: `https://example.com/${provider}`, hasFixedReference,
});
const referenceItem = {
  provider: 'runpod', gpuModel: 'H100 SXM', product: 'secure-cloud', billingMode: 'on-demand', gpuCount: 1,
  currency: 'USD', rawPrice: 3.49, rawUnit: 'gpu_hour', priceUsdPerGpuHour: 3.49,
};
const referenceResponse = {
  status: 'historical', dataMode: 'historical', stale: true, source: 'official-public-pricing-pages',
  verifiedAt: '2026-09-09T00:00:00Z', count: 1, disclaimer: 'Historical reference only.',
  providers: [referenceProvider('runpod'), referenceProvider('modal'), referenceProvider('lambda'), referenceProvider('vast', false)],
  items: [referenceItem],
};

test('GPU reference adapter preserves verified historical data despite stale response metadata', () => {
  const result = adaptGPUPriceReferenceResponse(referenceResponse, { ...meta, stale: true, staleReason: 'historical reference' });
  assert.equal(result.available, true);
  assert.equal(result.items.length, 1);
  assert.equal(result.items[0].verifiedAt, referenceResponse.verifiedAt);
  assert.equal(result.items[0].sourceUrl, 'https://example.com/runpod');
  assert.equal(result.providers.find((item) => item.provider === 'vast').hasFixedReference, false);
});

test('GPU reference adapter fails closed on malformed records, dates, counts, or a fixed Vast price', () => {
  for (const response of [
    { ...referenceResponse, verifiedAt: 'unknown' },
    { ...referenceResponse, count: 2 },
    { ...referenceResponse, items: [{ ...referenceItem, rawPrice: 0 }] },
    { ...referenceResponse, items: [{ ...referenceItem, provider: 'vast' }] },
  ]) {
    const result = adaptGPUPriceReferenceResponse(response, { ...meta, stale: true });
    assert.equal(result.available, false);
    assert.deepEqual(result.items, []);
  }
});
