export type ReportView = 'zh-en' | 'zh' | 'en';

export function splitBilingual(content: string): { en: string; zh: string } {
  const raw = (content || '').trim();
  if (!raw) return { en: '', zh: '' };

  const parts = raw.split(/\r?\n-{3,}\r?\n/);
  if (parts.length >= 2) {
    const even = parts.length % 2 === 0;
    if (even && parts.length >= 4) {
      const enParts: string[] = [];
      const zhParts: string[] = [];
      for (let i = 0; i < parts.length; i += 2) {
        enParts.push(parts[i].trim());
        zhParts.push(parts[i + 1].trim());
      }
      return { en: enParts.filter(Boolean).join('\n\n'), zh: zhParts.filter(Boolean).join('\n\n') };
    }
    const en = parts[0].trim();
    const zh = parts.slice(1).join('\n---\n').trim();
    return { en, zh };
  }

  const cjk = (raw.match(/[\u4e00-\u9fff]/g) || []).length;
  if (cjk >= 8 || cjk * 10 > raw.length) {
    return { en: '', zh: raw };
  }
  return { en: raw, zh: '' };
}

export function pickBilingual(content: string, view: ReportView): { en: string; zh: string; mode: 'pair' | 'single' } {
  const { en, zh } = splitBilingual(content);
  if (view === 'zh') return { en: '', zh: zh || en, mode: 'single' };
  if (view === 'en') return { en: en || zh, zh: '', mode: 'single' };
  if (en && zh) return { en, zh, mode: 'pair' };
  const one = zh || en;
  return { en: one, zh: one, mode: 'single' };
}

export function formatBilingualLine(view: ReportView, zh: string, en: string): string {
  if (view === 'zh') return zh;
  if (view === 'en') return en;
  if (!zh) return en;
  if (!en || zh === en) return zh;
  return `${zh} · ${en}`;
}

export function appendBilingual(prev: string, headerEn: string, headerZh: string, chunk: string): string {
  const a = splitBilingual(prev);
  const b = splitBilingual(chunk);
  const wrap = (header: string, body: string) => {
    const h = header.trim();
    const t = body.trim();
    if (!h && !t) return '';
    if (!h) return t;
    if (!t) return `### ${h}`;
    return `### ${h}\n\n${t}`;
  };
  const en = [a.en, wrap(headerEn, b.en)].filter(Boolean).join('\n\n');
  const zh = [a.zh, wrap(headerZh, b.zh || (!b.en ? '' : b.en))].filter(Boolean).join('\n\n');
  const enOut = en.trim();
  const zhOut = zh.trim();
  if (enOut && zhOut && enOut !== zhOut) return `${enOut}\n\n---\n\n${zhOut}`;
  return enOut || zhOut;
}
