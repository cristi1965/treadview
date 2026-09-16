import React, { useEffect, useRef, useState, useCallback } from 'react';
import { HeatmapNodeWithPosition, HeatmapConfig } from '../../types/heatmap';
import { HeatmapTooltip } from './HeatmapTooltip';
import {
  createForceSimulation,
  drawNodes,
  drawLayerScatterGuides,
  findNodeAtPosition,
  HeatmapLayoutMode,
} from '../../utils/heatmap';

interface HeatmapProps {
  nodes: HeatmapNodeWithPosition[];
  config: HeatmapConfig;
  layout?: HeatmapLayoutMode;
  onNodeClick?: (node: HeatmapNodeWithPosition) => void;
}

export const Heatmap: React.FC<HeatmapProps> = ({ nodes, config, layout = 'layer-scatter', onNodeClick }) => {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [hoveredNode, setHoveredNode] = useState<HeatmapNodeWithPosition | null>(null);
  const [tooltipPos, setTooltipPos] = useState({ x: 0, y: 0 });
  const [isSimulationRunning, setIsSimulationRunning] = useState(true);
  const simulationRef = useRef<any>(null);

  const hoveredNodeRef = useRef<HeatmapNodeWithPosition | null>(null);
  useEffect(() => {
    hoveredNodeRef.current = hoveredNode;
  }, [hoveredNode]);

  const paint = useCallback(
    (ctx: CanvasRenderingContext2D, highlight: HeatmapNodeWithPosition | null) => {
      const dpr = window.devicePixelRatio || 1;
      ctx.setTransform(1, 0, 0, 1, 0, 0);
      ctx.clearRect(0, 0, ctx.canvas.width, ctx.canvas.height);
      if (layout === 'layer-scatter') {
        drawLayerScatterGuides(ctx, config.width, config.height);
      }
      drawNodes(ctx, nodes, highlight);
    },
    [layout, config.width, config.height, nodes]
  );

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const dpr = window.devicePixelRatio || 1;
    canvas.width = config.width * dpr;
    canvas.height = config.height * dpr;
    canvas.style.width = `${config.width}px`;
    canvas.style.height = `${config.height}px`;
  }, [config.width, config.height]);

  useEffect(() => {
    if (!canvasRef.current || nodes.length === 0) return;

    const canvas = canvasRef.current;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const simulation = createForceSimulation(
      nodes,
      config.width,
      config.height,
      config.forceStrength,
      config.collisionPadding,
      config.alphaDecay,
      layout
    );

    simulationRef.current = simulation;

    simulation.on('tick', () => {
      paint(ctx, hoveredNodeRef.current);
    });

    simulation.on('end', () => {
      setIsSimulationRunning(false);
    });

    setIsSimulationRunning(true);

    return () => {
      simulation.stop();
    };
  }, [nodes, config.width, config.height, config.forceStrength, config.collisionPadding, config.alphaDecay, layout, paint]);

  useEffect(() => {
    if (nodes.length === 0 || isSimulationRunning) return;
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (ctx) paint(ctx, hoveredNode);
  }, [hoveredNode, nodes, isSimulationRunning, paint]);

  const handleMouseMove = useCallback(
    (event: React.MouseEvent<HTMLCanvasElement>) => {
      const canvas = canvasRef.current;
      if (!canvas) return;

      const rect = canvas.getBoundingClientRect();
      const x = event.clientX - rect.left;
      const y = event.clientY - rect.top;

      const node = findNodeAtPosition(nodes, x, y);

      if (node) {
        setHoveredNode(node);
        setTooltipPos({ x: event.clientX, y: event.clientY });
        canvas.style.cursor = 'pointer';
      } else {
        setHoveredNode(null);
        canvas.style.cursor = 'default';
      }
    },
    [nodes]
  );

  const handleMouseLeave = useCallback(() => {
    setHoveredNode(null);
    if (canvasRef.current) {
      canvasRef.current.style.cursor = 'default';
    }
  }, []);

  const handleClick = useCallback(
    (event: React.MouseEvent<HTMLCanvasElement>) => {
      const canvas = canvasRef.current;
      if (!canvas) return;

      const rect = canvas.getBoundingClientRect();
      const x = event.clientX - rect.left;
      const y = event.clientY - rect.top;

      const node = findNodeAtPosition(nodes, x, y);

      if (node && onNodeClick) {
        onNodeClick(node);
      }
    },
    [nodes, onNodeClick]
  );

  const handleRestart = useCallback(() => {
    if (simulationRef.current) {
      simulationRef.current.alpha(1).restart();
      setIsSimulationRunning(true);
    }
  }, []);

  return (
    <div ref={containerRef} className="relative">
      <canvas
        ref={canvasRef}
        onMouseMove={handleMouseMove}
        onMouseLeave={handleMouseLeave}
        onClick={handleClick}
        className="block h-full w-full cursor-crosshair bg-[#06080F]"
      />

      <HeatmapTooltip node={hoveredNode} x={tooltipPos.x} y={tooltipPos.y} visible={!!hoveredNode} />

      {layout === 'force' && (
        <div className="absolute right-4 top-4 flex gap-2">
          <button
            onClick={handleRestart}
            disabled={isSimulationRunning}
            className="rounded-lg border border-line bg-surface px-3 py-2 text-sm text-ink transition-colors hover:bg-surface/80 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {isSimulationRunning ? 'Simulating...' : 'Restart'}
          </button>
        </div>
      )}
    </div>
  );
};
