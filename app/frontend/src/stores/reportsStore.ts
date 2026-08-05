import { create } from 'zustand';
import { Report, MarketEvent } from '../types/reports';
import { get as apiGet } from '../utils/api';

interface ReportsStore {
  // Data
  reports: Report[];
  marketEvents: MarketEvent[];
  expandedReportIds: Set<string>;
  loading: boolean;
  error: string | null;
  hasMore: boolean;
  total: number;
  
  // Actions
  fetchReports: (offset?: number) => Promise<void>;
  fetchMarketCalendar: () => Promise<void>;
  toggleReport: (id: string) => void;
  expandReport: (id: string) => void;
  collapseReport: (id: string) => void;
}

interface ReportsResponse {
  reports: Report[];
  total: number;
  hasMore: boolean;
}

interface MarketCalendarResponse {
  events: MarketEvent[];
}

export const useReportsStore = create<ReportsStore>((set, getState) => ({
  // Initial state
  reports: [],
  marketEvents: [],
  expandedReportIds: new Set(),
  loading: false,
  error: null,
  hasMore: true,
  total: 0,
  
  // Fetch reports
  fetchReports: async (offset = 0) => {
    set({ loading: true, error: null });
    
    try {
      const response = await apiGet<ReportsResponse>(
        `/api/reports?limit=5&offset=${offset}`
      );

      let reports = response.reports;
      if (offset === 0) {
        try {
          const latest = await fetch('/data/reports-latest.json').then((r) => (r.ok ? r.json() : null));
          if (latest?.id && !reports.some((r) => r.id === latest.id || (r.date === latest.date && r.title === latest.title))) {
            reports = [
              {
                id: latest.id,
                type: latest.type === 'pre' || latest.type === 'premarket' ? 'premarket' : 'postmarket',
                title: latest.title || latest.typeLabel || '最新盘报',
                date: latest.date || '',
                time: latest.timeET || '',
                summary: latest.title || '',
                content: latest.content || `> ${latest.title || '最新盘报'}\n\n来源: /data/reports-latest.json`,
                publishedAt: latest.publishedAt || latest.date || '',
              },
              ...reports,
            ];
          }
        } catch {
          // optional enrichment
        }
      }
      
      set((state) => ({
        reports: offset === 0 
          ? reports 
          : [...state.reports, ...response.reports],
        hasMore: response.hasMore,
        total: response.total,
        loading: false,
        expandedReportIds: offset === 0 && reports.length > 0
          ? new Set([reports[0].id])
          : state.expandedReportIds
      }));
    } catch (error) {
      console.error('Failed to fetch reports:', error);
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch reports',
        loading: false
      });
    }
  },
  
  // Fetch market calendar
  fetchMarketCalendar: async () => {
    try {
      const response = await apiGet<MarketCalendarResponse>('/api/market/calendar');
      set({ marketEvents: response.events });
    } catch (error) {
      console.error('Failed to fetch market calendar:', error);
    }
  },
  
  // Toggle report expand/collapse
  toggleReport: (id: string) => {
    set((state) => {
      const newSet = new Set(state.expandedReportIds);
      if (newSet.has(id)) {
        newSet.delete(id);
      } else {
        newSet.add(id);
      }
      return { expandedReportIds: newSet };
    });
  },
  
  // Expand a specific report
  expandReport: (id: string) => {
    set((state) => ({
      expandedReportIds: new Set(state.expandedReportIds).add(id)
    }));
  },
  
  // Collapse a specific report
  collapseReport: (id: string) => {
    set((state) => {
      const newSet = new Set(state.expandedReportIds);
      newSet.delete(id);
      return { expandedReportIds: newSet };
    });
  }
}));
