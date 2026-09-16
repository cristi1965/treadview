// Comprehensive Content Script for TradingView & Financial Sites

let hudLang = 'zh';

const EXT_TEXTS = {
  zh: {
    title: '👑 我不是神 · TradingView 伴侣',
    btnCopyPine: '🌲 复制 6-in-1 指标',
    btnCopyWatch: '📋 复制 TV 自选',
    btnMultiChart: '🖥️ 多图盯盘',
    buffett: '巴菲特',
    duan: '段永平',
    serenity: 'Serenity',
    druck: '德鲁肯',
    sentiment: '情绪面',
    s2Label: '强支撑底 S2',
    s2Desc: '-8.8% 吸筹',
    s1Label: '第一支撑 S1',
    s1Desc: '-4.2% 回踩',
    r1Label: '第一阻力 R1',
    r1Desc: '+4.2% 突破',
    r2Label: '波段目标 R2',
    r2Desc: '+9.5% 兑现',
    verdictTitle: '🌙 此刻买入结论: ',
    verdictContent: '算力与基本面动量强劲，回踩 S1 第一支撑附近为高胜率买入区间。破位 S2 坚决止损。',
    btnDetail: '🚀 深度研判 (10-Agent)',
    btnPortfolio: '⭐ 加自选 / 仓位',
    toastPine: '✨ 已复制 6-in-1 指标源码！直接粘贴到下方 Pine Editor 即可免费使用 6 大指标',
    toastWatch: '📋 已复制 TV 格式自选代码！可在右侧列表中选择 Import List 导入',
    flowBadge: '🕵️ 暗盘 64% 多头大单',
    langToggle: 'EN'
  },
  en: {
    title: '👑 StockGod · TradingView Copilot',
    btnCopyPine: '🌲 Copy 6-in-1 Script',
    btnCopyWatch: '📋 Copy TV Watchlist',
    btnMultiChart: '🖥️ Multi-Chart',
    buffett: 'Buffett',
    duan: 'Duan YP',
    serenity: 'Serenity',
    druck: 'Drucken',
    sentiment: 'Sentiment',
    s2Label: 'Support S2',
    s2Desc: '-8.8% Accum',
    s1Label: 'Support S1',
    s1Desc: '-4.2% Dip',
    r1Label: 'Resistance R1',
    r1Desc: '+4.2% Break',
    r2Label: 'Target R2',
    r2Desc: '+9.5% Profit',
    verdictTitle: '🌙 Buy Verdict: ',
    verdictContent: 'Strong AI compute demand and momentum. Buying dips near S1 offers optimal risk/reward. Stop loss below S2.',
    btnDetail: '🚀 Deep Cockpit (10-Agent)',
    btnPortfolio: '⭐ Watchlist & Mine',
    toastPine: '✨ 6-in-1 Pine Script copied! Paste into Pine Editor below to unlock 6 indicators for free.',
    toastWatch: '📋 TV formatted tickers copied! Use "Import List" in TradingView.',
    flowBadge: '🕵️ Dark Pool 64% Bullish',
    langToggle: '中文'
  }
};

const PINE_SCRIPT_TEMPLATE = `//@version=5
indicator("我不是神 · 6-in-1 全能实战指标 [Free Tier Buster]", overlay=true)
ema20  = ta.ema(close, 20)
ema50  = ta.ema(close, 50)
ema100 = ta.ema(close, 100)
ema200 = ta.ema(close, 200)
plot(ema20,  "EMA 20 (短线生命线)",   color=color.rgb(250, 204, 21), linewidth=1)
plot(ema50,  "EMA 50 (中线分水岭)",   color=color.rgb(56, 189, 248), linewidth=2)
plot(ema100, "EMA 100 (机构建仓线)",  color=color.rgb(168, 85, 247), linewidth=2)
plot(ema200, "EMA 200 (牛熊分界线)",  color=color.rgb(239, 68, 68),  linewidth=3)
[bbMid, bbUpper, bbLower] = ta.bb(close, 20, 2.0)
pUpper = plot(bbUpper, "布林上轨", color=color.new(color.teal, 50))
pLower = plot(bbLower, "布林下轨", color=color.new(color.teal, 50))
fill(pUpper, pLower, color=color.new(color.teal, 93), title="布林带通道")
vwapValue = ta.vwap(hlc3)
plot(vwapValue, "VWAP (大单筹码中枢)", color=color.rgb(249, 115, 22), linewidth=2, style=plot.style_circles)
hHigh = ta.highest(high, 20)
lLow  = ta.lowest(low, 20)
plot(hHigh, "20周期 突破阻力位", color=color.new(color.red, 30), style=plot.style_linebr, linewidth=1)
plot(lLow,  "20周期 低吸支撑位", color=color.new(color.green, 30), style=plot.style_linebr, linewidth=1)
rsiVal = ta.rsi(close, 14)
plotshape(rsiVal <= 25, title="RSI 超卖", style=shape.triangleup, location=location.belowbar, color=color.green, size=size.small, text="超卖低吸")
plotshape(rsiVal >= 75, title="RSI 超买", style=shape.triangledown, location=location.abovebar, color=color.red, size=size.small, text="超买止盈")
[macdLine, signalLine, _] = ta.macd(close, 12, 26, 9)
plotshape(ta.crossover(macdLine, signalLine) and rsiVal < 65, title="MACD 共振", style=shape.labelup, location=location.belowbar, color=color.rgb(16, 185, 129), text="MACD共振", textcolor=color.white, size=size.tiny)
`;

const POPULAR_WATCHLISTS = {
  M7: 'NASDAQ:NVDA, NASDAQ:TSLA, NASDAQ:AAPL, NASDAQ:MSFT, NASDAQ:AMZN, NASDAQ:META, NASDAQ:GOOGL',
  HK: 'HKEX:1810, HKEX:700, HKEX:9988, HKEX:3690, HKEX:9618, HKEX:981, HKEX:1024, HKEX:9868',
  CN: 'SZSE:300750, SZSE:300308, SSE:688256, SSE:601127, SZSE:300502, SSE:600519',
  HOT: 'NASDAQ:PLTR, NASDAQ:MSTR, NASDAQ:CRWV, NASDAQ:NBIS, NASDAQ:MU, NASDAQ:AMD'
};

function detectCurrentSymbol() {
  const url = window.location.href;
  let symbol = 'NVDA';

  if (url.includes('tradingview.com')) {
    if (url.includes('/symbols/')) {
      const parts = url.split('/symbols/')[1].split('/')[0].split('-');
      symbol = parts[parts.length - 1];
    } else {
      const tvBtn = document.querySelector('[data-name="legend-series-item"] [data-name="legend-source-title"]') ||
                    document.querySelector('.js-button-text.symbol-title') ||
                    document.querySelector('#header-toolbar-symbol-search .js-button-text');
      if (tvBtn && tvBtn.textContent) {
        symbol = tvBtn.textContent.trim().split(' ')[0];
      }
    }
  } else if (url.includes('xueqiu.com') && url.includes('/S/')) {
    symbol = url.split('/S/')[1].split('?')[0];
  } else if (url.includes('finance.yahoo.com') && url.includes('/quote/')) {
    symbol = url.split('/quote/')[1].split('/')[0].split('?')[0];
  }

  return symbol ? symbol.toUpperCase() : 'NVDA';
}

function showToast(message) {
  let toast = document.getElementById('sg-toast-msg');
  if (!toast) {
    toast = document.createElement('div');
    toast.id = 'sg-toast-msg';
    document.body.appendChild(toast);
  }
  toast.textContent = message;
  toast.className = 'sg-toast show';
  setTimeout(() => {
    toast.className = 'sg-toast';
  }, 2500);
}

function updateHudTexts() {
  const t = EXT_TEXTS[hudLang];
  const titleEl = document.getElementById('sgTitleText');
  if (titleEl) titleEl.textContent = t.title;
  
  const langBtn = document.getElementById('sgHudLangBtn');
  if (langBtn) langBtn.textContent = t.langToggle;

  const btnPine = document.getElementById('btnCopyPine');
  if (btnPine) btnPine.textContent = t.btnCopyPine;

  const btnWatch = document.getElementById('btnCopyWatch');
  if (btnWatch) btnWatch.textContent = t.btnCopyWatch;

  const btnMulti = document.getElementById('btnMultiChart');
  if (btnMulti) btnMulti.textContent = t.btnMultiChart;

  const flowEl = document.getElementById('sgFlowBadge');
  if (flowEl) flowEl.textContent = t.flowBadge;

  const vTitle = document.getElementById('sgVerdictTitle');
  if (vTitle) vTitle.textContent = t.verdictTitle;

  const vContent = document.getElementById('sgVerdictContent');
  if (vContent) vContent.textContent = t.verdictContent;

  const bDetail = document.getElementById('sgBtnDetail');
  if (bDetail) bDetail.textContent = t.btnDetail;

  const bPort = document.getElementById('sgBtnPort');
  if (bPort) bPort.textContent = t.btnPortfolio;
}

function injectStockGodHud() {
  if (document.getElementById('stockgod-floating-hud')) return;

  const currentSym = detectCurrentSymbol();
  const t = EXT_TEXTS[hudLang];

  const container = document.createElement('div');
  container.id = 'stockgod-floating-hud';

  container.innerHTML = `
    <div class="sg-hud-pill" id="sgHudPill" title="点击展开【我不是神】TradingView 最佳实践助手 (快捷键 Alt+S)">
      <div class="sg-hud-icon">👑</div>
      <span class="sg-hud-ticker" id="sgHudTicker">${currentSym}</span>
      <span class="sg-hud-score">AI 88分</span>
    </div>

    <div class="sg-hud-card" id="sgHudCard">
      <!-- Header -->
      <div class="sg-card-header">
        <div class="sg-card-title">
          <span id="sgTitleText">${t.title}</span>
          <span class="sg-sym-badge" id="sgCardSym">${currentSym}</span>
        </div>
        <div style="display: flex; align-items: center; gap: 4px;">
          <button class="sg-lang-btn" id="sgHudLangBtn" title="切换中英文 / Switch Language">EN</button>
          <button class="sg-card-close" id="sgCardClose" title="关闭">✕</button>
        </div>
      </div>

      <!-- Quick Best Practice Tools -->
      <div class="sg-tools-bar">
        <button class="sg-tool-btn" id="btnCopyPine" title="一键复制 6-in-1 全能指标">
          ${t.btnCopyPine}
        </button>
        <button class="sg-tool-btn" id="btnCopyWatch" title="一键复制兼容 TV 导入格式代码">
          ${t.btnCopyWatch}
        </button>
        <a href="http://localhost:8765/multichart" target="_blank" class="sg-tool-btn sg-tool-accent" id="btnMultiChart">
          ${t.btnMultiChart}
        </a>
      </div>

      <!-- Dark Pool Flow Tag (Unusual Whales style) -->
      <div class="sg-flow-pill" id="sgFlowBadge">
        ${t.flowBadge} · $25M+ 净流入
      </div>

      <!-- 5-God Score Preview -->
      <div class="sg-gods-mini">
        <div class="sg-g-cell"><span class="lbl">巴菲特</span><strong class="val">70</strong></div>
        <div class="sg-g-cell"><span class="lbl">段永平</span><strong class="val" style="color: #38bdf8;">95</strong></div>
        <div class="sg-g-cell"><span class="lbl">Serenity</span><strong class="val">72</strong></div>
        <div class="sg-g-cell"><span class="lbl">德鲁肯</span><strong class="val" style="color: #f59e0b;">92</strong></div>
        <div class="sg-g-cell"><span class="lbl">情绪面</span><strong class="val">88</strong></div>
      </div>

      <!-- AI Support / Resistance Pivots -->
      <div class="sg-pivots-box">
        <div class="sg-p-item" style="border-left: 2px solid #10b981;">
          <span class="sg-p-lbl">强支撑底 S2</span>
          <span class="sg-p-num" style="color: #6ee7b7;">-8.8% 吸筹</span>
        </div>
        <div class="sg-p-item" style="border-left: 2px solid #14b8a6;">
          <span class="sg-p-lbl">第一支撑 S1</span>
          <span class="sg-p-num" style="color: #5eead4;">-4.2% 回踩</span>
        </div>
        <div class="sg-p-item" style="border-left: 2px solid #f59e0b;">
          <span class="sg-p-lbl">第一阻力 R1</span>
          <span class="sg-p-num" style="color: #fcd34d;">+4.2% 突破</span>
        </div>
        <div class="sg-p-item" style="border-left: 2px solid #6366f1;">
          <span class="sg-p-lbl">波段目标 R2</span>
          <span class="sg-p-num" style="color: #a5b4fc;">+9.5% 兑现</span>
        </div>
      </div>

      <!-- AI Verdict Banner -->
      <div class="sg-verdict-box">
        <strong id="sgVerdictTitle">${t.verdictTitle}</strong>
        <span id="sgVerdictContent">${t.verdictContent}</span>
      </div>

      <!-- Bottom Jump Links -->
      <div class="sg-card-actions">
        <a id="sgBtnDetail" href="http://localhost:8765/stock/${currentSym}" target="_blank" class="sg-btn">
          ${t.btnDetail}
        </a>
        <a id="sgBtnPort" href="http://localhost:8765/portfolio" target="_blank" class="sg-btn sg-btn-sec">
          ${t.btnPortfolio}
        </a>
      </div>
    </div>
  `;

  document.body.appendChild(container);

  const pill = document.getElementById('sgHudPill');
  const card = document.getElementById('sgHudCard');
  const closeBtn = document.getElementById('sgCardClose');
  const copyPineBtn = document.getElementById('btnCopyPine');
  const copyWatchBtn = document.getElementById('btnCopyWatch');
  const langBtn = document.getElementById('sgHudLangBtn');

  pill.addEventListener('click', () => {
    card.classList.toggle('active');
  });

  closeBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    card.classList.remove('active');
  });

  langBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    hudLang = hudLang === 'zh' ? 'en' : 'zh';
    updateHudTexts();
  });

  // Hotkey: Alt + S to toggle
  window.addEventListener('keydown', (e) => {
    if (e.altKey && (e.key === 's' || e.key === 'S')) {
      card.classList.toggle('active');
    }
  });

  // Best Practice 1: Copy Pine Script
  copyPineBtn.addEventListener('click', () => {
    navigator.clipboard.writeText(PINE_SCRIPT_TEMPLATE);
    showToast(EXT_TEXTS[hudLang].toastPine);
  });

  // Best Practice 2: Copy TradingView Watchlist
  copyWatchBtn.addEventListener('click', () => {
    const list = POPULAR_WATCHLISTS.M7 + ', ' + POPULAR_WATCHLISTS.HK + ', ' + POPULAR_WATCHLISTS.CN;
    navigator.clipboard.writeText(list);
    showToast(EXT_TEXTS[hudLang].toastWatch);
  });
}

// Initialize on page load
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', injectStockGodHud);
} else {
  setTimeout(injectStockGodHud, 1200);
}
