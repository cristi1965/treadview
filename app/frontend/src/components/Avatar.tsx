import React, { useState } from 'react';

interface AvatarProps {
  src?: string;
  letter?: string;
  size?: number;
  party?: 'D' | 'R';
  className?: string;
  slug?: string;
}

export const Avatar: React.FC<AvatarProps> = ({ 
  src, 
  letter, 
  size = 56, 
  party,
  className = '',
  slug
}) => {
  const [failed, setFailed] = useState(false);

  // If explicit src is empty but we have a slug, try loading the local customized image
  const avatarSrc = src || (slug ? `/images/avatars/${slug}.jpg` : undefined);

  // 如果有图片并且加载没有失败，显示图片头像
  if (avatarSrc && !failed) {
    return (
      <img
        src={avatarSrc}
        alt={letter || ''}
        className={`rounded-full object-cover border border-line bg-surface-2 ${className}`}
        style={{ width: size, height: size }}
        onError={() => setFailed(true)}
      />
    );
  }

  // 否则显示字母头像
  const bgColor = party === 'D' 
    ? 'bg-[#3B82F6]' 
    : party === 'R' 
    ? 'bg-[#EF4444]' 
    : 'bg-surface-3';

  const fontSize = size > 40 ? 'text-[22px]' : 'text-sm';

  return (
    <div
      className={`rounded-full inline-flex items-center justify-center font-semibold ${bgColor} text-white ${fontSize} ${className}`}
      style={{ width: size, height: size }}
    >
      {letter?.charAt(0).toUpperCase() || '?'}
    </div>
  );
};
