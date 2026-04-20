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
 * SVG schematic renderer for matching networks.
 *
 * Walks a list of MatchComponents (in source-to-load order) and lays
 * them out on a horizontal rail.  Series elements sit on the rail;
 * shunt elements drop to ground below the rail.  Works uniformly for
 * L, π, T, γ, β topologies (any 2- or 3-element series/shunt chain).
 */
import React from 'react';
import type { MatchComponent } from '@/api/client';

const WIRE = '#aabbcc';
const COMP = '#ffaa44';
const GND = '#888899';
const LBL = '#ccdde8';
const VAL = '#66ddaa';

const SLOT_W = 75;
const RAIL_Y = 40;          // y=10 (value) + y=22 (label) above the rail
const SHUNT_LEN = 44;
const GND_Y = RAIL_Y + SHUNT_LEN + 6;
const BELOW_GND = 16;       // space for the "Source" / "Load" labels under the grounds
const LEFT_PAD = 40;        // horizontal padding for labels not to clip
const RIGHT_PAD = 40;

const formatVal = (c: MatchComponent): string => {
  const v = c.value;
  if (c.kind === 'L') {
    if (v < 1e-6) return `${(v * 1e9).toFixed(1)} nH`;
    if (v < 1e-3) return `${(v * 1e6).toFixed(2)} µH`;
    return `${(v * 1e3).toFixed(2)} mH`;
  }
  if (c.kind === 'C') {
    if (v < 1e-9) return `${(v * 1e12).toFixed(1)} pF`;
    if (v < 1e-6) return `${(v * 1e9).toFixed(2)} nF`;
    return `${(v * 1e6).toFixed(2)} µF`;
  }
  if (c.kind === 'R') return `${v.toFixed(1)} Ω`;
  if (c.kind === 'transformer') return `ratio ${v.toFixed(2)}:1`;
  return v.toString();
};

/** Horizontal inductor centred at (cx, cy). */
const InductorH: React.FC<{ cx: number; cy: number; w?: number }> = ({ cx, cy, w = 36 }) => {
  const x0 = cx - w / 2;
  const humps = 4;
  const humpW = w / humps;
  let d = `M ${x0} ${cy}`;
  for (let i = 0; i < humps; i++) {
    const sx = x0 + i * humpW;
    d += ` A ${humpW / 2} 6 0 0 1 ${sx + humpW} ${cy}`;
  }
  return <path d={d} fill="none" stroke={COMP} strokeWidth="2" />;
};

/** Horizontal capacitor centred at (cx, cy). */
const CapacitorH: React.FC<{ cx: number; cy: number }> = ({ cx, cy }) => (
  <g>
    <line x1={cx - 4} y1={cy - 9} x2={cx - 4} y2={cy + 9} stroke={COMP} strokeWidth="2" />
    <line x1={cx + 4} y1={cy - 9} x2={cx + 4} y2={cy + 9} stroke={COMP} strokeWidth="2" />
  </g>
);

/** Vertical inductor between two y values. */
const InductorV: React.FC<{ cx: number; y1: number; y2: number }> = ({ cx, y1, y2 }) => {
  const h = y2 - y1;
  const humps = 4;
  const humpH = h / humps;
  let d = `M ${cx} ${y1}`;
  for (let i = 0; i < humps; i++) {
    const sy = y1 + i * humpH;
    d += ` A 6 ${humpH / 2} 0 0 1 ${cx} ${sy + humpH}`;
  }
  return <path d={d} fill="none" stroke={COMP} strokeWidth="2" />;
};

/** Vertical capacitor centred at (cx, cy). */
const CapacitorV: React.FC<{ cx: number; cy: number }> = ({ cx, cy }) => (
  <g>
    <line x1={cx - 9} y1={cy - 4} x2={cx + 9} y2={cy - 4} stroke={COMP} strokeWidth="2" />
    <line x1={cx - 9} y1={cy + 4} x2={cx + 9} y2={cy + 4} stroke={COMP} strokeWidth="2" />
  </g>
);

/** Ground triangle stack at (cx, cy). */
const Ground: React.FC<{ cx: number; cy: number }> = ({ cx, cy }) => (
  <g>
    <line x1={cx - 12} y1={cy} x2={cx + 12} y2={cy} stroke={GND} strokeWidth="2" />
    <line x1={cx - 8} y1={cy + 4} x2={cx + 8} y2={cy + 4} stroke={GND} strokeWidth="2" />
    <line x1={cx - 4} y1={cy + 8} x2={cx + 4} y2={cy + 8} stroke={GND} strokeWidth="2" />
  </g>
);

/** Transformer symbol: two coils (primary, secondary) facing each other
 *  with a row of parallel lines between them representing the core. */
const Transformer: React.FC<{ cx: number; cy: number; w?: number; ratioLabel?: string }> = ({
  cx, cy, w = 60, ratioLabel,
}) => {
  const x0 = cx - w / 2;
  const xMid = cx;
  const xPrimary = x0 + 12;
  const xSecondary = x0 + w - 12;
  const humpH = 24 / 4;
  // Vertical primary coil
  let pPath = `M ${xPrimary} ${cy - 12}`;
  for (let i = 0; i < 4; i++) {
    pPath += ` A 5 ${humpH / 2} 0 0 0 ${xPrimary} ${cy - 12 + (i + 1) * humpH}`;
  }
  let sPath = `M ${xSecondary} ${cy - 12}`;
  for (let i = 0; i < 4; i++) {
    sPath += ` A 5 ${humpH / 2} 0 0 1 ${xSecondary} ${cy - 12 + (i + 1) * humpH}`;
  }
  return (
    <g>
      <path d={pPath} fill="none" stroke={COMP} strokeWidth="2" />
      <path d={sPath} fill="none" stroke={COMP} strokeWidth="2" />
      {/* Core lines (laminations) */}
      <line x1={xMid - 8} y1={cy - 12} x2={xMid - 8} y2={cy + 12} stroke={GND} strokeWidth="1" />
      <line x1={xMid - 4} y1={cy - 12} x2={xMid - 4} y2={cy + 12} stroke={GND} strokeWidth="1" />
      <line x1={xMid + 4} y1={cy - 12} x2={xMid + 4} y2={cy + 12} stroke={GND} strokeWidth="1" />
      <line x1={xMid + 8} y1={cy - 12} x2={xMid + 8} y2={cy + 12} stroke={GND} strokeWidth="1" />
      {ratioLabel && (
        <text x={cx} y={cy + 28} fill={LBL} fontSize="11" textAnchor="middle">
          {ratioLabel}
        </text>
      )}
    </g>
  );
};

/** AC source: a circle with a sine wave inside, on a vertical wire. */
const SourceSymbol: React.FC<{ cx: number; cy: number }> = ({ cx, cy }) => (
  <g>
    <circle cx={cx} cy={cy} r={9} fill="#0a0a0a" stroke={COMP} strokeWidth="2" />
    <path d={`M ${cx-5} ${cy} Q ${cx-2.5} ${cy-4}, ${cx} ${cy} T ${cx+5} ${cy}`}
      fill="none" stroke={COMP} strokeWidth="1.5" />
  </g>
);

/** Load symbol: rectangle on a vertical wire, like an antenna feed point. */
const LoadSymbol: React.FC<{ cx: number; cy: number }> = ({ cx, cy }) => (
  <g>
    <rect x={cx-7} y={cy-9} width={14} height={18} fill="#0a0a0a" stroke={COMP} strokeWidth="2" />
  </g>
);

/** Renders one schematic for a complete network. */
export const MatchSchematic: React.FC<{ components: MatchComponent[] }> = ({ components }) => {
  if (components.length === 0) return null;
  const N = components.length;
  // Layout: source terminal at LEFT_PAD; load at totalW-RIGHT_PAD;
  // components evenly distributed between them.
  const totalW = LEFT_PAD + (N + 1) * SLOT_W + RIGHT_PAD;
  const xSource = LEFT_PAD;
  const xLoad = totalW - RIGHT_PAD;
  const slots = Array.from({ length: N }, (_, i) => xSource + (i + 1) * (xLoad - xSource) / (N + 1));
  const totalH = GND_Y + BELOW_GND + 6;

  return (
    <svg
      width={totalW}
      height={totalH}
      viewBox={`0 0 ${totalW} ${totalH}`}
      preserveAspectRatio="xMidYMid meet"
      className="match-schematic"
    >
      {/* Top rail from source to load */}
      <line x1={xSource} y1={RAIL_Y} x2={xLoad} y2={RAIL_Y} stroke={WIRE} strokeWidth="1.5" />

      {/* Source: AC source symbol on the vertical between the rail
          terminal and ground.  Replaces the old plain wire that
          looked like a short circuit. */}
      <circle cx={xSource} cy={RAIL_Y} r={3} fill={WIRE} />
      <line x1={xSource} y1={RAIL_Y} x2={xSource} y2={(RAIL_Y + GND_Y) / 2 - 9} stroke={WIRE} strokeWidth="1.5" />
      <SourceSymbol cx={xSource} cy={(RAIL_Y + GND_Y) / 2} />
      <line x1={xSource} y1={(RAIL_Y + GND_Y) / 2 + 9} x2={xSource} y2={GND_Y} stroke={WIRE} strokeWidth="1.5" />
      <Ground cx={xSource} cy={GND_Y} />
      <text x={xSource} y={GND_Y + BELOW_GND} fill={LBL} fontSize="11" textAnchor="middle">Source</text>

      {/* Load: rectangular impedance symbol on the vertical between
          the rail terminal and ground. */}
      <circle cx={xLoad} cy={RAIL_Y} r={3} fill={WIRE} />
      <line x1={xLoad} y1={RAIL_Y} x2={xLoad} y2={(RAIL_Y + GND_Y) / 2 - 9} stroke={WIRE} strokeWidth="1.5" />
      <LoadSymbol cx={xLoad} cy={(RAIL_Y + GND_Y) / 2} />
      <line x1={xLoad} y1={(RAIL_Y + GND_Y) / 2 + 9} x2={xLoad} y2={GND_Y} stroke={WIRE} strokeWidth="1.5" />
      <Ground cx={xLoad} cy={GND_Y} />
      <text x={xLoad} y={GND_Y + BELOW_GND} fill={LBL} fontSize="11" textAnchor="middle">Load</text>

      {/* Components */}
      {components.map((c, i) => {
        const cx = slots[i];
        if (c.position === 'series') {
          // Transformer is a special case — wider symbol on the rail.
          if (c.kind === 'transformer') {
            return (
              <g key={i}>
                <rect x={cx - 32} y={RAIL_Y - 14} width={64} height={28} fill="#0a0a0a" />
                <Transformer cx={cx} cy={RAIL_Y} ratioLabel={c.label.split(' ')[1]} />
                <text x={cx} y={RAIL_Y - 22} fill={LBL} fontSize="10" textAnchor="middle" dominantBaseline="hanging">
                  {c.label.split(',')[0]}
                </text>
              </g>
            );
          }
          // Replace a chunk of the rail with the symbol.
          return (
            <g key={i}>
              {/* Cover the rail */}
              <rect x={cx - 22} y={RAIL_Y - 10} width={44} height={20} fill="#0a0a0a" />
              {c.kind === 'L'
                ? <InductorH cx={cx} cy={RAIL_Y} />
                : <CapacitorH cx={cx} cy={RAIL_Y} />}
              <text x={cx} y={RAIL_Y - 18} fill={LBL} fontSize="10" textAnchor="middle" dominantBaseline="hanging">
                {c.label.split(' ')[0]}
              </text>
              <text x={cx} y={RAIL_Y - 30} fill={VAL} fontSize="10" textAnchor="middle" dominantBaseline="hanging">
                {formatVal(c)}
              </text>
            </g>
          );
        }
        // Shunt: drop from rail to ground
        const yMid = RAIL_Y + SHUNT_LEN / 2;
        return (
          <g key={i}>
            {c.kind === 'L' ? (
              <InductorV cx={cx} y1={RAIL_Y + 4} y2={RAIL_Y + SHUNT_LEN - 4} />
            ) : (
              <>
                <line x1={cx} y1={RAIL_Y} x2={cx} y2={yMid - 4} stroke={WIRE} strokeWidth="1.5" />
                <CapacitorV cx={cx} cy={yMid} />
                <line x1={cx} y1={yMid + 4} x2={cx} y2={GND_Y} stroke={WIRE} strokeWidth="1.5" />
              </>
            )}
            {c.kind === 'L' && (
              <line x1={cx} y1={RAIL_Y + SHUNT_LEN} x2={cx} y2={GND_Y} stroke={WIRE} strokeWidth="1.5" />
            )}
            <Ground cx={cx} cy={GND_Y} />
            <text x={cx + 14} y={yMid} fill={LBL} fontSize="10" textAnchor="start">
              {c.label.split(' ')[0]}
            </text>
            <text x={cx + 14} y={yMid + 12} fill={VAL} fontSize="10" textAnchor="start">
              {formatVal(c)}
            </text>
          </g>
        );
      })}
    </svg>
  );
};

export default MatchSchematic;
