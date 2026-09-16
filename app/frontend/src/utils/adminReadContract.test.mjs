import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const utilsDir = path.dirname(fileURLToPath(import.meta.url));
const sourceRoot = path.resolve(utilsDir, '..');
const read = (relative) => fs.readFileSync(path.join(sourceRoot, relative), 'utf8');

test('sensitive cockpit reads use the recoverable admin session', () => {
  const store = read('stores/analysisStore.ts');
  for (const endpoint of ['analysis/status', 'analysis/history', 'config']) {
    assert.ok(store.includes(`authorizedFetch(\`${'${API_BASE}'}/${endpoint}`));
  }
  const settings = read('pages/Settings.tsx');
  assert.match(settings, /authorizedFetch\(apiUrl\('\/api\/system\/status'\)/);
});

test('WebSocket waits for admin bootstrap before opening the protected endpoint', () => {
  const hook = read('hooks/useWebSocket.ts');
  assert.match(hook, /await bootstrapLocalAdminSession\(\)/);
  assert.ok(hook.indexOf('await bootstrapLocalAdminSession()') < hook.indexOf('new WebSocket(wsUrl())'));
});

test('authorized requests establish the local session before their first protected fetch', () => {
  const api = read('utils/api.ts');
  const preflight = 'if (!token && !localSessionEstablished) await establishLocalAdminSession();';
  assert.ok(api.includes(preflight));
  assert.ok(api.indexOf(preflight) < api.indexOf("const response = await fetch(firstInput"));
});
