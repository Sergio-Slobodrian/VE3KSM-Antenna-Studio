/**
 * Near-field E/H heat-map viewer.
 *
 * Renders a 2D color-mapped grid of |E| or |H| on a user-specified
 * observation plane (XZ, XY, or YZ).  The grid parameters are controlled
 * via input fields; pressing "Compute" triggers a POST /api/nearfield
 * request that runs the MoM solve + Hertzian-dipole near-field evaluation.
 */
import React, { useState, useCallback, useRef, useEffect, useMemo } from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import { computeNearField } from '@/api/client';
import type { NearFieldResult } from '@/types';

type Plane = 'xz' | 'xy' | 'yz';
type FieldType = 'E' | 'H';

/** Jet-style colour map: blue → cyan → green → yellow → red */
function jetColor(t: number): string {
  const c = Math.max(0, Math.min(1, t));
  let r: number, g: number, b: number;
  if (c < 0.25) {
    r = 0;
    g = 4 * c;
    b = 1;
  } else if (c < 0.5) {
    r = 0;
    g = 1;
    b = 1 - 4 * (c - 0.25);
  } else if (c < 0.75) {
    r = 4 * (c - 0.5);
    g = 1;
    b = 0;
  } else {
    r = 1;
    g = 1 - 4 * (c - 0.75);
    b = 0;
  }
  return `rgb(${Math.round(r * 255)},${Math.round(g * 255)},${Math.round(b * 255)})`;
}

/** Aspect-ratio-aware heat-map + wire overlay + colour bar. */
const DisplayGrid: React.FC<{
  result: NearFieldResult;
  fieldType: FieldType;
  dynRange: number;
  min1: number; max1: number; min2: number; max2: number;
  axis1Label: string; axis2Label: string;
  wireLines: { a1: number; a2: number; b1: number; b2: number }[];
  canvasRef: React.RefObject<HTMLCanvasElement | null>;
}> = ({ result, fieldType, dynRange, min1, max1, min2, max2,
        axis1Label, axis2Label, wireLines, canvasRef }) => {

  // Compute display dimensions preserving the physical aspect ratio.
  // Fit the larger physical extent into MAX_PX; scale the other proportionally.
  const MAX_PX = 560;
  const physW = Math.abs(max1 - min1) || 1;
  const physH = Math.abs(max2 - min2) || 1;
  const aspect = physW / physH; // width / height in physical metres
  let dispW: number, dispH: number;
  if (aspect >= 1) {
    dispW = MAX_PX;
    dispH = Math.round(MAX_PX / aspect);
  } else {
    dispH = MAX_PX;
    dispW = Math.round(MAX_PX * aspect);
  }

  // Render heat map onto canvas
  useEffect(() => {
    if (!canvasRef.current) return;
    const canvas = canvasRef.current;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const { steps1, steps2, points } = result;
    canvas.width = steps1;
    canvas.height = steps2;

    const maxDB = fieldType === 'E' ? result.e_max_db : result.h_max_db;
    const floorDB = maxDB - dynRange;

    const img = ctx.createImageData(steps1, steps2);
    for (let i2 = 0; i2 < steps2; i2++) {
      for (let i1 = 0; i1 < steps1; i1++) {
        const pt = points[i2 * steps1 + i1];
        const dB = fieldType === 'E' ? pt.e_mag_db : pt.h_mag_db;
        const t = Math.max(0, Math.min(1, (dB - floorDB) / dynRange));

        const color = jetColor(t);
        const m = color.match(/rgb\((\d+),(\d+),(\d+)\)/);
        const r = m ? parseInt(m[1]) : 0;
        const g = m ? parseInt(m[2]) : 0;
        const b = m ? parseInt(m[3]) : 0;

        // Canvas y=0 is top; axis2 increases upward → flip
        const canvasRow = steps2 - 1 - i2;
        const off = (canvasRow * steps1 + i1) * 4;
        img.data[off] = r;
        img.data[off + 1] = g;
        img.data[off + 2] = b;
        img.data[off + 3] = 255;
      }
    }
    ctx.putImageData(img, 0, 0);
  }, [result, fieldType, dynRange, canvasRef]);

  const peakDB = (fieldType === 'E' ? result.e_max_db : result.h_max_db);

  return (
    <div style={{ position: 'relative', display: 'inline-block', marginLeft: 28 }}>
      {/* Heat-map canvas */}
      <canvas
        ref={canvasRef}
        style={{
          width: dispW,
          height: dispH,
          imageRendering: 'pixelated',
          border: '1px solid #555',
        }}
      />

      {/* Wire overlay as SVG on top of the canvas */}
      <svg
        viewBox={`${min1} ${min2} ${physW} ${physH}`}
        preserveAspectRatio="none"
        style={{
          position: 'absolute',
          top: 0, left: 0,
          width: dispW,
          height: dispH,
          pointerEvents: 'none',
        }}
      >
        {/* SVG y increases downward; axis2 increases upward → flip */}
        <g transform={`scale(1,-1) translate(0,${-(min2 + max2)})`}>
          {wireLines.map((l, i) => (
            <line key={i}
                  x1={l.a1} y1={l.a2} x2={l.b1} y2={l.b2}
                  stroke="white" strokeWidth={physW / 200}
                  strokeLinecap="round" />
          ))}
        </g>
      </svg>

      {/* Axis labels */}
      <div style={{ textAlign: 'center', fontSize: 12, marginTop: 2 }}>
        {axis1Label} ({min1} to {max1} m)
      </div>
      <div style={{
        position: 'absolute', left: -24, top: '50%',
        transform: 'rotate(-90deg) translateX(-50%)',
        fontSize: 12, whiteSpace: 'nowrap',
      }}>
        {axis2Label} ({min2} to {max2} m)
      </div>

      {/* Colour bar */}
      <div style={{ marginTop: 8, display: 'flex', alignItems: 'center', gap: 4 }}>
        <span style={{ fontSize: 11 }}>
          {(peakDB - dynRange).toFixed(1)} dB
        </span>
        <div style={{
          width: 200, height: 14,
          background: 'linear-gradient(to right, ' +
            Array.from({ length: 10 }, (_, i) => jetColor(i / 9)).join(', ') + ')',
          border: '1px solid #555',
        }} />
        <span style={{ fontSize: 11 }}>
          {peakDB.toFixed(1)} dB ({fieldType === 'E' ? 'V/m' : 'A/m'})
        </span>
      </div>
    </div>
  );
};

const NearFieldViewer: React.FC = () => {
  const wires = useAntennaStore((s) => s.wires);
  const source = useAntennaStore((s) => s.source);
  const loads = useAntennaStore((s) => s.loads);
  const transmissionLines = useAntennaStore((s) => s.transmissionLines);
  const ground = useAntennaStore((s) => s.ground);
  const frequency = useAntennaStore((s) => s.frequency);
  const referenceImpedance = useAntennaStore((s) => s.referenceImpedance);

  const [plane, setPlane] = useState<Plane>('xz');
  const [fixedCoord, setFixedCoord] = useState(0);
  const [min1, setMin1] = useState(-15);
  const [max1, setMax1] = useState(15);
  const [min2, setMin2] = useState(-2);
  const [max2, setMax2] = useState(15);
  const [steps, setSteps] = useState(80);
  const [fieldType, setFieldType] = useState<FieldType>('E');
  const [dynRange, setDynRange] = useState(40);

  const [result, setResult] = useState<NearFieldResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const canvasRef = useRef<HTMLCanvasElement>(null);

  const handleCompute = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await computeNearField(
        wires, source, loads, transmissionLines,
        ground, frequency, referenceImpedance,
        {
          plane,
          fixed_coord: fixedCoord,
          min1, max1, min2, max2,
          steps1: steps,
          steps2: steps,
        },
      );
      setResult(res);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [wires, source, loads, transmissionLines, ground, frequency,
      referenceImpedance, plane, fixedCoord, min1, max1, min2, max2, steps]);

  // Wire segments projected onto the current plane (for overlay)
  const wireLines = useMemo(() => {
    const lines: { a1: number; a2: number; b1: number; b2: number }[] = [];
    for (const w of wires) {
      let a1: number, a2: number, b1: number, b2: number;
      switch (plane) {
        case 'xy': a1 = w.x1; a2 = w.y1; b1 = w.x2; b2 = w.y2; break;
        case 'xz': a1 = w.x1; a2 = w.z1; b1 = w.x2; b2 = w.z2; break;
        case 'yz': a1 = w.y1; a2 = w.z1; b1 = w.y2; b2 = w.z2; break;
      }
      lines.push({ a1, a2, b1, b2 });
    }
    return lines;
  }, [wires, plane]);

  const axis1Label = result?.axis1_label ?? (plane === 'yz' ? 'y' : 'x');
  const axis2Label = result?.axis2_label ?? (plane === 'xy' ? 'y' : 'z');

  return (
    <div style={{ padding: 12 }}>
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center', marginBottom: 10 }}>
        <label>
          Plane:&nbsp;
          <select value={plane} onChange={(e) => setPlane(e.target.value as Plane)}>
            <option value="xz">XZ</option>
            <option value="xy">XY</option>
            <option value="yz">YZ</option>
          </select>
        </label>
        <label>
          {plane === 'xy' ? 'Z' : plane === 'xz' ? 'Y' : 'X'}=&nbsp;
          <input type="number" step="0.1" value={fixedCoord}
                 onChange={(e) => setFixedCoord(+e.target.value)}
                 style={{ width: 60 }} /> m
        </label>
        <label>
          {axis1Label} min:&nbsp;
          <input type="number" step="1" value={min1}
                 onChange={(e) => setMin1(+e.target.value)}
                 style={{ width: 60 }} />
        </label>
        <label>
          max:&nbsp;
          <input type="number" step="1" value={max1}
                 onChange={(e) => setMax1(+e.target.value)}
                 style={{ width: 60 }} />
        </label>
        <label>
          {axis2Label} min:&nbsp;
          <input type="number" step="1" value={min2}
                 onChange={(e) => setMin2(+e.target.value)}
                 style={{ width: 60 }} />
        </label>
        <label>
          max:&nbsp;
          <input type="number" step="1" value={max2}
                 onChange={(e) => setMax2(+e.target.value)}
                 style={{ width: 60 }} />
        </label>
        <label>
          Grid:&nbsp;
          <input type="number" min={10} max={200} value={steps}
                 onChange={(e) => setSteps(+e.target.value)}
                 style={{ width: 50 }} />
        </label>
      </div>

      <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 10 }}>
        <label>
          Field:&nbsp;
          <select value={fieldType} onChange={(e) => setFieldType(e.target.value as FieldType)}>
            <option value="E">|E| (V/m)</option>
            <option value="H">|H| (A/m)</option>
          </select>
        </label>
        <label>
          Dyn range:&nbsp;
          <input type="number" min={6} max={120} step={6} value={dynRange}
                 onChange={(e) => setDynRange(+e.target.value)}
                 style={{ width: 50 }} /> dB
        </label>
        <button onClick={handleCompute} disabled={loading}
                style={{ padding: '4px 12px', fontWeight: 600 }}>
          {loading ? 'Computing...' : 'Compute Near-Field'}
        </button>
      </div>

      {error && <div style={{ color: '#d44', marginBottom: 8 }}>{error}</div>}

      {result && (
        <DisplayGrid
          result={result}
          fieldType={fieldType}
          dynRange={dynRange}
          min1={min1} max1={max1} min2={min2} max2={max2}
          axis1Label={axis1Label} axis2Label={axis2Label}
          wireLines={wireLines}
          canvasRef={canvasRef}
        />
      )}

      {!result && !loading && (
        <div style={{ color: '#999', marginTop: 20 }}>
          Set observation plane and click "Compute Near-Field" to visualize E/H fields.
        </div>
      )}
    </div>
  );
};

export default NearFieldViewer;
