import React, { useState } from 'react';
import { Stock } from '../../types/stocks';
import { Input } from '../common';
import { useDebounce } from '../../hooks/useDebounce';
import { get } from '../../utils/api';
import { loadUsMarketStocks } from '../../utils/stockgodData';

interface StockSelectorProps {
  value: Stock | null;
  onChange: (stock: Stock | null) => void;
  placeholder: string;
}

interface SearchResponse {
  results: Stock[];
}

export const StockSelector: React.FC<StockSelectorProps> = ({
  value,
  onChange,
  placeholder
}) => {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<Stock[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [showResults, setShowResults] = useState(false);
  
  const debouncedQuery = useDebounce(query, 300);

  React.useEffect(() => {
    if (debouncedQuery && debouncedQuery.length >= 2) {
      searchStocks(debouncedQuery);
    } else {
      setResults([]);
    }
  }, [debouncedQuery]);

  const searchStocks = async (q: string) => {
    setIsSearching(true);
    try {
      const all = await loadUsMarketStocks().catch(() => null);
      if (all) {
        const lower = q.toLowerCase();
        const local = all
          .filter((s) => s.symbol.toLowerCase().includes(lower) || s.name.toLowerCase().includes(lower))
          .slice(0, 12);
        if (local.length > 0) {
          setResults(local);
          setShowResults(true);
          return;
        }
      }
      const response = await get<SearchResponse>(`/api/stocks/search?q=${encodeURIComponent(q)}`);
      setResults(response.results || []);
      setShowResults(true);
    } catch (error) {
      console.error('Search failed:', error);
      setResults([]);
    } finally {
      setIsSearching(false);
    }
  };

  const handleSelect = (stock: Stock) => {
    onChange(stock);
    setQuery('');
    setResults([]);
    setShowResults(false);
  };

  const handleClear = () => {
    onChange(null);
    setQuery('');
    setResults([]);
  };

  if (value) {
    return (
      <div className="border border-line rounded-lg p-4 bg-surface-2">
        <div className="flex items-start justify-between mb-2">
          <div>
            <div className="font-mono text-lg font-bold text-ink">
              {value.symbol}
            </div>
            <div className="text-sm text-muted">{value.name}</div>
          </div>
          <button
            className="text-sm text-faint hover:text-accent transition"
            onClick={handleClear}
          >
            ✕
          </button>
        </div>
        <div className="text-sm text-muted">
          ${value.price.toFixed(2)} <span className={value.changePercent >= 0 ? 'text-up' : 'text-down'}>
            {value.changePercent >= 0 ? '+' : ''}{value.changePercent.toFixed(2)}%
          </span>
        </div>
      </div>
    );
  }

  return (
    <div className="relative">
      <Input
        type="text"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder={placeholder}
        onFocus={() => query && setShowResults(true)}
        onBlur={() => setTimeout(() => setShowResults(false), 200)}
      />
      
      {showResults && results.length > 0 && (
        <div className="absolute z-10 w-full mt-1 border border-line rounded-lg bg-surface shadow-lg max-h-60 overflow-auto">
          {results.map((stock) => (
            <button
              key={stock.symbol}
              className="w-full text-left px-4 py-3 hover:bg-surface-2 transition border-b border-line last:border-b-0"
              onClick={() => handleSelect(stock)}
            >
              <div className="font-mono font-semibold text-sm text-ink">
                {stock.symbol}
              </div>
              <div className="text-xs text-muted truncate">{stock.name}</div>
            </button>
          ))}
        </div>
      )}
      
      {isSearching && (
        <div className="absolute right-3 top-3 text-xs text-faint">
          搜索中...
        </div>
      )}
    </div>
  );
};
