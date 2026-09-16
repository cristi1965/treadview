import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const srcRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

const sourceFiles = (dir) => fs.readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
  const absolute = path.join(dir, entry.name);
  if (entry.isDirectory()) return sourceFiles(absolute);
  return /\.(ts|tsx)$/.test(entry.name) ? [absolute] : [];
});

test('only settings may POST the protected market refresh endpoint', () => {
  const callers = sourceFiles(srcRoot)
    .filter((file) => fs.readFileSync(file, 'utf8').includes('/api/market/refresh'))
    .map((file) => path.relative(srcRoot, file));

  assert.deepEqual(callers, [path.join('pages', 'Settings.tsx')]);
  const settings = fs.readFileSync(path.join(srcRoot, 'pages', 'Settings.tsx'), 'utf8');
  assert.match(settings, /verifyAdminToken\(\)/);
  assert.match(settings, /authorizedFetch\(apiUrl\('\/api\/market\/refresh\?top=500'\), \{ method: 'POST' \}\)/);
});
