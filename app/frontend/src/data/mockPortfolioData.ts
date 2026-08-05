import { WatchlistItem } from '../types/portfolio';

// Mock 观察列表数据 (用于演示)
export const mockWatchlistData: WatchlistItem[] = [
  {
    symbol: 'NVDA',
    name: 'NVIDIA Corporation',
    addedAt: new Date('2024-01-15'),
    price: 194.83,
    changePercent: -1.39,
    avgScore: 66
  },
  {
    symbol: 'AAPL',
    name: 'Apple Inc.',
    addedAt: new Date('2024-01-10'),
    price: 308.63,
    changePercent: 4.84,
    avgScore: 60
  },
  {
    symbol: 'MSFT',
    name: 'Microsoft Corporation',
    addedAt: new Date('2024-01-08'),
    price: 390.49,
    changePercent: 1.62,
    avgScore: 70
  },
  {
    symbol: 'GOOGL',
    name: 'Alphabet Inc. Class A',
    addedAt: new Date('2024-01-05'),
    price: 359.91,
    changePercent: -0.36,
    avgScore: 71
  }
];
