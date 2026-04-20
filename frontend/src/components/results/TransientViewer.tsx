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
 * Time-domain transient analysis viewer.
 *
 * Runs a frequency-domain sweep then IFFT to show the antenna's
 * impulse/transient response at the feed point.  Displays:
 *
 *  - Time-domain waveform (reflected voltage, input voltage, or current)
 *  - Excitation pulse for reference
 *  - Frequency-domain transfer function |H(f)| and phase
 *  - Summary metrics: peak, ringdown time, FWHM
 */
import React, { useState, useCallback, useEffect } from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import { computeTransient } from '@/api/client';

const PULSE_TYPES = [
  { value: 'gaussian', label: 'Gaussian' },
  { value: 'step', label: 'Step (RC rise)' },
  { value: 'modulated_gaussian', label: 'Modulated Gaussian' },
];

const RESPONSE_TYPES = [
  { value: 'reflection', label: 'Reflected voltage (S11)' },
  { value: 'input', label: 'Input voltage (Z/(Z+Z0))' },
  { value: 'current', label: 'Feed current (1/Z)' },
];

/** Shared chart data descriptor passed to both small and zoomed views. */
interface ChartData {
  data: { x: number; y: number }[];
  data2?: { x: number; y: number }[];
  xLabel: string;
  yLabel: string;
  title: string;
  color?: string;
  color2?: string;
  label1?: string;
  label2?: string;
}

// ─── Tick computation ───────────────────────────────────────────────

/** Pick ~count "nice" tick values spanning [lo, hi]. */
function niceTicks(lo: number, hi: number, count: number): number[] {
  const span = hi - lo;
  if (span === 0) return [lo];
  const rough = span / count;
  const mag = Math.pow(10, Math.floor(Math.log10(rough)));
  const residual = rough / mag;
  let nice: number;
  if (residual <= 1.5) nice = mag;
  else if (residual <= 3) nice = 2 * mag;
  else if (residual <= 7) nice = 5 * mag;
  else nice = 10 * mag;
  const start = Math.ceil(lo / nice) * nice;
  const ticks: number[] = [];
  for (let v = start; v <= hi + nice * 0.001; v += nice) {
    ticks.push(+v.toPrecision(12));
  }
  return ticks;
}

/** Smart tick label: avoid unnecessary decimals but keep precision. */
function formatTickLabel(v: number): string {
  const abs = Math.abs(v);
  if (abs === 0) return '0';
  if (abs >= 10000) return v.toFixed(0);
  if (abs >= 100) return v.toFixed(1);
  if (abs >= 1) return v.toFixed(2);
  if (abs >= 0.01) return v.toFixed(3);
  return v.toExponential(2);
}

// ─── Full-featured SVG chart (used for both thumbnail and zoom) ─────

const DetailChart: React.FC<{
  chart: ChartData;
  width: number;
  height: number;
  showGrid?: boolean;
  xTicks?: number;
  yTicks?: number;
  fontSize?: number;
  strokeWidth?: number;
}> = ({ chart, width, height, showGrid = false, xTicks = 2, yTicks = 2, fontSize = 8, strokeWidth = 1.5 }) => {
  const { data, data2, xLabel, yLabel, color = '#4fc3f7', color2 = '#ff9800', label1, label2 } = chart;
  const PAD_L = showGrid ? 70 : 55;
  const PAD_R = 20;
  const PAD_T = showGrid ? 30 : 20;
  const PAD_B = showGrid ? 45 : 35;

  if (data.length < 2) return null;

  const allX = data.map((p) => p.x);
  const allY = data.map((p) => p.y);
  if (data2) allY.push(...data2.map((p) => p.y));

  const xMin = Math.min(...allX);
  const xMax = Math.max(...allX);
  const yMin = Math.min(...allY);
  const yMax = Math.max(...allY);
  const xSpan = xMax - xMin || 1;
  const ySpan = yMax - yMin || 1;

  const sx = (v: number) => PAD_L + ((v - xMin) / xSpan) * (width - PAD_L - PAD_R);
  const sy = (v: number) => height - PAD_B - ((v - yMin) / ySpan) * (height - PAD_T - PAD_B);

  const toPath = (pts: { x: number; y: number }[]) =>
    pts.map((p, i) => `${i === 0 ? 'M' : 'L'}${sx(p.x).toFixed(1)},${sy(p.y).toFixed(1)}`).join(' ');

  const xt = niceTicks(xMin, xMax, xTicks);
  const yt = niceTicks(yMin, yMax, yTicks);

  return (
    <svg width={width} height={height} style={{ display: 'block' }}>
      {/* Grid lines */}
      {showGrid && xt.map((v) => (
        <line key={`gx${v}`} x1={sx(v)} y1={PAD_T} x2={sx(v)} y2={height - PAD_B}
              stroke="#333" strokeWidth={0.5} />
      ))}
      {showGrid && yt.map((v) => (
        <line key={`gy${v}`} x1={PAD_L} y1={sy(v)} x2={width - PAD_R} y2={sy(v)}
              stroke="#333" strokeWidth={0.5} />
      ))}
      {/* Axes */}
      <line x1={PAD_L} y1={height - PAD_B} x2={width - PAD_R} y2={height - PAD_B} stroke="#666" strokeWidth={1} />
      <line x1={PAD_L} y1={PAD_T} x2={PAD_L} y2={height - PAD_B} stroke="#666" strokeWidth={1} />
      {/* Zero line if in range */}
      {yMin < 0 && yMax > 0 && (
        <line x1={PAD_L} y1={sy(0)} x2={width - PAD_R} y2={sy(0)} stroke="#555" strokeWidth={0.7} strokeDasharray="6,3" />
      )}
      {/* X ticks + labels */}
      {xt.map((v) => (
        <g key={`xt${v}`}>
          <line x1={sx(v)} y1={height - PAD_B} x2={sx(v)} y2={height - PAD_B + 4} stroke="#666" strokeWidth={1} />
          <text x={sx(v)} y={height - PAD_B + 5 + fontSize} textAnchor="middle" fontSize={fontSize} fill="#999">
            {formatTickLabel(v)}
          </text>
        </g>
      ))}
      {/* Y ticks + labels */}
      {yt.map((v) => (
        <g key={`yt${v}`}>
          <line x1={PAD_L - 4} y1={sy(v)} x2={PAD_L} y2={sy(v)} stroke="#666" strokeWidth={1} />
          <text x={PAD_L - 6} y={sy(v) + fontSize / 2.5} textAnchor="end" fontSize={fontSize} fill="#999">
            {formatTickLabel(v)}
          </text>
        </g>
      ))}
      {/* Axis labels */}
      <text x={(PAD_L + width - PAD_R) / 2} y={height - 4} textAnchor="middle" fontSize={fontSize + 2} fill="#aaa">{xLabel}</text>
      <text x={12} y={(PAD_T + height - PAD_B) / 2} textAnchor="middle" fontSize={fontSize + 2} fill="#aaa"
            transform={`rotate(-90, 12, ${(PAD_T + height - PAD_B) / 2})`}>{yLabel}</text>
      {/* Title (zoomed view only) */}
      {showGrid && (
        <text x={width / 2} y={16} textAnchor="middle" fontSize={fontSize + 3} fill="#ccc" fontWeight={600}>
          {chart.title}
        </text>
      )}
      {/* Data2 (background trace) */}
      {data2 && data2.length > 1 && (
        <path d={toPath(data2)} fill="none" stroke={color2} strokeWidth={strokeWidth * 0.8} opacity={0.7} />
      )}
      {/* Data1 */}
      <path d={toPath(data)} fill="none" stroke={color} strokeWidth={strokeWidth} />
      {/* Legend */}
      {label1 && (
        <g>
          <line x1={PAD_L + 10} y1={PAD_T + 2} x2={PAD_L + 25} y2={PAD_T + 2} stroke={color} strokeWidth={2} />
          <text x={PAD_L + 28} y={PAD_T + 5} fontSize={fontSize + 1} fill="#ccc">{label1}</text>
        </g>
      )}
      {label2 && data2 && (
        <g>
          <line x1={PAD_L + 10} y1={PAD_T + 14} x2={PAD_L + 25} y2={PAD_T + 14} stroke={color2} strokeWidth={2} />
          <text x={PAD_L + 28} y={PAD_T + 17} fontSize={fontSize + 1} fill="#ccc">{label2}</text>
        </g>
      )}
    </svg>
  );
};

// ─── CSV export helper ──────────────────────────────────────────────

function exportCsv(chart: ChartData) {
  const hasTrace2 = chart.data2 && chart.data2.length > 0;
  const hdr = hasTrace2
    ? `${chart.xLabel},${chart.label1 || 'Trace 1'},${chart.label2 || 'Trace 2'}`
    : `${chart.xLabel},${chart.yLabel}`;

  const rows: string[] = [hdr];
  const n = chart.data.length;
  for (let i = 0; i < n; i++) {
    const p = chart.data[i];
    if (hasTrace2 && chart.data2 && i < chart.data2.length) {
      rows.push(`${p.x},${p.y},${chart.data2[i].y}`);
    } else {
      rows.push(`${p.x},${p.y}`);
    }
  }
  const blob = new Blob([rows.join('\n')], { type: 'text/csv' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = chart.title.replace(/[^a-zA-Z0-9_()-]/g, '_') + '.csv';
  a.click();
  URL.revokeObjectURL(url);
}

// ─── Zoom modal overlay ─────────────────────────────────────────────

const ZoomModal: React.FC<{
  chart: ChartData;
  onClose: () => void;
}> = ({ chart, onClose }) => {
  // Close on Escape
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [onClose]);

  return (
    <div
      onClick={onClose}
      style={{
        position: 'fixed', inset: 0, zIndex: 9999,
        background: 'rgba(0,0,0,0.75)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          background: '#1e1e1e',
          border: '1px solid #444',
          borderRadius: 8,
          padding: 16,
          maxWidth: '95vw',
          maxHeight: '90vh',
          overflow: 'auto',
        }}
      >
        <DetailChart chart={chart} width={900} height={480}
                     showGrid xTicks={10} yTicks={8} fontSize={11} strokeWidth={2} />
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: 10 }}>
          <button onClick={() => exportCsv(chart)}
                  style={{ padding: '4px 14px', fontSize: 12, fontWeight: 600, cursor: 'pointer' }}>
            Export CSV
          </button>
          <span style={{ fontSize: 11, color: '#666' }}>Press Esc or click outside to close</span>
          <button onClick={onClose}
                  style={{ padding: '4px 14px', fontSize: 12, cursor: 'pointer' }}>
            Close
          </button>
        </div>
      </div>
    </div>
  );
};

// ─── Clickable thumbnail chart ──────────────────────────────────────

const ClickableChart: React.FC<{ chart: ChartData }> = ({ chart }) => {
  const [zoomed, setZoomed] = useState(false);

  return (
    <>
      <div
        onClick={() => setZoomed(true)}
        style={{ cursor: 'zoom-in', display: 'inline-block' }}
        title="Click to enlarge"
      >
        <DetailChart chart={chart} width={520} height={200} xTicks={4} yTicks={3} />
      </div>
      {zoomed && <ZoomModal chart={chart} onClose={() => setZoomed(false)} />}
    </>
  );
};

const TransientViewer: React.FC = () => {
  const wires = useAntennaStore((s) => s.wires);
  const source = useAntennaStore((s) => s.source);
  const loads = useAntennaStore((s) => s.loads);
  const transmissionLines = useAntennaStore((s) => s.transmissionLines);
  const ground = useAntennaStore((s) => s.ground);
  const frequency = useAntennaStore((s) => s.frequency);
  const referenceImpedance = useAntennaStore((s) => s.referenceImpedance);
  const weather = useAntennaStore((s) => s.weather);

  // Settings
  const [freqStart, setFreqStart] = useState(frequency.freqStart || Math.max(0.1, frequency.frequencyMhz - 5));
  const [freqEnd, setFreqEnd] = useState(frequency.freqEnd || frequency.frequencyMhz + 5);
  const [numFreqs, setNumFreqs] = useState(128);
  const [pulseType, setPulseType] = useState('gaussian');
  const [pulseWidth, setPulseWidth] = useState(2.0);
  const [centerFreq, setCenterFreq] = useState(frequency.frequencyMhz);
  const [response, setResponse] = useState('reflection');

  // State (result persisted in store)
  const result = useAntennaStore((s) => s.transientResult);
  const setTransientResult = useAntennaStore((s) => s.setTransientResult);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleCompute = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await computeTransient(
        wires, source, loads, transmissionLines,
        ground, frequency, referenceImpedance,
        {
          freqStartMhz: freqStart,
          freqEndMhz: freqEnd,
          numFreqs,
          pulseType,
          pulseWidthNs: pulseWidth,
          centerFreqMhz: pulseType === 'modulated_gaussian' ? centerFreq : undefined,
          response,
        },
        weather,
      );
      setTransientResult(res);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [wires, source, loads, transmissionLines, ground, frequency, referenceImpedance,
      freqStart, freqEnd, numFreqs, pulseType, pulseWidth, centerFreq, response, weather, setTransientResult]);

  const responseLabel = RESPONSE_TYPES.find((r) => r.value === response)?.label ?? response;

  return (
    <div style={{ padding: 12, overflowY: 'auto', maxHeight: '100%' }}>
      {/* Configuration */}
      <div style={{ marginBottom: 16 }}>
        <h4 style={{ margin: '0 0 6px', fontSize: 13, color: '#ddd' }}>Frequency Range</h4>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
          <label style={labelSmall}>Start MHz:</label>
          <input type="number" step="any" value={freqStart}
                 onChange={(e) => setFreqStart(+e.target.value)}
                 style={{ ...inputStyle, width: 80 }} />
          <label style={labelSmall}>End MHz:</label>
          <input type="number" step="any" value={freqEnd}
                 onChange={(e) => setFreqEnd(+e.target.value)}
                 style={{ ...inputStyle, width: 80 }} />
          <label style={labelSmall}>Points:</label>
          <select value={numFreqs} onChange={(e) => setNumFreqs(+e.target.value)} style={{ ...inputStyle, width: 65 }}>
            {[32, 64, 128, 256, 512].map((n) => <option key={n} value={n}>{n}</option>)}
          </select>
        </div>
      </div>

      <div style={{ marginBottom: 16 }}>
        <h4 style={{ margin: '0 0 6px', fontSize: 13, color: '#ddd' }}>Excitation Pulse</h4>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
          <select value={pulseType} onChange={(e) => setPulseType(e.target.value)}
                  style={{ ...inputStyle, width: 160 }}>
            {PULSE_TYPES.map((p) => <option key={p.value} value={p.value}>{p.label}</option>)}
          </select>
          <label style={labelSmall}>Width (ns):</label>
          <input type="number" step="0.1" min="0.1" value={pulseWidth}
                 onChange={(e) => setPulseWidth(+e.target.value)}
                 style={{ ...inputStyle, width: 60 }} />
          {pulseType === 'modulated_gaussian' && (
            <>
              <label style={labelSmall}>Carrier MHz:</label>
              <input type="number" step="any" value={centerFreq}
                     onChange={(e) => setCenterFreq(+e.target.value)}
                     style={{ ...inputStyle, width: 80 }} />
            </>
          )}
        </div>
      </div>

      <div style={{ display: 'flex', gap: 12, alignItems: 'center', marginBottom: 16, flexWrap: 'wrap' }}>
        <label style={labelSmall}>Response:</label>
        <select value={response} onChange={(e) => setResponse(e.target.value)}
                style={{ ...inputStyle, width: 180 }}>
          {RESPONSE_TYPES.map((r) => <option key={r.value} value={r.value}>{r.label}</option>)}
        </select>
        <button onClick={handleCompute} disabled={loading}
                style={{ padding: '4px 16px', fontWeight: 600 }}>
          {loading ? 'Computing...' : 'Compute Transient'}
        </button>
      </div>

      {error && <div style={{ color: '#d44', marginBottom: 8 }}>{error}</div>}

      {/* Results */}
      {result && (
        <div>
          {/* Metrics */}
          <div style={{ display: 'flex', gap: 20, flexWrap: 'wrap', marginBottom: 16 }}>
            <MetricBox label="Peak amplitude" value={result.peak_amplitude.toExponential(3)} />
            <MetricBox label="Peak time" value={`${result.peak_time_ns.toFixed(2)} ns`} />
            <MetricBox label="Ringdown (-20 dB)" value={`${result.ringdown_time_ns.toFixed(2)} ns`} />
            <MetricBox label="Pulse FWHM" value={`${result.pulse_fwhm_ns.toFixed(2)} ns`} />
          </div>

          {/* Time-domain waveform — click to zoom */}
          <div style={{ marginBottom: 16 }}>
            <div style={{ fontSize: 12, color: '#999', marginBottom: 4 }}>
              Time-domain response: {responseLabel}
              <span style={{ marginLeft: 8, fontSize: 10, color: '#666' }}>(click chart to zoom)</span>
            </div>
            <ClickableChart chart={{
              data: result.waveform.map((p) => ({ x: p.time_ns, y: p.amplitude })),
              data2: result.excitation.map((p) => ({ x: p.time_ns, y: p.amplitude })),
              xLabel: 'Time (ns)',
              yLabel: 'Amplitude',
              title: `Time-domain ${responseLabel}`,
              label1: 'Response',
              label2: 'Excitation',
            }} />
          </div>

          {/* Frequency-domain magnitude — click to zoom */}
          {result.frequencies.length > 1 && (
            <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 12, color: '#999', marginBottom: 4 }}>
                Transfer function |H(f)|
                <span style={{ marginLeft: 8, fontSize: 10, color: '#666' }}>(click chart to zoom)</span>
              </div>
              <ClickableChart chart={{
                data: result.frequencies.map((f, i) => ({ x: f, y: result.mag_response[i] })),
                xLabel: 'Frequency (MHz)',
                yLabel: '|H(f)| (dB)',
                title: 'Transfer Function Magnitude |H(f)|',
                color: '#66bb6a',
              }} />
            </div>
          )}

          {/* Frequency-domain phase — click to zoom */}
          {result.frequencies.length > 1 && (
            <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 12, color: '#999', marginBottom: 4 }}>
                Transfer function phase
                <span style={{ marginLeft: 8, fontSize: 10, color: '#666' }}>(click chart to zoom)</span>
              </div>
              <ClickableChart chart={{
                data: result.frequencies.map((f, i) => ({ x: f, y: result.phase_response[i] })),
                xLabel: 'Frequency (MHz)',
                yLabel: 'Phase (deg)',
                title: 'Transfer Function Phase',
                color: '#ab47bc',
              }} />
            </div>
          )}
        </div>
      )}

      {!result && !loading && (
        <div style={{ color: '#999', marginTop: 8, fontSize: 12 }}>
          Configure the frequency range and excitation pulse, then click "Compute Transient"
          to see the time-domain response at the antenna feed point. The analysis runs a
          dense frequency sweep, then applies an inverse FFT to produce the transient waveform.
          Useful for UWB antenna design, pulse fidelity, and ringdown analysis.
        </div>
      )}
    </div>
  );
};

/** Small metric display box. */
const MetricBox: React.FC<{ label: string; value: string }> = ({ label, value }) => (
  <div style={{ fontSize: 12 }}>
    <span style={{ color: '#888' }}>{label}: </span>
    <span style={{ color: '#eee', fontWeight: 500 }}>{value}</span>
  </div>
);

const inputStyle: React.CSSProperties = {
  padding: '2px 4px',
  fontSize: 11,
  background: '#2a2a2a',
  color: '#ddd',
  border: '1px solid #555',
  borderRadius: 3,
};

const labelSmall: React.CSSProperties = {
  fontSize: 11,
  color: '#999',
};

export default TransientViewer;
