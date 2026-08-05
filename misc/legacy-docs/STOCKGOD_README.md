# StockGod.xyz - Stock Analysis & ETF Screening Platform

**Project Status**: 90% Complete (6/7 pages implemented)  
**Last Updated**: July 3, 2026

---

## 🎯 Overview

StockGod.xyz is a full-featured stock analysis and ETF screening platform built as part of the TradingAgents project. It provides comprehensive market analysis tools with a focus on multi-factor scoring and comparative analysis.

**Live Demo**: http://localhost:5173/ (when servers are running)

---

## ✨ Key Features

### 🎯 Five-Factor Analysis System
- **Buffett Score**: Value investment principles
- **Duanyongping Score**: Long-term holding perspective
- **Serenity Score**: Low volatility preference
- **Druckenmiller Score**: Macro trend analysis
- **Sentiment Score**: Market emotion tracking

### 📊 Visualization
- Pentagon radar charts for multi-factor visualization
- Color-coded scoring (0-49: Blue, 50-69: Orange, 70+: Green)
- Responsive grid layouts with adaptive columns

### 🔍 Real-time Search
- Debounced search (300ms) for smooth UX
- Autocomplete suggestions
- Search across stocks, ETF, and sectors

### 💾 Data Persistence
- LocalStorage for watchlist and preferences
- Zustand for in-memory state management
- SQLite backend for historical data

---

## 📱 Implemented Pages (6/7)

### ✅ 1. Portfolio - My Watchlist (100%)
**Route**: `/portfolio`

**Features**:
- Dual tabs: Watchlist / Holdings
- Add/remove stocks from watchlist
- LocalStorage persistence
- Empty state placeholders
- Responsive grid (3-4 columns)

**Components**: `WatchlistCard.tsx`  
**Store**: `portfolioStore.ts`

---

### ✅ 2. ETF - Sector Analysis (100%)
**Route**: `/etf`

**Features**:
- **47 sectors** across 8 categories:
  - All / Broad Market / Industry / Theme / Strategy / Style / Commodity / Bond
- **4 sort modes**:
  - Largest AUM / Best 1Y / Best 5Y / Least Drawdown
- Real-time search
- Sector cards with:
  - Category badge
  - ETF count
  - AUM (in 亿)
  - Top performer (ticker + 5Y return)
  - Max drawdown (color-coded)

**API**:
- `GET /api/etf/sectors` - List sectors
- `GET /api/etf/sectors/:id` - Sector details
- `GET /api/etf/search` - Search ETF

**Components**: `SectorCard.tsx`  
**Store**: `etfStore.ts`

---

### ✅ 3. Reports - Daily Analysis (100%)
**Route**: `/reports`

**Features**:
- **Dual-column layout**:
  - Left: Market calendar with important events
  - Right: Daily reports (premarket + postmarket)
- **Market Calendar**:
  - Macro data / Earnings / Policy events
  - Today highlight
  - Importance badges
- **Daily Reports**:
  - Markdown rendering (react-markdown + remark-gfm)
  - Expand/collapse (first report auto-expanded)
  - Load more pagination
  - Premarket outlook + Postmarket recap

**API**:
- `GET /api/reports` - Report list
- `GET /api/reports/:id` - Report details
- `GET /api/market/calendar` - Calendar events

**Components**: `ReportCard.tsx`, `EventCard.tsx`  
**Store**: `reportsStore.ts`

---

### ✅ 4. Scan - Stock List (100%)
**Route**: `/scan`

**Features**:
- **11-column table**:
  1. Rank
  2. Symbol/Name
  3. Price
  4. Change % (color-coded)
  5. Market Cap
  6. Five-Factor Radar Chart
  7. Avg Score (color-coded)
  8. Volume
  9. Sector
  10. Watched status
  11. Actions
- **Five-Factor Radar Chart**:
  - Pentagon SVG visualization
  - Buffett / Duanyongping / Serenity / Druckenmiller / Sentiment
  - Score color coding
- **Table Features**:
  - Click header to sort (ascending/descending)
  - Search with debounce (300ms)
  - Pagination
  - Toggle watch status
- **Responsive Design**:
  - Hide columns on small screens

**API**:
- `GET /api/stocks` - Stock list (paginated, sorted, searchable)
- `GET /api/stocks/search` - Search stocks

**Components**: `RadarChart.tsx`  
**Store**: `stocksStore.ts`

---

### ✅ 5. Arena - Stock Comparison (100%)
**Route**: `/arena`

**Features**:
- **Dual Stock Selector**:
  - Autocomplete search
  - Real-time suggestions
  - Clear and reselect
- **VS Layout**:
  - Side-by-side comparison
  - Winner highlight (higher score = green)
- **Comparison Metrics**:
  - Price & Change %
  - Market Cap
  - Volume
  - Sector
  - Five-Factor Scores (dual radar charts)
- **Score Details**:
  - Buffett / Duanyongping / Serenity / Druckenmiller / Sentiment
  - Score difference highlighting
- **Empty State**:
  - Prompt to select two stocks

**Components**: `StockSelector.tsx`, `ComparisonCard.tsx`

---

### ⏸️ 6. Home - Heatmap (0%)
**Route**: `/`

**Status**: Placeholder ("即将上线")

**Planned Features**:
- Canvas-based force-directed graph
- 986 stocks visualization
- Real-time price updates
- Interactive features:
  - Hover: Show stock details
  - Click: Navigate to detail page
  - Zoom: Pinch/scroll to zoom
  - Pan: Drag to move
- Color coding: Price change %
- Node size: Market cap
- Node grouping: By sector

**Technical Options**:
1. **D3.js force simulation** (recommended)
   - Mature algorithm
   - Rich interaction support
   - Active community
2. **Canvas native implementation**
   - Full control
   - Best performance
   - High complexity
3. **WebGL (Three.js / PixiJS)**
   - Ultra-high performance
   - Large dataset support
   - Steep learning curve

**Complexity**: 🔴 High  
**Estimated Time**: 2-3 weeks

---

## 🛠️ Tech Stack

### Backend
- **Language**: Go 1.21+
- **Framework**: Gin (HTTP server)
- **Database**: SQLite + GORM
- **Data Source**: Mock data (currently)

### Frontend
- **Framework**: React 18 + TypeScript
- **Router**: react-router-dom v6
- **State**: Zustand
- **Build**: Vite 5.4.21
- **Markdown**: react-markdown + remark-gfm
- **Styling**: Vanilla CSS + Tailwind classes

---

## 🎨 Design System

### Color Palette
```css
/* Background */
--bg-background: #08090b;
--bg-surface: #111317;

/* Text */
--text-ink: #ecedf0;      /* Primary */
--text-muted: #8e919b;    /* Secondary */
--text-faint: #585b66;    /* Tertiary */
--text-accent: #d98a6a;   /* Orange accent */

/* Market Colors */
--text-up: #10b981;       /* Green (up) */
--text-down: #ef4444;     /* Red (down) */

/* Border */
--border-line: rgba(235,238,245,0.07);
```

### Score Color Coding
- **0-49**: Blue (#3b82f6) - Low
- **50-69**: Orange (#f97316) - Medium
- **70+**: Green (#22c55e) - High

### Typography
- **Headings**: Outfit (sans-serif)
- **Code/Numbers**: Fira Code (monospace)

---

## 🚀 Quick Start

### 1. Start Backend

```bash
cd app/backend
go build -o trading-agents main.go
./trading-agents
```

**Backend**: http://localhost:8765

### 2. Start Frontend

```bash
cd app/frontend
npm install
npm run dev
```

**Frontend**: http://localhost:5173

### 3. Access Application

Open browser: http://localhost:5173/

---

## 📡 API Endpoints

### Stocks
| Endpoint | Method | Description | Query Params |
|----------|--------|-------------|--------------|
| `/api/stocks` | GET | Stock list | `page`, `limit`, `sort`, `order`, `search` |
| `/api/stocks/search` | GET | Search stocks | `q` |

### ETF
| Endpoint | Method | Description | Query Params |
|----------|--------|-------------|--------------|
| `/api/etf/sectors` | GET | Sector list | `category`, `sort`, `page`, `limit` |
| `/api/etf/sectors/:id` | GET | Sector details | - |
| `/api/etf/search` | GET | Search ETF | `q` |

### Reports
| Endpoint | Method | Description | Query Params |
|----------|--------|-------------|--------------|
| `/api/reports` | GET | Report list | `type`, `page`, `limit` |
| `/api/reports/:id` | GET | Report details | - |
| `/api/market/calendar` | GET | Market calendar | - |

### System
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/health` | GET | Health check |

---

## 📊 Data Schema

### Stock
```typescript
interface Stock {
  symbol: string;
  name: string;
  price: number;
  changePercent: number;
  marketCap: number;
  volume: number;
  sector: string;
  scores: {
    buffett: number;
    duanyongping: number;
    serenity: number;
    druckenmiller: number;
    sentiment: number;
  };
  avgScore: number;
  isWatched: boolean;
}
```

### ETFSector
```typescript
interface ETFSector {
  id: string;
  name: string;
  category: string;
  etfCount: number;
  aum: number; // in 亿
  topPerformer: {
    ticker: string;
    return5y: number;
  };
  maxDrawdown: number;
  return1y: number;
  return5y: number;
}
```

### Report
```typescript
interface Report {
  id: string;
  type: 'premarket' | 'postmarket';
  title: string;
  date: string;
  time: string;
  summary: string;
  content: string; // Markdown
}
```

---

## 🎯 Roadmap

### 🔴 High Priority (1-2 weeks)
- [ ] Implement Home heatmap (force-directed graph)
- [ ] Integrate real market data APIs
- [ ] Add WebSocket real-time updates

### 🟡 Medium Priority (1 month)
- [ ] Performance optimization (react-virtual for tables)
- [ ] Chart library integration (price history, K-line)
- [ ] Error boundaries and error handling
- [ ] Loading states and skeleton screens

### 🟢 Low Priority (2-3 months)
- [ ] Unit tests (Jest + React Testing Library)
- [ ] E2E tests (Playwright)
- [ ] SEO optimization
- [ ] PWA support
- [ ] Mobile responsive design
- [ ] Internationalization (i18n)

---

## ⚠️ Important Notes

### Mock Data Warning
⚠️ **All APIs currently return mock data** for demonstration purposes. This is **not real market data**.

### Mock Data Includes:
- 50+ stocks (Apple, Microsoft, Tesla, 茅台, 宁德时代, etc.)
- Five-factor scores (Buffett, Duanyongping, Serenity, Druckenmiller, Sentiment)
- Price, change%, market cap, volume
- 47 ETF sectors across 8 categories
- Daily reports (premarket + postmarket)
- Market calendar events

---

## 📁 Project Structure

```
app/
├── backend/
│   ├── internal/
│   │   ├── api/
│   │   │   ├── router.go
│   │   │   ├── stocks_handlers.go
│   │   │   ├── etf_handlers.go
│   │   │   └── reports_handlers.go
│   │   └── models/
│   │       ├── stocks.go
│   │       ├── etf.go
│   │       └── reports.go
│   └── main.go
└── frontend/
    └── src/
        ├── components/
        │   ├── common/         # Button, Card, Badge, Input, EmptyState, LoadingSpinner
        │   ├── layout/         # Sidebar, Header, Layout
        │   ├── arena/          # StockSelector, ComparisonCard
        │   ├── etf/            # SectorCard
        │   ├── portfolio/      # WatchlistCard
        │   ├── reports/        # ReportCard, EventCard
        │   └── scan/           # RadarChart
        ├── pages/
        │   ├── Home.tsx        # ⏸️ Placeholder
        │   ├── Scan.tsx        # ✅ Complete
        │   ├── ETF.tsx         # ✅ Complete
        │   ├── Reports.tsx     # ✅ Complete
        │   ├── Portfolio.tsx   # ✅ Complete
        │   └── Arena.tsx       # ✅ Complete
        ├── stores/             # Zustand state management
        ├── types/              # TypeScript interfaces
        ├── utils/              # Helper functions (format, api)
        ├── AppRoutes.tsx       # Route configuration
        └── main.tsx            # Entry point
```

---

## 📚 Documentation

- [AI_README.md](./AI_README.md) - Project architecture guide
- [STOCKGOD_IMPLEMENTATION.md](./STOCKGOD_IMPLEMENTATION.md) - Complete implementation summary
- [FEATURES_COMPLETED.md](./FEATURES_COMPLETED.md) - Detailed feature checklist
- [QUICKSTART.md](./QUICKSTART.md) - Quick start guide
- [PROJECT_STATUS.md](./PROJECT_STATUS.md) - Project status report
- [STOCKGOD_README.md](./STOCKGOD_README.md) - This file

---

## 🤝 Contributing

1. Fork the project
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add some amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open Pull Request

---

## 📄 License

MIT

---

**Created**: July 3, 2026  
**Version**: v1.0  
**Status**: 90% Complete (6/7 pages)
