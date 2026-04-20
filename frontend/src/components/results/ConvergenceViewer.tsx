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
 * Convergence reporter: re-runs the simulation at 2× segmentation and
 * compares impedance, SWR, and gain to the 1× result.  A small delta
 * means the user's mesh is adequately resolved; a large delta means
 * they should increase segment counts.
 */
import React, { useState, useCallback } from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import { checkConvergence } from '@/api/client';

/** Colour-coded delta display: green < 2%, yellow 2–5%, red > 5%. */
const DeltaCell: React.FC<{ value: number; unit?: string }> = ({ value, unit = '%' }) => {
  const abs = Math.abs(value);
  let color = '#66bb6a'; // green
  if (abs >= 5) color = '#ef5350'; // red
  else if (abs >= 2) color = '#ffa726'; // orange

  return (
    <span style={{ color, fontWeight: 500 }}>
      {value >= 0 ? '+' : ''}{value.toFixed(2)}{unit}
    </span>
  );
};

const ConvergenceViewer: React.FC = () => {
  const wires = useAntennaStore((s) => s.wires);
  const source = useAntennaStore((s) => s.source);
  const loads = useAntennaStore((s) => s.loads);
  const transmissionLines = useAntennaStore((s) => s.transmissionLines);
  const ground = useAntennaStore((s) => s.ground);
  const frequency = useAntennaStore((s) => s.frequency);
  const referenceImpedance = useAntennaStore((s) => s.referenceImpedance);
  const weather = useAntennaStore((s) => s.weather);

  const result = useAntennaStore((s) => s.convergenceResult);
  const setConvergenceResult = useAntennaStore((s) => s.setConvergenceResult);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleCheck = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await checkConvergence(
        wires, source, loads, transmissionLines,
        ground, frequency, referenceImpedance, weather,
      );
      setConvergenceResult(res);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [wires, source, loads, transmissionLines, ground, frequency, referenceImpedance, weather, setConvergenceResult]);

  return (
    <div style={{ padding: 12 }}>
      <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 12 }}>
        <button onClick={handleCheck} disabled={loading}
                style={{ padding: '4px 14px', fontWeight: 600 }}>
          {loading ? 'Checking…' : 'Check Convergence'}
        </button>
        {result && (
          <span style={{
            fontSize: 12,
            fontWeight: 600,
            color: result.converged ? '#66bb6a' : '#ffa726',
          }}>
            {result.converged ? '✓ Converged' : '⚠ Not converged'}
          </span>
        )}
      </div>

      {error && <div style={{ color: '#d44', marginBottom: 8 }}>{error}</div>}

      {result && (
        <div>
          {/* Verdict banner */}
          <div style={{
            padding: '8px 12px',
            marginBottom: 16,
            borderRadius: 4,
            background: result.converged ? 'rgba(102,187,106,0.1)' : 'rgba(255,167,38,0.1)',
            border: `1px solid ${result.converged ? '#66bb6a' : '#ffa726'}`,
            fontSize: 12,
            color: '#ddd',
          }}>
            {result.verdict}
          </div>

          {/* Comparison table */}
          <table style={{ borderCollapse: 'collapse', fontSize: 12, minWidth: 500, marginBottom: 16 }}>
            <thead>
              <tr style={{ borderBottom: '1px solid #555' }}>
                <th style={thStyle}>Parameter</th>
                <th style={thStyle}>1× ({result.total_segments_1x} segs)</th>
                <th style={thStyle}>2× ({result.total_segments_2x} segs)</th>
                <th style={thStyle}>Change</th>
              </tr>
            </thead>
            <tbody>
              <tr style={rowStyle}>
                <td style={tdLabel}>Resistance R (Ω)</td>
                <td style={tdVal}>{result.impedance_r_1x.toFixed(2)}</td>
                <td style={tdVal}>{result.impedance_r_2x.toFixed(2)}</td>
                <td style={tdVal}><DeltaCell value={result.delta_r_pct} /></td>
              </tr>
              <tr style={rowStyle}>
                <td style={tdLabel}>Reactance X (Ω)</td>
                <td style={tdVal}>{result.impedance_x_1x.toFixed(2)}</td>
                <td style={tdVal}>{result.impedance_x_2x.toFixed(2)}</td>
                <td style={tdVal}><DeltaCell value={result.delta_x_pct} /></td>
              </tr>
              <tr style={rowStyle}>
                <td style={tdLabel}>|Z| magnitude</td>
                <td style={tdVal}>—</td>
                <td style={tdVal}>—</td>
                <td style={tdVal}><DeltaCell value={result.delta_z_mag_pct} /></td>
              </tr>
              <tr style={rowStyle}>
                <td style={tdLabel}>SWR</td>
                <td style={tdVal}>{result.swr_1x.toFixed(3)}</td>
                <td style={tdVal}>{result.swr_2x.toFixed(3)}</td>
                <td style={tdVal}><DeltaCell value={result.delta_swr_pct} /></td>
              </tr>
              <tr style={rowStyle}>
                <td style={tdLabel}>Peak Gain (dBi)</td>
                <td style={tdVal}>{result.gain_dbi_1x.toFixed(2)}</td>
                <td style={tdVal}>{result.gain_dbi_2x.toFixed(2)}</td>
                <td style={tdVal}><DeltaCell value={result.delta_gain_db} unit=" dB" /></td>
              </tr>
            </tbody>
          </table>

          {/* Visual delta bar */}
          <div style={{ marginBottom: 16 }}>
            <div style={{ fontSize: 12, color: '#999', marginBottom: 6 }}>
              Impedance magnitude change: {Math.abs(result.delta_z_mag_pct).toFixed(2)}%
            </div>
            <div style={{ position: 'relative', height: 20, background: '#2a2a2a', borderRadius: 4, overflow: 'hidden', maxWidth: 400 }}>
              {/* Threshold markers */}
              <div style={{ position: 'absolute', left: '10%', top: 0, bottom: 0, width: 1, background: '#66bb6a', opacity: 0.5 }} />
              <div style={{ position: 'absolute', left: '20%', top: 0, bottom: 0, width: 1, background: '#ffa726', opacity: 0.5 }} />
              <div style={{ position: 'absolute', left: '50%', top: 0, bottom: 0, width: 1, background: '#ef5350', opacity: 0.5 }} />
              {/* Bar */}
              <div style={{
                height: '100%',
                width: `${Math.min(100, Math.abs(result.delta_z_mag_pct) * 10)}%`,
                background: Math.abs(result.delta_z_mag_pct) < 2 ? '#66bb6a' :
                             Math.abs(result.delta_z_mag_pct) < 5 ? '#ffa726' : '#ef5350',
                borderRadius: 4,
                transition: 'width 0.3s ease',
              }} />
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', maxWidth: 400, fontSize: 9, color: '#666', marginTop: 2 }}>
              <span>0%</span>
              <span>1% excellent</span>
              <span>2% good</span>
              <span>5% marginal</span>
              <span>10%+</span>
            </div>
          </div>
        </div>
      )}

      {!result && !loading && (
        <div style={{ color: '#999', marginTop: 20 }}>
          Click "Check Convergence" to compare your current segmentation with a 2× refined
          mesh.  The solver runs twice — once at your settings and once with doubled segments
          on every wire — then reports how much the impedance, SWR, and gain changed.
          A change below 2% indicates good convergence; above 5% suggests you need more segments.
        </div>
      )}
    </div>
  );
};

const thStyle: React.CSSProperties = {
  textAlign: 'left',
  padding: '4px 10px',
  fontWeight: 600,
  color: '#bbb',
};

const rowStyle: React.CSSProperties = {
  borderBottom: '1px solid #333',
};

const tdLabel: React.CSSProperties = {
  padding: '4px 10px',
  color: '#aaa',
};

const tdVal: React.CSSProperties = {
  padding: '4px 10px',
  color: '#eee',
};

export default ConvergenceViewer;
