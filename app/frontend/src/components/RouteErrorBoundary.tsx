import React from 'react';
import { Link, useLocation } from 'react-router-dom';
import { Layout, StockGodShell } from './layout';

type RouteSurface = 'cockpit' | 'stockgod' | 'notes';

interface BoundaryProps {
  children: React.ReactNode;
  resetKey: string;
  surface: RouteSurface;
}

interface BoundaryState { failed: boolean; }

const RecoveryContent: React.FC<{ onRetry: () => void; surface: RouteSurface }> = ({ onRetry, surface }) => (
  <section role="alert" className="mx-auto flex min-h-[420px] max-w-xl flex-col items-center justify-center px-6 text-center">
    <h1 className="text-xl font-semibold text-ink">页面暂时无法显示</h1>
    <p className="mt-2 text-sm text-muted">本页发生了意外错误。可以重试当前页面，或返回一个稳定入口。</p>
    <div className="mt-6 flex flex-wrap justify-center gap-3">
      <button type="button" onClick={onRetry} className="rounded-lg bg-accent px-4 py-2 text-sm font-semibold text-black">重试</button>
      <Link to={surface === 'cockpit' ? '/dashboard' : '/market'} className="rounded-lg border border-line px-4 py-2 text-sm text-muted">
        返回{surface === 'cockpit' ? '研究工作台' : '实验行情'}
      </Link>
    </div>
  </section>
);

class RouteErrorBoundaryCore extends React.Component<BoundaryProps, BoundaryState> {
  state: BoundaryState = { failed: false };
  static getDerivedStateFromError(): BoundaryState { return { failed: true }; }
  componentDidCatch(error: Error, info: React.ErrorInfo) { console.error('Route render failed:', error, info); }
  componentDidUpdate(previous: BoundaryProps) {
    if (this.state.failed && previous.resetKey !== this.props.resetKey) this.setState({ failed: false });
  }
  private retry = () => this.setState({ failed: false });
  render() {
    if (!this.state.failed) return this.props.children;
    const content = <RecoveryContent surface={this.props.surface} onRetry={this.retry} />;
    if (this.props.surface === 'cockpit') return <Layout title="页面恢复">{content}</Layout>;
    if (this.props.surface === 'stockgod') return <StockGodShell title="页面恢复">{content}</StockGodShell>;
    return <div className="min-h-screen bg-base text-ink">{content}</div>;
  }
}

export const RouteErrorBoundary: React.FC<{ children: React.ReactNode; surface: RouteSurface }> = ({ children, surface }) => {
  const location = useLocation();
  return <RouteErrorBoundaryCore resetKey={`${location.pathname}${location.search}${location.hash}`} surface={surface}>{children}</RouteErrorBoundaryCore>;
};
