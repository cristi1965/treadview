#!/usr/bin/env node
import fs from 'node:fs/promises';
import path from 'node:path';

function parseArgs(argv) {
  const args = {
    capture: 'recovered_source/stockgod-live-capture',
    out: 'recovered_source/stockgod-replay',
  };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--capture') args.capture = argv[++i];
    else if (arg === '--out') args.out = argv[++i];
    else if (arg === '--help' || arg === '-h') {
      console.log('Usage: node scripts/stockgod_recovery/build_replay_manifest.mjs --capture recovered_source/stockgod-live-capture --out recovered_source/stockgod-replay');
      process.exit(0);
    }
  }
  return args;
}

function requestKey(rawUrl, origin) {
  const url = new URL(rawUrl, origin);
  return `${url.pathname}${url.search}`;
}

function normalizedRequestKey(key) {
  const url = new URL(key, 'https://stockgod.xyz');
  if (!url.searchParams.has('_rsc')) return key;

  const parts = [];
  for (const name of [...new Set([...url.searchParams.keys()])].sort()) {
    const values = url.searchParams.getAll(name).sort();
    for (const value of values) {
      if (name === '_rsc') parts.push('_rsc=*');
      else parts.push(`${encodeURIComponent(name)}=${encodeURIComponent(value)}`);
    }
  }
  const query = parts.join('&');
  return `${url.pathname}${query ? `?${query}` : ''}`;
}

function routePath(route) {
  if (/^https?:\/\//.test(route)) {
    const url = new URL(route);
    return `${url.pathname}${url.search}`;
  }
  return route.startsWith('/') ? route : `/${route}`;
}

function fixtureName(routeName, fileName) {
  return `${routeName}_${fileName}`;
}

function preferred(existing, next) {
  if (!existing) return next;
  if (next.status >= 200 && next.status < 300 && !(existing.status >= 200 && existing.status < 300)) return next;
  if ((next.bytes || 0) > (existing.bytes || 0)) return next;
  return existing;
}

async function copyIfExists(from, to) {
  try {
    await fs.copyFile(from, to);
    return true;
  } catch {
    return false;
  }
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const fixtureDir = path.join(args.out, 'fixtures');
  const pageDir = path.join(args.out, 'pages');
  await fs.rm(args.out, { recursive: true, force: true });
  await fs.mkdir(fixtureDir, { recursive: true });
  await fs.mkdir(pageDir, { recursive: true });

  const index = JSON.parse(await fs.readFile(path.join(args.capture, 'capture-index.json'), 'utf8'));
  const manifest = {
    origin: index.origin,
    generatedAt: new Date().toISOString(),
    pages: {},
    responses: {},
    normalizedResponses: {},
  };

  for (const route of index.routes) {
    const routeDir = path.join(args.capture, 'routes', route.name);
    let summary;
    try {
      summary = JSON.parse(await fs.readFile(path.join(routeDir, 'summary.json'), 'utf8'));
    } catch {
      continue;
    }
    if (summary.pageError || !summary.textLength) continue;

    const pageFile = `${route.name}.html`;
    await copyIfExists(path.join(routeDir, 'page.html'), path.join(pageDir, pageFile));
    manifest.pages[routePath(summary.route)] = {
      title: summary.title,
      route: summary.route,
      file: `pages/${pageFile}`,
      contentType: 'text/html; charset=utf-8',
      status: 200,
    };

    for (const body of summary.apiBodies || []) {
      if (!body.file) continue;
      const key = requestKey(body.url, index.origin);
      const name = fixtureName(route.name, body.file);
      await copyIfExists(path.join(routeDir, body.file), path.join(fixtureDir, name));
      manifest.responses[key] = preferred(manifest.responses[key], {
        route: summary.route,
        sourceUrl: body.url,
        file: `fixtures/${name}`,
        status: body.status,
        contentType: body.contentType || 'application/octet-stream',
        bytes: body.bytes || 0,
      });
      const normalized = normalizedRequestKey(key);
      if (normalized !== key) {
        manifest.normalizedResponses[normalized] = preferred(manifest.normalizedResponses[normalized], manifest.responses[key]);
      }
    }
  }

  await fs.writeFile(path.join(args.out, 'manifest.json'), JSON.stringify(manifest, null, 2));
  console.log(`Wrote ${path.join(args.out, 'manifest.json')}`);
  console.log(`Pages: ${Object.keys(manifest.pages).length}`);
  console.log(`Responses: ${Object.keys(manifest.responses).length}`);
  console.log(`Normalized responses: ${Object.keys(manifest.normalizedResponses).length}`);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
