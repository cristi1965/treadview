import test from 'node:test';
import assert from 'node:assert/strict';
import { buildPortfolioRisk } from './portfolioRisk';
import type { HoldingItem } from '../types/portfolio';

const holding = (patch: Partial<HoldingItem>): HoldingItem => ({
  symbol: 'AAPL',
  name: 'Apple',
  quantity: 10,
  avgCost: 100,
  purchaseDate: new Date('2026-01-01'),
  venue: 'us',
  currency: 'USD',
  ...patch,
});

test('separates currencies and computes concentration only with full quote coverage', () => {
  const result = buildPortfolioRisk([
    holding({ symbol: 'AAPL', currentValue: 1_200 }),
    holding({ symbol: 'MSFT', currentValue: 800 }),
    holding({ symbol: '600519', venue: 'cn', currency: 'CNY', currentValue: 2_000 }),
  ]);

  assert.equal(result.groups.length, 2);
  const usd = result.groups.find((group) => group.currency === 'USD');
  const cny = result.groups.find((group) => group.currency === 'CNY');
  assert.equal(usd?.marketValue, 2_000);
  assert.equal(usd?.top1Weight, 60);
  assert.equal(cny?.marketValue, 2_000);
});

test('does not turn a missing quote into a portfolio loss', () => {
  const result = buildPortfolioRisk([
    holding({ symbol: 'AAPL', currentValue: 1_200 }),
    holding({ symbol: 'MSFT', currentValue: undefined }),
  ]);
  const usd = result.groups[0];
  assert.equal(usd.quotedCount, 1);
  assert.equal(usd.marketValue, undefined);
  assert.equal(usd.profitLoss, undefined);
  assert.equal(usd.top1Weight, undefined);
});

test('flags a position above the configured concentration limit', () => {
  const result = buildPortfolioRisk([
    holding({ symbol: 'AAPL', currentValue: 900 }),
    holding({ symbol: 'MSFT', currentValue: 100 }),
  ], 25);
  assert.equal(result.groups[0].largestSymbol, 'AAPL');
  assert.equal(result.groups[0].concentrationWarning, true);
  assert.equal(result.groups[0].top3Weight, 100);
});

test('includes cash, exposure, sector concentration and fully covered planned loss', () => {
  const result = buildPortfolioRisk([
    holding({ symbol: 'AAPL', sector: 'Technology', stopLoss: 90, currentValue: 1_200 }),
    holding({ symbol: 'MSFT', sector: 'Technology', stopLoss: 95, currentValue: 800 }),
  ], 70, { USD: 1_000 }, 60);
  const usd = result.groups[0];
  assert.equal(usd.totalEquity, 3_000);
  assert.equal(usd.grossExposure, 2_000);
  assert.equal(Math.round(usd.exposurePct || 0), 67);
  assert.equal(usd.largestSector, 'Technology');
  assert.equal(usd.largestSectorWeight, 100);
  assert.equal(usd.sectorConcentrationWarning, true);
  assert.equal(usd.stopLossCoveragePct, 100);
  assert.equal(usd.maxPlannedLoss, 150);
  assert.equal(usd.maxPlannedLossPct, 5);
});

test('blocks valuation-derived risk when a quote or sector/stop plan is missing', () => {
  const result = buildPortfolioRisk([
    holding({ symbol: 'AAPL', sector: 'Technology', stopLoss: 90, currentValue: 1_200 }),
    holding({ symbol: 'MSFT', currentValue: undefined }),
  ], 25, { USD: 500 });
  const usd = result.groups[0];
  assert.equal(usd.totalEquity, undefined);
  assert.equal(usd.grossExposure, undefined);
  assert.equal(usd.exposurePct, undefined);
  assert.equal(usd.largestSectorWeight, undefined);
  assert.equal(usd.maxPlannedLoss, undefined);
  assert.equal(usd.stopCoveredCount, 1);
});
