import React, { useState } from 'react';

interface AvatarProps {
  src?: string;
  letter?: string;
  size?: number;
  party?: 'D' | 'R';
  className?: string;
  slug?: string;
  name?: string;
}

// 知名投资人与政要高清真实头像库
const PORTRAIT_MAP: Record<string, string> = {
  // 传奇大佬 (Local static portraits in /images/avatars/)
  'howard-marks': '/images/avatars/howard-marks.jpg',
  'david-einhorn': '/images/avatars/david-einhorn.jpg',
  'bill-nygren': '/images/avatars/bill-nygren.jpg',
  'lee-ainslie': '/images/avatars/lee-ainslie.jpg',
  'christopher-bloomstran': '/images/avatars/christopher-bloomstran.jpg',
  'mason-hawkins': '/images/avatars/mason-hawkins.jpg',
  'viking-global-investors': '/images/avatars/viking-global-investors.jpg',

  // 中文别名映射
  '霍华德马克斯': '/images/avatars/howard-marks.jpg',
  '大卫艾因霍恩': '/images/avatars/david-einhorn.jpg',
  '比尔尼格伦': '/images/avatars/bill-nygren.jpg',
  '李安斯利': '/images/avatars/lee-ainslie.jpg',
};

// 预置渐变与徽章样式
const PALETTES = [
  { bg: 'linear-gradient(135deg, #d97706 0%, #b45309 100%)', text: '#fef3c7', border: '#f59e0b' },
  { bg: 'linear-gradient(135deg, #2563eb 0%, #1d4ed8 100%)', text: '#dbeafe', border: '#3b82f6' },
  { bg: 'linear-gradient(135deg, #059669 0%, #047857 100%)', text: '#d1fae5', border: '#10b981' },
  { bg: 'linear-gradient(135deg, #7c3aed 0%, #6d28d9 100%)', text: '#ede9fe', border: '#8b5cf6' },
  { bg: 'linear-gradient(135deg, #e11d48 0%, #be123c 100%)', text: '#ffe4e6', border: '#f43f5e' },
  { bg: 'linear-gradient(135deg, #0891b2 0%, #0e7490 100%)', text: '#cffafe', border: '#06b6d4' },
  { bg: 'linear-gradient(135deg, #ea580c 0%, #c2410c 100%)', text: '#ffedd5', border: '#f97316' },
  { bg: 'linear-gradient(135deg, #4f46e5 0%, #4338ca 100%)', text: '#e0e7ff', border: '#6366f1' },
];

export const Avatar: React.FC<AvatarProps> = ({
  src,
  letter,
  size = 56,
  party,
  className = '',
  slug,
  name,
}) => {
  const [imgErrorCount, setImgErrorCount] = useState(0);

  // 清洗名称或 slug 为 lookup key
  const cleanKey = (slug || name || '')
    .toLowerCase()
    .trim()
    .replace(/\s+/g, '-')
    .replace(/[^a-z0-9\u4e00-\u9fa5-]/g, '');

  const effectiveSlug = slug || cleanKey;

  // 图像尝试链路: 1. 自定义/映射 -> 2. 本地 .svg -> 3. 本地 .jpg -> 4. 纯本地 CSS Monogram
  const getImageSource = () => {
    if (src) return src;
    if (PORTRAIT_MAP[cleanKey]) return PORTRAIT_MAP[cleanKey];
    if (imgErrorCount === 0 && effectiveSlug) return `/images/avatars/${effectiveSlug}.svg`;
    if (imgErrorCount === 1 && effectiveSlug) return `/images/avatars/${effectiveSlug}.jpg`;
    return null;
  };

  const imageSource = getImageSource();

  if (imageSource && imgErrorCount < 2) {
    return (
      <img
        src={imageSource}
        alt={name || letter || 'avatar'}
        className={`rounded-full object-cover shrink-0 border border-slate-700/80 bg-slate-900 shadow-md ${className}`}
        style={{ width: size, height: size }}
        onError={() => setImgErrorCount((prev) => prev + 1)}
      />
    );
  }

  // 提取高质量 Monogram 标识
  const computeMonogram = () => {
    const raw = (name || slug || letter || '').trim();
    if (!raw) return 'G';

    // 中文名提取
    const chineseMatch = raw.match(/[\u4e00-\u9fa5]/g);
    if (chineseMatch && chineseMatch.length >= 2) {
      return chineseMatch.slice(0, 2).join('');
    } else if (chineseMatch && chineseMatch.length === 1) {
      return chineseMatch[0];
    }

    // 英文名提取首字母组合 (如 Warren Buffett -> WB)
    const words = raw.replace(/[^a-zA-Z0-9\s]/g, ' ').split(/\s+/).filter(Boolean);
    if (words.length >= 2) {
      return (words[0][0] + words[1][0]).toUpperCase();
    }
    return raw.slice(0, 2).toUpperCase();
  };

  const monogram = computeMonogram();

  // 国会山党派专属配色
  if (party) {
    const isDem = party === 'D';
    const bg = isDem ? '#2563eb' : '#dc2626';
    const border = isDem ? '#60a5fa' : '#f87171';
    return (
      <div
        className={`rounded-full shrink-0 inline-flex items-center justify-center font-bold text-white shadow-md select-none ${className}`}
        style={{
          width: size,
          height: size,
          backgroundColor: bg,
          border: `2px solid ${border}`,
          fontSize: size > 44 ? 18 : size > 32 ? 14 : 11,
          boxShadow: `0 4px 12px ${bg}40`,
        }}
      >
        {letter?.charAt(0).toUpperCase() || (isDem ? 'D' : 'R')}
      </div>
    );
  }

  // 计算哈希分配专属高端渐变色
  const hash = Array.from(cleanKey || monogram || 'G').reduce((acc, char) => acc + char.charCodeAt(0), 0);
  const palette = PALETTES[Math.abs(hash) % PALETTES.length];

  return (
    <div
      className={`rounded-full shrink-0 inline-flex items-center justify-center font-black tracking-wider shadow-md select-none ${className}`}
      style={{
        width: size,
        height: size,
        background: palette.bg,
        color: palette.text,
        border: `2px solid ${palette.border}60`,
        fontSize: size > 48 ? (monogram.length > 2 ? 14 : 18) : size > 34 ? 13 : 10,
        boxShadow: '0 4px 12px rgba(0,0,0,0.3)',
      }}
    >
      {monogram}
    </div>
  );
};
