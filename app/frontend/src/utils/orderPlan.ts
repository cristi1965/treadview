export type TradeStrategy = 'BUY_LOW' | 'BREAKOUT' | 'PROTECT_STOP';
export type OrderMarket = 'US' | 'CN';
export type OrderCurrency = 'USD' | 'CNY';

export interface OrderPlan {
  side: 'BUY' | 'SELL';
  market: OrderMarket;
  currency: OrderCurrency;
  orderType: 'LIMIT' | 'STOP_MARKET';
  tif: 'GTC';
  entry: number | null;
  triggerPrice: number | null;
  protectiveStop: number | null;
  takeProfit: number | null;
  quantity: number;
  maxLoss: number;
  maxRiskAmount: number;
  maxNotionalAmount: number;
  plannedNotional: number;
  riskLimitPassed: boolean;
}

export interface OrderPlanInput {
  strategy: TradeStrategy;
  symbol: string;
  price: number;
  investableCapital: number;
  existingQuantity?: number;
}

export type OrderPlanResult =
  | { ok: true; plan: OrderPlan }
  | { ok: false; error: string };

const roundPrice = (value: number) => Number(value.toFixed(2));
export const inferOrderMarket = (symbol: string): OrderMarket => /^\d{6}$/.test(symbol.trim()) ? 'CN' : 'US';
export const inferOrderCurrency = (symbol: string): OrderCurrency => inferOrderMarket(symbol) === 'CN' ? 'CNY' : 'USD';

export function createOrderPlan(input: OrderPlanInput): OrderPlanResult {
  const { strategy } = input;
  const price = Number(input.price);
  const capital = Number(input.investableCapital);
  if (!(price > 0)) return { ok: false, error: '缺少有效报价' };
  if (!(capital > 0) && strategy !== 'PROTECT_STOP') return { ok: false, error: '对应币种的服务器 Paper 可用资金必须大于 0' };

  const market = inferOrderMarket(input.symbol);
  const currency = inferOrderCurrency(input.symbol);
  const maxRiskAmount = capital * 0.015;
  const maxNotionalAmount = capital * 0.15;
  const entry = strategy === 'BUY_LOW' ? roundPrice(price * 0.982) : null;
  const triggerPrice = strategy === 'BREAKOUT'
    ? roundPrice(price * 1.01)
    : strategy === 'PROTECT_STOP'
      ? roundPrice(price * 0.97)
      : null;
  const plannedExecutionPrice = entry ?? triggerPrice ?? price;
  const protectiveStop = strategy === 'PROTECT_STOP' ? null : roundPrice(plannedExecutionPrice * 0.98);
  const takeProfit = strategy === 'PROTECT_STOP' ? null : roundPrice(plannedExecutionPrice * 1.06);
  const lossPerShare = strategy === 'PROTECT_STOP'
    ? Math.max(price - (triggerPrice ?? price), 0)
    : Math.max(plannedExecutionPrice - (protectiveStop ?? plannedExecutionPrice), 0);
  const riskQuantity = lossPerShare > 0 ? Math.floor(maxRiskAmount / lossPerShare) : 0;
  const notionalQuantity = Math.floor(maxNotionalAmount / plannedExecutionPrice);
  const cappedQuantity = Math.max(0, Math.min(riskQuantity, notionalQuantity));

  let quantity = cappedQuantity;
  if (strategy === 'PROTECT_STOP') {
    const existing = Math.floor(Number(input.existingQuantity) || 0);
    if (existing <= 0) return { ok: false, error: '保护止损需要填写现有持仓数量' };
    quantity = existing;
  }
  if (quantity <= 0) return { ok: false, error: '资金与风险上限不足以生成 1 股订单' };

  const maxLoss = roundPrice(quantity * lossPerShare);
  const plannedNotional = roundPrice(quantity * plannedExecutionPrice);
  if (strategy !== 'PROTECT_STOP' && maxLoss > maxRiskAmount) {
    return { ok: false, error: `计划最大亏损 ${maxLoss.toFixed(2)} 超过 1.5% 风险预算 ${maxRiskAmount.toFixed(2)}` };
  }
  if (strategy !== 'PROTECT_STOP' && plannedNotional > maxNotionalAmount) {
    return { ok: false, error: `计划名义仓位 ${plannedNotional.toFixed(2)} 超过 15% 仓位上限 ${maxNotionalAmount.toFixed(2)}` };
  }
  return {
    ok: true,
    plan: {
      side: strategy === 'PROTECT_STOP' ? 'SELL' : 'BUY',
      market,
      currency,
      orderType: strategy === 'BUY_LOW' ? 'LIMIT' : 'STOP_MARKET',
      tif: 'GTC',
      entry,
      triggerPrice,
      protectiveStop,
      takeProfit,
      quantity,
      maxLoss,
      maxRiskAmount: roundPrice(maxRiskAmount),
      maxNotionalAmount: roundPrice(maxNotionalAmount),
      plannedNotional,
      riskLimitPassed: true,
    },
  };
}
