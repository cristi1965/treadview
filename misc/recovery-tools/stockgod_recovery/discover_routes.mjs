#!/usr/bin/env node
import fs from 'node:fs/promises';
import path from 'node:path';

function parseArgs(argv) {
  const args = {
    capture: 'recovered_source/stockgod-live-capture',
    out: 'recovered_source/stockgod-discovered-routes.txt',
    limit: 80,
    whales: 30,
    usStocks: 30,
    aStocks: 15,
  };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--capture') args.capture = argv[++i];
    else if (arg === '--out') args.out = argv[++i];
    else if (arg === '--limit') args.limit = Number(argv[++i] || args.limit);
    else if (arg === '--whales') args.whales = Number(argv[++i] || args.whales);
    else if (arg === '--us-stocks') args.usStocks = Number(argv[++i] || args.usStocks);
    else if (arg === '--a-stocks') args.aStocks = Number(argv[++i] || args.aStocks);
    else if (arg === '--help' || arg === '-h') {
      console.log('Usage: node scripts/stockgod_recovery/discover_routes.mjs --capture recovered_source/stockgod-live-capture --out recovered_source/stockgod-discovered-routes.txt --limit 80');
      process.exit(0);
    }
  }
  return args;
}

function cleanRoute(href) {
  try {
    const url = new URL(href, 'https://stockgod.xyz');
    if (url.hostname !== 'stockgod.xyz') return '';
    if (url.pathname.startsWith('/_next') || url.pathname.startsWith('/api/') || url.pathname.startsWith('/data/')) return '';
    url.hash = '';
    url.searchParams.delete('_rsc');
    const query = url.searchParams.toString();
    return `${url.pathname}${query ? `?${query}` : ''}`;
  } catch {
    return '';
  }
}

function scoreRoute(route) {
  if (['/', '/scan', '/etf', '/whales', '/arena', '/reports', '/portfolio', '/about', '/terms', '/privacy', '/how-to-buy'].includes(route)) return 0;
  if (route.startsWith('/whales/howard-marks')) return 0;
  if (route.startsWith('/whales/')) return 1000;
  if (/^\/stock\/(AAPL|MSFT|GOOG|GOOGL|META|AMZN|NVDA|TSM|BRK\\.B|BRK\\.A|PDD|ELV|CBOE|ZTS|VRSN|BLK|COF|DIS|MA|V|CRM|SPOT|FCX|CORZ|TLN|XP|BLCO)(\\?|$)/.test(route)) return 900;
  if (/^\/stock\/(600519|601318|300750|000333|000858|601398|000001|600036|002475)(\\?|$)/.test(route)) return 820;
  if (route.startsWith('/stock/')) return 500;
  return 100;
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const index = JSON.parse(await fs.readFile(path.join(args.capture, 'capture-index.json'), 'utf8'));
  const existing = new Set(index.routes.map((route) => route.route));
  const discovered = new Set();

  for (const route of index.routes) {
    const summaryPath = path.join(args.capture, 'routes', route.name, 'summary.json');
    const summary = JSON.parse(await fs.readFile(summaryPath, 'utf8'));
    for (const link of summary.links || []) {
      const cleaned = cleanRoute(link.href);
      if (cleaned && !existing.has(cleaned)) discovered.add(cleaned);
    }
  }

  const rankedAll = [...discovered]
    .map((route) => ({ route, score: scoreRoute(route) }))
    .filter((item) => item.score > 0)
    .sort((a, b) => b.score - a.score || a.route.localeCompare(b.route));

  const selected = [];
  const addBucket = (predicate, limit) => {
    for (const item of rankedAll) {
      if (selected.length >= args.limit) break;
      if (selected.includes(item.route) || !predicate(item.route)) continue;
      selected.push(item.route);
      if (selected.filter(predicate).length >= limit) break;
    }
  };

  addBucket((route) => route.startsWith('/whales/'), args.whales);
  addBucket((route) => route.startsWith('/stock/') && route.includes('market=us'), args.usStocks);
  addBucket((route) => route.startsWith('/stock/') && route.includes('market=a'), args.aStocks);
  addBucket(() => true, args.limit);

  await fs.mkdir(path.dirname(args.out), { recursive: true });
  await fs.writeFile(args.out, `${selected.join('\n')}\n`);
  console.log(`Wrote ${args.out}`);
  console.log(`Discovered: ${discovered.size}`);
  console.log(`Selected: ${selected.length}`);
  console.log(selected.join('\n'));
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
