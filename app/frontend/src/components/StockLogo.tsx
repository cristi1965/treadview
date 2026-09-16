import React, { useState } from 'react';

interface StockLogoProps {
  symbol: string;
  size?: number;
  className?: string;
}

const STOCK_BRAND_COLORS: Record<string, { bg: string; text: string; label: string }> = {
  NVDA: { bg: 'bg-[#76B900]/20 border-[#76B900]/40', text: 'text-[#76B900]', label: 'NV' },
  TSLA: { bg: 'bg-[#E82127]/20 border-[#E82127]/40', text: 'text-[#E82127]', label: 'T' },
  AAPL: { bg: 'bg-slate-700/30 border-slate-500/40', text: 'text-slate-200', label: '🍎' },
  MSFT: { bg: 'bg-[#00A4EF]/20 border-[#00A4EF]/40', text: 'text-[#00A4EF]', label: 'MS' },
  GOOGL: { bg: 'bg-[#4285F4]/20 border-[#4285F4]/40', text: 'text-[#4285F4]', label: 'G' },
  GOOG: { bg: 'bg-[#4285F4]/20 border-[#4285F4]/40', text: 'text-[#4285F4]', label: 'G' },
  AMZN: { bg: 'bg-[#FF9900]/20 border-[#FF9900]/40', text: 'text-[#FF9900]', label: 'a' },
  META: { bg: 'bg-[#0668E1]/20 border-[#0668E1]/40', text: 'text-[#0668E1]', label: '∞' },
  PLTR: { bg: 'bg-slate-800 border-slate-600', text: 'text-amber-400', label: 'P' },
  MSTR: { bg: 'bg-[#F7931A]/20 border-[#F7931A]/40', text: 'text-[#F7931A]', label: '₿' },
  CRWV: { bg: 'bg-[#00D26A]/20 border-[#00D26A]/40', text: 'text-[#00D26A]', label: 'CW' },
  NBIS: { bg: 'bg-[#8B5CF6]/20 border-[#8B5CF6]/40', text: 'text-[#8B5CF6]', label: 'NB' },
  MU: { bg: 'bg-[#003B71]/30 border-[#003B71]/50', text: 'text-sky-400', label: 'MU' },
  AMD: { bg: 'bg-[#ED1C24]/20 border-[#ED1C24]/40', text: 'text-[#ED1C24]', label: 'AMD' },
  ARM: { bg: 'bg-[#0091BD]/20 border-[#0091BD]/40', text: 'text-[#0091BD]', label: 'ARM' },
  IREN: { bg: 'bg-emerald-500/20 border-emerald-500/40', text: 'text-emerald-400', label: 'IR' },
  BE: { bg: 'bg-teal-500/20 border-teal-500/40', text: 'text-teal-300', label: 'BE' },
  COHR: { bg: 'bg-indigo-500/20 border-indigo-500/40', text: 'text-indigo-300', label: 'CO' },
  // A-Shares
  '300750': { bg: 'bg-red-500/20 border-red-500/40', text: 'text-red-400', label: '宁德' },
  '300308': { bg: 'bg-cyan-500/20 border-cyan-500/40', text: 'text-cyan-300', label: '旭创' },
  '688256': { bg: 'bg-blue-500/20 border-blue-500/40', text: 'text-blue-400', label: '寒武' },
  '601127': { bg: 'bg-orange-500/20 border-orange-500/40', text: 'text-orange-400', label: '赛力' },
  '300502': { bg: 'bg-purple-500/20 border-purple-500/40', text: 'text-purple-300', label: '易盛' },
  '600519': { bg: 'bg-amber-500/20 border-amber-500/40', text: 'text-amber-300', label: '茅台' },
  '60519': { bg: 'bg-amber-500/20 border-amber-500/40', text: 'text-amber-300', label: '茅台' },
};

export const StockLogo: React.FC<StockLogoProps> = ({
  symbol,
  size = 28,
  className = '',
}) => {
  const cleanSym = symbol.trim().toUpperCase();
  const [failed, setFailed] = useState(false);

  const logoUrl = `https://financialmodelingprep.com/image-stock/${cleanSym}.png`;
  const brand = STOCK_BRAND_COLORS[cleanSym] || {
    bg: 'bg-slate-800 border-slate-700',
    text: 'text-amber-400',
    label: cleanSym.slice(0, 2),
  };

  // FMP has no reliable coverage for arbitrary tickers (especially A-shares).
  // Only request a remote logo for known covered brands; unknown symbols use
  // the deterministic local badge and avoid guaranteed 404 requests.
  const canLoadRemoteLogo = Boolean(STOCK_BRAND_COLORS[cleanSym]) && !/^\d{6}$/.test(cleanSym);
  if (!failed && canLoadRemoteLogo) {
    return (
      <img
        src={logoUrl}
        alt={cleanSym}
        className={`rounded-md object-contain border border-slate-700/50 bg-slate-900/80 p-0.5 shadow-sm transition group-hover:scale-105 ${className}`}
        style={{ width: size, height: size }}
        onError={() => setFailed(true)}
      />
    );
  }

  return (
    <div
      className={`rounded-md border flex shrink-0 items-center justify-center font-extrabold text-[10px] shadow-sm ${brand.bg} ${brand.text} ${className}`}
      style={{ width: size, height: size }}
    >
      {brand.label}
    </div>
  );
};
