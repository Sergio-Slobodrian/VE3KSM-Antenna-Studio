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
import React, { useState, useCallback } from 'react';
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

/** Generic SVG line chart for time or frequency domain. */
const LineChart: React.FC<{
  data: { x: number; y: number }[];
  data2?: { x: number; y: number }[];
  width?: number;
  height?: number;
  xLabel: string;
  yLabel: string;
  color?: string;
  color2?: string;
  label1?: string;
  label2?: string;
}> = ({ data, data2, width = 520, height = 200, xLabel, yLabel, color = '#4fc3f7', color2 = '#ff9800', label1, label2 }) => {
  const PAD_L = 55;
  const PAD_R = 15;
  const PAD_T = 20;
  const PAD_B = 35;

  if (data.length < 2) return null;

  const allX = data.map((p) => p.x);
  const allY = data.map((p) => p.y);
  if (data2) {
    allY.push(...data2.map((p) => p.y));
  }

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

  const formatTick = (v: number) => {
    if (Math.abs(v) >= 1000) return v.toFixed(0);
    if (Math.abs(v) >= 1) return v.toFixed(1);
    return v.toExponential(1);
  };

  return (
    <svg width={width} height={height} style={{ display: 'block' }}>
      {/* Axes */}
      <line x1={PAD_L} y1={height - PAD_B} x2={width - PAD_R} y2={height - PAD_B} stroke="#555" strokeWidth={1} />
      <line x1={PAD_L} y1={PAD_T} x2={PAD_L} y2={height - PAD_B} stroke="#555" strokeWidth={1} />
      {/* Zero line if in range */}
      {yMin < 0 && yMax > 0 && (
        <line x1={PAD_L} y1={sy(0)} x2={width - PAD_R} y2={sy(0)} stroke="#444" strokeWidth={0.5} strokeDasharray="4,4" />
      )}
      {/* Axis labels */}
      <text x={(PAD_L + width - PAD_R) / 2} y={height - 4} textAnchor="middle" fontSize={10} fill="#aaa">{xLabel}</text>
      <text x={8} y={(PAD_T + height - PAD_B) / 2} textAnchor="middle" fontSize={10} fill="#aaa"
            transform={`rotate(-90, 8, ${(PAD_T + height - PAD_B) / 2})`}>{yLabel}</text>
      {/* Ticks */}
      <text x={PAD_L} y={height - PAD_B + 13} textAnchor="middle" fontSize={8} fill="#888">{formatTick(xMin)}</text>
      <text x={width - PAD_R} y={height - PAD_B + 13} textAnchor="middle" fontSize={8} fill="#888">{formatTick(xMax)}</text>
      <text x={PAD_L - 3} y={height - PAD_B + 3} textAnchor="end" fontSize={8} fill="#888">{formatTick(yMin)}</text>
      <text x={PAD_L - 3} y={PAD_T + 4} textAnchor="end" fontSize={8} fill="#888">{formatTick(yMax)}</text>
      {/* Data2 (background) */}
      {data2 && data2.length > 1 && (
        <path d={toPath(data2)} fill="none" stroke={color2} strokeWidth={1.2} opacity={0.7} />
      )}
      {/* Data1 */}
      <path d={toPath(data)} fill="none" stroke={color} strokeWidth={1.5} />
      {/* Legend */}
      {label1 && (
        <g>
          <line x1={PAD_L + 10} y1={PAD_T + 2} x2={PAD_L + 25} y2={PAD_T + 2} stroke={color} strokeWidth={2} />
          <text x={PAD_L + 28} y={PAD_T + 5} fontSize={9} fill="#ccc">{label1}</text>
        </g>
      )}
      {label2 && data2 && (
        <g>
          <line x1={PAD_L + 10} y1={PAD_T + 14} x2={PAD_L + 25} y2={PAD_T + 14} stroke={color2} strokeWidth={2} />
          <text x={PAD_L + 28} y={PAD_T + 17} fontSize={9} fill="#ccc">{label2}</text>
        </g>
      )}
    </svg>
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
      );
      setTransientResult(res);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [wires, source, loads, transmissionLines, ground, frequency, referenceImpedance,
      freqStart, freqEnd, numFreqs, pulseType, pulseWidth, centerFreq, response, setTransientResult]);

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

          {/* Time-domain waveform */}
          <div style={{ marginBottom: 16 }}>
            <div style={{ fontSize: 12, color: '#999', marginBottom: 4 }}>
              Time-domain response: {responseLabel}
            </div>
            <LineChart
              data={result.waveform.map((p) => ({ x: p.time_ns, y: p.amplitude }))}
              data2={result.excitation.map((p) => ({ x: p.time_ns, y: p.amplitude }))}
              xLabel="Time (ns)"
              yLabel="Amplitude"
              label1="Response"
              label2="Excitation"
            />
          </div>

          {/* Frequency-domain magnitude */}
          {result.frequencies.length > 1 && (
            <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 12, color: '#999', marginBottom: 4 }}>
                Transfer function |H(f)| (dB)
              </div>
              <LineChart
                data={result.frequencies.map((f, i) => ({ x: f, y: result.mag_response[i] }))}
                xLabel="Frequency (MHz)"
                yLabel="|H(f)| (dB)"
                color="#66bb6a"
              />
            </div>
          )}

          {/* Frequency-domain phase */}
          {result.frequencies.length > 1 && (
            <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 12, color: '#999', marginBottom: 4 }}>
                Transfer function phase (degrees)
              </div>
              <LineChart
                data={result.frequencies.map((f, i) => ({ x: f, y: result.phase_response[i] }))}
                xLabel="Frequency (MHz)"
                yLabel="Phase (deg)"
                color="#ab47bc"
              />
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
