/**
 * Characteristic Mode Analysis (CMA) viewer.
 *
 * Displays:
 *  - A bar chart of modal significance (MS) for each mode
 *  - A table of mode index, eigenvalue λ, MS, and characteristic angle α
 *  - A current-distribution bar for the selected mode
 *
 * The user presses "Compute CMA" to trigger a POST /api/cma request.
 * CMA is source-free: the backend assembles the Z-matrix without excitation
 * and solves the generalised eigenproblem X·J = λ·R·J.
 */
import React, { useState, useCallback, useRef, useEffect } from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import { computeCMA } from '@/api/client';
import type { CMAResult, CMAMode } from '@/types';

/** Jet-style colour map (same as NearFieldViewer) for current magnitudes. */
function jetColor(t: number): string {
  const c = Math.max(0, Math.min(1, t));
  let r: number, g: number, b: number;
  if (c < 0.25) {
    r = 0; g = 4 * c; b = 1;
  } else if (c < 0.5) {
    r = 0; g = 1; b = 1 - 4 * (c - 0.25);
  } else if (c < 0.75) {
    r = 4 * (c - 0.5); g = 1; b = 0;
  } else {
    r = 1; g = 1 - 4 * (c - 0.75); b = 0;
  }
  return `rgb(${Math.round(r * 255)},${Math.round(g * 255)},${Math.round(b * 255)})`;
}

/** Horizontal bar chart showing modal significance for each mode. */
const MSBarChart: React.FC<{
  modes: CMAMode[];
  selectedMode: number;
  onSelectMode: (idx: number) => void;
}> = ({ modes, selectedMode, onSelectMode }) => {
  const BAR_H = 22;
  const GAP = 3;
  const MAX_W = 400;
  const LABEL_W = 50;
  const displayModes = modes.slice(0, 20); // Show top 20

  return (
    <svg
      width={MAX_W + LABEL_W + 60}
      height={displayModes.length * (BAR_H + GAP) + 10}
      style={{ display: 'block' }}
    >
      {displayModes.map((m, i) => {
        const y = i * (BAR_H + GAP) + 5;
        const w = m.modal_significance * MAX_W;
        const isSelected = m.index === selectedMode;
        return (
          <g key={m.index} onClick={() => onSelectMode(m.index)}
             style={{ cursor: 'pointer' }}>
            <text x={LABEL_W - 4} y={y + BAR_H / 2 + 4}
                  textAnchor="end" fontSize={11} fill="#ccc">
              Mode {m.index}
            </text>
            <rect x={LABEL_W} y={y} width={w} height={BAR_H}
                  rx={3}
                  fill={isSelected ? '#4fc3f7' : '#66bb6a'}
                  stroke={isSelected ? '#fff' : 'none'}
                  strokeWidth={isSelected ? 1.5 : 0}
                  opacity={0.9} />
            <text x={LABEL_W + w + 4} y={y + BAR_H / 2 + 4}
                  fontSize={11} fill="#aaa">
              {m.modal_significance.toFixed(3)}
            </text>
          </g>
        );
      })}
    </svg>
  );
};

/** Modal current magnitude strip for a single mode. */
const CurrentStrip: React.FC<{ mode: CMAMode }> = ({ mode }) => {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const mags = mode.current_magnitudes;

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || !mags || mags.length === 0) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const n = mags.length;
    canvas.width = n;
    canvas.height = 1;
    const img = ctx.createImageData(n, 1);
    for (let i = 0; i < n; i++) {
      const color = jetColor(mags[i]);
      const match = color.match(/rgb\((\d+),(\d+),(\d+)\)/);
      const off = i * 4;
      img.data[off] = match ? parseInt(match[1]) : 0;
      img.data[off + 1] = match ? parseInt(match[2]) : 0;
      img.data[off + 2] = match ? parseInt(match[3]) : 0;
      img.data[off + 3] = 255;
    }
    ctx.putImageData(img, 0, 0);
  }, [mags]);

  if (!mags || mags.length === 0) return null;

  return (
    <div style={{ marginTop: 8 }}>
      <div style={{ fontSize: 12, marginBottom: 4, color: '#bbb' }}>
        Modal current distribution (Mode {mode.index}) — normalised magnitude
      </div>
      <canvas
        ref={canvasRef}
        style={{
          width: '100%',
          maxWidth: 500,
          height: 24,
          imageRendering: 'pixelated',
          border: '1px solid #555',
        }}
      />
      <div style={{ display: 'flex', justifyContent: 'space-between', maxWidth: 500, fontSize: 10, color: '#888' }}>
        <span>Segment 0</span>
        <span>Segment {mags.length - 1}</span>
      </div>
      {/* Jet colour-map legend */}
      <div style={{ marginTop: 10, maxWidth: 500 }}>
        <div style={{ fontSize: 11, color: '#aaa', marginBottom: 3 }}>Current magnitude</div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <span style={{ fontSize: 10, color: '#888' }}>0</span>
          <div style={{
            flex: 1,
            height: 12,
            borderRadius: 2,
            border: '1px solid #555',
            background: `linear-gradient(to right, ${jetColor(0)}, ${jetColor(0.125)}, ${jetColor(0.25)}, ${jetColor(0.375)}, ${jetColor(0.5)}, ${jetColor(0.625)}, ${jetColor(0.75)}, ${jetColor(0.875)}, ${jetColor(1)})`,
          }} />
          <span style={{ fontSize: 10, color: '#888' }}>1</span>
        </div>
        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 9, color: '#666', marginTop: 1, paddingLeft: 14, paddingRight: 14 }}>
          <span>low</span>
          <span>mid</span>
          <span>high</span>
        </div>
      </div>
    </div>
  );
};

const CMAViewer: React.FC = () => {
  const wires = useAntennaStore((s) => s.wires);
  const source = useAntennaStore((s) => s.source);
  const loads = useAntennaStore((s) => s.loads);
  const transmissionLines = useAntennaStore((s) => s.transmissionLines);
  const ground = useAntennaStore((s) => s.ground);
  const frequency = useAntennaStore((s) => s.frequency);
  const referenceImpedance = useAntennaStore((s) => s.referenceImpedance);

  const [result, setResult] = useState<CMAResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedMode, setSelectedMode] = useState(1);

  const handleCompute = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await computeCMA(
        wires, source, loads, transmissionLines,
        ground, frequency, referenceImpedance,
      );
      setResult(res);
      setSelectedMode(1);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [wires, source, loads, transmissionLines, ground, frequency, referenceImpedance]);

  const selectedModeData = result?.modes.find((m) => m.index === selectedMode) ?? null;

  return (
    <div style={{ padding: 12 }}>
      <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 12 }}>
        <button onClick={handleCompute} disabled={loading}
                style={{ padding: '4px 14px', fontWeight: 600 }}>
          {loading ? 'Computing…' : 'Compute CMA'}
        </button>
        {result && (
          <span style={{ fontSize: 12, color: '#aaa' }}>
            {result.num_modes} modes at {result.freq_mhz.toFixed(3)} MHz
          </span>
        )}
      </div>

      {error && <div style={{ color: '#d44', marginBottom: 8 }}>{error}</div>}

      {result && result.modes.length > 0 && (
        <>
          {/* Modal significance bar chart */}
          <div style={{ marginBottom: 16 }}>
            <h4 style={{ margin: '0 0 6px', fontSize: 13, color: '#ddd' }}>
              Modal Significance (click to select)
            </h4>
            <MSBarChart
              modes={result.modes}
              selectedMode={selectedMode}
              onSelectMode={setSelectedMode}
            />
          </div>

          {/* Mode table */}
          <div style={{ overflowX: 'auto', marginBottom: 16 }}>
            <table style={{ borderCollapse: 'collapse', fontSize: 12, minWidth: 500 }}>
              <thead>
                <tr style={{ borderBottom: '1px solid #555' }}>
                  <th style={thStyle}>Mode</th>
                  <th style={thStyle}>Eigenvalue λ</th>
                  <th style={thStyle}>Modal Sig. MS</th>
                  <th style={thStyle}>Char. Angle α (°)</th>
                  <th style={thStyle}>Behaviour</th>
                </tr>
              </thead>
              <tbody>
                {result.modes.slice(0, 20).map((m) => (
                  <tr key={m.index}
                      onClick={() => setSelectedMode(m.index)}
                      style={{
                        cursor: 'pointer',
                        background: m.index === selectedMode ? 'rgba(79,195,247,0.15)' : 'transparent',
                        borderBottom: '1px solid #333',
                      }}>
                    <td style={tdStyle}>{m.index}</td>
                    <td style={tdStyle}>{m.eigenvalue.toFixed(4)}</td>
                    <td style={tdStyle}>{m.modal_significance.toFixed(4)}</td>
                    <td style={tdStyle}>{m.characteristic_angle.toFixed(1)}</td>
                    <td style={tdStyle}>{modeBehaviour(m)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Current distribution strip for selected mode */}
          {selectedModeData && <CurrentStrip mode={selectedModeData} />}
        </>
      )}

      {!result && !loading && (
        <div style={{ color: '#999', marginTop: 20 }}>
          Click "Compute CMA" to perform Characteristic Mode Analysis at the current frequency.
          CMA decomposes the antenna's radiation behaviour into orthogonal natural modes,
          independent of the excitation source.
        </div>
      )}
    </div>
  );
};

/** Classify a mode based on its characteristic angle. */
function modeBehaviour(m: CMAMode): string {
  const alpha = m.characteristic_angle;
  if (Math.abs(alpha - 180) < 5) return 'Resonant';
  if (alpha < 180) return 'Inductive (stores H)';
  return 'Capacitive (stores E)';
}

const thStyle: React.CSSProperties = {
  textAlign: 'left',
  padding: '4px 10px',
  fontWeight: 600,
  color: '#bbb',
};

const tdStyle: React.CSSProperties = {
  padding: '4px 10px',
  color: '#ddd',
};

export default CMAViewer;
