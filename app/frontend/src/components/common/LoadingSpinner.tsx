import React from 'react';

interface LoadingSpinnerProps {
  size?: 'sm' | 'md' | 'lg';
  className?: string;
}

export const LoadingSpinner: React.FC<LoadingSpinnerProps> = ({
  size = 'md',
  className = ''
}) => {
  const sizeStyles = {
    sm: 'w-4 h-4 border-2',
    md: 'w-8 h-8 border-2',
    lg: 'w-12 h-12 border-3'
  };
  
  return (
    <div className={`flex items-center justify-center ${className}`}>
      <div
        className={`
          ${sizeStyles[size]}
          border-accent border-t-transparent
          rounded-full animate-spin
        `}
      />
    </div>
  );
};

export const LoadingPage: React.FC = () => {
  return (
    <div className="flex items-center justify-center min-h-screen">
      <LoadingSpinner size="lg" />
    </div>
  );
};
