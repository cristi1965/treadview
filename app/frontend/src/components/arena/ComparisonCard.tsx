import React from 'react';
import { Stock } from '../../types/stocks';
import { RadarChart } from '../scan';
import { formatPrice, formatPercent, formatAUM } from '../../utils/format';

interface ComparisonCardProps {
  stockA: Stock;
  stockB: Stock;
}

export const ComparisonCard: React.FC<ComparisonCardProps> = ({ stockA, stockB }) => {
  const getScoreColor = (score: number): string => {
    if (score >= 70) return 'text-up';
    if (score >= 50) return 'text-accent';
    return 'text-faint';
  };

  const compareValue = (a: number, b: number): 'A' | 'B' | 'tie' => {
    if (Math.abs(a - b) < 0.01) return 'tie';
    return a > b ? 'A' : 'B';
  };

  return (
    <div className="space-y-6">
      {/* 五方评分对比 */}
      <section className="border border-line rounded-xl p-6 bg-surface">
        <h3 className="text-lg font-semibold text-ink mb-4">五方评分对比</h3>
        
        <div className="grid grid-cols-2 gap-8">
          {/* Stock A */}
          <div>
            <div className="text-center mb-4">
              <RadarChart scores={stockA.scores} size={120} />
            </div>
            <div className="space-y-2">
              <div className="flex justify-between text-sm">
                <span className="text-muted">巴菲特</span>
                <span className={`font-mono font-semibold ${getScoreColor(stockA.scores.buffett)}`}>
                  {stockA.scores.buffett}
                </span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted">段永平</span>
                <span className={`font-mono font-semibold ${getScoreColor(stockA.scores.duanyongping)}`}>
                  {stockA.scores.duanyongping}
                </span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted">Serenity</span>
                <span className={`font-mono font-semibold ${getScoreColor(stockA.scores.serenity)}`}>
                  {stockA.scores.serenity}
                </span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted">德鲁肯米勒</span>
                <span className={`font-mono font-semibold ${getScoreColor(stockA.scores.druckenmiller)}`}>
                  {stockA.scores.druckenmiller}
                </span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted">情绪资金面</span>
                <span className={`font-mono font-semibold ${getScoreColor(stockA.scores.sentiment)}`}>
                  {stockA.scores.sentiment}
                </span>
              </div>
              <div className="flex justify-between text-sm pt-2 border-t border-line">
                <span className="text-ink font-semibold">平均分</span>
                <span className={`font-mono text-lg font-bold ${getScoreColor(stockA.avgScore)}`}>
                  {Math.round(stockA.avgScore)}
                </span>
              </div>
            </div>
          </div>
          
          {/* Stock B */}
          <div>
            <div className="text-center mb-4">
              <RadarChart scores={stockB.scores} size={120} />
            </div>
            <div className="space-y-2">
              <div className="flex justify-between text-sm">
                <span className="text-muted">巴菲特</span>
                <span className={`font-mono font-semibold ${getScoreColor(stockB.scores.buffett)}`}>
                  {stockB.scores.buffett}
                </span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted">段永平</span>
                <span className={`font-mono font-semibold ${getScoreColor(stockB.scores.duanyongping)}`}>
                  {stockB.scores.duanyongping}
                </span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted">Serenity</span>
                <span className={`font-mono font-semibold ${getScoreColor(stockB.scores.serenity)}`}>
                  {stockB.scores.serenity}
                </span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted">德鲁肯米勒</span>
                <span className={`font-mono font-semibold ${getScoreColor(stockB.scores.druckenmiller)}`}>
                  {stockB.scores.druckenmiller}
                </span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted">情绪资金面</span>
                <span className={`font-mono font-semibold ${getScoreColor(stockB.scores.sentiment)}`}>
                  {stockB.scores.sentiment}
                </span>
              </div>
              <div className="flex justify-between text-sm pt-2 border-t border-line">
                <span className="text-ink font-semibold">平均分</span>
                <span className={`font-mono text-lg font-bold ${getScoreColor(stockB.avgScore)}`}>
                  {Math.round(stockB.avgScore)}
                </span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* 基本指标对比 */}
      <section className="border border-line rounded-xl p-6 bg-surface">
        <h3 className="text-lg font-semibold text-ink mb-4">基本指标对比</h3>
        
        <div className="space-y-3">
          <div className="grid grid-cols-3 gap-4 text-sm">
            <div className="text-muted">指标</div>
            <div className="text-center font-mono text-ink">{stockA.symbol}</div>
            <div className="text-center font-mono text-ink">{stockB.symbol}</div>
          </div>
          
          <div className="grid grid-cols-3 gap-4 text-sm py-2 border-t border-line">
            <div className="text-muted">当前价格</div>
            <div className={`text-center font-mono ${compareValue(stockA.price, stockB.price) === 'A' ? 'font-bold text-ink' : 'text-muted'}`}>
              {formatPrice(stockA.price)}
            </div>
            <div className={`text-center font-mono ${compareValue(stockB.price, stockA.price) === 'B' ? 'font-bold text-ink' : 'text-muted'}`}>
              {formatPrice(stockB.price)}
            </div>
          </div>
          
          <div className="grid grid-cols-3 gap-4 text-sm py-2 border-t border-line">
            <div className="text-muted">涨跌幅</div>
            <div className={`text-center font-mono font-semibold ${stockA.changePercent >= 0 ? 'text-up' : 'text-down'}`}>
              {formatPercent(stockA.changePercent)}
            </div>
            <div className={`text-center font-mono font-semibold ${stockB.changePercent >= 0 ? 'text-up' : 'text-down'}`}>
              {formatPercent(stockB.changePercent)}
            </div>
          </div>
          
          <div className="grid grid-cols-3 gap-4 text-sm py-2 border-t border-line">
            <div className="text-muted">市值</div>
            <div className={`text-center font-mono ${compareValue(stockA.marketCap, stockB.marketCap) === 'A' ? 'font-bold text-ink' : 'text-muted'}`}>
              {formatAUM(stockA.marketCap)}
            </div>
            <div className={`text-center font-mono ${compareValue(stockB.marketCap, stockA.marketCap) === 'B' ? 'font-bold text-ink' : 'text-muted'}`}>
              {formatAUM(stockB.marketCap)}
            </div>
          </div>
          
          <div className="grid grid-cols-3 gap-4 text-sm py-2 border-t border-line">
            <div className="text-muted">行业</div>
            <div className="text-center text-ink">{stockA.sector}</div>
            <div className="text-center text-ink">{stockB.sector}</div>
          </div>
        </div>
      </section>
    </div>
  );
};
