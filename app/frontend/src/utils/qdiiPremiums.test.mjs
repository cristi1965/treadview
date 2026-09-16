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
const { selectTrustedQDIIItems } = loadModule(
  new URL('./qdiiPremiums.ts', import.meta.url),
  { './dataTrust': dataTrust },
);

const meta = {
  source: 'eastmoney:FundMNFInfo',
  dataTime: '2026-09-05T00:00:00Z',
  stale: false,
  staleReason: '',
  partialErrors: [],
};
const completeItem = {
  code: '513100', name: '纳指ETF', nav: 1, navDate: '2026-09-05', price: 1.08,
  pricePct: 1, premiumPct: 8, level: 'danger', index: 'nasdaq', status: 'complete',
  priceObservedAt: '2026-09-07T02:00:00Z', priceSource: 'eastmoney:FundMNFInfo',
};

test('QDII adapter exposes numeric items only for a wholly live and complete response', () => {
  const live = selectTrustedQDIIItems({ items: [completeItem], count: 1, status: 'live' }, meta);
  assert.equal(live.trust.state, 'live');
  assert.equal(live.items.length, 1);

  for (const responseStatus of ['stale', 'unknown', undefined]) {
    const result = selectTrustedQDIIItems({ items: [completeItem], count: 1, status: responseStatus }, meta);
    assert.notEqual(result.trust.state, 'live');
    assert.deepEqual(result.items, []);
  }

  const countMismatch = selectTrustedQDIIItems({ items: [completeItem], count: 2, status: 'live' }, meta);
  assert.equal(countMismatch.trust.state, 'unavailable');
  assert.deepEqual(countMismatch.items, []);
});

test('QDII adapter fails closed on stale metadata or any incomplete critical item', () => {
  const stale = selectTrustedQDIIItems(
    { items: [completeItem], count: 1, status: 'live' },
    { ...meta, stale: true, staleReason: 'expired' },
  );
  assert.equal(stale.trust.state, 'stale');
  assert.deepEqual(stale.items, []);

  const incomplete = selectTrustedQDIIItems({
    items: [{ ...completeItem, price: null, level: 'unknown', status: 'unknown' }],
    count: 1,
    status: 'live',
  }, meta);
  assert.equal(incomplete.trust.state, 'unavailable');
  assert.deepEqual(incomplete.items, []);

  for (const missingField of ['priceObservedAt', 'priceSource']) {
    const item = { ...completeItem };
    delete item[missingField];
    const missingProvenance = selectTrustedQDIIItems({ items: [item], count: 1, status: 'live' }, meta);
    assert.equal(missingProvenance.trust.state, 'unavailable');
    assert.deepEqual(missingProvenance.items, []);
  }
});
