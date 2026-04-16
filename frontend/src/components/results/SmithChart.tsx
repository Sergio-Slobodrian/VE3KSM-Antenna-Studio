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
import React, { useMemo, useState, useRef } from 'react';
import { useAntennaStore } from '@/store/antennaStore';

const SIZE = 480;            // intrinsic SVG drawing size
const CENTER = SIZE / 2;
const BASE_RADIUS = SIZE / 2 - 24;

/** Compute zoom + pan that centers on and fills the data's bounding box.
 *  Returns { zoom, panX, panY } so the auto view is actually useful for
 *  datasets that cluster far from the origin (e.g. |Γ| ≈ 1). */
function autoFit(points: { re: number; im: number }[]): { zoom: number; panX: number; panY: number } {
  if (points.length === 0) return { zoom: 1, panX: 0, panY: 0 };
  // Filter to |Γ| <= 1 (physical points only).
  const valid = points.filter((p) => Math.sqrt(p.re * p.re + p.im * p.im) <= 1.0);
  if (valid.length === 0) return { zoom: 1, panX: 0, panY: 0 };
  let reMin = Infinity, reMax = -Infinity, imMin = Infinity, imMax = -Infinity;
  for (const p of valid) {
    if (p.re < reMin) reMin = p.re;
    if (p.re > reMax) reMax = p.re;
    if (p.im < imMin) imMin = p.im;
    if (p.im > imMax) imMax = p.im;
  }
  const spread = Math.max(reMax - reMin, imMax - imMin, 0.01);
  // Zoom so the spread fills 70% of the chart diameter.
  const z = Math.min(20, Math.max(1, 1.4 / spread));
  // Pan to center the data bbox in the viewport.
  const cRe = (reMin + reMax) / 2;
  const cIm = (imMin + imMax) / 2;
  // In SVG space: center of data at (CENTER + cRe*r, CENTER - cIm*r).
  // We want that at (CENTER, CENTER), so panX = -cRe*r, panY = cIm*r.
  const r = BASE_RADIUS * z;
  return { zoom: z, panX: -cRe * r, panY: cIm * r };
}

/** Convert a Γ = (re, im) to SVG (x, y) at the current zoom. */
function makeToXY(zoom: number) {
  const r = BASE_RADIUS * zoom;
  return (re: number, im: number): [number, number] => [CENTER + re * r, CENTER - im * r];
}

const SmithChart: React.FC = () => {
  const { simulationResult, sweepResult, referenceImpedance } = useAntennaStore();
  const [zoomOverride, setZoomOverride] = useState<number | null>(null);
  const [pan, setPan] = useState<{ x: number; y: number }>({ x: 0, y: 0 });
  const [dragging, setDragging] = useState(false);
  const dragRef = useRef<{ startX: number; startY: number; basePan: { x: number; y: number } } | null>(null);

  const single = simulationResult?.reflection;
  const sweep = sweepResult?.reflections ?? [];

  // Pick the zoom level: explicit override wins; otherwise auto-fit.
  const allPoints: { re: number; im: number }[] = useMemo(() => {
    const pts = [...sweep];
    if (single) pts.push(single);
    return pts;
  }, [sweep, single]);

  const autoResult = useMemo(() => autoFit(allPoints), [allPoints]);
  const zoom = zoomOverride ?? autoResult.zoom;

  // Note: pan is intentionally NOT reset when zoom changes.  Use the
  // Recenter button if the chart drifts off after zooming.

  // Pointer handlers for click-drag panning.
  const onPointerDown = (e: React.PointerEvent<SVGSVGElement>) => {
    if (e.button !== 0) return;
    e.currentTarget.setPointerCapture?.(e.pointerId);
    dragRef.current = {
      startX: e.clientX, startY: e.clientY, basePan: { ...pan },
    };
    setDragging(true);
  };
  const onPointerMove = (e: React.PointerEvent<SVGSVGElement>) => {
    if (dragRef.current === null) return;
    const dx = e.clientX - dragRef.current.startX;
    const dy = e.clientY - dragRef.current.startY;
    const target = e.currentTarget as SVGSVGElement;
    const rect = target.getBoundingClientRect();
    const scaleX = SIZE / rect.width;
    const scaleY = SIZE / rect.height;
    setPan({
      x: dragRef.current.basePan.x + dx * scaleX,
      y: dragRef.current.basePan.y + dy * scaleY,
    });
  };
  const endDrag = () => {
    dragRef.current = null;
    setDragging(false);
  };

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

  // Build sub-paths, splitting wherever |Γ| > 1 (non-physical points
  // from the negative-R numerical regime).  Without this filter the
  // SVG line alternates between inside/outside the unit circle and
  // the clipPath creates a scalloped "sawtooth" along the edge.
  const sweepSubPaths: string[] = [];
  let currentPath = '';
  for (let i = 0; i < sweep.length; i++) {
    const { re, im } = sweep[i];
    const mag = Math.sqrt(re * re + im * im);
    if (mag > 1.0) {
      // End the current sub-path; skip this point.
      if (currentPath) {
        sweepSubPaths.push(currentPath);
        currentPath = '';
      }
      continue;
    }
    const [x, y] = toXY(re, im);
    if (!currentPath) {
      currentPath = `M ${x.toFixed(2)} ${y.toFixed(2)}`;
    } else {
      currentPath += ` L ${x.toFixed(2)} ${y.toFixed(2)}`;
    }
  }
  if (currentPath) sweepSubPaths.push(currentPath);
  const sweepPath = sweepSubPaths.join(' ');

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
          onClick={() => { setZoomOverride(null); setPan({ x: autoResult.panX, y: autoResult.panY }); }}
          title="Reset to auto-zoom + pan (fits the data)"
        >
          Auto
        </button>
        <button
          className="btn btn-outline btn-small"
          onClick={() => setPan({ x: autoResult.panX, y: autoResult.panY })}
          title="Recenter the chart"
        >
          Recenter
        </button>
        <span className="muted small" style={{ marginLeft: 8 }}>
          drag to pan
        </span>
      </div>

      <svg
        viewBox={`0 0 ${SIZE} ${SIZE}`}
        className="smith-svg"
        preserveAspectRatio="xMidYMid meet"
        style={{ cursor: dragging ? 'grabbing' : 'grab' }}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={endDrag}
        onPointerCancel={endDrag}
        onPointerLeave={endDrag}
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
          <g transform={`translate(${pan.x}, ${pan.y})`}>
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
