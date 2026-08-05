#!/usr/bin/env node
import fs from 'node:fs/promises';
import path from 'node:path';

function parseArgs(argv) {
  const args = {
    capture: 'recovered_source/stockgod-live-capture',
    out: 'recovered_source/stockgod-contracts',
  };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--capture') args.capture = argv[++i];
    else if (arg === '--out') args.out = argv[++i];
    else if (arg === '--help' || arg === '-h') {
      console.log('Usage: node scripts/stockgod_recovery/build_contracts.mjs --capture recovered_source/stockgod-live-capture --out recovered_source/stockgod-contracts');
      process.exit(0);
    }
  }
  return args;
}

function normalizeEndpoint(rawUrl) {
  const url = new URL(rawUrl);
  const params = [...url.searchParams.keys()].sort();
  return `${url.pathname}${params.length ? `?${params.join('&')}` : ''}`;
}

function inferJsonShape(value, depth = 0) {
  if (depth > 5) return typeof value;
  if (Array.isArray(value)) {
    return {
      type: 'array',
      length: value.length,
      item: value.length > 0 ? inferJsonShape(value[0], depth + 1) : 'unknown',
    };
  }
  if (value && typeof value === 'object') {
    const entries = Object.entries(value).slice(0, 80);
    return {
      type: 'object',
      keys: entries.map(([key]) => key),
      fields: Object.fromEntries(entries.map(([key, fieldValue]) => [key, inferJsonShape(fieldValue, depth + 1)])),
    };
  }
  return typeof value;
}

async function readJsonIfPossible(file) {
  const stat = await fs.stat(file);
  if (stat.size > 2_000_000) {
    return { text: '', json: null, skipped: true };
  }
  const text = await fs.readFile(file, 'utf8');
  try {
    return { text, json: JSON.parse(text) };
  } catch {
    return { text, json: null };
  }
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  await fs.mkdir(args.out, { recursive: true });
  await fs.mkdir(path.join(args.out, 'fixtures'), { recursive: true });

  const index = JSON.parse(await fs.readFile(path.join(args.capture, 'capture-index.json'), 'utf8'));
  const contracts = {
    origin: index.origin,
    generatedAt: new Date().toISOString(),
    routes: [],
    endpoints: {},
    rsc: {},
    skippedRoutes: [],
  };

  for (const route of index.routes) {
    const routeDir = path.join(args.capture, 'routes', route.name);
    const summaryPath = path.join(routeDir, 'summary.json');
    let summary;
    try {
      summary = JSON.parse(await fs.readFile(summaryPath, 'utf8'));
    } catch {
      contracts.skippedRoutes.push({ route: route.route, reason: 'missing summary' });
      continue;
    }
    if (summary.pageError || !summary.textLength) {
      contracts.skippedRoutes.push({ route: route.route, reason: summary.pageError || 'empty page' });
      continue;
    }
    const routeContract = {
      route: route.route,
      title: summary.title,
      textLength: summary.textLength,
      headings: summary.headings,
      links: summary.links.map((link) => link.href).slice(0, 160),
      controls: summary.controls,
      endpoints: [],
      rscFiles: [],
    };

    for (const body of summary.apiBodies || []) {
      if (!body.file) continue;
      const sourceFile = path.join(routeDir, body.file);
      const fixtureName = `${route.name}_${body.file}`;
      const fixturePath = path.join(args.out, 'fixtures', fixtureName);
      await fs.copyFile(sourceFile, fixturePath);

      const endpoint = normalizeEndpoint(body.url);
      const parsed = await readJsonIfPossible(sourceFile);
      const item = {
        route: route.route,
        url: body.url,
        endpoint,
        status: body.status,
        contentType: body.contentType,
        fixture: `fixtures/${fixtureName}`,
        bytes: body.bytes,
        jsonShape: parsed.json ? inferJsonShape(parsed.json) : null,
        textSample: parsed.text ? parsed.text.slice(0, 400) : '',
        skippedBodyRead: Boolean(parsed.skipped),
      };
      routeContract.endpoints.push(item);
      if (!contracts.endpoints[endpoint]) {
        contracts.endpoints[endpoint] = {
          count: 0,
          routes: [],
          samples: [],
        };
      }
      contracts.endpoints[endpoint].count += 1;
      if (!contracts.endpoints[endpoint].routes.includes(route.route)) {
        contracts.endpoints[endpoint].routes.push(route.route);
      }
      if (contracts.endpoints[endpoint].samples.length < 5) {
        contracts.endpoints[endpoint].samples.push(item);
      }
    }

    const flightPath = path.join(routeDir, 'next-flight-chunks.json');
    const flightChunks = JSON.parse(await fs.readFile(flightPath, 'utf8'));
    const rscFixtureName = `${route.name}_next-flight-chunks.json`;
    await fs.copyFile(flightPath, path.join(args.out, 'fixtures', rscFixtureName));
    routeContract.rscFiles.push(`fixtures/${rscFixtureName}`);
    contracts.rsc[route.route] = {
      chunkCount: flightChunks.length,
      fixture: `fixtures/${rscFixtureName}`,
      samples: flightChunks.slice(0, 3).map((chunk) => chunk.slice(0, 1200)),
    };

    contracts.routes.push(routeContract);
  }

  await fs.writeFile(path.join(args.out, 'contracts.json'), JSON.stringify(contracts, null, 2));

  const lines = [
    '# StockGod Recovery Contracts',
    '',
    `Origin: ${contracts.origin}`,
    `Generated: ${contracts.generatedAt}`,
    '',
    '## Routes',
    '',
    '| Route | Title | Text | Endpoints | RSC chunks |',
    '| --- | --- | ---: | ---: | ---: |',
    ...contracts.routes.map((route) => `| \`${route.route}\` | ${route.title.replace(/\|/g, '\\|')} | ${route.textLength} | ${route.endpoints.length} | ${contracts.rsc[route.route]?.chunkCount || 0} |`),
    '',
    '## Endpoints',
    '',
    '| Endpoint | Samples | Routes |',
    '| --- | ---: | --- |',
    ...Object.entries(contracts.endpoints).map(([endpoint, info]) => {
      const routes = info.routes.slice(0, 20).join(', ');
      return `| \`${endpoint}\` | ${info.count} | ${routes}${info.routes.length > 20 ? ', ...' : ''} |`;
    }),
    '',
    '## Skipped Routes',
    '',
    `Skipped: ${contracts.skippedRoutes.length}`,
    '',
  ];
  await fs.writeFile(path.join(args.out, 'CONTRACTS.md'), lines.join('\n'));
  console.log(`Wrote ${path.join(args.out, 'contracts.json')}`);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
