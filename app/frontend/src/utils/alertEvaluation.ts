export interface AlertRule {
  id: string;
  symbol: string;
  op: '>=' | '<=';
  price: number;
  enabled: boolean;
  lastFiredAt?: number;
}

export interface AlertHit<T extends AlertRule> {
  alert: T;
  price: number;
}

export function evaluatePriceAlerts<T extends AlertRule>(
  alerts: T[],
  quotes: Record<string, { price: number }>,
  now: number,
  cooldownMs: number
): { alerts: T[]; hits: Array<AlertHit<T>> } {
  const hits: Array<AlertHit<T>> = [];
  const next = alerts.map((alert) => {
    if (!alert.enabled) return alert;
    const quote = quotes[alert.symbol] || quotes[alert.symbol.toUpperCase()];
    if (!quote || !(quote.price > 0)) return alert;
    const matched = alert.op === '>=' ? quote.price >= alert.price : quote.price <= alert.price;
    if (!matched || (alert.lastFiredAt && now - alert.lastFiredAt < cooldownMs)) return alert;
    const updated = { ...alert, lastFiredAt: now };
    hits.push({ alert: updated, price: quote.price });
    return updated;
  });
  return { alerts: next, hits };
}
