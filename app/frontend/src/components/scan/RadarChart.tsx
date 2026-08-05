import React from 'react';
import { FiveFactorScores } from '../../types/stocks';

interface RadarChartProps {
  scores: FiveFactorScores;
  size?: number;
}

export const RadarChart: React.FC<RadarChartProps> = ({ scores, size = 28 }) => {
  // Pentagon vertices (5 points, starting from top, clockwise)
  const center = size / 2;
  const radius = (size / 2) * 0.85; // Leave some padding
  
  // Calculate pentagon points
  const angleOffset = -Math.PI / 2; // Start from top
  const angleStep = (2 * Math.PI) / 5;
  
  const getPoint = (index: number, scale: number) => {
    const angle = angleOffset + index * angleStep;
    const x = center + radius * scale * Math.cos(angle);
    const y = center + radius * scale * Math.sin(angle);
    return { x, y };
  };
  
  // Five dimensions in order: Buffett, Duanyongping, Serenity, Druckenmiller, Sentiment
  const scoreValues = [
    scores.buffett / 100,
    scores.duanyongping / 100,
    scores.serenity / 100,
    scores.druckenmiller / 100,
    scores.sentiment / 100,
  ];
  
  // Generate polygon points for data
  const dataPoints = scoreValues
    .map((score, i) => getPoint(i, score))
    .map(p => `${p.x},${p.y}`)
    .join(' ');
  
  // Generate background pentagon (max scale)
  const bgPoints = [0, 1, 2, 3, 4]
    .map(i => getPoint(i, 1))
    .map(p => `${p.x},${p.y}`)
    .join(' ');
  
  // Calculate average score for color
  const avgScore = (scores.buffett + scores.duanyongping + scores.serenity + scores.druckenmiller + scores.sentiment) / 5;
  
  // Color based on average score
  const getFillColor = (avg: number) => {
    if (avg >= 70) return '#22c55e'; // Green
    if (avg >= 50) return '#f97316'; // Orange
    return '#3b82f6'; // Blue
  };
  
  const fillColor = getFillColor(avgScore);
  
  // Tooltip text
  const tooltipText = `巴菲特 ${scores.buffett} · 段永平 ${scores.duanyongping} · Serenity ${scores.serenity} · 德鲁肯米勒 ${scores.druckenmiller} · 情绪资金面 ${scores.sentiment}`;
  
  return (
    <svg
      viewBox={`0 0 ${size} ${size}`}
      className="inline-block"
      style={{ width: size, height: size }}
    >
      <title>{tooltipText}</title>
      
      {/* Background pentagon (reference frame) */}
      <polygon
        points={bgPoints}
        fill="none"
        stroke="rgba(255,255,255,0.1)"
        strokeWidth="0.5"
      />
      
      {/* Data polygon (filled) */}
      <polygon
        points={dataPoints}
        fill={fillColor}
        fillOpacity="0.3"
        stroke={fillColor}
        strokeWidth="1"
      />
    </svg>
  );
};
