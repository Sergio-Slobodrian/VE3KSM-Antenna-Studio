/**
 * 2D polar plot of the azimuth and elevation cuts through the 3D far-
 * field pattern.  Uses an inline SVG; no charting library required.
 *
 * The azimuth cut is rendered as a full 360° polar plot.
 * The elevation cut is rendered as a half-disc (-90° to +90°).
 *
 * Both plots use a fixed dynamic range (default 30 dB) below the peak,
 * with rings at -3, -10, -20, -30 dB for visual reference.
 */
import React, { useState } from 'react';
import { useAntennaStore } from '@/store/antennaStore';

const SIZE = 320;
const CENTER = SIZE / 2;
const RADIUS = SIZE / 2 - 30;
const DR_DB = 30; // dynamic range below peak shown on the plot

type Cut = 'azimuth' | 'elevation';

const PolarCut: React.FC = () => {
  const { simulationResult } = useAntennaStore();
  const [cut, setCut] = useState<Cut>('azimuth');

  if (!simulationResult) {
    return (
      <div className="polar-cut-panel placeholder">
        <p className="muted">Run a simulation to see polar cuts.</p>
      </div>
    );
  }

  const cuts = simulationResult.polarCuts;
  const xs = cut === 'azimuth' ? cuts.azimuthDeg : cuts.elevationDeg;
  const ys = cut === 'azimuth' ? cuts.azimuthGainDb : cuts.elevationGainDb;
  const fixed = cut === 'azimuth'
    ? `elevation ${cuts.fixedElevationDeg.toFixed(1)}°`
    : `azimuth ${cuts.fixedAzimuthDeg.toFixed(1)}°`;
  const isFullCircle = cut === 'azimuth';

  if (xs.length === 0) {
    return (
      <div className="polar-cut-panel placeholder">
        <p className="muted">No {cut} cut data in this result.</p>
      </div>
    );
  }

  const peak = Math.max(...ys);
  const points = xs.map((deg, i) => {
    const dB = ys[i];
    // Map dB to radial distance: peak → RADIUS, peak − DR_DB → 0.
    let r = ((dB - peak + DR_DB) / DR_DB) * RADIUS;
    if (r < 0) r = 0;
    if (r > RADIUS) r = RADIUS;
    let angleRad: number;
    if (isFullCircle) {
      // Azimuth cut: 0° at top (east in plot), increasing clockwise.
      angleRad = ((deg - 90) * Math.PI) / 180;
    } else {
      // Elevation cut: 0° (horizon) points right, +90° (zenith) at top.
      // Negate so positive elevation arcs upward in SVG (y-down).
      angleRad = (-deg * Math.PI) / 180;
    }
    const x = CENTER + r * Math.cos(angleRad);
    const y = CENTER + r * Math.sin(angleRad);
    return { x, y, deg, dB };
  });

  const path = points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(2)} ${p.y.toFixed(2)}`).join(' ') +
    (isFullCircle ? ' Z' : '');

  // Reference rings at -3, -10, -20, -30 dB (relative to peak).
  const ringDB = [-3, -10, -20, -30];
  return (
    <div className="polar-cut-panel">
      <div className="polar-cut-controls">
        <button
          className={`tab-btn ${cut === 'azimuth' ? 'tab-active' : ''}`}
          onClick={() => setCut('azimuth')}
        >
          Azimuth
        </button>
        <button
          className={`tab-btn ${cut === 'elevation' ? 'tab-active' : ''}`}
          onClick={() => setCut('elevation')}
        >
          Elevation
        </button>
        <span className="muted small" style={{ marginLeft: 12 }}>
          peak {peak.toFixed(2)} dBi · cut at {fixed} · {DR_DB} dB scale
        </span>
      </div>
      <svg width={SIZE} height={SIZE} className="polar-cut-svg">
        {/* Outer ring */}
        <circle cx={CENTER} cy={CENTER} r={RADIUS} fill="none" stroke="#777" strokeWidth={1} />
        {/* Reference rings */}
        {ringDB.map((db) => {
          const r = ((db + DR_DB) / DR_DB) * RADIUS;
          if (r <= 0) return null;
          return (
            <g key={db}>
              <circle cx={CENTER} cy={CENTER} r={r} fill="none" stroke="#444" strokeDasharray="2 3" />
              <text x={CENTER + r} y={CENTER - 2} fontSize={10} fill="#888">{db} dB</text>
            </g>
          );
        })}
        {/* Cardinal/spoke lines */}
        {(isFullCircle ? [0, 45, 90, 135, 180, 225, 270, 315] : [-90, -45, 0, 45, 90]).map((deg) => {
          const angleRad = isFullCircle
            ? ((deg - 90) * Math.PI) / 180
            : ((-deg * Math.PI) / 180);
          const x = CENTER + RADIUS * Math.cos(angleRad);
          const y = CENTER + RADIUS * Math.sin(angleRad);
          return (
            <g key={deg}>
              <line x1={CENTER} y1={CENTER} x2={x} y2={y} stroke="#333" strokeWidth={1} />
              <text x={x} y={y} fontSize={10} fill="#aaa" textAnchor="middle" dy="-4">
                {deg}°
              </text>
            </g>
          );
        })}
        {/* The trace */}
        <path d={path} fill="rgba(80,150,255,0.18)" stroke="#5096ff" strokeWidth={1.5} />
      </svg>
    </div>
  );
};

export default PolarCut;
