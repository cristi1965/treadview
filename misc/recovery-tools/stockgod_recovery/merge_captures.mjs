#!/usr/bin/env node
import fs from 'node:fs/promises';
import path from 'node:path';

function parseArgs(argv) {
  const args = {
    base: 'recovered_source/stockgod-all-capture',
    overlay: 'recovered_source/stockgod-retry-capture',
    out: 'recovered_source/stockgod-full-capture',
  };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--base') args.base = argv[++i];
    else if (arg === '--overlay') args.overlay = argv[++i];
    else if (arg === '--out') args.out = argv[++i];
    else if (arg === '--help' || arg === '-h') {
      console.log('Usage: node scripts/stockgod_recovery/merge_captures.mjs --base recovered_source/stockgod-all-capture --overlay recovered_source/stockgod-retry-capture --out recovered_source/stockgod-full-capture');
      process.exit(0);
    }
  }
  return args;
}

async function copyRoute(sourceRoot, routeName, outRoot) {
  await fs.rm(path.join(outRoot, 'routes', routeName), { recursive: true, force: true });
  await fs.cp(path.join(sourceRoot, 'routes', routeName), path.join(outRoot, 'routes', routeName), { recursive: true });
}

function usable(route) {
  return !route.pageError && route.textLength > 0;
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  await fs.rm(args.out, { recursive: true, force: true });
  await fs.mkdir(path.join(args.out, 'routes'), { recursive: true });

  const base = JSON.parse(await fs.readFile(path.join(args.base, 'capture-index.json'), 'utf8'));
  const overlay = JSON.parse(await fs.readFile(path.join(args.overlay, 'capture-index.json'), 'utf8'));
  const byRoute = new Map();

  for (const route of base.routes) {
    byRoute.set(route.route, { source: args.base, item: route });
  }
  for (const route of overlay.routes) {
    const existing = byRoute.get(route.route)?.item;
    if (!existing || (usable(route) && !usable(existing)) || (usable(route) && route.textLength >= existing.textLength)) {
      byRoute.set(route.route, { source: args.overlay, item: route });
    }
  }

  for (const { source, item } of byRoute.values()) {
    await copyRoute(source, item.name, args.out);
  }

  const index = {
    origin: base.origin || overlay.origin,
    startedAt: base.startedAt,
    finishedAt: new Date().toISOString(),
    routes: [...byRoute.values()].map(({ item }) => item),
  };
  await fs.writeFile(path.join(args.out, 'capture-index.json'), JSON.stringify(index, null, 2));
  console.log(`Wrote ${path.join(args.out, 'capture-index.json')}`);
  console.log(`Routes: ${index.routes.length}`);
  console.log(`Usable: ${index.routes.filter(usable).length}`);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
