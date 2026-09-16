import assert from 'node:assert/strict';
import test from 'node:test';
import { normalizeCopilotMarketData } from './copilotMarketData';
import { createLatestRequestGate } from './latestRequest';

const liveMeta = {
  source: 'alpaca-iex',
  dataTime: '2026-09-15T08:00:00Z',
  refreshedAt: '2026-09-15T08:00:01Z',
  stale: false,
  staleReason: '',
  refreshable: true,
  partialErrors: [],
};

test('Copilot keeps only market facts with complete live provenance', () => {
  assert.deepEqual(
    normalizeCopilotMarketData({ price: 211, pct: -3.2, meta: liveMeta }),
    { price: 211, pct: -3.2, meta: liveMeta },
  );
  assert.equal(normalizeCopilotMarketData({ price: 211, pct: -3.2 }), undefined);
  assert.equal(normalizeCopilotMarketData({ price: 211, pct: -3.2, meta: { ...liveMeta, refreshedAt: undefined } }), undefined);
  assert.equal(normalizeCopilotMarketData({ price: 211, pct: -3.2, meta: { ...liveMeta, refreshable: undefined } }), undefined);
  assert.equal(normalizeCopilotMarketData({ price: 211, pct: -3.2, meta: { ...liveMeta, stale: true } }), undefined);
  assert.equal(normalizeCopilotMarketData({ price: 211, pct: -3.2, meta: { ...liveMeta, partialErrors: ['missing quote'] } }), undefined);
  assert.equal(normalizeCopilotMarketData({ meta: liveMeta }), undefined);
});

test('latest request gate invalidates every earlier request', () => {
  const gate = createLatestRequestGate();
  const first = gate.begin();
  assert.equal(gate.isCurrent(first), true);
  const second = gate.begin();
  assert.equal(gate.isCurrent(first), false);
  assert.equal(gate.isCurrent(second), true);
  gate.invalidate();
  assert.equal(gate.isCurrent(second), false);
});
