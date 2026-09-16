import type { HoldingItem } from '../types/portfolio';
import { currencyForVenue, inferVenue, type CashBalances } from '../types/portfolio';

export interface PortfolioRiskGroup {
  currency: 'USD' | 'CNY';
  holdingCount: number;
  quotedCount: number;
  costBasis: number;
  knownMarketValue: number;
  cashBalance: number;
  marketValue?: number;
  totalEquity?: number;
  grossExposure?: number;
  exposurePct?: number;
  profitLoss?: number;
  profitLossPercent?: number;
  top1Weight?: number;
  top3Weight?: number;
  largestSymbol?: string;
  concentrationWarning: boolean;
  sectorCoveredCount: number;
  largestSector?: string;
  largestSectorWeight?: number;
  sectorConcentrationWarning: boolean;
  stopCoveredCount: number;
  stopLossCoveragePct: number;
  maxPlannedLoss?: number;
  maxPlannedLossPct?: number;
}

export interface PortfolioRiskSummary {
  groups: PortfolioRiskGroup[];
  totalHoldingCount: number;
  quotedHoldingCount: number;
}

export function buildPortfolioRisk(
  holdings: HoldingItem[],
  maxPositionPct = 25,
  cashBalances: Partial<CashBalances> = {},
  maxSectorPct = 40
): PortfolioRiskSummary {
  const currencies: Array<'USD' | 'CNY'> = ['USD', 'CNY'];
  const groups = currencies.flatMap((currency) => {
    const rows = holdings.filter((holding) => {
      const venue = holding.venue || inferVenue(holding.symbol);
      return (holding.currency || currencyForVenue(venue)) === currency;
    });
    if (!rows.length) return [];

    const costBasis = rows.reduce((sum, row) => sum + row.avgCost * row.quantity, 0);
    const cashBalance = Math.max(0, Number(cashBalances[currency]) || 0);
    const quotedRows = rows.filter(
      (row) => typeof row.currentValue === 'number' && Number.isFinite(row.currentValue) && row.currentValue >= 0
    );
    const knownMarketValue = quotedRows.reduce((sum, row) => sum + (row.currentValue || 0), 0);
    const complete = quotedRows.length === rows.length;
    const weights = knownMarketValue > 0
      ? quotedRows
          .map((row) => ({ symbol: row.symbol, weight: ((row.currentValue || 0) / knownMarketValue) * 100 }))
          .sort((a, b) => b.weight - a.weight)
      : [];
    const top1Weight = complete ? weights[0]?.weight : undefined;
    const top3Weight = complete ? weights.slice(0, 3).reduce((sum, row) => sum + row.weight, 0) : undefined;
    const profitLoss = complete ? knownMarketValue - costBasis : undefined;
    const totalEquity = complete ? knownMarketValue + cashBalance : undefined;
    const exposurePct = totalEquity && totalEquity > 0 ? (knownMarketValue / totalEquity) * 100 : undefined;
    const sectorRows = rows.filter((row) => Boolean(row.sector?.trim()));
    const sectorValues = new Map<string, number>();
    if (complete && sectorRows.length === rows.length) {
      for (const row of sectorRows) {
        const sector = row.sector!.trim();
        sectorValues.set(sector, (sectorValues.get(sector) || 0) + (row.currentValue || 0));
      }
    }
    const largestSectorEntry = [...sectorValues.entries()].sort((a, b) => b[1] - a[1])[0];
    const largestSectorWeight = complete && knownMarketValue > 0 && largestSectorEntry
      ? (largestSectorEntry[1] / knownMarketValue) * 100
      : undefined;
    const coveredRows = rows.filter((row) => typeof row.stopLoss === 'number' && Number.isFinite(row.stopLoss) && row.stopLoss > 0);
    const coveredCost = coveredRows.reduce((sum, row) => sum + row.avgCost * row.quantity, 0);
    const stopLossCoveragePct = costBasis > 0 ? (coveredCost / costBasis) * 100 : 0;
    const allStopsCovered = coveredRows.length === rows.length;
    const maxPlannedLoss = allStopsCovered
      ? coveredRows.reduce((sum, row) => sum + Math.max(0, row.avgCost - row.stopLoss!) * row.quantity, 0)
      : undefined;
    const maxPlannedLossPct = maxPlannedLoss !== undefined && totalEquity && totalEquity > 0
      ? (maxPlannedLoss / totalEquity) * 100
      : undefined;

    return [{
      currency,
      holdingCount: rows.length,
      quotedCount: quotedRows.length,
      costBasis,
      knownMarketValue,
      cashBalance,
      marketValue: complete ? knownMarketValue : undefined,
      totalEquity,
      grossExposure: complete ? knownMarketValue : undefined,
      exposurePct,
      profitLoss,
      profitLossPercent: profitLoss !== undefined && costBasis > 0 ? (profitLoss / costBasis) * 100 : undefined,
      top1Weight,
      top3Weight,
      largestSymbol: complete ? weights[0]?.symbol : undefined,
      concentrationWarning: typeof top1Weight === 'number' && top1Weight > maxPositionPct,
      sectorCoveredCount: sectorRows.length,
      largestSector: largestSectorEntry?.[0],
      largestSectorWeight,
      sectorConcentrationWarning: typeof largestSectorWeight === 'number' && largestSectorWeight > maxSectorPct,
      stopCoveredCount: coveredRows.length,
      stopLossCoveragePct,
      maxPlannedLoss,
      maxPlannedLossPct,
    }];
  });

  return {
    groups,
    totalHoldingCount: holdings.length,
    quotedHoldingCount: groups.reduce((sum, group) => sum + group.quotedCount, 0),
  };
}
