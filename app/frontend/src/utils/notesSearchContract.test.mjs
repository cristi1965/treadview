import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';

const notes = fs.readFileSync(new URL('../pages/Notes.tsx', import.meta.url), 'utf8');

test('notes search merges body hits and exposes search failures', () => {
  assert.ok(notes.includes('/api/notes/search?q='));
  assert.ok(notes.includes('bodyHits.has(item.id)'));
  assert.ok(notes.includes('正文搜索失败'));
});

test('notes storage writes are guarded and surfaced', () => {
  assert.match(notes, /try \{\s*localStorage\.setItem\(READ_KEY/);
  assert.ok(notes.includes('阅读记录无法写入'));
});
