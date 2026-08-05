import React from 'react';

interface CardProps {
  children: React.ReactNode;
  onClick?: () => void;
  className?: string;
  hover?: boolean;
}

export const Card: React.FC<CardProps> = ({
  children,
  onClick,
  className = '',
  hover = true
}) => {
  const baseStyles = 'border border-line bg-surface rounded-xl transition';
  const hoverStyles = hover ? 'hover:bg-surface-2 hover:border-accent/20 cursor-pointer' : '';
  const clickableStyles = onClick ? 'cursor-pointer' : '';
  
  return (
    <div
      onClick={onClick}
      className={`${baseStyles} ${hoverStyles} ${clickableStyles} ${className}`}
    >
      {children}
    </div>
  );
};
