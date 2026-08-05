import React from 'react';

interface FilterButtonsProps {
  options: Array<{ label: string; value: string }>;
  active: string;
  onChange: (value: string) => void;
}

export const FilterButtons: React.FC<FilterButtonsProps> = ({ 
  options, 
  active, 
  onChange 
}) => {
  return (
    <div className="flex flex-wrap gap-2">
      {options.map(option => (
        <button
          key={option.value}
          onClick={() => onChange(option.value)}
          className={`inline-flex items-center gap-1.5 rounded-md px-3 py-2 sm:py-1.5 text-[13px] font-medium transition ${
            active === option.value
              ? 'bg-surface-3 text-ink'
              : 'text-muted hover:text-ink'
          }`}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
};
