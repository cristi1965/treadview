import React, { useState } from 'react';
import { LockKeyhole, LogIn } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { StockGodShell } from '../components/layout/StockGodShell';

export const StockGodMacroGate: React.FC = () => {
  const navigate = useNavigate();
  const [token, setToken] = useState('');
  const [error, setError] = useState('');

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    if (!token.trim()) {
      setError('请输入访问口令');
      return;
    }

    sessionStorage.setItem('stockgod_macro_token', token.trim());
    navigate('/dashboard/macro');
  };

  return (
    <StockGodShell title="宏观驾驶舱">
      <main className="mx-auto flex min-h-[calc(100vh-180px)] w-full max-w-[760px] items-center px-4 py-10">
        <section className="w-full rounded-lg border border-line bg-surface/80 p-6 shadow-[0_18px_70px_rgba(0,0,0,0.28)] backdrop-blur md:p-8">
          <div className="mb-7 flex items-start gap-4">
            <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg border border-line bg-base text-accent">
              <LockKeyhole size={22} />
            </span>
            <div>
              <p className="mb-2 text-xs font-semibold uppercase tracking-[0.18em] text-faint">
                Private Macro Console
              </p>
              <h1 className="text-[28px] font-semibold leading-tight text-ink md:text-[34px]">
                宏观驾驶舱
              </h1>
              <p className="mt-3 text-sm leading-6 text-muted">
                私密工具 · 输入访问口令(与 /stats 同一个)
              </p>
            </div>
          </div>

          <form onSubmit={handleSubmit} className="flex flex-col gap-3 sm:flex-row">
            <label className="sr-only" htmlFor="macro-token">
              Token
            </label>
            <input
              id="macro-token"
              type="password"
              value={token}
              onChange={(event) => {
                setToken(event.target.value);
                setError('');
              }}
              placeholder="Token"
              className="h-11 min-w-0 flex-1 rounded-lg border border-line bg-base px-4 font-mono text-sm text-ink outline-none transition placeholder:text-faint focus:border-accent focus:ring-2 focus:ring-accent/20"
            />
            <button
              type="submit"
              className="inline-flex h-11 items-center justify-center gap-2 rounded-lg bg-accent px-5 text-sm font-semibold text-white transition hover:brightness-110"
            >
              <LogIn size={16} />
              进入
            </button>
          </form>

          {error && <p className="mt-3 text-sm text-down">{error}</p>}
        </section>
      </main>
    </StockGodShell>
  );
};
