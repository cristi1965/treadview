import React, { Component, ErrorInfo, ReactNode } from 'react';
import { RefreshCw, AlertTriangle, Home } from 'lucide-react';

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  public state: State = {
    hasError: false,
    error: null,
  };

  public static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('[ErrorBoundary caught an error]:', error, errorInfo);
  }

  private handleReset = () => {
    this.setState({ hasError: false, error: null });
    window.location.href = '/';
  };

  private handleReload = () => {
    this.setState({ hasError: false, error: null });
    window.location.reload();
  };

  public render() {
    if (this.state.hasError) {
      return (
        <div className="flex min-h-screen w-full flex-col items-center justify-center bg-[#090d14] px-4 text-slate-100">
          <div className="w-full max-w-md rounded-2xl border border-rose-500/30 bg-slate-900/90 p-6 text-center shadow-2xl shadow-rose-950/30 backdrop-blur-md">
            <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-rose-500/10 text-rose-400 ring-1 ring-rose-500/30">
              <AlertTriangle size={28} />
            </div>

            <h2 className="mt-4 text-lg font-bold text-slate-100">页面渲染异常</h2>
            <p className="mt-2 text-xs leading-relaxed text-slate-400">
              系统遇到未捕获的渲染异常。已自动为你保护本地数据，点击下方按钮即可一键恢复。
            </p>

            {this.state.error && (
              <div className="mt-3 max-h-24 overflow-y-auto rounded-lg bg-black/60 p-2.5 text-left font-mono text-[11px] text-rose-300/90 border border-slate-800">
                {this.state.error.message || String(this.state.error)}
              </div>
            )}

            <div className="mt-6 flex items-center justify-center gap-3">
              <button
                type="button"
                onClick={this.handleReload}
                className="flex items-center gap-2 rounded-xl bg-slate-800 px-4 py-2 text-xs font-semibold text-slate-200 hover:bg-slate-700 transition"
              >
                <RefreshCw size={14} /> 刷新重试
              </button>
              <button
                type="button"
                onClick={this.handleReset}
                className="flex items-center gap-2 rounded-xl bg-gradient-to-r from-amber-500 to-amber-600 px-5 py-2 text-xs font-semibold text-slate-950 shadow hover:brightness-110 transition"
              >
                <Home size={14} /> 返回首页
              </button>
            </div>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}
