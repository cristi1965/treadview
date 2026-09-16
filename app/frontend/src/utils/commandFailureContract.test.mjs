import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const srcRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const read = (...parts) => fs.readFileSync(path.join(srcRoot, ...parts), 'utf8');

test('command store exposes read failures and rejects unsuccessful writes', () => {
  const source = read('stores', 'commandStore.ts');

  for (const state of ['tradesError', 'statsError', 'eventsError']) {
    assert.match(source, new RegExp(`${state}: string \\| null`));
    assert.match(source, new RegExp(`set\\(\\{[^}]*${state}: errorMessage`));
  }
  assert.match(source, /set\(\{ trades: \[\], tradesError:/);
  assert.match(source, /set\(\{ stats: null, statsError:/);
  assert.match(source, /set\(\{ events: \[\], eventsError:/);
  assert.match(source, /await apiPost\('\/api\/trades', \{ \.\.\.tradeData, environment: 'PAPER' \}\)/);
  assert.match(source, /await apiPost\('\/api\/events', eventData\)/);
  assert.doesNotMatch(source, /Failed to (?:fetch|add) (?:trades|stats|events)/);
});

test('journal preserves the form on save failure and provides load retry', () => {
  const source = read('pages', 'Journal.tsx');
  const catchAt = source.indexOf('} catch (error) {');
  const resetAt = source.indexOf("setSymbol('')");

  assert.ok(resetAt > -1 && catchAt > resetAt, 'form reset must remain inside the successful try branch');
  assert.match(source, /setSaveError\(error instanceof Error/);
  assert.match(source, /role="alert"/);
  assert.match(source, /Promise\.all\(\[fetchTrades\(\), fetchStats\(\)\]\)/);
  assert.match(source, /disabled=\{saving\}/);
});

test('macro exposes both feed and custom-event failures and preserves failed saves', () => {
  const source = read('pages', 'Macro.tsx');
  const catchAt = source.indexOf('} catch (error) {', source.indexOf('const handleSubmit'));
  const closeAt = source.indexOf('setShowAddForm(false)', source.indexOf('const handleSubmit'));

  assert.ok(closeAt > -1 && catchAt > closeAt, 'form close must remain inside the successful try branch');
  assert.match(source, /eventsError && <button[\s\S]{0,300}?fetchEvents/);
  assert.match(source, /feedError && <button[\s\S]{0,300}?fetchFeed/);
  assert.match(source, /setSaveError\(error instanceof Error/);
  assert.match(source, /disabled=\{saving\}/);
});
