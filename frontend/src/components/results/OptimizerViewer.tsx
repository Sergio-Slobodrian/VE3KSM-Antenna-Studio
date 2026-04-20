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
 * PSO Optimizer configuration and results viewer.
 *
 * Lets the user:
 *  1. Define tuneable variables by selecting wire fields and bounds
 *  2. Configure objective goals (minimise SWR, maximise gain, etc.)
 *  3. Optionally specify a frequency band for worst-case optimisation
 *  4. Set PSO hyper-parameters (particles, iterations)
 *  5. Run the optimiser and view convergence, best metrics, and apply results
 */
import React, { useState, useCallback } from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import { runOptimizer } from '@/api/client';
import type { OptimVariable, OptimGoal } from '@/types';

// Available metrics the user can target
const METRICS = [
  { value: 'swr', label: 'SWR', defaultTarget: 1.0, defaultWeight: 10 },
  { value: 'gain', label: 'Peak Gain (dBi)', defaultTarget: 8.0, defaultWeight: 5 },
  { value: 'front_to_back', label: 'F/B Ratio (dB)', defaultTarget: 20.0, defaultWeight: 3 },
  { value: 'impedance_r', label: 'Resistance (Ω)', defaultTarget: 50.0, defaultWeight: 2 },
  { value: 'impedance_x', label: 'Reactance (Ω)', defaultTarget: 0.0, defaultWeight: 2 },
  { value: 'efficiency', label: 'Efficiency', defaultTarget: 1.0, defaultWeight: 1 },
];

const FIELDS = ['x1', 'y1', 'z1', 'x2', 'y2', 'z2', 'radius'] as const;

type FieldName = typeof FIELDS[number];

/** Get the current value of a wire field */
function getWireFieldValue(wire: { x1: number; y1: number; z1: number; x2: number; y2: number; z2: number; radius: number }, field: string): number {
  return (wire as Record<string, number>)[field] ?? 0;
}

const OptimizerViewer: React.FC = () => {
  const wires = useAntennaStore((s) => s.wires);
  const source = useAntennaStore((s) => s.source);
  const loads = useAntennaStore((s) => s.loads);
  const transmissionLines = useAntennaStore((s) => s.transmissionLines);
  const ground = useAntennaStore((s) => s.ground);
  const frequency = useAntennaStore((s) => s.frequency);
  const referenceImpedance = useAntennaStore((s) => s.referenceImpedance);
  const weather = useAntennaStore((s) => s.weather);
  const updateWire = useAntennaStore((s) => s.updateWire);

  // Variables (persisted in store)
  const variables = useAntennaStore((s) => s.optimVariables);
  const setVariables = useAntennaStore((s) => s.setOptimVariables);
  // Goals (persisted in store)
  const goals = useAntennaStore((s) => s.optimGoals);
  const setGoals = useAntennaStore((s) => s.setOptimGoals);
  // Band settings
  const [useBand, setUseBand] = useState(false);
  const [bandStart, setBandStart] = useState(frequency.freqStart || frequency.frequencyMhz - 0.5);
  const [bandEnd, setBandEnd] = useState(frequency.freqEnd || frequency.frequencyMhz + 0.5);
  const [bandSteps, setBandSteps] = useState(5);
  // PSO settings
  const [particles, setParticles] = useState(20);
  const [iterations, setIterations] = useState(40);
  // State (result persisted in store)
  const result = useAntennaStore((s) => s.optimResult);
  const setOptimResult = useAntennaStore((s) => s.setOptimResult);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // ─── Variable management ─────────────────
  const addVariable = useCallback(() => {
    if (wires.length === 0) return;
    const w = wires[0];
    const current = getWireFieldValue(w, 'z2');
    setVariables([
      ...variables,
      {
        name: `var_${variables.length + 1}`,
        wire_index: 0,
        field: 'z2',
        min: current * 0.75,
        max: current * 1.25,
      },
    ]);
  }, [wires, variables, setVariables]);

  const removeVariable = useCallback((idx: number) => {
    setVariables(variables.filter((_, i) => i !== idx));
  }, [variables, setVariables]);

  const updateVariable = useCallback((idx: number, patch: Partial<OptimVariable>) => {
    setVariables(
      variables.map((v, i) => {
        if (i !== idx) return v;
        const updated = { ...v, ...patch };
        // Auto-update bounds when wire/field changes
        if (patch.wire_index !== undefined || patch.field !== undefined) {
          const wireIdx = patch.wire_index ?? v.wire_index;
          const field = patch.field ?? v.field;
          if (wireIdx >= 0 && wireIdx < wires.length) {
            const val = getWireFieldValue(wires[wireIdx], field);
            if (val !== 0) {
              updated.min = val * 0.75;
              updated.max = val * 1.25;
            } else {
              updated.min = -1;
              updated.max = 1;
            }
          }
        }
        return updated;
      }),
    );
  }, [wires, variables, setVariables]);

  // ─── Goal management ─────────────────
  const addGoal = useCallback(() => {
    const unused = METRICS.find((m) => !goals.some((g) => g.metric === m.value));
    if (unused) {
      setGoals([
        ...goals,
        { metric: unused.value, target: unused.defaultTarget, weight: unused.defaultWeight },
      ]);
    }
  }, [goals, setGoals]);

  const removeGoal = useCallback((idx: number) => {
    setGoals(goals.filter((_, i) => i !== idx));
  }, [goals, setGoals]);

  const updateGoal = useCallback((idx: number, patch: Partial<OptimGoal>) => {
    setGoals(
      goals.map((g, i) => (i === idx ? { ...g, ...patch } : g)),
    );
  }, [goals, setGoals]);

  // ─── Auto-generate Yagi variables ─────────
  const autoYagi = useCallback(() => {
    if (wires.length < 3) return;
    const names = ['reflector', 'driven', 'director'];
    const newVars: OptimVariable[] = [];
    for (let i = 0; i < 3 && i < wires.length; i++) {
      const w = wires[i];
      // Half-length (Y2)
      const halfLen = Math.abs(w.y2);
      if (halfLen > 0) {
        newVars.push({
          name: `${names[i]}_half_length`,
          wire_index: i,
          field: 'y2',
          min: +(halfLen * 0.75).toFixed(4),
          max: +(halfLen * 1.25).toFixed(4),
        });
      }
      // Spacing (X) - skip driven element
      if (i !== 1 && w.x1 !== 0) {
        newVars.push({
          name: `${names[i]}_spacing`,
          wire_index: i,
          field: 'x1',
          min: +(w.x1 * (w.x1 > 0 ? 0.5 : 1.5)).toFixed(4),
          max: +(w.x1 * (w.x1 > 0 ? 1.5 : 0.5)).toFixed(4),
        });
      }
    }
    setVariables(newVars);
  }, [wires, setVariables]);

  // ─── Run optimizer ─────────────────
  const handleRun = useCallback(async () => {
    if (variables.length === 0) {
      setError('Add at least one variable to optimise');
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const res = await runOptimizer(
        wires, source, loads, transmissionLines,
        ground, frequency, referenceImpedance,
        variables, goals, weather,
        {
          freqStartMhz: useBand ? bandStart : undefined,
          freqEndMhz: useBand ? bandEnd : undefined,
          freqSteps: useBand ? bandSteps : undefined,
          particles,
          iterations,
        },
      );
      setOptimResult(res);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [wires, source, loads, transmissionLines, ground, frequency, referenceImpedance,
      variables, goals, weather, useBand, bandStart, bandEnd, bandSteps, particles, iterations, setOptimResult]);

  // ─── Apply optimised geometry ─────────────────
  const handleApply = useCallback(() => {
    if (!result) return;
    // Apply best_params back to wires
    for (const v of variables) {
      const val = result.best_params[v.name];
      if (val !== undefined && v.wire_index >= 0 && v.wire_index < wires.length) {
        const wire = wires[v.wire_index];
        const patch: Record<string, number> = {};
        patch[v.field] = val;
        // For symmetric elements (y2 = -y1), also update the mirror
        if (v.field === 'y2') {
          patch.y1 = -Math.abs(val);
          patch.y2 = Math.abs(val);
        }
        updateWire(wire.id, patch);
      }
    }
  }, [result, variables, wires, updateWire]);

  return (
    <div style={{ padding: 12, overflowY: 'auto', maxHeight: '100%' }}>
      {/* Variables section */}
      <div style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
          <h4 style={{ margin: 0, fontSize: 13, color: '#ddd' }}>Variables</h4>
          <button onClick={addVariable} style={btnSmall}>+ Add</button>
          {wires.length >= 3 && (
            <button onClick={autoYagi} style={btnSmall} title="Auto-detect Yagi element lengths and spacings">
              Auto (Yagi)
            </button>
          )}
        </div>
        {variables.map((v, i) => (
          <div key={i} style={{ display: 'flex', gap: 4, alignItems: 'center', marginBottom: 4, flexWrap: 'wrap' }}>
            <input value={v.name} onChange={(e) => updateVariable(i, { name: e.target.value })}
                   style={{ ...inputStyle, width: 120 }} placeholder="Name" />
            <select value={v.wire_index} onChange={(e) => updateVariable(i, { wire_index: +e.target.value })}
                    style={{ ...inputStyle, width: 70 }}>
              {wires.map((_, wi) => (
                <option key={wi} value={wi}>Wire {wi + 1}</option>
              ))}
            </select>
            <select value={v.field} onChange={(e) => updateVariable(i, { field: e.target.value })}
                    style={{ ...inputStyle, width: 65 }}>
              {FIELDS.map((f) => <option key={f} value={f}>{f}</option>)}
            </select>
            <label style={labelSmall}>Min:</label>
            <input type="number" step="any" value={v.min}
                   onChange={(e) => updateVariable(i, { min: +e.target.value })}
                   style={{ ...inputStyle, width: 80 }} />
            <label style={labelSmall}>Max:</label>
            <input type="number" step="any" value={v.max}
                   onChange={(e) => updateVariable(i, { max: +e.target.value })}
                   style={{ ...inputStyle, width: 80 }} />
            <button onClick={() => removeVariable(i)} style={btnDel}>×</button>
          </div>
        ))}
        {variables.length === 0 && (
          <div style={{ fontSize: 11, color: '#888' }}>No variables defined. Add variables or use "Auto (Yagi)" for a 3-element beam.</div>
        )}
      </div>

      {/* Goals section */}
      <div style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
          <h4 style={{ margin: 0, fontSize: 13, color: '#ddd' }}>Objectives</h4>
          <button onClick={addGoal} style={btnSmall}>+ Add</button>
        </div>
        {goals.map((g, i) => (
          <div key={i} style={{ display: 'flex', gap: 4, alignItems: 'center', marginBottom: 4 }}>
            <select value={g.metric} onChange={(e) => {
              const m = METRICS.find((x) => x.value === e.target.value);
              updateGoal(i, { metric: e.target.value, target: m?.defaultTarget ?? 0, weight: m?.defaultWeight ?? 1 });
            }} style={{ ...inputStyle, width: 140 }}>
              {METRICS.map((m) => <option key={m.value} value={m.value}>{m.label}</option>)}
            </select>
            <label style={labelSmall}>Target:</label>
            <input type="number" step="any" value={g.target}
                   onChange={(e) => updateGoal(i, { target: +e.target.value })}
                   style={{ ...inputStyle, width: 70 }} />
            <label style={labelSmall}>Weight:</label>
            <input type="number" step="any" value={g.weight}
                   onChange={(e) => updateGoal(i, { weight: +e.target.value })}
                   style={{ ...inputStyle, width: 55 }} />
            <button onClick={() => removeGoal(i)} style={btnDel}>×</button>
          </div>
        ))}
      </div>

      {/* Band evaluation */}
      <div style={{ marginBottom: 16 }}>
        <label style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 12, color: '#bbb' }}>
          <input type="checkbox" checked={useBand} onChange={(e) => setUseBand(e.target.checked)} />
          Optimise across frequency band (worst-case)
        </label>
        {useBand && (
          <div style={{ display: 'flex', gap: 6, marginTop: 4 }}>
            <label style={labelSmall}>Start MHz:</label>
            <input type="number" step="any" value={bandStart}
                   onChange={(e) => setBandStart(+e.target.value)}
                   style={{ ...inputStyle, width: 80 }} />
            <label style={labelSmall}>End MHz:</label>
            <input type="number" step="any" value={bandEnd}
                   onChange={(e) => setBandEnd(+e.target.value)}
                   style={{ ...inputStyle, width: 80 }} />
            <label style={labelSmall}>Steps:</label>
            <input type="number" value={bandSteps} min={2} max={20}
                   onChange={(e) => setBandSteps(+e.target.value)}
                   style={{ ...inputStyle, width: 50 }} />
          </div>
        )}
      </div>

      {/* PSO settings */}
      <div style={{ display: 'flex', gap: 12, alignItems: 'center', marginBottom: 16, flexWrap: 'wrap' }}>
        <label style={labelSmall}>Particles:</label>
        <input type="number" value={particles} min={5} max={50}
               onChange={(e) => setParticles(+e.target.value)}
               style={{ ...inputStyle, width: 50 }} />
        <label style={labelSmall}>Iterations:</label>
        <input type="number" value={iterations} min={5} max={100}
               onChange={(e) => setIterations(+e.target.value)}
               style={{ ...inputStyle, width: 50 }} />
        <button onClick={handleRun} disabled={loading}
                style={{ padding: '4px 16px', fontWeight: 600 }}>
          {loading ? 'Optimising…' : 'Run Optimizer'}
        </button>
      </div>

      {error && <div style={{ color: '#d44', marginBottom: 8 }}>{error}</div>}

      {/* Results */}
      {result && (
        <div>
          <h4 style={{ margin: '0 0 8px', fontSize: 13, color: '#ddd' }}>Results</h4>

          {/* Best metrics */}
          <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', marginBottom: 12 }}>
            {result.best_metrics && Object.entries(result.best_metrics).map(([k, v]) => (
              <div key={k} style={{ fontSize: 12 }}>
                <span style={{ color: '#888' }}>{k}: </span>
                <span style={{ color: '#eee', fontWeight: 500 }}>{typeof v === 'number' ? v.toFixed(3) : v}</span>
              </div>
            ))}
          </div>

          {/* Best parameters */}
          <div style={{ marginBottom: 12 }}>
            <div style={{ fontSize: 12, color: '#999', marginBottom: 4 }}>Best parameters:</div>
            <table style={{ borderCollapse: 'collapse', fontSize: 12 }}>
              <tbody>
                {Object.entries(result.best_params).map(([k, v]) => (
                  <tr key={k} style={{ borderBottom: '1px solid #333' }}>
                    <td style={{ padding: '2px 10px 2px 0', color: '#aaa' }}>{k}</td>
                    <td style={{ padding: '2px 0', color: '#eee' }}>{v.toFixed(6)} m</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Convergence chart */}
          {result.convergence.length > 0 && (
            <div style={{ marginBottom: 12 }}>
              <div style={{ fontSize: 12, color: '#999', marginBottom: 4 }}>
                Convergence (best cost: {result.best_cost.toFixed(4)})
              </div>
              <ConvergenceChart data={result.convergence} />
            </div>
          )}

          {/* Apply button */}
          <button onClick={handleApply} style={{ padding: '6px 20px', fontWeight: 600 }}>
            Apply Optimised Geometry
          </button>
        </div>
      )}

      {!result && !loading && (
        <div style={{ color: '#999', marginTop: 8, fontSize: 12 }}>
          Configure variables and objectives, then click "Run Optimizer" to start
          a Particle Swarm Optimization (PSO) that tunes the antenna geometry.
        </div>
      )}
    </div>
  );
};

/** Simple SVG convergence chart */
const ConvergenceChart: React.FC<{ data: number[] }> = ({ data }) => {
  const W = 460;
  const H = 120;
  const PAD = 30;
  const n = data.length;
  if (n < 2) return null;

  const maxVal = Math.max(...data);
  const minVal = Math.min(...data);
  const range = maxVal - minVal || 1;

  const points = data.map((v, i) => {
    const x = PAD + (i / (n - 1)) * (W - 2 * PAD);
    const y = H - PAD - ((v - minVal) / range) * (H - 2 * PAD);
    return `${x},${y}`;
  }).join(' ');

  return (
    <svg width={W} height={H} style={{ display: 'block' }}>
      {/* axes */}
      <line x1={PAD} y1={H - PAD} x2={W - PAD} y2={H - PAD} stroke="#555" strokeWidth={1} />
      <line x1={PAD} y1={PAD} x2={PAD} y2={H - PAD} stroke="#555" strokeWidth={1} />
      {/* labels */}
      <text x={W / 2} y={H - 4} textAnchor="middle" fontSize={10} fill="#888">Generation</text>
      <text x={4} y={H / 2} textAnchor="middle" fontSize={10} fill="#888"
            transform={`rotate(-90, 8, ${H / 2})`}>Cost</text>
      {/* y-axis ticks */}
      <text x={PAD - 3} y={PAD + 4} textAnchor="end" fontSize={9} fill="#888">
        {maxVal.toFixed(1)}
      </text>
      <text x={PAD - 3} y={H - PAD + 4} textAnchor="end" fontSize={9} fill="#888">
        {minVal.toFixed(1)}
      </text>
      {/* curve */}
      <polyline points={points} fill="none" stroke="#4fc3f7" strokeWidth={1.5} />
    </svg>
  );
};

// ─── Style constants ─────────────────

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

const btnSmall: React.CSSProperties = {
  fontSize: 11,
  padding: '2px 8px',
  cursor: 'pointer',
};

const btnDel: React.CSSProperties = {
  fontSize: 13,
  padding: '0 5px',
  cursor: 'pointer',
  color: '#d44',
  background: 'transparent',
  border: 'none',
};

export default OptimizerViewer;
