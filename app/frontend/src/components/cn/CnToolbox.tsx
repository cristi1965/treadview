import React, { useState } from 'react';
import { CnRoutinesPanel } from './CnRoutinesPanel';
import { CnPicksPanel } from './CnPicksPanel';
import { useI18n } from '../../i18n';

type Tab = 'routines' | 'picks';

/** 合并稳健套路 + A股精选；两面板保活，避免切 tab 重复打贵 API。 */
export const CnToolbox: React.FC = () => {
  const { t } = useI18n();
  const [tab, setTab] = useState<Tab>('routines');

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-1.5 rounded-xl border border-line bg-surface px-3 py-2">
        <span className="mr-1 font-mono text-[10px] uppercase tracking-wider text-faint">{t('cn.box')}</span>
        {(
          [
            { id: 'routines' as const, label: t('cn.routines') },
            { id: 'picks' as const, label: t('cn.picks') },
          ] as const
        ).map((item) => (
          <button
            key={item.id}
            type="button"
            onClick={() => setTab(item.id)}
            className={`rounded-md px-3 py-1.5 text-xs font-medium transition ${
              tab === item.id ? 'bg-accent/15 text-accent' : 'bg-surface-2 text-muted hover:text-ink'
            }`}
          >
            {item.label}
          </button>
        ))}
      </div>
      <div className={tab === 'routines' ? '' : 'hidden'}>
        <CnRoutinesPanel />
      </div>
      <div className={tab === 'picks' ? '' : 'hidden'}>
        <CnPicksPanel />
      </div>
    </div>
  );
};
