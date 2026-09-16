import { get, post } from './api';
import type { OrderPlan } from './orderPlan';

export type PaperOrderStatus = 'ACCEPTED' | 'REJECTED' | 'CANCELLED' | 'SIMULATED_FILLED';

export interface PaperOrderStatusEvent {
  status: PaperOrderStatus;
  at: string;
  reason?: string;
}

export interface PaperFillQuote {
  price: number;
  source: string;
  observedAt: string;
  providerURL?: string;
}

export interface PaperFill {
  id: number;
  createdAt: string;
  fillId: string;
  paperOrderId: number;
  sequence: number;
  quantity: number;
  price: number;
  quote: PaperFillQuote;
  slippage: number;
  fee: number;
  realizedPnL: number;
  policyVersion?: string;
  policyReasons?: string[];
}

export interface PaperPolicyCheck {
  policyVersion: string;
  reasons?: string[];
  checkedAt: string;
  passed: boolean;
}

export interface PaperOrder {
  id: number;
  clientOrderId: string;
  researchRunId?: string;
  researchTicker?: string;
  environment: 'PAPER';
  symbol: string;
  side: 'BUY' | 'SELL';
  market: 'US' | 'CN';
  currency: 'USD' | 'CNY';
  orderType: 'LIMIT' | 'STOP_MARKET';
  timeInForce: 'GTC';
  referencePrice: number;
  entry: number | null;
  triggerPrice: number | null;
  protectiveStop: number | null;
  takeProfit: number | null;
  parentOrderId?: number;
  ocoGroupId?: string;
  protectionRemainingQty?: number;
  protectionInitialized?: boolean;
  quantity: number;
  quoteSource: string;
  quoteTime: string;
  riskSnapshot: {
    policyVersion: string;
    policyReasons?: string[];
    investableCapital: number;
    maxLoss: number;
    maxRiskAmount: number;
    plannedNotional: number;
    maxNotionalAmount: number;
    portfolioGrossExposure: number;
    portfolioGrossExposurePct: number;
    maxPortfolioExposurePct: number;
    symbolExposurePct: number;
    maxSymbolExposurePct: number;
    sector: string;
    sectorExposurePct: number;
    maxSectorExposurePct: number;
    openOrderMaxLoss: number;
    openOrderMaxLossPct: number;
    maxOpenOrderLossPct: number;
    averageDailyVolume?: number;
    plannedADVPercent?: number;
    maxOrderADVPercent: number;
    liquiditySource?: string;
    liquidityComplete: boolean;
    portfolioDegraded: boolean;
    portfolioOffendingSymbols?: string[];
    riskWarnings?: string[];
    stopCoveragePct: number;
    minStopCoveragePct: number;
    dailyLossPct: number;
    maxDailyLossPct: number;
    drawdownPct: number;
    maxDrawdownPct: number;
    stressLossPct: number;
    maxStressLossPct: number;
    riskLimitPassed: boolean;
    submissionPolicyCheck?: PaperPolicyCheck;
    fillPolicyCheck?: PaperPolicyCheck;
  };
  status: PaperOrderStatus;
  statusHistory: PaperOrderStatusEvent[];
  rejectionReason?: string;
  fillPrice?: number;
  fillQty?: number;
  remainingQty: number;
  slippage?: number;
  fee?: number;
  realizedPnL: number;
  filledAt?: string;
  fillModel?: string;
  fillQuote?: PaperFillQuote;
  fills?: PaperFill[];
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface PaperAccount {
  currency: 'USD' | 'CNY';
  initialCash: number;
  cash: number;
  reservedCash: number;
  version: number;
}

export interface PaperPosition {
  currency: 'USD' | 'CNY';
  symbol: string;
  quantity: number;
  reservedQuantity: number;
  averageCost: number;
  version: number;
}

export interface PaperAccountEnvelope {
  environment: 'PAPER';
  accounts: PaperAccount[];
  positions: PaperPosition[];
  disclaimer: string;
}

export interface PaperPortfolioSymbolRisk {
  symbol: string;
  sector: string;
  positionQuantity: number;
  reservedQuantity: number;
  stopCoveredQuantity: number;
  stopCoveragePct: number;
  markPrice?: number;
  markSource?: string;
  markTime?: string;
  valuationError?: string;
  positionMarketValue?: number;
  acceptedBuyNotional: number;
  acceptedSellNotional: number;
  grossExposure?: number;
  exposurePct?: number;
  averageCost: number;
  unrealizedPnL?: number;
  averageDailyVolume?: number;
  plannedADVPercent?: number;
  liquiditySource?: string;
  liquidityError?: string;
  liquidityComplete: boolean;
}

export interface PaperPortfolioCurrencyRisk {
  currency: 'USD' | 'CNY';
  initialCash: number;
  availableCash: number;
  reservedCash: number;
  positionMarketValue?: number;
  openBuyNotional: number;
  openSellNotional: number;
  grossExposure?: number;
  totalEquity?: number;
  grossExposurePct?: number;
  largestSymbol?: string;
  largestSymbolExposure?: number;
  largestSymbolPct?: number;
  largestSector?: string;
  largestSectorExposure?: number;
  largestSectorPct?: number;
  positionQuantity: number;
  stopCoveredQuantity: number;
  stopCoveragePct: number;
  openOrderMaxPlannedLoss: number;
  openOrderMaxLossPct?: number;
  valuationComplete: boolean;
  sectorComplete: boolean;
  liquidityComplete: boolean;
  realizedPnLToday: number;
  unrealizedPnL?: number;
  dayStartEquity?: number;
  dayStartObservedAt?: string;
  dailyPnL?: number;
  dailyLossPct?: number;
  dailyRiskComplete: boolean;
  peakEquity?: number;
  peakEquityAt?: string;
  peakDrawdown?: number;
  peakDrawdownPct?: number;
  stressScenarios: Array<{
    name: string;
    kind: 'market' | 'sector-concentration' | 'liquidity';
    status: 'known' | 'unknown';
    assumption?: string;
    unknownReason?: string;
    priceShockPct?: number;
    equityAfter?: number;
    pnl?: number;
    liquidityUsagePct?: number;
  }>;
  degraded: boolean;
  degradationReasons?: string[];
  unknownSymbols: string[];
  symbols: PaperPortfolioSymbolRisk[];
  sectors: Array<{ sector: string; grossExposure?: number; exposurePct?: number }>;
}

export interface PaperPortfolioRisk {
  environment: 'PAPER';
  generatedAt: string;
  projected: boolean;
  riskComplete: boolean;
  degraded: boolean;
  unknownSymbols: string[];
  sources: string[];
  sourceErrors?: string[];
  limits: {
    policyVersion: string;
    maxPortfolioExposurePct: number;
    maxSymbolExposurePct: number;
    maxSectorExposurePct: number;
    maxOpenOrderLossPct: number;
    maxOrderADVPercent: number;
    minStopCoveragePct: number;
    maxDailyLossPct: number;
    maxDrawdownPct: number;
    maxStressLossPct: number;
  };
  groups: PaperPortfolioCurrencyRisk[];
  baseCurrency: {
    currency: 'CNY';
    status: 'known' | 'unknown';
    totalEquity?: number;
    fxSource?: string;
    fxObservedAt?: string;
    reason?: string;
    requiredForCurrentPolicy: boolean;
  };
  disclaimer: string;
}

export interface PaperStopScannerStatus {
  running: boolean;
  interval: string;
  sessionPolicy: string;
  calendarCoverage: string;
  auditRetention: string;
  lastResult?: {
    startedAt: string;
    completedAt: string;
    quoteObservedAt?: string;
    dailyBaselinesCreated: number;
    candidates: number;
    triggered: number;
    skipped: number;
    closedMarket: number;
    errors: number;
    equityCheckpointed: boolean;
    details?: string[];
  };
  recoveryErrors?: string[];
  disclaimer: string;
}

export interface PaperDailyEquityBaseline {
  id: number;
  currency: 'USD' | 'CNY';
  marketDate: string;
  observedAt: string;
  equity: number;
}

export interface PaperDailyEquityBaselineEnvelope {
  environment: 'PAPER';
  currency: 'USD' | 'CNY';
  marketDate: string;
  created: boolean;
  baseline: PaperDailyEquityBaseline;
  disclaimer: string;
}

export interface PaperAuditEvent {
  id: number;
  createdAt: string;
  completedAt?: string;
  requestId: string;
  actor: string;
  action: string;
  target: string;
  status: string;
  httpStatus: number;
  payloadHash?: string;
  outcomeHash?: string;
  hashVersion: number;
  hash: string;
}

export interface PaperAuditList {
  events: PaperAuditEvent[];
  count: number;
  pendingCount: number;
  requiresAttention: boolean;
}

export interface PaperAuditVerification {
  valid: boolean;
  count: number;
  brokenAtId: number;
  reason: string;
  algorithm: string;
  pendingCount: number;
  requiresAttention: boolean;
}

export interface PaperOrderRequest {
  clientOrderId: string;
  researchRunId?: string;
  researchTicker?: string;
  environment: 'PAPER';
  symbol: string;
  side: OrderPlan['side'];
  market: OrderPlan['market'];
  currency: OrderPlan['currency'];
  orderType: OrderPlan['orderType'];
  timeInForce: OrderPlan['tif'];
  referencePrice: number;
  entry: number | null;
  triggerPrice: number | null;
  protectiveStop: number | null;
  takeProfit: number | null;
  quantity: number;
  quoteSource: string;
  quoteTime: string;
  riskSnapshot: Pick<
    PaperOrder['riskSnapshot'],
    'investableCapital' | 'maxLoss' | 'maxRiskAmount' | 'plannedNotional' | 'maxNotionalAmount' | 'riskLimitPassed'
  >;
}

export interface PaperOrderEnvelope {
  order: PaperOrder;
  idempotentReplay: boolean;
  equityCheckpointed?: boolean;
  warning?: string;
  error?: string;
  fillRejected?: boolean;
  rejectionReasons?: string[];
  disclaimer: string;
}

export interface PaperOrderList {
  environment: 'PAPER';
  orders: PaperOrder[];
  total: number;
  hiddenTerminalCount: number;
  truncated: boolean;
  disclaimer: string;
}

export function buildPaperOrderRequest(input: {
  clientOrderId: string;
  symbol: string;
  plan: OrderPlan;
  referencePrice: number;
  investableCapital: number;
  quoteSource: string;
  quoteTime: string;
  researchRunId?: string;
  researchTicker?: string;
}): PaperOrderRequest {
  if (!input.plan.riskLimitPassed) throw new Error('风险校验未通过，禁止创建 paper 订单');
  return {
    clientOrderId: input.clientOrderId,
    researchRunId: input.researchRunId,
    researchTicker: input.researchTicker,
    environment: 'PAPER',
    symbol: input.symbol.trim().toUpperCase(),
    side: input.plan.side,
    market: input.plan.market,
    currency: input.plan.currency,
    orderType: input.plan.orderType,
    timeInForce: input.plan.tif,
    referencePrice: input.referencePrice,
    entry: input.plan.entry,
    triggerPrice: input.plan.triggerPrice,
    protectiveStop: input.plan.protectiveStop,
    takeProfit: input.plan.takeProfit,
    quantity: input.plan.quantity,
    quoteSource: input.quoteSource,
    quoteTime: input.quoteTime,
    riskSnapshot: {
      investableCapital: input.investableCapital,
      maxLoss: input.plan.maxLoss,
      maxRiskAmount: input.plan.maxRiskAmount,
      plannedNotional: input.plan.plannedNotional,
      maxNotionalAmount: input.plan.maxNotionalAmount,
      riskLimitPassed: input.plan.riskLimitPassed,
    },
  };
}

export const validatePaperOrder = (request: PaperOrderRequest) =>
  post<{ environment: 'PAPER'; valid: boolean; degraded: boolean; warnings?: string[]; rejectionReasons: string[]; portfolio?: PaperPortfolioRisk }>('/api/paper-orders/validate', request);

export const submitPaperOrder = (request: PaperOrderRequest) =>
  post<PaperOrderEnvelope>('/api/paper-orders', request);

export const listPaperOrders = () => get<PaperOrderList>('/api/paper-orders');

export const getPaperAccount = () => get<PaperAccountEnvelope>('/api/paper-orders/account');

export const getPaperPortfolioRisk = () => get<PaperPortfolioRisk>('/api/paper-orders/portfolio-risk');

export const getPaperStopScannerStatus = () => get<PaperStopScannerStatus>('/api/paper-orders/scheduler');

export const establishPaperDailyEquityBaseline = (currency: 'USD' | 'CNY') =>
  post<PaperDailyEquityBaselineEnvelope>(`/api/paper-orders/daily-baseline?currency=${currency}`);

export const listPaperAuditEvents = () => get<PaperAuditList>('/api/admin/audit?limit=500');

export const verifyPaperAuditChain = () => get<PaperAuditVerification>('/api/admin/audit/verify');

export function paperSchedulerPollDelay(interval: string | undefined): number {
  const match = interval?.trim().match(/^(\d+(?:\.\d+)?)(ms|s|m)$/i);
  if (!match) return 5_000;
  const unitScale = { ms: 1, s: 1_000, m: 60_000 }[match[2].toLowerCase() as 'ms' | 's' | 'm'];
  return Math.min(60_000, Math.max(1_000, Number(match[1]) * unitScale));
}

export const cancelPaperOrder = (clientOrderId: string) =>
  post<PaperOrderEnvelope>(`/api/paper-orders/${encodeURIComponent(clientOrderId)}/cancel`);

export const simulatePaperFill = (clientOrderId: string, fill: { fillId: string; quantity: number }) =>
  post<PaperOrderEnvelope>(`/api/paper-orders/${encodeURIComponent(clientOrderId)}/simulated-fill`, fill);
