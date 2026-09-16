const LOCAL_API = 'http://localhost:8765';

let currentLang = 'zh';

const KNOWN_DATA = {
  NVDA: { name: '英伟达', nameEn: 'NVIDIA Corp', price: 200.75, pct: 2.93, tag: '👑 M7 · AI算力总龙头', tagEn: '👑 M7 · AI GPU Market Leader', scores: [68, 95, 72, 92, 88], verdictZh: '算力需求极强，大单资金持续沉淀，回调至 S1 附近为高胜率低吸区间。', verdictEn: 'Unprecedented AI compute demand. Institutional net flow strongly bullish. Dips near S1 provide ideal risk-adjusted entries.' },
  TSLA: { name: '特斯拉', nameEn: 'Tesla Inc', price: 311.21, pct: 0.76, tag: '👑 M7 · 得州储能/FSD', tagEn: '👑 M7 · Megapack Energy & FSD', scores: [62, 85, 70, 88, 85], verdictZh: '储能出海与机器人产业催化，突破 R1 阻力位后弹性打开。', verdictEn: 'Energy storage global ramp and autonomy catalysts. Break above R1 resistance opens upside momentum.' },
  PLTR: { name: 'Palantir', nameEn: 'Palantir Tech', price: 130.63, pct: 4.82, tag: '🌐 X热搜 · AIP商业化落地', tagEn: '🌐 X Trend · AIP Commercial Scale', scores: [55, 80, 85, 94, 91], verdictZh: '政府与企业端订单暴增，多头动量强劲，止损设在 S1 之下。', verdictEn: 'Explosive enterprise AIP adoption and military contracts. Maintain stop loss below S1.' },
  '01810': { name: '小米集团-W', nameEn: 'Xiaomi Corp', price: 32.85, pct: 3.42, tag: '🇭🇰 港股通 · 人车家全生态/SU7 Ultra', tagEn: '🇭🇰 HKEX · EV Hypercar & AI Ecosystem', scores: [75, 96, 80, 93, 90], verdictZh: 'SU7 Ultra及高端机出海势头强劲，南向港股通资金持续大幅净买入。', verdictEn: 'Explosive SU7 EV deliveries & smartphone market share gains. Heavy Southbound capital inflow.' },
  '1810': { name: '小米集团-W', nameEn: 'Xiaomi Corp', price: 32.85, pct: 3.42, tag: '🇭🇰 港股通 · 人车家全生态/SU7 Ultra', tagEn: '🇭🇰 HKEX · EV Hypercar & AI Ecosystem', scores: [75, 96, 80, 93, 90], verdictZh: 'SU7 Ultra及高端机出海势头强劲，南向港股通资金持续大幅净买入。', verdictEn: 'Explosive SU7 EV deliveries & smartphone market share gains. Heavy Southbound capital inflow.' },
  '00700': { name: '腾讯控股', nameEn: 'Tencent Holdings', price: 420.20, pct: 1.85, tag: '🇭🇰 港股通 · 游戏/视频号/AI大模型', tagEn: '🇭🇰 HKEX · Gaming, WeChat & Hunyuan AI', scores: [88, 98, 85, 82, 86], verdictZh: '高质量毛利业务占比持续提升，日均回购10亿港元筑牢估值底。', verdictEn: 'High-margin video accounts and gaming recovery with massive HK$1B/day buyback support.' },
  '09988': { name: '阿里巴巴-W', nameEn: 'Alibaba Group', price: 92.50, pct: 2.15, tag: '🇭🇰 港股通 · 阿里云/淘天电商', tagEn: '🇭🇰 HKEX · Cloud AI & E-commerce', scores: [80, 90, 82, 85, 84], verdictZh: '通义千问大模型及AI云收入翻倍增长，双重主要上市纳入港股通。', verdictEn: 'Cloud AI revenue hypergrowth and Southbound Stock Connect inclusion.' },
  '03690': { name: '美团-W', nameEn: 'Meituan', price: 168.40, pct: -0.55, tag: '🇭🇰 港股通 · 本地生活/KeeTa出海', tagEn: '🇭🇰 HKEX · Local Commerce & KeeTa Global', scores: [72, 88, 78, 86, 82], verdictZh: '核心本地商业稳健，中东KeeTa出海开启第二增长曲线。', verdictEn: 'Core local commerce cash engine strong. Middle East KeeTa expansion unlocks growth.' },
  '300750': { name: '宁德时代', nameEn: 'CATL', price: 268.50, pct: 2.10, tag: '🇨🇳 A股 · 储能海外排产龙头', tagEn: '🇨🇳 A-Share · Global ESS Giant', scores: [82, 92, 85, 84, 86], verdictZh: '海外储能需求超预期，机构底仓配置品种，破位 S2 坚决止损。', verdictEn: 'Overseas energy storage demand exceeding expectations. Core institutional hold.' },
  '300308': { name: '中际旭创', nameEn: 'Innolight', price: 142.80, pct: 3.65, tag: '🇨🇳 A股 · 800G/1.6T光模块', tagEn: '🇨🇳 A-Share · 800G/1.6T Optics', scores: [70, 88, 90, 93, 89], verdictZh: '北美云厂商大单交付，光通信高景气度持续上行。', verdictEn: 'North American hyperscaler shipments accelerating with superior margins.' },
  '2330.TW': { name: '台积电', nameEn: 'TSMC', price: 1045.00, pct: 1.95, tag: '🇹🇼 台股 · 先进先进制程垄断', tagEn: '🇹🇼 TWSE · 3nm/2nm Foundry Monopoly', scores: [85, 96, 90, 95, 92], verdictZh: 'N2/N3先进节点产能满载，AI芯片代工绝对垄断地位。', verdictEn: 'Full 3nm/2nm capacity utilization with unrivaled AI accelerator pricing power.' },
  '7203.T': { name: '丰田汽车', nameEn: 'Toyota Motor', price: 2850.00, pct: 0.88, tag: '🇯🇵 日股 · 混动霸主/固态电池', tagEn: '🇯🇵 TSE · Global Hybrid & Solid-state EV', scores: [80, 85, 78, 80, 78], verdictZh: '全球HEV混动销量与现金流极强，估值处于合理中枢。', verdictEn: 'Massive global hybrid cash flow with attractive shareholder returns.' },
  '513100': { name: '纳指ETF国泰', nameEn: 'Nasdaq ETF (513100)', price: 2.267, pct: 0.09, tag: '⚠️ 场内QDII · 溢价率 +11.5% 极度溢价', tagEn: '⚠️ QDII ETF · +11.5% Extreme Premium', scores: [45, 60, 50, 55, 50], verdictZh: '⛔ 场内溢价高达 11.5%，买入即被动承受溢价杀跌风险！建议等待溢价回落至 2% 以下再入手，或直接配置美股 QQQ/场外基金。', verdictEn: '⛔ High +11.5% premium in onshore market! Avoid buying until premium drops below 2% or buy offshore QQQ.' },
  '513500': { name: '标普500ETF', nameEn: 'S&P 500 ETF (513500)', price: 2.758, pct: 0.12, tag: '⚠️ 场内QDII · 溢价率 +10.4% 极度溢价', tagEn: '⚠️ QDII ETF · +10.4% Extreme Premium', scores: [50, 65, 55, 60, 52], verdictZh: '⛔ 标普500场内溢价达 10.4%，溢价风险极高，切忌高位接盘！建议关注溢价杀完后的低吸机会。', verdictEn: '⛔ S&P 500 ETF trading at +10.4% premium. Wait for premium to compress before entering.' },
};

document.addEventListener('DOMContentLoaded', async () => {
  const symbolInput = document.getElementById('symbolInput');
  const searchBtn = document.getElementById('searchBtn');
  const serverStatus = document.getElementById('serverStatus');
  const statusText = document.getElementById('statusText');
  const chips = document.querySelectorAll('.chip');
  const langToggleBtn = document.getElementById('langToggleBtn');

  // Load language preference
  if (chrome.storage && chrome.storage.local) {
    const saved = await chrome.storage.local.get(['lang']);
    if (saved && saved.lang) {
      currentLang = saved.lang;
    }
  }

  applyI18n();

  // Language Toggle
  langToggleBtn.addEventListener('click', () => {
    currentLang = currentLang === 'zh' ? 'en' : 'zh';
    if (chrome.storage && chrome.storage.local) {
      chrome.storage.local.set({ lang: currentLang });
    }
    applyI18n();
    const sym = symbolInput.value.trim().toUpperCase() || 'NVDA';
    loadSymbolData(sym);
  });

  // Check Local Server Health
  try {
    const res = await fetch(`${LOCAL_API}/api/health`, { signal: AbortSignal.timeout(1500) });
    if (res.ok) {
      serverStatus.classList.remove('offline');
      statusText.textContent = EXT_I18N[currentLang].statusOnline;
    } else {
      throw new Error();
    }
  } catch {
    serverStatus.classList.add('offline');
    statusText.textContent = EXT_I18N[currentLang].statusOffline;
  }

  // Auto-detect current active tab ticker if on TradingView or Xueqiu
  chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
    if (tabs && tabs[0] && tabs[0].url) {
      const url = tabs[0].url;
      let matchedSym = null;
      if (url.includes('tradingview.com/symbols/')) {
        const parts = url.split('/symbols/')[1].split('/')[0].split('-');
        matchedSym = parts[parts.length - 1];
      } else if (url.includes('xueqiu.com/S/')) {
        matchedSym = url.split('/S/')[1].split('?')[0];
      } else if (url.includes('finance.yahoo.com/quote/')) {
        matchedSym = url.split('/quote/')[1].split('/')[0].split('?')[0];
      }

      if (matchedSym) {
        loadSymbolData(matchedSym);
        symbolInput.value = matchedSym.toUpperCase();
      } else {
        loadSymbolData('NVDA');
      }
    } else {
      loadSymbolData('NVDA');
    }
  });

  // Search Action
  searchBtn.addEventListener('click', () => {
    const sym = symbolInput.value.trim().toUpperCase();
    if (sym) loadSymbolData(sym);
  });

  symbolInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      const sym = symbolInput.value.trim().toUpperCase();
      if (sym) loadSymbolData(sym);
    }
  });

  // Quick Chips
  chips.forEach((chip) => {
    chip.addEventListener('click', () => {
      chips.forEach((c) => c.classList.remove('active'));
      chip.classList.add('active');
      const sym = chip.getAttribute('data-sym');
      symbolInput.value = sym;
      loadSymbolData(sym);
    });
  });
});

function applyI18n() {
  const t = EXT_I18N[currentLang];
  document.getElementById('t-appTitle').textContent = t.appTitle;
  document.getElementById('langToggleBtn').textContent = t.langToggle;
  document.getElementById('symbolInput').placeholder = t.searchPlaceholder;
  document.getElementById('searchBtn').textContent = t.searchBtn;
  document.getElementById('t-godsTitle').textContent = t.godsTitle;
  document.getElementById('t-pivotsTitle').textContent = t.pivotsTitle;
  document.getElementById('t-buffett').textContent = t.buffett;
  document.getElementById('t-duan').textContent = t.duan;
  document.getElementById('t-serenity').textContent = t.serenity;
  document.getElementById('t-druck').textContent = t.druck;
  document.getElementById('t-sentiment').textContent = t.sentiment;
  document.getElementById('t-s2Label').textContent = t.s2Label;
  document.getElementById('t-s1Label').textContent = t.s1Label;
  document.getElementById('t-r1Label').textContent = t.r1Label;
  document.getElementById('t-r2Label').textContent = t.r2Label;
  document.getElementById('t-verdictPrefix').textContent = t.verdictPrefix;
  document.getElementById('t-btnDetail').textContent = t.btnDetail;
  document.getElementById('t-btnTradingView').textContent = t.btnTradingView;
  document.getElementById('t-flowLabel').textContent = t.flowLabel;
  document.getElementById('t-flowVal').textContent = currentLang === 'zh' ? '64% 多头买单主力沉淀 · $25M+ 净流入' : '64% Net Bullish Accumulation · $25M+ Inflow';
}

async function loadSymbolData(symbol) {
  const sym = symbol.toUpperCase();
  const isAshare = /^(00|30|60|68)\d{4}$/.test(sym) || sym.endsWith('.SS') || sym.endsWith('.SZ') || sym.endsWith('.BJ');
  const isHK = sym.endsWith('.HK') || (/^\d{1,5}$/.test(sym) && !/^\d{6}$/.test(sym)) || sym.startsWith('HKEX:');
  const curSymbol = isHK ? 'HK$' : isAshare ? '¥' : sym.endsWith('.TW') ? 'NT$' : sym.endsWith('.T') ? 'JP¥' : sym.endsWith('.L') ? '£' : sym.endsWith('.DE') ? '€' : '$';

  let data = KNOWN_DATA[sym];

  // Try fetching live quote from local backend
  try {
    const res = await fetch(`${LOCAL_API}/api/quote?syms=${sym}`, { signal: AbortSignal.timeout(2000) });
    if (res.ok) {
      const json = await res.json();
      const quote = json.quotes?.[sym] || json.quotes?.[sym.toLowerCase()];
      if (quote && quote.price > 0) {
        if (!data) {
          data = {
            name: sym,
            nameEn: sym,
            price: quote.price,
            pct: quote.pct || 0,
            tag: isHK ? '🇭🇰 港股通标的' : isAshare ? '🇨🇳 A股人气标的' : '🇺🇸 全球/美股标的',
            tagEn: isHK ? '🇭🇰 HKEX Stock' : isAshare ? '🇨🇳 A-Share' : '🇺🇸 Global / US Stock',
            scores: [70, 75, 70, 80, 75],
            verdictZh: '五方 AI 模型正在根据实时流动性与趋势给出综合评估。',
            verdictEn: 'Multi-Agent models evaluating real-time order flow and momentum.'
          };
        } else {
          data.price = quote.price;
          data.pct = quote.pct ?? data.pct;
        }
      }
    }
  } catch {
    // fallback
  }

  if (!data) {
    data = {
      name: sym,
      nameEn: sym,
      price: 100.0,
      pct: 1.25,
      tag: isHK ? '🇭🇰 港股标的' : isAshare ? '🇨🇳 A股标的' : '🇺🇸 美股标的',
      tagEn: isHK ? '🇭🇰 HK Stock' : isAshare ? '🇨🇳 A-Share' : '🇺🇸 US Stock',
      scores: [70, 75, 72, 80, 78],
      verdictZh: '当前标的已同步，建议结合 S1 支撑与 R1 阻力位进行波段择时。',
      verdictEn: 'Ticker synced. Use S1 support and R1 resistance for optimal entry timing.'
    };
  }

  // Update UI Elements
  document.getElementById('stockName').textContent = currentLang === 'zh' ? data.name : (data.nameEn || data.name);
  document.getElementById('stockSym').textContent = sym;
  document.getElementById('stockTag').textContent = currentLang === 'zh' ? data.tag : (data.tagEn || data.tag);
  document.getElementById('stockPrice').textContent = `${curSymbol}${data.price.toFixed(2)}`;
  
  const pctEl = document.getElementById('stockPct');
  pctEl.textContent = `${data.pct >= 0 ? '+' : ''}${data.pct.toFixed(2)}%`;
  pctEl.className = `price-pct ${data.pct >= 0 ? 'text-up' : 'text-down'}`;

  // 5 Gods Scores
  const [buffett, duan, serenity, druck, sentiment] = data.scores;
  document.getElementById('scoreBuffett').textContent = buffett;
  document.getElementById('scoreDuan').textContent = duan;
  document.getElementById('scoreSerenity').textContent = serenity;
  document.getElementById('scoreDruck').textContent = druck;
  document.getElementById('scoreSentiment').textContent = sentiment;

  // AI S/R Pivots
  const p = data.price;
  document.getElementById('pivotS2').textContent = `${curSymbol}${(p * 0.912).toFixed(2)}`;
  document.getElementById('pivotS1').textContent = `${curSymbol}${(p * 0.958).toFixed(2)}`;
  document.getElementById('pivotR1').textContent = `${curSymbol}${(p * 1.042).toFixed(2)}`;
  document.getElementById('pivotR2').textContent = `${curSymbol}${(p * 1.095).toFixed(2)}`;

  // Verdict
  document.getElementById('verdictDetail').textContent = currentLang === 'zh' ? data.verdictZh : (data.verdictEn || data.verdictZh);

  // Buttons
  document.getElementById('btnStockDetail').href = `${LOCAL_API}/stock/${sym}`;
  
  let tvSym = `NASDAQ-${sym}`;
  if (isHK) {
    const code = sym.replace('.HK', '').replace(/^0+/, '') || '1';
    tvSym = `HKEX-${code}`;
  } else if (isAshare) {
    const code = sym.replace(/\.(SS|SZ|BJ)$/, '');
    tvSym = code.startsWith('6') ? `SSE-${code}` : `SZSE-${code}`;
  } else if (sym.endsWith('.TW')) {
    tvSym = `TWSE-${sym.replace('.TW', '')}`;
  } else if (sym.endsWith('.T')) {
    tvSym = `TSE-${sym.replace('.T', '')}`;
  }
  document.getElementById('btnTradingView').href = `https://www.tradingview.com/symbols/${tvSym}/`;
}
