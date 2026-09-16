import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';

const read = (relative) => fs.readFileSync(new URL(relative, import.meta.url), 'utf8');

test('whales page and global refresh share one store-backed data state', () => {
  const page = read('../pages/Whales.tsx');
  const store = read('../stores/whalesStore.ts');
  const controller = read('../components/SystemDataSyncController.tsx');

  assert.doesNotMatch(page, /useState<Investor\[\]>/);
  assert.doesNotMatch(page, /useState<CongressMember\[\]>/);
  for (const token of ['investorsLoading', 'investorsError', 'investorsMeta', 'consensusMeta', 'congressMeta']) {
    assert.ok(page.includes(token), `page missing ${token}`);
  }
  assert.ok(store.includes('refreshWhalesPage'));
  assert.ok(controller.includes('refreshWhalesPage'));
});

test('unverified congress bootstrap rows stay outside the signal leaderboard', () => {
  const page = read('../pages/Whales.tsx');
  assert.ok(page.includes('verifiedCongressMembers'));
  assert.ok(page.includes('未核验历史样本'));
  assert.ok(page.includes('不作为投资信号'));
  for (const relative of ['../components/WhalesBoard.tsx', '../pages/WhalesSimple.tsx', '../pages/WhalesPro.tsx', '../pages/WhalesProV2.tsx']) {
    assert.doesNotMatch(read(relative), /\btriggerSync\(\)/, `${relative} must remain read-only on mount`);
  }
});
