#!/usr/bin/env node
import fs from 'node:fs/promises';
import path from 'node:path';

function parseArgs(argv) {
  const args = {
    replay: 'recovered_source/stockgod-replay',
    out: 'recovered_source/stockgod-data-store',
  };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--replay') args.replay = argv[++i];
    else if (arg === '--out') args.out = argv[++i];
  }
  return args;
}

function classify(key) {
  if (key.startsWith('/api/quote')) return 'quotes';
  if (key.startsWith('/api/')) return 'api';
  if (key.startsWith('/data/')) return 'data';
  if (key.includes('_rsc=')) return 'rsc';
  if (key.startsWith('/stock/')) return 'stock-rsc';
  if (key.startsWith('/whales/')) return 'whale-rsc';
  return 'other';
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  await fs.rm(args.out, { recursive: true, force: true });
  await fs.mkdir(path.join(args.out, 'fixtures'), { recursive: true });

  const manifest = JSON.parse(await fs.readFile(path.join(args.replay, 'manifest.json'), 'utf8'));
  const index = {
    generatedAt: new Date().toISOString(),
    sourceReplay: args.replay,
    groups: {},
    endpoints: {},
  };

  for (const [key, entry] of Object.entries(manifest.responses || {})) {
    const group = classify(key);
    const source = path.join(args.replay, entry.file);
    const name = key.replace(/[^a-zA-Z0-9._-]+/g, '_').replace(/^_+|_+$/g, '').slice(0, 180) || 'root';
    const ext = entry.contentType?.includes('json') ? '.json' : '.txt';
    const fixture = `fixtures/${group}_${name}${ext}`;
    await fs.copyFile(source, path.join(args.out, fixture)).catch(() => {});
    const item = {
      key,
      group,
      sourceUrl: entry.sourceUrl,
      route: entry.route,
      status: entry.status,
      contentType: entry.contentType,
      bytes: entry.bytes,
      fixture,
    };
    index.endpoints[key] = item;
    if (!index.groups[group]) index.groups[group] = { count: 0, samples: [] };
    index.groups[group].count += 1;
    if (index.groups[group].samples.length < 20) index.groups[group].samples.push(key);
  }

  await fs.writeFile(path.join(args.out, 'index.json'), JSON.stringify(index, null, 2));
  console.log(`Wrote ${path.join(args.out, 'index.json')}`);
  console.table(Object.entries(index.groups).map(([group, info]) => ({ group, count: info.count })));
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
