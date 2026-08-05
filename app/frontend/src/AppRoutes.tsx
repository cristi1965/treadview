import React from 'react';
import { BrowserRouter, Routes, Route, Navigate, useParams } from 'react-router-dom';
import {
  Home,
  Scan,
  ETF,
  Reports,
  Portfolio,
  Arena,
  Whales,
  Dashboard,
  Journal,
  Notes,
  Macro,
  StockGodMacroGate,
  History,
  Settings,
  StockDetail
} from './pages';
import { WhaleDetailPage } from './pages/WhaleDetail';
import { PoliticianDetailPage } from './pages/PoliticianDetail';
import { NotFoundPage, StaticPage } from './pages/StaticPage';
import { Layout, StockGodShell } from './components/layout';
import { useWebSocket } from './hooks/useWebSocket';

const WhaleDetailRoute: React.FC = () => {
  const { slug } = useParams<{ slug: string }>();
  return (
    <StockGodShell title="聪明钱">
      <WhaleDetailPage slug={slug || ''} />
    </StockGodShell>
  );
};

const StockDetailRoute: React.FC = () => {
  const { symbol } = useParams<{ symbol: string }>();
  return (
    <StockGodShell title={(symbol || '股票').toUpperCase()}>
      <StockDetail />
    </StockGodShell>
  );
};

const NotesRoute: React.FC = () => <Notes />;

export const AppRoutes: React.FC = () => {
  useWebSocket();

  return (
    <BrowserRouter>
      <Routes>
        {/* StockGod.xyz Routes */}
        <Route path="/" element={<Home />} />
        <Route path="/scan" element={<Scan />} />
        <Route path="/etf" element={<ETF />} />
        <Route path="/etf/:id" element={<ETF />} />
        <Route path="/reports" element={<Reports />} />
        <Route path="/macro" element={<StockGodMacroGate />} />
        <Route path="/portfolio" element={<Portfolio />} />
        <Route path="/arena" element={<Arena />} />
        <Route path="/notes" element={<NotesRoute />} />
        <Route path="/notes/:id" element={<NotesRoute />} />
        <Route path="/reports/:id" element={<Reports />} />
        <Route path="/about" element={<StaticPage kind="about" />} />
        <Route path="/terms" element={<StaticPage kind="terms" />} />
        <Route path="/privacy" element={<StaticPage kind="privacy" />} />
        <Route path="/how-to-buy" element={<StaticPage kind="how-to-buy" />} />
        
        <Route path="/whales" element={<Whales />} />
        <Route path="/whales/congress/:slug" element={<PoliticianDetailPage />} />
        <Route path="/whales/:slug" element={<WhaleDetailRoute />} />
        <Route path="/stock/:symbol" element={<StockDetailRoute />} />
        
        {/* TradingAgents cockpit — keep Layout shell */}
        <Route path="/dashboard" element={<Layout title="Strategy Evaluation"><Dashboard /></Layout>} />
        <Route path="/dashboard/macro" element={<Layout title="Macro Calendar"><Macro /></Layout>} />
        <Route path="/journal" element={<Layout title="Trade Journal"><Journal /></Layout>} />
        <Route path="/history" element={<Layout title="History Logs"><History /></Layout>} />
        <Route path="/settings" element={<Layout title="Settings"><Settings /></Layout>} />
        
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </BrowserRouter>
  );
};
