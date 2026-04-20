// Copyright 2026 Sergio Slobodrian
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
  const isAzimuth = cut === 'azimuth';
  const fixed = isAzimuth
    ? `elevation ${cuts.fixedElevationDeg.toFixed(1)}°`
    : `azimuth ${cuts.fixedAzimuthDeg.toFixed(1)}°`;

  const frontXs = isAzimuth ? cuts.azimuthDeg : cuts.elevationDeg;
  const frontYs = isAzimuth ? cuts.azimuthGainDb : cuts.elevationGainDb;
  const backXs  = isAzimuth ? [] : (cuts.elevationBackDeg ?? []);
  const backYs  = isAzimuth ? [] : (cuts.elevationBackGainDb ?? []);

  if (frontXs.length === 0) {
    return (
      <div className="polar-cut-panel placeholder">
        <p className="muted">No {cut} cut data in this result.</p>
      </div>
    );
  }

  const allGains = [...frontYs, ...backYs];
  const peak = Math.max(...allGains);

  const toXY = (deg: number, dB: number, isBack: boolean) => {
    let r = ((dB - peak + DR_DB) / DR_DB) * RADIUS;
    if (r < 0) r = 0;
    if (r > RADIUS) r = RADIUS;
    let angleRad: number;
    if (isAzimuth) {
      // Azimuth: 0° at top, increasing clockwise.
      angleRad = ((deg - 90) * Math.PI) / 180;
    } else if (!isBack) {
      // Elevation front side: 0° horizon right, +90° zenith top.
      angleRad = (-deg * Math.PI) / 180;
    } else {
      // Elevation back side: mirrored to left half.
      // deg=0 → left (π), deg=90 → top (-π/2), deg=-90 → bottom (π/2)
      angleRad = Math.PI + (deg * Math.PI) / 180;
    }
    return { x: CENTER + r * Math.cos(angleRad), y: CENTER + r * Math.sin(angleRad) };
  };

  let path: string;
  if (isAzimuth) {
    const pts = frontXs.map((deg, i) => toXY(deg, frontYs[i], false));
    path = pts.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(2)} ${p.y.toFixed(2)}`).join(' ') + ' Z';
  } else {
    // Elevation: front sorted ascending (-90→+90), back sorted descending (+90→-90)
    // This traces: bottom→right→top (front) then top→left→bottom (back) — closed circle.
    const frontPairs = frontXs.map((deg, i) => ({ deg, dB: frontYs[i] })).sort((a, b) => a.deg - b.deg);
    const backPairs  = backXs.map((deg, i) => ({ deg, dB: backYs[i] })).sort((a, b) => b.deg - a.deg);
    const allPts = [
      ...frontPairs.map(({ deg, dB }) => toXY(deg, dB, false)),
      ...backPairs.map(({ deg, dB }) => toXY(deg, dB, true)),
    ];
    path = allPts.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(2)} ${p.y.toFixed(2)}`).join(' ') + ' Z';
  }

  // Reference rings at -3, -10, -20, -30 dB (relative to peak).
  const ringDB = [-3, -10, -20, -30];

  // Spokes: azimuth uses 8 cardinal/intercardinal; elevation uses 8 positions for full circle.
  // elevSpokes: plotDeg where 0=fwd/right, 90=zenith/top, 180=back/left, 270=nadir/bottom.
  const elevSpokeLabels: Record<number, string> = { 0: '0°', 45: '45°', 90: '90°', 135: '135°', 180: '180°', 225: '225°', 270: '270°', 315: '315°' };

  return (
    <div className="polar-cut-panel">
      <div className="polar-cut-controls">
        <button
          className={`tab-btn ${isAzimuth ? 'tab-active' : ''}`}
          onClick={() => setCut('azimuth')}
        >
          Azimuth
        </button>
        <button
          className={`tab-btn ${!isAzimuth ? 'tab-active' : ''}`}
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
        {/* Spoke lines and labels */}
        {isAzimuth
          ? [0, 45, 90, 135, 180, 225, 270, 315].map((deg) => {
              const angleRad = ((deg - 90) * Math.PI) / 180;
              const x = CENTER + RADIUS * Math.cos(angleRad);
              const y = CENTER + RADIUS * Math.sin(angleRad);
              return (
                <g key={deg}>
                  <line x1={CENTER} y1={CENTER} x2={x} y2={y} stroke="#333" strokeWidth={1} />
                  <text x={x} y={y} fontSize={10} fill="#aaa" textAnchor="middle" dy="-4">{deg}°</text>
                </g>
              );
            })
          : [0, 45, 90, 135, 180, 225, 270, 315].map((plotDeg) => {
              // plotDeg: 0=fwd, 90=zenith, 180=back, 270=nadir
              const angleRad = -(plotDeg * Math.PI) / 180;
              const x = CENTER + RADIUS * Math.cos(angleRad);
              const y = CENTER + RADIUS * Math.sin(angleRad);
              return (
                <g key={plotDeg}>
                  <line x1={CENTER} y1={CENTER} x2={x} y2={y} stroke="#333" strokeWidth={1} />
                  <text x={x} y={y} fontSize={10} fill="#aaa" textAnchor="middle" dy="-4">
                    {elevSpokeLabels[plotDeg]}
                  </text>
                </g>
              );
            })
        }
        {/* The trace */}
        <path d={path} fill="rgba(80,150,255,0.18)" stroke="#5096ff" strokeWidth={1.5} />
      </svg>
    </div>
  );
};

export default PolarCut;
