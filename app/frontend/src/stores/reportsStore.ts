import { create } from 'zustand';
import { Report, MarketEvent } from '../types/reports';
import { getWithMeta as apiGetWithMeta, type APIResponseMeta } from '../utils/api';

interface ReportsStore {
  // Data
  reports: Report[];
  marketEvents: MarketEvent[];
  expandedReportIds: Set<string>;
  loading: boolean;
  error: string | null;
  calendarError: string | null;
  reportsMeta: APIResponseMeta | null;
  calendarMeta: APIResponseMeta | null;
  hasMore: boolean;
  total: number;
  
  // Actions
  fetchReports: (offset?: number) => Promise<void>;
  fetchReportById: (id: string) => Promise<void>;
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

interface ReportDetailResponse {
  report: Report;
}

interface MarketCalendarResponse {
  events: MarketEvent[];
}

let reportsRequestSequence = 0;

export const useReportsStore = create<ReportsStore>((set, getState) => ({
  // Initial state
  reports: [],
  marketEvents: [],
  expandedReportIds: new Set(),
  loading: false,
  error: null,
  calendarError: null,
  reportsMeta: null,
  calendarMeta: null,
  hasMore: true,
  total: 0,
  
  // Fetch reports
  fetchReports: async (offset = 0) => {
    const requestSequence = ++reportsRequestSequence;
    set({ loading: true, error: null });
    try {
      const result = await apiGetWithMeta<ReportsResponse>(
        `/api/reports?limit=5&offset=${offset}`
      );
      const response = result.data;

      if (requestSequence !== reportsRequestSequence) return;

      set((state) => ({
        reports: offset === 0
          ? response.reports
          : [...state.reports, ...response.reports],
        hasMore: response.hasMore,
        total: response.total,
        reportsMeta: result.meta,
        loading: false,
        expandedReportIds: offset === 0 && response.reports.length > 0
          ? new Set([response.reports[0].id])
          : state.expandedReportIds
      }));
    } catch (error) {
      if (requestSequence !== reportsRequestSequence) return;
      console.error('Failed to fetch reports:', error);
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch reports',
        loading: false
      });
    }
  },

  fetchReportById: async (id) => {
    const requestSequence = ++reportsRequestSequence;
    set({
      loading: true,
      error: null,
      reports: [],
      reportsMeta: null,
      expandedReportIds: new Set(),
    });
    try {
      const result = await apiGetWithMeta<ReportDetailResponse>(`/api/reports/${encodeURIComponent(id)}`);
      const report = result.data.report;
      if (!report?.id || !report.date) throw new Error('报告详情返回格式无效');
      if (requestSequence !== reportsRequestSequence) return;
      set((state) => ({
        reports: [report, ...state.reports.filter((item) => item.id !== report.id)],
        reportsMeta: result.meta,
        loading: false,
        expandedReportIds: new Set(state.expandedReportIds).add(report.id),
      }));
    } catch (error) {
      if (requestSequence !== reportsRequestSequence) return;
      set({
        error: error instanceof Error ? error.message : 'Failed to load report',
        loading: false,
      });
    }
  },
  
  // Fetch market calendar
  fetchMarketCalendar: async () => {
    set({ calendarError: null });
    try {
      const result = await apiGetWithMeta<MarketCalendarResponse>('/api/market/calendar');
      set({ marketEvents: result.data.events, calendarMeta: result.meta, calendarError: null });
    } catch (error) {
      console.error('Failed to fetch market calendar:', error);
      set({ calendarError: error instanceof Error ? error.message : '日历加载失败', calendarMeta: null });
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
