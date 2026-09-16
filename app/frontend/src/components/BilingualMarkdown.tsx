import React from 'react';
import ReactMarkdown from 'react-markdown';
import { useUiStore, ReportView } from '../stores/uiStore';
import { pickBilingual } from '../utils/bilingual';
import { useI18n } from '../i18n';

export const ReportViewToggle: React.FC = () => {
  const { t } = useI18n();
  const view = useUiStore((s) => s.reportView);
  const setReportView = useUiStore((s) => s.setReportView);
  const opts: { id: ReportView; label: string }[] = [
    { id: 'zh-en', label: t('report.viewZhEn') },
    { id: 'en', label: t('report.viewEn') },
    { id: 'zh', label: t('report.viewZh') },
  ];

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', flexWrap: 'wrap' }}>
      <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{t('report.viewHint')}</span>
      <div style={{ display: 'inline-flex', borderRadius: '0.5rem', border: '1px solid var(--border-color)', padding: '2px' }}>
        {opts.map((o) => (
          <button
            key={o.id}
            type="button"
            onClick={() => setReportView(o.id)}
            style={{
              border: 'none',
              borderRadius: '0.375rem',
              padding: '0.35rem 0.7rem',
              fontSize: '0.75rem',
              fontWeight: 600,
              cursor: 'pointer',
              background: view === o.id ? 'rgba(255,255,255,0.1)' : 'transparent',
              color: view === o.id ? 'white' : 'var(--text-secondary)',
            }}
          >
            {o.label}
          </button>
        ))}
      </div>
    </div>
  );
};

export const BilingualMarkdown: React.FC<{ content: string }> = ({ content }) => {
  const { t } = useI18n();
  const view = useUiStore((s) => s.reportView);
  const picked = pickBilingual(content, view);
  const text = (picked.zh || picked.en || '').trim();

  if (!text) {
    return (
      <div style={{ color: 'var(--text-muted)', fontStyle: 'italic', fontSize: '0.9rem' }}>
        {t('report.empty')}
      </div>
    );
  }

  if (picked.mode === 'pair') {
    return (
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 1fr)',
          gap: '1rem',
        }}
      >
        <div>
          <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)', marginBottom: '0.4rem', fontFamily: 'var(--font-mono)' }}>
            {t('report.colZh')}
          </div>
          <div className="markdown-body">
            <ReactMarkdown>{picked.zh}</ReactMarkdown>
          </div>
        </div>
        <div>
          <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)', marginBottom: '0.4rem', fontFamily: 'var(--font-mono)' }}>
            {t('report.colEn')}
          </div>
          <div className="markdown-body">
            <ReactMarkdown>{picked.en}</ReactMarkdown>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="markdown-body">
      <ReactMarkdown>{picked.zh || picked.en}</ReactMarkdown>
    </div>
  );
};
