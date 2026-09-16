import React from 'react';
import { Link } from 'react-router-dom';
import { FlashItem } from '../../types/reports';
import { I18nKey, useI18n } from '../../i18n';

const KIND_LABEL: Record<string, I18nKey> = {
  macro: 'flash.kindMacro',
  earnings: 'flash.kindEarnings',
  company: 'flash.kindCompany',
  market: 'flash.kindMarket',
  calendar: 'flash.calTitle',
};

const pad = (n: number) => String(n).padStart(2, '0');

export const parseFlashTime = (value: string): Date | null => {
  if (!value) return null;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
};

export const flashClock = (value: string): string => {
  const date = parseFlashTime(value);
  if (!date) return '--:--:--';
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
};

export const FlashItemRow: React.FC<{ item: FlashItem; isNew?: boolean; last?: boolean }> = ({ item, isNew, last }) => {
  const { t, language } = useI18n();
  const heavy = item.importance >= 3;
  const focus = item.importance === 2;
  const title = (language === 'en' && item.titleEn) || item.title;
  const body = (language === 'en' && item.bodyEn) || item.body;

  return (
    <li className={`pl-4 transition-colors duration-700 ${isNew ? 'bg-accent/[0.09]' : ''}`}>
      <div className={`flex min-h-[44px] gap-3 py-3 pr-4 ${last ? '' : 'border-b border-ios-hairline'}`}>
        <time className="w-[58px] shrink-0 pt-px font-mono text-[12px] tabular-nums text-ios-label-2">
          {flashClock(item.time)}
        </time>

        <span className="relative -my-3 flex w-3 shrink-0 justify-center self-stretch">
          <span aria-hidden className="absolute inset-y-0 w-px bg-white/[0.13]" />
          <span
            className={`relative mt-[18px] h-[7px] w-[7px] self-start rounded-full ${
              heavy ? 'bg-ios-red' : focus ? 'bg-ios-orange' : 'bg-ios-label-3'
            }`}
          />
        </span>

        <div className="min-w-0 flex-1">
          <p className={`text-[14px] leading-snug ${heavy ? 'font-semibold text-ios-red' : focus ? 'font-medium text-ios-label' : 'text-ios-label'}`}>
            {heavy && (
              <span className="mr-1.5 rounded-[5px] bg-ios-red/15 px-1.5 py-px align-[1px] text-[10px] font-semibold text-ios-red">
                {t('flash.hot')}
              </span>
            )}
            {title}
          </p>

          {body && <p className="mt-1 text-[12.5px] leading-relaxed text-ios-label-2">{body}</p>}

          <div className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-[11px] text-ios-label-3">
            <span className="rounded-[5px] bg-white/[0.07] px-1.5 py-px text-ios-label-2">
              {t(KIND_LABEL[item.kind] ?? 'flash.kindMarket')}
            </span>
            {item.tickers?.map((ticker) => (
              <Link
                key={ticker}
                to={`/stock/${ticker}?market=us`}
                className="rounded-[5px] bg-white/[0.07] px-1.5 py-px font-mono font-medium text-ios-blue transition hover:bg-white/[0.12]"
              >
                {ticker}
              </Link>
            ))}
            {item.source && <span>{item.source}</span>}
            {item.link && (
              <a
                href={item.link}
                target="_blank"
                rel="noreferrer noopener"
                className="text-ios-blue transition hover:underline"
              >
                {t('flash.origin')} ↗
              </a>
            )}
          </div>
        </div>
      </div>
    </li>
  );
};
