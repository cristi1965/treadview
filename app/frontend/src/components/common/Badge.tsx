import React from 'react';

interface BadgeProps {
  children: React.ReactNode;
  variant?: 'default' | 'success' | 'warning' | 'error' | 'info';
  size?: 'sm' | 'md';
  className?: string;
}

export const Badge: React.FC<BadgeProps> = ({
  children,
  variant = 'default',
  size = 'md',
  className = ''
}) => {
  const baseStyles = 'inline-flex items-center rounded-md font-medium';
  
  const variantStyles = {
    default: 'bg-surface-2 text-muted border border-line',
    success: 'bg-green-500/15 text-green-500 border border-green-500/30',
    warning: 'bg-amber-500/15 text-amber-500 border border-amber-500/30',
    error: 'bg-red-500/15 text-red-500 border border-red-500/30',
    info: 'bg-blue-500/15 text-blue-500 border border-blue-500/30'
  };
  
  const sizeStyles = {
    sm: 'px-1.5 py-0.5 text-[10px]',
    md: 'px-2 py-1 text-xs'
  };
  
  return (
    <span className={`${baseStyles} ${variantStyles[variant]} ${sizeStyles[size]} ${className}`}>
      {children}
    </span>
  );
};
