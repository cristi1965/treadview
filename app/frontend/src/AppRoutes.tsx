import React, { Suspense } from 'react';
import { BrowserRouter, Routes, Route, Navigate, useLocation, useParams } from 'react-router-dom';
import { Layout, StockGodShell } from './components/layout';
import { RouteErrorBoundary } from './components/RouteErrorBoundary';
import { useWebSocket } from './hooks/useWebSocket';
import { SystemDataSyncController } from './components/SystemDataSyncController';
import { useI18n } from './i18n';

const lazyPage = <T, K extends keyof T>(loader: () => Promise<T>, name: K) =>
  React.lazy(async () => ({ default: (await loader())[name] as React.ComponentType<any> }));

const HomePage = lazyPage(() => import('./pages/Home'), 'Home');
const Scan = lazyPage(() => import('./pages/Scan'), 'Scan');
const ETF = lazyPage(() => import('./pages/ETF'), 'ETF');
const Reports = lazyPage(() => import('./pages/Reports'), 'Reports');
const Portfolio = lazyPage(() => import('./pages/Portfolio'), 'Portfolio');
const Arena = lazyPage(() => import('./pages/Arena'), 'Arena');
const Whales = lazyPage(() => import('./pages/Whales'), 'Whales');
const Dashboard = lazyPage(() => import('./pages/Dashboard'), 'Dashboard');
const Journal = lazyPage(() => import('./pages/Journal'), 'Journal');
const Notes = lazyPage(() => import('./pages/Notes'), 'Notes');
const Macro = lazyPage(() => import('./pages/Macro'), 'Macro');
const History = lazyPage(() => import('./pages/History'), 'History');
const Settings = lazyPage(() => import('./pages/Settings'), 'Settings');
const StockDetail = lazyPage(() => import('./pages/StockDetail'), 'StockDetail');
const MultiChart = lazyPage(() => import('./pages/MultiChart'), 'MultiChart');
const TacticalCommand = lazyPage(() => import('./pages/TacticalCommand'), 'TacticalCommand');
const TradingCopilot = lazyPage(() => import('./pages/TradingCopilot'), 'TradingCopilot');
const ResearchLab = lazyPage(() => import('./pages/ResearchLab'), 'ResearchLab');
const GPUPricesPage = lazyPage(() => import('./pages/GPUPrices'), 'GPUPrices');
const WhaleDetailPage = lazyPage(() => import('./pages/WhaleDetail'), 'WhaleDetailPage');
const PoliticianDetailPage = lazyPage(() => import('./pages/PoliticianDetail'), 'PoliticianDetailPage');
const StaticPage = lazyPage(() => import('./pages/StaticPage'), 'StaticPage');
const NotFoundPage = lazyPage(() => import('./pages/StaticPage'), 'NotFoundPage');

const WhaleDetailRoute: React.FC = () => {
  const { slug } = useParams<{ slug: string }>();
  const { t } = useI18n();
  return <StockGodShell title={t('whales.title')}><WhaleDetailPage slug={slug || ''} /></StockGodShell>;
};

const StockDetailRoute: React.FC = () => {
  const { symbol } = useParams<{ symbol: string }>();
  return <StockGodShell title={(symbol || 'STOCK').toUpperCase()}><StockDetail /></StockGodShell>;
};

const NotesRoute: React.FC = () => <Notes />;
const CockpitDashboard: React.FC = () => { const { t } = useI18n(); return <Layout title={t('layout.research')}><Dashboard /></Layout>; };
const CockpitLab: React.FC = () => { const { t } = useI18n(); return <Layout title={t('layout.lab')}><ResearchLab /></Layout>; };
const CockpitMacro: React.FC = () => { const { t } = useI18n(); return <Layout title={t('layout.macro')}><Macro /></Layout>; };
const CockpitJournal: React.FC = () => { const { t } = useI18n(); return <Layout title={t('layout.journal')}><Journal /></Layout>; };
const CockpitHistory: React.FC = () => { const { t } = useI18n(); return <Layout title={t('layout.history')}><History /></Layout>; };
const CockpitSettings: React.FC = () => { const { t } = useI18n(); return <Layout title={t('layout.settings')}><Settings /></Layout>; };
const CockpitTactical: React.FC = () => <Layout title="夜间不盯盘战术指挥所"><TacticalCommand /></Layout>;
const CockpitCopilot: React.FC = () => <Layout title="AI 交易问答终端"><TradingCopilot /></Layout>;

const AnalysisWebSocketController: React.FC = () => {
  const location = useLocation();
  useWebSocket(location.pathname === '/lab');
  return null;
};

const stockgod = (element: React.ReactNode) => <RouteErrorBoundary surface="stockgod">{element}</RouteErrorBoundary>;
const cockpit = (element: React.ReactNode) => <RouteErrorBoundary surface="cockpit">{element}</RouteErrorBoundary>;
const notes = (element: React.ReactNode) => <RouteErrorBoundary surface="notes">{element}</RouteErrorBoundary>;
const Home: React.FC = () => stockgod(<HomePage />);
const GPUPrices: React.FC = () => stockgod(<GPUPricesPage />);

export const AppRoutes: React.FC = () => (
  <BrowserRouter>
    <AnalysisWebSocketController />
    <SystemDataSyncController />
    <Suspense fallback={<div role="status" className="flex min-h-screen items-center justify-center bg-base text-muted">页面加载中…</div>}>
      <Routes>
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/market" element={<Home />} />
        <Route path="/copilot" element={cockpit(<CockpitCopilot />)} />
        <Route path="/scan" element={stockgod(<Scan />)} />
        <Route path="/multichart" element={stockgod(<MultiChart />)} />
        <Route path="/gpu-prices" element={<GPUPrices />} />
        <Route path="/tactical" element={cockpit(<CockpitTactical />)} />
        <Route path="/etf" element={stockgod(<ETF />)} />
        <Route path="/etf/:id" element={stockgod(<ETF />)} />
        <Route path="/reports" element={stockgod(<Reports />)} />
        <Route path="/macro" element={<Navigate to="/dashboard/macro" replace />} />
        <Route path="/portfolio" element={stockgod(<Portfolio />)} />
        <Route path="/arena" element={stockgod(<Arena />)} />
        <Route path="/notes" element={notes(<NotesRoute />)} />
        <Route path="/notes/:id" element={notes(<NotesRoute />)} />
        <Route path="/reports/:id" element={stockgod(<Reports />)} />
        <Route path="/about" element={stockgod(<StaticPage kind="about" />)} />
        <Route path="/terms" element={stockgod(<StaticPage kind="terms" />)} />
        <Route path="/privacy" element={stockgod(<StaticPage kind="privacy" />)} />
        <Route path="/how-to-buy" element={<Navigate to="/tactical" replace />} />
        <Route path="/whales" element={stockgod(<Whales />)} />
        <Route path="/whales/congress/:slug" element={stockgod(<PoliticianDetailPage />)} />
        <Route path="/whales/:slug" element={stockgod(<WhaleDetailRoute />)} />
        <Route path="/stock/:symbol" element={stockgod(<StockDetailRoute />)} />
        <Route path="/dashboard" element={cockpit(<CockpitDashboard />)} />
        <Route path="/lab" element={cockpit(<CockpitLab />)} />
        <Route path="/dashboard/macro" element={cockpit(<CockpitMacro />)} />
        <Route path="/journal" element={cockpit(<CockpitJournal />)} />
        <Route path="/history" element={cockpit(<CockpitHistory />)} />
        <Route path="/settings" element={cockpit(<CockpitSettings />)} />
        <Route path="*" element={stockgod(<NotFoundPage />)} />
      </Routes>
    </Suspense>
  </BrowserRouter>
);
