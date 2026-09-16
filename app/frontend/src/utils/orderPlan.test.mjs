import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

const source = fs.readFileSync(new URL('./orderPlan.ts', import.meta.url), 'utf8');
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
}).outputText;
const module = { exports: {} };
new Function('module', 'exports', compiled)(module, module.exports);
const { createOrderPlan, inferOrderCurrency } = module.exports;

test('isolates paper capital by the symbol market currency', () => {
  assert.equal(inferOrderCurrency('NVDA'), 'USD');
  assert.equal(inferOrderCurrency('300750'), 'CNY');
});

test('buy order quantity obeys risk and notional caps', () => {
  const result = createOrderPlan({ strategy: 'BUY_LOW', symbol: 'NVDA', price: 100, investableCapital: 10_000 });
  assert.equal(result.ok, true);
  assert.equal(result.plan.quantity, 15);
  assert.equal(result.plan.side, 'BUY');
  assert.equal(result.plan.market, 'US');
  assert.ok(result.plan.maxLoss <= 150);
  assert.ok(result.plan.quantity * result.plan.entry <= 1_500);
  assert.equal(result.plan.triggerPrice, null);
  assert.ok(result.plan.protectiveStop < result.plan.entry);
});

test('protect stop requires an existing position but is allowed to reduce oversized risk', () => {
  assert.equal(createOrderPlan({ strategy: 'PROTECT_STOP', symbol: '300750', price: 200, investableCapital: 20_000 }).ok, false);
  const result = createOrderPlan({ strategy: 'PROTECT_STOP', symbol: '300750', price: 200, investableCapital: 20_000, existingQuantity: 60 });
  assert.equal(result.ok, true);
  assert.ok(result.plan.maxLoss > result.plan.maxRiskAmount);
});

test('protect stop remains available when the paper account has no free cash', () => {
  const result = createOrderPlan({ strategy: 'PROTECT_STOP', symbol: 'NVDA', price: 100, investableCapital: 0, existingQuantity: 10 });
  assert.equal(result.ok, true);
  assert.equal(result.plan.side, 'SELL');
  assert.equal(result.plan.maxRiskAmount, 0);
});

test('protect stop produces a paper-ready plan only inside both limits', () => {
  const result = createOrderPlan({ strategy: 'PROTECT_STOP', symbol: '300750', price: 200, investableCapital: 20_000, existingQuantity: 10 });
  assert.equal(result.ok, true);
  assert.equal(result.plan.side, 'SELL');
  assert.equal(result.plan.market, 'CN');
  assert.equal(result.plan.currency, 'CNY');
  assert.equal(result.plan.entry, null);
  assert.equal(result.plan.protectiveStop, null);
  assert.equal(result.plan.takeProfit, null);
  assert.ok(result.plan.triggerPrice < 200);
  assert.equal(result.plan.riskLimitPassed, true);
});

test('breakout uses a buy stop above market with a separate protective stop', () => {
  const result = createOrderPlan({ strategy: 'BREAKOUT', symbol: 'PLTR', price: 100, investableCapital: 10_000 });
  assert.equal(result.ok, true);
  assert.equal(result.plan.side, 'BUY');
  assert.equal(result.plan.orderType, 'STOP_MARKET');
  assert.ok(result.plan.triggerPrice > 100);
  assert.ok(result.plan.protectiveStop < result.plan.triggerPrice);
  assert.equal(result.plan.entry, null);
});

test('invalid quote cannot produce an order', () => {
  const result = createOrderPlan({ strategy: 'BREAKOUT', symbol: 'PLTR', price: 0, investableCapital: 10_000 });
  assert.deepEqual(result, { ok: false, error: '缺少有效报价' });
});
