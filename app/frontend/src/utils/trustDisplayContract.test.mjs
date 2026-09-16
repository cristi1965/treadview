import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const srcRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const read = (...parts) => fs.readFileSync(path.join(srcRoot, ...parts), 'utf8');

const productionSources = () => {
  const files = [];
  const visit = (directory) => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const full = path.join(directory, entry.name);
      if (entry.isDirectory()) visit(full);
      else if (/\.(?:ts|tsx|js|jsx)$/.test(entry.name) && !/\.test\./.test(entry.name)) files.push(full);
    }
  };
  visit(srcRoot);
  return files.map((file) => fs.readFileSync(file, 'utf8')).join('\n');
};

const productionFiles = () => {
  const files = [];
  const visit = (directory) => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const full = path.join(directory, entry.name);
      if (entry.isDirectory()) visit(full);
      else if (/\.(?:ts|tsx|js|jsx)$/.test(entry.name) && !/\.test\./.test(entry.name)) files.push(full);
    }
  };
  visit(srcRoot);
  return files;
};

test('key watchlist contains only manual identifiers and gates all quotes on provenance', () => {
  const source = read('components', 'KeyWatchlistPanel.tsx');

  assert.doesNotMatch(source, /\b(?:price|pct|score|verdict):\s*-?\d/);
  assert.doesNotMatch(source, /AI \u8bc4\u5206|item\.score|item\.verdict|item\.price|item\.pct/);
  assert.match(source, /watchReason:/);
  assert.match(source, /\u4eba\u5de5\u89c2\u5bdf\u6e05\u5355/);
  assert.match(source, /result\.meta\.source && result\.meta\.dataTime/);
  assert.match(source, /!result\.meta\.stale/);
  assert.match(source, /setLiveQuotes\(usable \? result\.quotes : \{\}\)/);
  assert.match(source, /to="\/settings"/);
});

test('home removes old overview signals and exposes macro and overview failures', () => {
  const source = read('pages', 'Home.tsx');
  const overview = read('components', 'AshareMarketRadar.tsx');

  assert.match(source, /panelStocksRef\.current = \(panel\?\.stocks \|\| \{\}\)/);
  assert.match(source, /scores: undefined, avgScore: undefined, divergence: undefined/);
  assert.match(source, /setMacroTickers\(\[\]\)/);
  assert.match(source, /getWithMeta<\{ series: any\[\] \}>\('\/api\/macro'\)/);
  assert.match(source, /macroTrust\.state === 'live'/);
  assert.match(source, /宏观数值已隐藏/);
  assert.match(source, /历史脉冲快照/);
  assert.match(source, /价格、涨跌与过热度仅供回看/);
  assert.ok(source.indexOf('grid grid-cols-3 gap-1') < source.indexOf('<KeyWatchlistPanel />'), 'core heatmap controls must precede supporting panels');
  assert.match(source, /mode === '脉冲热力' && region === 'CN' && <CnToolbox/);
  assert.doesNotMatch(source, /introOpen|setIntroOpen/);
  const pulseData = read('utils', 'pulseData.ts');
  assert.match(pulseData, /mode: 'best'/);
  assert.match(pulseData, /getWithMeta/);
  assert.match(source, /\u8bc4\u5206\u6982\u89c8\u4e0d\u53ef\u7528/);
  assert.match(source, /panelResponse\.meta\.source/);
  assert.match(source, /!panelResponse\.meta\.stale/);
  assert.match(source, /validation\?\.status === 'validated'/);
  assert.match(source, /validation\?\.reasons\?\.join/);
  assert.doesNotMatch(source, /avgScore:\s*score \?\? c\.heat/);
  assert.match(overview, /catch \(err\) \{[\s\S]*setData\(null\)/);
  assert.match(overview, /state=\{error \? 'unavailable'/);
  assert.doesNotMatch(overview, /仍显示上次成功数据/);
});

test('night-buy fabrication and random investment paths are absent from production source', () => {
  const source = productionSources();
  const chart = read('components', 'TradingViewAdvancedChart.tsx');

  assert.equal(fs.existsSync(path.join(srcRoot, 'components', 'NightBuyAssistantModal.tsx')), false);
  assert.doesNotMatch(source, /NightBuyAssistant|Math\.random/);
  assert.doesNotMatch(source, /Top3 财经热门|稳步讨论中|夜间买入助手|此刻能不能买/);
  assert.doesNotMatch(source, /建议顺应主线趋势|高确信度指引未来方向/);
  assert.doesNotMatch(chart, /price\s*=\s*(?:100|150)|stopLoss|takeProfit|止损|止盈/);
  assert.doesNotMatch(read('pages', 'StockDetail.tsx'), /stockArchetype|主场 段永平|共识偏多|谨慎观察/);
});

test('TradingView embed is display-only and fails visibly when the external widget is blocked', () => {
  const chart = read('components', 'TradingViewAdvancedChart.tsx');

  assert.match(chart, /setEmbedState\('loading'\)/);
  assert.match(chart, /setEmbedState\('unavailable'\)/);
  assert.match(chart, /MutationObserver/);
  assert.match(chart, /querySelector\('iframe'\)/);
  assert.match(chart, /addEventListener\('load'/);
  assert.doesNotMatch(chart, /fetch\('https:\/\/www\.tradingview-widget\.com\/'/);
  assert.match(chart, /}, 8000\)/);
  assert.match(chart, /TradingView 暂时无法加载/);
  assert.match(chart, /不参与评分、提醒或 Paper 订单计算/);
});

test('dashboard stock chart samples only trusted quotes and exposes recovery metadata', () => {
  const chart = read('components', 'StockChart.tsx');

  assert.match(chart, /fetchQuoteResult/);
  assert.match(chart, /assessDataTrust/);
  assert.match(chart, /trust\.state === 'live'/);
  assert.match(chart, /setChartData\(\[\]\)/);
  assert.match(chart, /dataTime=\{meta\?\.dataTime/);
  assert.match(chart, /source=\{meta\?\.source/);
  assert.match(chart, /onRetry=/);
  assert.doesNotMatch(chart, /fetchLiveQuotes|实时 quote 采样/);
});

test('scan and stock detail fail closed when quote evidence is stale', () => {
  const scan = read('pages', 'Scan.tsx');
  const store = read('stores', 'stocksStore.ts');
  const detail = read('pages', 'StockDetail.tsx');

  assert.match(store, /quoteState: result\.meta\.stale \? 'stale' : 'unavailable'/);
  assert.match(store, /result\.meta\.source && result\.meta\.dataTime/);
  assert.match(scan, /quoteUsable && hasCurrentQuote\(stock\) \? formatPrice\(stock\.price\) : '\u2014'/);
  assert.match(scan, /静态清单仍可浏览，但价格、涨跌和判后变化已隐藏/);
  assert.match(detail, /静态快照价格 · 不可用于决策/);
  assert.match(detail, /quoteStatus\.state === 'live' \? formatPercent/);
});

test('built bundle does not serve removed fabricated decision copy', { skip: !fs.existsSync(path.resolve(srcRoot, '..', 'dist', 'assets')) }, () => {
  const assets = path.resolve(srcRoot, '..', 'dist', 'assets');
  const bundle = fs.readdirSync(assets)
    .filter((name) => name.endsWith('.js'))
    .map((name) => fs.readFileSync(path.join(assets, name), 'utf8'))
    .join('\n');

  assert.doesNotMatch(bundle, /NightBuyAssistant|Top3 财经热门|稳步讨论中|夜间买入助手|此刻能不能买/);
  assert.doesNotMatch(bundle, /建议顺应主线趋势|高确信度指引未来方向|price\s*[:=]\s*(?:100|150)[,;}]/);
});

test('scan and stock detail hide unverified score fallbacks and generated conclusions', () => {
  const scan = read('pages', 'Scan.tsx');
  const detail = read('pages', 'StockDetail.tsx');
  const data = read('utils', 'stockgodData.ts');

  assert.doesNotMatch(scan, /count:\s*(?:6151|5504)/);
  assert.match(scan, /stock\.judged \? Math\.round\(stock\.avgScore\) : '\u2014'/);
  assert.match(data, /!result\.meta\.stale && hasProvenance/);
  assert.match(detail, /scoreStatus\.state !== 'live'/);
  assert.match(detail, /\u65e7\u5feb\u7167\u5206\u6570\u548c\u6d3e\u751f\u7ed3\u8bba\u5df2\u9690\u85cf/);
  assert.doesNotMatch(detail, /nvdaNarratives|genericNarrative|\|\| 100/);
	assert.match(data, /panel\.validation\?\.status === 'validated'/);
	assert.match(detail, /scoreValidated/);
	assert.match(detail, /未验证评分不作为投资信号/);
	for (const token of ['train_samples', 'test_samples', 'benchmark', 'reasons']) {
		assert.ok(detail.includes(token), `missing score validation field ${token}`);
	}
});

test('protected portfolio surfaces link to session-token settings and explains readiness semantics', () => {
  const portfolio = read('pages', 'Portfolio.tsx');
  const tactical = read('pages', 'TacticalCommand.tsx');
  const settings = read('pages', 'Settings.tsx');

  assert.match(portfolio, /!hasAdminAccess/);
  assert.match(portfolio, /to="\/settings"/);
  assert.match(tactical, /!hasAdminAccess/);
  assert.match(tactical, /to="\/settings"/);
  assert.match(settings, /HttpOnly/);
  assert.match(settings, /useAdminSession/);
  assert.match(settings, /高级恢复：手动 Bearer 令牌/);
  assert.match(settings, /ready \u53ea\u8868\u793a[\s\S]*不等于 server live/);
	assert.match(settings, /\/api\/health 仅表示进程存活/);
});

test('both shells share worst-state freshness control and disclose unknown plus oldest known time', () => {
  const control = read('components', 'layout', 'GlobalDataFreshnessControl.tsx');
  const stockGodShell = read('components', 'layout', 'StockGodShell.tsx');
  const cockpitLayout = read('components', 'layout', 'Layout.tsx');

  assert.match(control, /Math\.min/);
  assert.match(control, /hasUnknownTime/);
  assert.match(control, /实时数据未就绪/);
  assert.match(control, /aria-expanded=\{detailsOpen\}/);
  assert.match(control, /live-data-diagnostics/);
  assert.match(control, /endpointDetails\.map/);
  assert.match(control, /数据源不可刷新，仅重新读取了旧快照/);
  assert.match(control, /getWithMeta<any>/);
  assert.doesNotMatch(control, /title=\{statusTitle/);
  assert.doesNotMatch(control, /newestDataTimestamp|Math\.max\(\.\.\.timestamps\)|Date\.now\(\)/);
  for (const shell of [stockGodShell, cockpitLayout]) {
    assert.match(shell, /GlobalDataFreshnessControl/);
    assert.match(shell, /useGlobalDataFreshness/);
    assert.match(shell, /我不是神/);
    assert.doesNotMatch(shell, /NEMO/);
  }
});

test('market movers never label a stale snapshot as live', () => {
  const movers = read('components', 'calendar', 'MarketMovers.tsx');
  assert.match(movers, /getWithMeta<MoversData>/);
  assert.match(movers, /assessDataTrust/);
  assert.match(movers, /state === 'stale' \? '历史快照'/);
  assert.match(movers, /<DataStatus state="stale"/);
  assert.doesNotMatch(movers, /fetch\(apiUrl\('\/api\/premarket-movers'\)\)/);
});

test('sentiment and calendar surfaces retain and display response trust metadata', () => {
  const sentiment = read('components', 'sentiment', 'SentimentPanel.tsx');
  const upcoming = read('components', 'calendar', 'UpcomingEvents.tsx');
  const macro = read('pages', 'Macro.tsx');

  for (const source of [sentiment, upcoming, macro]) {
    assert.match(source, /getWithMeta/);
    assert.match(source, /assessDataTrust/);
    assert.match(source, /<DataStatus/);
    assert.match(source, /dataTime=\{/);
    assert.match(source, /source=\{/);
  }
  assert.doesNotMatch(upcoming, /catch\s*\{[\s\S]*setEvents\(\[\]\)/);
  assert.match(sentiment, /trust\?\.state !== 'unavailable'/);
  assert.match(upcoming, /trust\?\.state === 'unavailable' \? \[\] : events/);
  assert.match(macro, /feedTrust\?\.state === 'unavailable' \? \[\] : feed/);
});

test('arena, A-share routines, and QDII fail closed on untrusted response metadata', () => {
  const arena = read('pages', 'Arena.tsx');
  const routines = read('components', 'cn', 'CnRoutinesPanel.tsx');
  const qdii = read('components', 'QDIIPremiumRadar.tsx');

  for (const source of [arena, routines]) {
    assert.match(source, /getWithMeta/);
    assert.match(source, /assessDataTrust/);
    assert.match(source, /dataTime/);
    assert.match(source, /source/);
    assert.match(source, /onRetry/);
  }
  assert.match(arena, /arenaTrust\.state === 'live'/);
  assert.match(arena, /排名、账户价值、收益、持仓价格与交易结论已隐藏/);
  assert.match(routines, /trust\.state !== 'live'/);
  assert.match(routines, /不显示价格、今日动作、买卖点或提醒/);
  assert.match(qdii, /useQDIIPremiums/);
  assert.match(qdii, /风险结论和告警已暂停/);
  assert.doesNotMatch(qdii, /setLastUpdated|最后更新: \{lastUpdated/);
});

test('arena never carries the US-only comparison surface into CN mode', () => {
  const arena = read('pages', 'Arena.tsx');
  assert.match(arena, /const selectMarket/);
  assert.match(arena, /setTab\('board'\)/);
  assert.match(arena, /market === 'us' && \(/);
});

test('QDII endpoint has one shared consumer and every product surface uses its hook', () => {
  const endpoint = '/api/etf/premiums';
  const consumers = productionFiles()
    .filter((file) => fs.readFileSync(file, 'utf8').includes(endpoint))
    .map((file) => path.relative(srcRoot, file));

  assert.deepEqual(consumers, [path.join('hooks', 'useQDIIPremiums.ts')]);
  const hook = read('hooks', 'useQDIIPremiums.ts');
  const radar = read('components', 'QDIIPremiumRadar.tsx');
  const tactical = read('pages', 'TacticalCommand.tsx');
  assert.match(hook, /getWithMeta/);
  assert.match(hook, /selectTrustedQDIIItems/);
  for (const source of [radar, tactical]) {
    assert.match(source, /useQDIIPremiums/);
    assert.doesNotMatch(source, /getWithMeta<.*QDII|apiGet<.*QDII/);
  }
  assert.match(tactical, /qdiiTrust\.state !== 'live'/);
  assert.match(tactical, /溢价率、危险数量与风险结论已隐藏/);
});

test('dashboard prioritizes mobile recovery and research while collapsing the long watchlist', () => {
  const dashboard = read('pages', 'Dashboard.tsx');
  const watchlist = read('components', 'KeyWatchlistPanel.tsx');
  assert.match(dashboard, /href="#research-entry"/);
  assert.match(dashboard, /to="\/settings"/);
  assert.match(dashboard, /id="research-entry"/);
  assert.match(watchlist, /mobileExpanded/);
  assert.match(watchlist, /index >= 6/);
  assert.match(watchlist, /展开其余/);
});

test('QDII live copy describes neutral premium bands without action conclusions', () => {
  const qdii = read('components', 'QDIIPremiumRadar.tsx');
  for (const token of ['折价观察', '高溢价偏离', '溢价偏高', '溢价偏离较低']) assert.ok(qdii.includes(token));
  assert.doesNotMatch(qdii, /折价机会|别买|安全|加仓|减仓|操作铁律|分批吸筹/);
});

test('QDII notifications remain neutral and disclose NAV timing limits', () => {
  const alerts = read('stores', 'alertsStore.ts');
  for (const token of ['不构成买卖建议', '净值存在公布时滞', '不代表可套利', '不代表风险消失']) assert.ok(alerts.includes(token));
  for (const forbidden of ['严禁高位接盘', '折价套利机会', '可理性布局', '溢价风险释放完毕']) assert.ok(!alerts.includes(forbidden));
});
