/**
 * Inline-SVG Smith chart with zoom.
 *
 * Plots the complex reflection coefficient Γ at the user's reference
 * impedance Z0.  In single-frequency mode a single dot is shown; in
 * sweep mode the locus across frequency is traced and the markers at
 * the start/end of the sweep are highlighted.
 *
 * The chart auto-zooms by default so a small-magnitude locus fills a
 * useful portion of the viewport.  An explicit zoom slider overrides
 * the auto choice.  At zoom = 1 the entire unit circle is visible; at
 * zoom = N only the |Γ| ≤ 1/N inner region is shown, with everything
 * else clipped against the SVG boundary.
 */
import React, { useMemo, useState } from 'react';
import { useAntennaStore } from '@/store/antennaStore';

const SIZE = 480;            // intrinsic SVG drawing size
const CENTER = SIZE / 2;
const BASE_RADIUS = SIZE / 2 - 24;

/** Reflection points to consider when computing the auto-zoom level. */
function maxGammaMag(points: { re: number; im: number }[]): number {
  let m = 0;
  for (const p of points) {
    const r = Math.sqrt(p.re * p.re + p.im * p.im);
    if (r > m) m = r;
  }
  return m;
}

/** Suggest a zoom level so the largest |Γ| in the data fills ~80% of the
 *  visible chart radius.  Capped at 10× to avoid losing chart context. */
function autoZoomLevel(points: { re: number; im: number }[]): number {
  const m = maxGammaMag(points);
  if (m <= 0.01) return 10;
  const target = 0.8;
  const z = Math.min(10, Math.max(1, target / m));
  // Snap to nice values for stability across re-renders.
  const ladder = [1, 1.25, 1.5, 2, 2.5, 3, 4, 5, 6, 8, 10];
  let best = 1;
  for (const v of ladder) if (v <= z) best = v;
  return best;
}

/** Convert a Γ = (re, im) to SVG (x, y) at the current zoom. */
function makeToXY(zoom: number) {
  const r = BASE_RADIUS * zoom;
  return (re: number, im: number): [number, number] => [CENTER + re * r, CENTER - im * r];
}

const SmithChart: React.FC = () => {
  const { simulationResult, sweepResult, referenceImpedance } = useAntennaStore();
  const [zoomOverride, setZoomOverride] = useState<number | null>(null);

  const single = simulationResult?.reflection;
  const sweep = sweepResult?.reflections ?? [];

  // Pick the zoom level: explicit override wins; otherwise auto-fit.
  const allPoints: { re: number; im: number }[] = useMemo(() => {
    const pts = [...sweep];
    if (single) pts.push(single);
    return pts;
  }, [sweep, single]);

  const autoZoom = useMemo(() => autoZoomLevel(allPoints), [allPoints]);
  const zoom = zoomOverride ?? autoZoom;

  if (!single && sweep.length === 0) {
    return (
      <div className="smith-panel placeholder">
        <p className="muted">Run a simulation or sweep to see the Smith chart.</p>
      </div>
    );
  }

  const z0 =
    simulationResult?.referenceImpedance ??
    sweepResult?.referenceImpedance ??
    referenceImpedance;

  const toXY = makeToXY(zoom);
  const r = BASE_RADIUS * zoom;
  const resistanceCircles = [0, 0.2, 0.5, 1, 2, 5];
  const reactanceArcs = [0.2, 0.5, 1, 2, 5, -0.2, -0.5, -1, -2, -5];

  const sweepPath = sweep
    .map(({ re, im }, i) => {
      const [x, y] = toXY(re, im);
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(2)} ${y.toFixed(2)}`;
    })
    .join(' ');

  return (
    <div className="smith-panel">
      <div className="smith-header">
        <span>Smith chart · Z₀ = {z0.toFixed(0)} Ω</span>
        {single && (
          <span className="muted small" style={{ marginLeft: 12 }}>
            Γ = {single.re.toFixed(3)} {single.im >= 0 ? '+' : '−'} j
            {Math.abs(single.im).toFixed(3)} · |Γ| ={' '}
            {Math.sqrt(single.re ** 2 + single.im ** 2).toFixed(3)}
          </span>
        )}
      </div>

      <div className="smith-controls">
        <label>
          Zoom
          <input
            type="range"
            min={1}
            max={10}
            step={0.25}
            value={zoom}
            onChange={(e) => setZoomOverride(parseFloat(e.target.value))}
          />
          <span className="muted small">{zoom.toFixed(2)}×</span>
        </label>
        <button
          className="btn btn-outline btn-small"
          onClick={() => setZoomOverride(null)}
          title="Reset to auto-zoom (fits the data)"
        >
          Auto
        </button>
      </div>

      <svg
        viewBox={`0 0 ${SIZE} ${SIZE}`}
        className="smith-svg"
        preserveAspectRatio="xMidYMid meet"
      >
        <defs>
          <clipPath id="smith-viewport">
            <rect x={0} y={0} width={SIZE} height={SIZE} />
          </clipPath>
        </defs>
        {/* Black background covering the full viewport so the
            rounded SVG looks coherent at any zoom. */}
        <rect x={0} y={0} width={SIZE} height={SIZE} fill="#0a0a0a" />
        <g clipPath="url(#smith-viewport)">
          {/* Unit circle (now sized by zoom). */}
          <circle
            cx={CENTER}
            cy={CENTER}
            r={r}
            fill="none"
            stroke="#777"
            strokeWidth={1.2}
          />
          {/* Real axis */}
          <line
            x1={CENTER - r}
            y1={CENTER}
            x2={CENTER + r}
            y2={CENTER}
            stroke="#444"
            strokeWidth={1}
          />
          {/* Constant-resistance circles */}
          {resistanceCircles.map((rn) => {
            const cx = CENTER + (rn / (1 + rn)) * r;
            const cy = CENTER;
            const rad = (1 / (1 + rn)) * r;
            return (
              <circle
                key={`r${rn}`}
                cx={cx}
                cy={cy}
                r={rad}
                fill="none"
                stroke={rn === 1 ? '#666' : '#333'}
                strokeWidth={rn === 1 ? 1.2 : 0.8}
              />
            );
          })}
          {/* Constant-reactance arcs */}
          {reactanceArcs.map((xn) => {
            const cx = CENTER + 1 * r;
            const cy = CENTER - (1 / xn) * r;
            const rad = (1 / Math.abs(xn)) * r;
            return (
              <circle
                key={`x${xn}`}
                cx={cx}
                cy={cy}
                r={rad}
                fill="none"
                stroke="#333"
                strokeWidth={0.8}
              />
            );
          })}
          {/* Sweep locus */}
          {sweepPath && (
            <path d={sweepPath} fill="none" stroke="#5096ff" strokeWidth={1.5} />
          )}
          {sweep.length > 0 &&
            (() => {
              const start = toXY(sweep[0].re, sweep[0].im);
              const end = toXY(sweep[sweep.length - 1].re, sweep[sweep.length - 1].im);
              return (
                <g>
                  <circle cx={start[0]} cy={start[1]} r={5} fill="#22c55e" />
                  <circle cx={end[0]} cy={end[1]} r={5} fill="#ef4444" />
                </g>
              );
            })()}
          {/* Single-frequency Γ marker */}
          {single &&
            (() => {
              const [x, y] = toXY(single.re, single.im);
              return (
                <g>
                  <circle cx={x} cy={y} r={6} fill="#facc15" stroke="#000" strokeWidth={1} />
                  <text x={x + 9} y={y + 4} fontSize={12} fill="#facc15">
                    Γ
                  </text>
                </g>
              );
            })()}
        </g>
      </svg>

      {sweep.length > 0 && (
        <div className="muted small">
          Sweep locus · green = first freq · red = last freq
        </div>
      )}
    </div>
  );
};

export default SmithChart;
