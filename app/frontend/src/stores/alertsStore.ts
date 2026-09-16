import { create } from 'zustand';
import { AssetVenue, inferVenue } from '../types/portfolio';
import { evaluatePriceAlerts } from '../utils/alertEvaluation';

export type AlertOp = '>=' | '<=';

export interface PriceAlert {
  id: string;
  symbol: string;
  venue: AssetVenue;
  op: AlertOp;
  price: number;
  enabled: boolean;
  lastFiredAt?: number;
}

export interface QDIIAlertItem {
  code: string;
  name: string;
  nav: number;
  price: number;
  pricePct: number;
  premiumPct: number;
  level: 'safe' | 'caution' | 'danger' | 'extreme';
}

export interface AlertQuoteContext {
  usable: boolean;
  dataTime?: string;
  source?: string;
  error?: string;
}

export type NotificationStatus = NotificationPermission | 'unsupported' | 'native';

interface AlertsStore {
  alerts: PriceAlert[];
  qdiiAlertsEnabled: boolean;
  setQdiiAlertsEnabled: (enabled: boolean) => void;
  addAlert: (symbol: string, op: AlertOp, price: number, venue?: AssetVenue) => boolean;
  removeAlert: (id: string) => void;
  toggleAlert: (id: string, enabled: boolean) => void;
  notificationStatus: NotificationStatus;
  monitorState: 'idle' | 'checking' | 'live' | 'stale' | 'error';
  lastCheckedAt?: string;
  quoteDataTime?: string;
  quoteSource?: string;
  monitorError?: string;
  requestNotifications: () => Promise<NotificationStatus>;
  sendTestNotification: () => Promise<boolean>;
  setMonitorChecking: () => void;
  checkQuotes: (quotes: Record<string, { price: number }>, context?: AlertQuoteContext) => void;
  setMonitorError: (message: string) => void;
  checkQDIIPremiums: (items: QDIIAlertItem[]) => void;
}

const KEY = 'stockgod_price_alerts';
const QDII_KEY = 'stockgod_qdii_alerts_enabled';
const COOLDOWN_MS = 15 * 60 * 1000; // 15 mins for price alerts
const QDII_COOLDOWN_MS = 10 * 60 * 1000; // 10 mins for QDII premium warning

const load = (): PriceAlert[] => {
  try {
    return JSON.parse(localStorage.getItem(KEY) || '[]');
  } catch {
    return [];
  }
};

const save = (alerts: PriceAlert[]) => {
  try {
    localStorage.setItem(KEY, JSON.stringify(alerts));
  } catch {
    /* ignore */
  }
};

// Web Audio API 警报蜂鸣音
function playBeep(frequency = 880, duration = 0.2, type: OscillatorType = 'sine') {
  try {
    const AudioContextClass = window.AudioContext || (window as any).webkitAudioContext;
    if (!AudioContextClass) return;
    const ctx = new AudioContextClass();
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.type = type;
    osc.frequency.setValueAtTime(frequency, ctx.currentTime);
    gain.gain.setValueAtTime(0.15, ctx.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + duration);
    osc.connect(gain);
    gain.connect(ctx.destination);
    osc.start();
    osc.stop(ctx.currentTime + duration);
  } catch {
    // ignore
  }
}

async function notify(title: string, body: string, isUrgent = false) {
  if (isUrgent) {
    playBeep(987, 0.25, 'triangle');
    setTimeout(() => playBeep(1318, 0.35, 'triangle'), 150);
  }

  try {
    // Wails desktop native notification if present
    const w = window as any;
    if (w?.go?.main?.DesktopApp?.Notify) {
      await w.go.main.DesktopApp.Notify(title, body);
      return;
    }
  } catch {
    /* fall through */
  }

  if (typeof Notification !== 'undefined') {
    if (Notification.permission === 'granted') {
      new Notification(title, {
        body,
        icon: '/logo.png',
        tag: title,
      });
    }
  }
}

function notificationStatus(): NotificationStatus {
  const w = window as any;
  if (w?.go?.main?.DesktopApp?.Notify) return 'native';
  if (typeof Notification === 'undefined') return 'unsupported';
  return Notification.permission;
}

const QDII_FIRED_KEY = 'stockgod_qdii_fired_map';
const loadQdiiFiredMap = (): Record<string, number> => {
  try {
    return JSON.parse(localStorage.getItem(QDII_FIRED_KEY) || '{}');
  } catch {
    return {};
  }
};
const saveQdiiFiredMap = (m: Record<string, number>) => {
  try {
    localStorage.setItem(QDII_FIRED_KEY, JSON.stringify(m));
  } catch {
    /* ignore */
  }
};
const lastQdiiFiredMap: Record<string, number> = loadQdiiFiredMap();

export const useAlertsStore = create<AlertsStore>((set, get) => ({
  alerts: load(),
  qdiiAlertsEnabled: localStorage.getItem(QDII_KEY) !== 'false',
  notificationStatus: notificationStatus(),
  monitorState: 'idle',

  requestNotifications: async () => {
    const current = notificationStatus();
    if (current === 'native' || current === 'unsupported' || current === 'denied') {
      set({ notificationStatus: current });
      return current;
    }
    const next = await Notification.requestPermission();
    set({ notificationStatus: next });
    return next;
  },

  sendTestNotification: async () => {
    const status = notificationStatus();
    set({ notificationStatus: status });
    if (status !== 'granted' && status !== 'native') return false;
    await notify('我不是神 · 通知测试', '价格提醒已启用。仅在应用运行且联网时检查。');
    return true;
  },

  setMonitorChecking: () => set({ monitorState: 'checking', monitorError: undefined }),

  setQdiiAlertsEnabled: (enabled) => {
    localStorage.setItem(QDII_KEY, String(enabled));
    set({ qdiiAlertsEnabled: enabled });
  },

  addAlert: (symbol, op, price, venue) => {
    const sym = symbol.trim().toUpperCase();
    const px = Math.round(price * 10000) / 10000;
    const exists = get().alerts.some(
      (a) => a.symbol === sym && a.op === op && Math.abs(a.price - px) < 1e-6 && a.enabled
    );
    if (exists) return false;
    const row: PriceAlert = {
      id: `${sym}-${op}-${px}-${Date.now()}`,
      symbol: sym,
      venue: venue || inferVenue(sym),
      op,
      price: px,
      enabled: true,
    };
    const next = [...get().alerts, row];
    save(next);
    set({ alerts: next });
    return true;
  },

  removeAlert: (id) => {
    const next = get().alerts.filter((a) => a.id !== id);
    save(next);
    set({ alerts: next });
  },

  toggleAlert: (id, enabled) => {
    const next = get().alerts.map((a) => (a.id === id ? { ...a, enabled } : a));
    save(next);
    set({ alerts: next });
  },

  checkQuotes: (quotes, context = { usable: true }) => {
    const checkedAt = new Date().toISOString();
    if (!context.usable) {
      set({
        monitorState: context.error ? 'error' : 'stale',
        lastCheckedAt: checkedAt,
        quoteDataTime: context.dataTime,
        quoteSource: context.source,
        monitorError: context.error,
      });
      return;
    }
    const now = Date.now();
    const evaluated = evaluatePriceAlerts(get().alerts, quotes, now, COOLDOWN_MS);
    for (const hit of evaluated.hits) {
      void notify(
        `${hit.alert.symbol} 价格提醒`,
        `现价 ${hit.price.toFixed(2)} ${hit.alert.op} ${hit.alert.price}`,
        true
      );
    }
    if (evaluated.hits.length > 0) {
      save(evaluated.alerts);
      set({ alerts: evaluated.alerts });
    }
    set({
      monitorState: 'live',
      lastCheckedAt: checkedAt,
      quoteDataTime: context.dataTime,
      quoteSource: context.source,
      monitorError: context.error,
    });
  },

  setMonitorError: (message) => set({
    monitorState: 'error',
    lastCheckedAt: new Date().toISOString(),
    monitorError: message,
  }),

  checkQDIIPremiums: (items) => {
    if (!get().qdiiAlertsEnabled || !items || items.length === 0) return;
    const now = Date.now();

    for (const item of items) {
      const key = item.code;
      const lastTime = lastQdiiFiredMap[key] || 0;
      if (now - lastTime < QDII_COOLDOWN_MS) continue;

	  // 只描述可核验偏离与风险，不把净值滞后造成的偏离解释成交易指令。
      if (item.premiumPct >= 8.0) {
        lastQdiiFiredMap[key] = now;
        saveQdiiFiredMap(lastQdiiFiredMap);
        void notify(
		  `【QDII 高溢价偏离观察】`,
		  `${item.name} (${item.code}) 场内价相对已公布净值偏离 +${item.premiumPct.toFixed(2)}%。请复核净值日期、报价时间与流动性；该偏离不构成买卖建议。`,
          true
        );
        continue;
      }

	  // 5% 至 8% 的溢价偏离观察。
      if (item.level === 'danger' && item.premiumPct >= 5.0) {
        lastQdiiFiredMap[key] = now;
        saveQdiiFiredMap(lastQdiiFiredMap);
        void notify(
		  `【QDII 溢价偏离观察】`,
		  `${item.name} (${item.code}) 场内价相对已公布净值偏离 +${item.premiumPct.toFixed(2)}%。请结合净值日期、报价时间与流动性复核。`,
          false
        );
        continue;
      }

	  // 负偏离同样只作观察，不假定可套利。
      if (item.premiumPct < 0) {
        lastQdiiFiredMap[key] = now;
        saveQdiiFiredMap(lastQdiiFiredMap);
        void notify(
		  `【QDII 负偏离观察】`,
		  `${item.name} (${item.code}) 场内价相对已公布净值偏离 ${item.premiumPct.toFixed(2)}%。净值存在公布时滞，不代表可套利。`,
          false
        );
        continue;
      }

	  // 接近净值仅描述偏离收窄，不推断风险已经消失。
      if (item.premiumPct >= 0 && item.premiumPct <= 1.5) {
        lastQdiiFiredMap[key] = now;
        saveQdiiFiredMap(lastQdiiFiredMap);
        void notify(
		  `【QDII 偏离收窄观察】`,
		  `${item.name} (${item.code}) 场内价相对已公布净值偏离 +${item.premiumPct.toFixed(2)}%。这只表示当前偏离较小，不代表风险消失。`,
          false
        );
      }
    }
  },
}));
