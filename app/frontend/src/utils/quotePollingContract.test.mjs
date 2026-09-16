import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';

const read = (path) => fs.readFileSync(new URL(path, import.meta.url), 'utf8');

test('quote surfaces use tiered shared polling without a global stock refresh timer', () => {
  const scan = read('../pages/Scan.tsx');
  const portfolio = read('../pages/Portfolio.tsx');
  const detail = read('../pages/StockDetail.tsx');
  const tactical = read('../pages/TacticalCommand.tsx');
  const controller = read('../components/SystemDataSyncController.tsx');

  assert.match(scan, /startQuotePolling\(refreshQuotes, 45_000/);
  assert.match(portfolio, /startQuotePolling\(poll, 15_000/);
  assert.match(detail, /startQuotePolling\(tick, 15_000/);
  assert.match(tactical, /startQuotePolling\(poll, 30_000/);
  assert.match(tactical, /quoteFetchGeneration\.current/);
  assert.match(tactical, /setQuoteRefreshVersion\(\(value\) => value \+ 1\)/);
  assert.doesNotMatch(tactical, /fetchTacticalData/);
  assert.doesNotMatch(controller, /getState\(\)\.refreshQuotes\(\)/);
  assert.equal((scan.match(/fetchStocks\(\);/g) || []).length, 1);
});

test('shared quote polling pauses while hidden and prevents overlapping requests', () => {
  const source = read('./liveQuotes.ts');
  assert.match(source, /document\.hidden \|\| inFlight/);
  assert.match(source, /visibilitychange/);
  assert.match(source, /options\.immediate !== false/);
});
