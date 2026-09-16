import React from 'react';

interface InputProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  type?: 'text' | 'number' | 'email' | 'password';
  disabled?: boolean;
  className?: string;
  icon?: React.ReactNode;
  onFocus?: () => void;
  onBlur?: () => void;
}

export const Input: React.FC<InputProps> = ({
  value,
  onChange,
  placeholder = '',
  type = 'text',
  disabled = false,
  className = '',
  icon,
  onFocus,
  onBlur,
}) => {
  return (
    <div className={`relative ${className}`}>
      {icon && (
        <div className="absolute left-2.5 top-1/2 -translate-y-1/2 text-faint">
          {icon}
        </div>
      )}
      <input
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        disabled={disabled}
        onFocus={onFocus}
        onBlur={onBlur}
        className={`
          w-full rounded-lg border border-line bg-surface 
          py-1.5 text-sm text-ink placeholder-faint
          transition
          focus:border-faint focus:outline-none focus:ring-2 focus:ring-surface-2 focus:bg-surface
          disabled:opacity-50 disabled:cursor-not-allowed
          ${icon ? 'pl-9 pr-3' : 'px-3'}
        `}
      />
    </div>
  );
};
