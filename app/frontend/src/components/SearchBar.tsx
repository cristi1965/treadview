import React from 'react';
import { Search } from 'lucide-react';

interface SearchBarProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
}

export const SearchBar: React.FC<SearchBarProps> = ({ 
  value, 
  onChange, 
  placeholder = '搜代码 / 名称 / 板块…' 
}) => {
  return (
    <div className="relative w-full">
      <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-faint" />
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="w-full rounded-lg border border-line bg-surface py-1.5 pl-9 pr-12 text-sm placeholder-faint focus:border-faint focus:outline-none focus:ring-2 focus:ring-surface-2 focus:bg-surface transition"
      />
    </div>
  );
};
