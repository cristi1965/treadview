import React, { useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import { useStocksStore } from '../stores/stocksStore';
import { useWhalesStore } from '../stores/whalesStore';
import { useReportsStore } from '../stores/reportsStore';
import { useFlashStore } from '../stores/flashStore';
import { useETFStore } from '../stores/etfStore';
import { usePortfolioStore } from '../stores/portfolioStore';
import { get as apiGet } from '../utils/api';
import { useAlertsStore } from '../stores/alertsStore';
import { fetchQuoteResult } from '../utils/liveQuotes';

/**
 * SystemDataSyncController 只刷新当前路由需要的数据，避免全站并发请求拖慢首屏。
 * 3. Tab 窗口获得焦点 (focus) 时，自动重拉当前页面最新数据与快照；
 * 4. 每 30 秒后台轮询更新实时报价与快讯。
 */
export const SystemDataSyncController: React.FC = () => {
  const location = useLocation();
  const researchOnlyRoute = ['/', '/dashboard', '/history', '/lab', '/copilot', '/tactical', '/journal', '/settings']
    .some((path) => location.pathname === path || location.pathname.startsWith(`${path}/`));
  // 针对当前活动页面，立刻触发特定 Store 刷新。
  const refreshActivePageData = (pathname: string) => {
    if (pathname === '/market') {
      void useStocksStore.getState().fetchStocks();
      window.dispatchEvent(new Event('stockgod:home-data-refresh'));
    } else if (pathname.startsWith('/scan')) {
      void useStocksStore.getState().fetchStocks();
    } else if (pathname === '/whales') {
      void useWhalesStore.getState().refreshWhalesPage();
    } else if (pathname.startsWith('/reports')) {
      if (pathname === '/reports') void useReportsStore.getState().fetchReports(0);
      void useReportsStore.getState().fetchMarketCalendar();
      void useFlashStore.getState().fetchFlash();
    } else if (pathname.startsWith('/etf')) {
      void useETFStore.getState().fetchSectors(true);
    } else if (pathname.startsWith('/portfolio')) {
      usePortfolioStore.getState().loadWatchlist();
      usePortfolioStore.getState().loadHoldings();
    } else if (pathname.startsWith('/arena')) {
      void apiGet('/api/arena').catch(() => {});
    }
  };

  // 2. 路由切换时，重新获取当前页最新数据
  useEffect(() => {
    // /whales owns its initial filter-aware load; focus/manual refresh uses the same store action below.
    if (location.pathname !== '/whales' && location.pathname !== '/market' && !location.pathname.startsWith('/reports') && !location.pathname.startsWith('/etf')) {
      refreshActivePageData(location.pathname);
    }
  }, [location.pathname]);

  // Window focus refreshes page data. Quote polling is owned by each active surface.
  useEffect(() => {
    let alertInFlight = false;
    const checkPriceAlerts = async () => {
      if (alertInFlight || document.hidden) return;
      const alertsState = useAlertsStore.getState();
      const symbols = [...new Set(alertsState.alerts.filter((item) => item.enabled).map((item) => item.symbol))];
      if (!symbols.length) return;
      alertInFlight = true;
      alertsState.setMonitorChecking();
      try {
        const result = await fetchQuoteResult(symbols);
        const hasProvenance = Boolean(result.meta.source && result.meta.dataTime);
        const usable = !result.meta.stale && hasProvenance && Object.keys(result.quotes).length > 0;
        const details = [...result.errors, ...(result.missing.length ? [`缺少报价: ${result.missing.join(', ')}`] : [])];
        useAlertsStore.getState().checkQuotes(result.quotes, {
          usable,
          dataTime: result.meta.dataTime,
          source: result.meta.source,
          error: details.join('；') || (!hasProvenance ? '报价来源或数据时间未知' : undefined),
        });
      } catch (error) {
        useAlertsStore.getState().setMonitorError(error instanceof Error ? error.message : '提醒报价检查失败');
      } finally {
        alertInFlight = false;
      }
    };

    const handleSync = () => {
      if (!document.hidden) {
        refreshActivePageData(location.pathname);
        if (!researchOnlyRoute) void checkPriceAlerts();
      }
    };

    window.addEventListener('focus', handleSync);
    window.addEventListener('stockgod:manual-refresh', handleSync);
    document.addEventListener('visibilitychange', handleSync);
    if (!researchOnlyRoute) void checkPriceAlerts();

    const interval = setInterval(() => {
      if (!document.hidden && !researchOnlyRoute) {
        void useFlashStore.getState().fetchFlash();
        void checkPriceAlerts();
      }
    }, 12000);

    return () => {
      window.removeEventListener('focus', handleSync);
      window.removeEventListener('stockgod:manual-refresh', handleSync);
      document.removeEventListener('visibilitychange', handleSync);
      clearInterval(interval);
    };
  }, [location.pathname, researchOnlyRoute]);

  return null;
};
