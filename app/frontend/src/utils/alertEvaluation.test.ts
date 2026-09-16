import test from 'node:test';
import assert from 'node:assert/strict';
import { evaluatePriceAlerts, type AlertRule } from './alertEvaluation';

const row = (patch: Partial<AlertRule>): AlertRule => ({
  id: 'a', symbol: 'NVDA', op: '>=', price: 100, enabled: true, ...patch,
});

test('matches both threshold directions', () => {
  const result = evaluatePriceAlerts(
    [row({ id: 'up' }), row({ id: 'down', op: '<=', price: 90 })],
    { NVDA: { price: 105 } },
    10_000,
    1_000
  );
  assert.deepEqual(result.hits.map((hit) => hit.alert.id), ['up']);
});

test('respects cooldown and ignores missing quotes', () => {
  const result = evaluatePriceAlerts(
    [row({ id: 'cooldown', lastFiredAt: 9_500 }), row({ id: 'missing', symbol: 'AAPL' })],
    { NVDA: { price: 105 } },
    10_000,
    1_000
  );
  assert.equal(result.hits.length, 0);
  assert.equal(result.alerts[0].lastFiredAt, 9_500);
});
