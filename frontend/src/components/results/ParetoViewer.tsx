/**
 * Pareto multi-objective optimizer viewer (NSGA-II).
 *
 * Reuses the same variable/goal configuration as the single-objective
 * OptimizerViewer, but objectives have direction (minimize/maximize)
 * instead of target+weight.  Results are shown as:
 *
 *  - 2D scatter plot of the Pareto front (user picks X/Y axes)
 *  - Table of all Pareto-front solutions with metrics
 *  - Click a point to select it and apply its geometry
 */
import React, { useState, useCallback } from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import { runParetoOptimizer } from '@/api/client';
import type { OptimVariable, ParetoObjective, ParetoSolution } from '@/types';

const METRICS = [
  { value: 'swr', label: 'SWR', defaultDir: 'minimize' as const },
  { value: 'gain', label: 'Peak Gain (dBi)', defaultDir: 'maximize' as const },
  { value: 'front_to_back', label: 'F/B Ratio (dB)', defaultDir: 'maximize' as const },
  { value: 'impedance_r', label: 'Resistance (Ω)', defaultDir: 'minimize' as const },
  { value: 'impedance_x', label: 'Reactance (Ω)', defaultDir: 'minimize' as const },
  { value: 'efficiency', label: 'Efficiency', defaultDir: 'maximize' as const },
  { value: 'beamwidth_az', label: 'Beamwidth Az (°)', defaultDir: 'minimize' as const },
  { value: 'beamwidth_el', label: 'Beamwidth El (°)', defaultDir: 'minimize' as const },
];

const FIELDS = ['x1', 'y1', 'z1', 'x2', 'y2', 'z2', 'radius'] as const;

function getWireFieldValue(wire: Record<string, number>, field: string): number {
  return wire[field] ?? 0;
}

/** 2D scatter plot of the Pareto front */
const ParetoScatter: React.FC<{
  front: ParetoSolution[];
  allFronts: ParetoSolution[];
  xMetric: string;
  yMetric: string;
  selectedIdx: number;
  onSelect: (idx: number) => void;
}> = ({ front, allFronts, xMetric, yMetric, selectedIdx, onSelect }) => {
  const W = 480;
  const H = 340;
  const PAD = 50;

  // Compute extent from all points for stable axes
  const allPts = allFronts.map((s) => ({
    x: s.metrics[xMetric] ?? 0,
    y: s.metrics[yMetric] ?? 0,
    rank: s.rank,
  }));
  const frontPts = front.map((s) => ({
    x: s.metrics[xMetric] ?? 0,
    y: s.metrics[yMetric] ?? 0,
  }));

  if (frontPts.length === 0) return null;

  const xMin = Math.min(...allPts.map((p) => p.x));
  const xMax = Math.max(...allPts.map((p) => p.x));
  const yMin = Math.min(...allPts.map((p) => p.y));
  const yMax = Math.max(...allPts.map((p) => p.y));
  const xSpan = xMax - xMin || 1;
  const ySpan = yMax - yMin || 1;

  const scaleX = (v: number) => PAD + ((v - xMin) / xSpan) * (W - 2 * PAD);
  const scaleY = (v: number) => H - PAD - ((v - yMin) / ySpan) * (H - 2 * PAD);

  // Pareto front line (sorted by X)
  const sortedFront = [...frontPts].sort((a, b) => a.x - b.x);
  const linePath = sortedFront
    .map((p, i) => `${i === 0 ? 'M' : 'L'}${scaleX(p.x).toFixed(1)},${scaleY(p.y).toFixed(1)}`)
    .join(' ');

  const xLabel = METRICS.find((m) => m.value === xMetric)?.label ?? xMetric;
  const yLabel = METRICS.find((m) => m.value === yMetric)?.label ?? yMetric;

  return (
    <svg width={W} height={H} style={{ display: 'block', userSelect: 'none' }}>
      {/* Axes */}
      <line x1={PAD} y1={H - PAD} x2={W - PAD} y2={H - PAD} stroke="#555" strokeWidth={1} />
      <line x1={PAD} y1={PAD} x2={PAD} y2={H - PAD} stroke="#555" strokeWidth={1} />

      {/* Axis labels */}
      <text x={W / 2} y={H - 6} textAnchor="middle" fontSize={11} fill="#aaa">{xLabel}</text>
      <text x={10} y={H / 2} textAnchor="middle" fontSize={11} fill="#aaa"
            transform={`rotate(-90, 10, ${H / 2})`}>{yLabel}</text>

      {/* Axis ticks */}
      <text x={PAD} y={H - PAD + 14} textAnchor="middle" fontSize={9} fill="#888">{xMin.toFixed(2)}</text>
      <text x={W - PAD} y={H - PAD + 14} textAnchor="middle" fontSize={9} fill="#888">{xMax.toFixed(2)}</text>
      <text x={PAD - 4} y={H - PAD + 4} textAnchor="end" fontSize={9} fill="#888">{yMin.toFixed(2)}</text>
      <text x={PAD - 4} y={PAD + 4} textAnchor="end" fontSize={9} fill="#888">{yMax.toFixed(2)}</text>

      {/* Dominated points (faded) */}
      {allPts.map((p, i) =>
        p.rank > 0 ? (
          <circle key={`d${i}`} cx={scaleX(p.x)} cy={scaleY(p.y)} r={3}
                  fill="#666" opacity={0.3} />
        ) : null
      )}

      {/* Pareto front line */}
      {sortedFront.length > 1 && (
        <path d={linePath} fill="none" stroke="#4fc3f7" strokeWidth={1.5} opacity={0.5} />
      )}

      {/* Pareto front points */}
      {front.map((s, i) => {
        const px = scaleX(s.metrics[xMetric] ?? 0);
        const py = scaleY(s.metrics[yMetric] ?? 0);
        const isSel = i === selectedIdx;
        return (
          <circle key={`f${i}`} cx={px} cy={py} r={isSel ? 7 : 5}
                  fill={isSel ? '#ff9800' : '#4fc3f7'}
                  stroke={isSel ? '#fff' : 'none'}
                  strokeWidth={isSel ? 2 : 0}
                  style={{ cursor: 'pointer' }}
                  onClick={() => onSelect(i)} />
        );
      })}
    </svg>
  );
};

const ParetoViewer: React.FC = () => {
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
  const variables = useAntennaStore((s) => s.paretoVariables);
  const setVariables = useAntennaStore((s) => s.setParetoVariables);

  // Objectives (persisted in store, at least 2)
  const objectives = useAntennaStore((s) => s.paretoObjectives);
  const setObjectives = useAntennaStore((s) => s.setParetoObjectives);

  // Band settings
  const [useBand, setUseBand] = useState(false);
  const [bandStart, setBandStart] = useState(frequency.freqStart || frequency.frequencyMhz - 0.5);
  const [bandEnd, setBandEnd] = useState(frequency.freqEnd || frequency.frequencyMhz + 0.5);
  const [bandSteps, setBandSteps] = useState(5);

  // NSGA-II settings
  const [popSize, setPopSize] = useState(40);
  const [generations, setGenerations] = useState(30);

  // Scatter axis selection
  const [xAxis, setXAxis] = useState('swr');
  const [yAxis, setYAxis] = useState('gain');

  // State (result persisted in store)
  const result = useAntennaStore((s) => s.paretoResult);
  const setParetoResult = useAntennaStore((s) => s.setParetoResult);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedIdx, setSelectedIdx] = useState(0);

  // ─── Variable management ─────────────────
  const addVariable = useCallback(() => {
    if (wires.length === 0) return;
    const w = wires[0];
    const current = getWireFieldValue(w as unknown as Record<string, number>, 'z2');
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
        if (patch.wire_index !== undefined || patch.field !== undefined) {
          const wireIdx = patch.wire_index ?? v.wire_index;
          const field = patch.field ?? v.field;
          if (wireIdx >= 0 && wireIdx < wires.length) {
            const val = getWireFieldValue(wires[wireIdx] as unknown as Record<string, number>, field);
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

  // ─── Auto-generate Yagi variables ─────────
  const autoYagi = useCallback(() => {
    if (wires.length < 3) return;
    const names = ['reflector', 'driven', 'director'];
    const newVars: OptimVariable[] = [];
    for (let i = 0; i < 3 && i < wires.length; i++) {
      const w = wires[i];
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

  // ─── Objective management ─────────────────
  const addObjective = useCallback(() => {
    const unused = METRICS.find((m) => !objectives.some((o) => o.metric === m.value));
    if (unused) {
      setObjectives([...objectives, { metric: unused.value, direction: unused.defaultDir }]);
    }
  }, [objectives, setObjectives]);

  const removeObjective = useCallback((idx: number) => {
    if (objectives.length <= 2) return; // must have at least 2
    setObjectives(objectives.filter((_, i) => i !== idx));
  }, [objectives, setObjectives]);

  const updateObjective = useCallback((idx: number, patch: Partial<ParetoObjective>) => {
    setObjectives(
      objectives.map((o, i) => (i === idx ? { ...o, ...patch } : o)),
    );
  }, [objectives, setObjectives]);

  // ─── Run optimizer ─────────────────
  const handleRun = useCallback(async () => {
    if (variables.length === 0) {
      setError('Add at least one variable to optimise');
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const res = await runParetoOptimizer(
        wires, source, loads, transmissionLines,
        ground, frequency, referenceImpedance,
        variables, objectives, weather,
        {
          freqStartMhz: useBand ? bandStart : undefined,
          freqEndMhz: useBand ? bandEnd : undefined,
          freqSteps: useBand ? bandSteps : undefined,
          popSize,
          generations,
        },
      );
      setParetoResult(res);
      setSelectedIdx(0);
      // Auto-set scatter axes to first two objectives
      if (res.objectives.length >= 2) {
        setXAxis(res.objectives[0]);
        setYAxis(res.objectives[1]);
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [wires, source, loads, transmissionLines, ground, frequency, referenceImpedance,
      variables, objectives, weather, useBand, bandStart, bandEnd, bandSteps, popSize, generations, setParetoResult]);

  // ─── Apply selected solution ─────────────────
  const selectedSolution = result && result.front.length > selectedIdx ? result.front[selectedIdx] : null;

  const handleApply = useCallback(() => {
    if (!selectedSolution) return;
    for (const v of variables) {
      const val = selectedSolution.params[v.name];
      if (val !== undefined && v.wire_index >= 0 && v.wire_index < wires.length) {
        const wire = wires[v.wire_index];
        const patch: Record<string, number> = {};
        patch[v.field] = val;
        if (v.field === 'y2') {
          patch.y1 = -Math.abs(val);
          patch.y2 = Math.abs(val);
        }
        updateWire(wire.id, patch);
      }
    }
  }, [selectedSolution, variables, wires, updateWire]);

  // Metric labels lookup
  const metricLabel = useCallback((m: string) =>
    METRICS.find((x) => x.value === m)?.label ?? m, []);

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
            <button onClick={() => removeVariable(i)} style={btnDel}>&times;</button>
          </div>
        ))}
        {variables.length === 0 && (
          <div style={{ fontSize: 11, color: '#888' }}>No variables defined. Add variables or use "Auto (Yagi)".</div>
        )}
      </div>

      {/* Objectives section */}
      <div style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
          <h4 style={{ margin: 0, fontSize: 13, color: '#ddd' }}>Objectives (min 2)</h4>
          <button onClick={addObjective} style={btnSmall}>+ Add</button>
        </div>
        {objectives.map((o, i) => (
          <div key={i} style={{ display: 'flex', gap: 4, alignItems: 'center', marginBottom: 4 }}>
            <select value={o.metric} onChange={(e) => {
              const m = METRICS.find((x) => x.value === e.target.value);
              updateObjective(i, { metric: e.target.value, direction: m?.defaultDir ?? 'minimize' });
            }} style={{ ...inputStyle, width: 140 }}>
              {METRICS.map((m) => <option key={m.value} value={m.value}>{m.label}</option>)}
            </select>
            <select value={o.direction} onChange={(e) => updateObjective(i, { direction: e.target.value as 'minimize' | 'maximize' })}
                    style={{ ...inputStyle, width: 90 }}>
              <option value="minimize">Minimize</option>
              <option value="maximize">Maximize</option>
            </select>
            {objectives.length > 2 && (
              <button onClick={() => removeObjective(i)} style={btnDel}>&times;</button>
            )}
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

      {/* NSGA-II settings */}
      <div style={{ display: 'flex', gap: 12, alignItems: 'center', marginBottom: 16, flexWrap: 'wrap' }}>
        <label style={labelSmall}>Population:</label>
        <input type="number" value={popSize} min={10} max={80}
               onChange={(e) => setPopSize(+e.target.value)}
               style={{ ...inputStyle, width: 50 }} />
        <label style={labelSmall}>Generations:</label>
        <input type="number" value={generations} min={5} max={60}
               onChange={(e) => setGenerations(+e.target.value)}
               style={{ ...inputStyle, width: 50 }} />
        <button onClick={handleRun} disabled={loading}
                style={{ padding: '4px 16px', fontWeight: 600 }}>
          {loading ? 'Optimising...' : 'Run Pareto'}
        </button>
      </div>

      {error && <div style={{ color: '#d44', marginBottom: 8 }}>{error}</div>}

      {/* Results */}
      {result && result.front.length > 0 && (
        <div>
          <h4 style={{ margin: '0 0 8px', fontSize: 13, color: '#ddd' }}>
            Pareto Front ({result.front.length} solutions, {result.generations} generations)
          </h4>

          {/* Axis selector for scatter plot */}
          <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 8 }}>
            <label style={labelSmall}>X axis:</label>
            <select value={xAxis} onChange={(e) => setXAxis(e.target.value)} style={{ ...inputStyle, width: 130 }}>
              {METRICS.map((m) => <option key={m.value} value={m.value}>{m.label}</option>)}
            </select>
            <label style={labelSmall}>Y axis:</label>
            <select value={yAxis} onChange={(e) => setYAxis(e.target.value)} style={{ ...inputStyle, width: 130 }}>
              {METRICS.map((m) => <option key={m.value} value={m.value}>{m.label}</option>)}
            </select>
          </div>

          {/* Scatter plot */}
          <ParetoScatter
            front={result.front}
            allFronts={result.all_fronts}
            xMetric={xAxis}
            yMetric={yAxis}
            selectedIdx={selectedIdx}
            onSelect={setSelectedIdx}
          />

          {/* Solution table */}
          <div style={{ overflowX: 'auto', marginTop: 12, marginBottom: 12 }}>
            <table style={{ borderCollapse: 'collapse', fontSize: 11, minWidth: 400 }}>
              <thead>
                <tr style={{ borderBottom: '1px solid #555' }}>
                  <th style={thStyle}>#</th>
                  {result.objectives.map((o) => (
                    <th key={o} style={thStyle}>{metricLabel(o)}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {result.front.map((sol, i) => (
                  <tr key={i}
                      onClick={() => setSelectedIdx(i)}
                      style={{
                        cursor: 'pointer',
                        background: i === selectedIdx ? 'rgba(255,152,0,0.15)' : 'transparent',
                        borderBottom: '1px solid #333',
                      }}>
                    <td style={tdStyle}>{i + 1}</td>
                    {result.objectives.map((o) => (
                      <td key={o} style={tdStyle}>
                        {(sol.metrics[o] ?? 0).toFixed(3)}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Selected solution detail */}
          {selectedSolution && (
            <div style={{ marginBottom: 12 }}>
              <div style={{ fontSize: 12, color: '#999', marginBottom: 4 }}>
                Selected solution #{selectedIdx + 1} parameters:
              </div>
              <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', marginBottom: 8 }}>
                {Object.entries(selectedSolution.params).map(([k, v]) => (
                  <div key={k} style={{ fontSize: 11 }}>
                    <span style={{ color: '#888' }}>{k}: </span>
                    <span style={{ color: '#eee' }}>{v.toFixed(5)} m</span>
                  </div>
                ))}
              </div>
              <button onClick={handleApply} style={{ padding: '6px 20px', fontWeight: 600 }}>
                Apply Selected Design
              </button>
            </div>
          )}
        </div>
      )}

      {result && result.front.length === 0 && (
        <div style={{ color: '#d44' }}>No valid solutions found. Try relaxing variable bounds.</div>
      )}

      {!result && !loading && (
        <div style={{ color: '#999', marginTop: 8, fontSize: 12 }}>
          Configure variables and at least two objectives, then click "Run Pareto" to
          perform NSGA-II multi-objective optimization. The result is a Pareto front
          of non-dominated trade-off designs — pick the one that best balances your
          competing goals.
        </div>
      )}
    </div>
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

const thStyle: React.CSSProperties = {
  textAlign: 'left',
  padding: '4px 8px',
  fontWeight: 600,
  color: '#bbb',
};

const tdStyle: React.CSSProperties = {
  padding: '4px 8px',
  color: '#ddd',
};

export default ParetoViewer;
