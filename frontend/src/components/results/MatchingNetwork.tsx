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
 * Impedance matching network panel — backed by /api/match.
 *
 * The component takes the most recent simulation's feed-point impedance
 * and the user's reference Z0, posts them to the backend matching
 * designer, and renders the resulting candidate networks (L, π, T, γ,
 * β).  Each candidate shows its component list plus any rejection
 * notes.  A small Q-factor input controls the π / T narrow-band
 * designs.
 *
 * The "Design at" frequency input lets the user target a frequency
 * different from the simulation frequency.  When the target differs,
 * a background simulation is run at that frequency to obtain Z_ant,
 * then the matching network is designed for that impedance.
 */
import React, { useEffect, useState } from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import {
  designMatch, buildSimulateRequest, simulate,
  type MatchResult, type MatchSolution, type MatchComponent,
} from '@/api/client';
import MatchSchematic from './MatchSchematic';
import { formatImpedance } from '@/utils/conversions';

const TOPOLOGY_LABELS: Record<string, string> = {
  L: 'L-network (2 elements)',
  pi: 'π-network (3 elements)',
  T: 'T-network (3 elements)',
  gamma: 'γ-match (Yagi)',
  beta: 'β-match (hairpin)',
  toroid: 'Toroidal transformer (broadband)',
};

const formatCapacitance = (farads: number): string => {
  if (farads <= 0) return '—';
  if (farads < 1e-9) return `${(farads * 1e12).toFixed(2)} pF`;
  if (farads < 1e-6) return `${(farads * 1e9).toFixed(2)} nF`;
  return `${(farads * 1e6).toFixed(3)} µF`;
};

const formatInductance = (henries: number): string => {
  if (henries <= 0) return '—';
  if (henries < 1e-6) return `${(henries * 1e9).toFixed(2)} nH`;
  if (henries < 1e-3) return `${(henries * 1e6).toFixed(2)} µH`;
  return `${(henries * 1e3).toFixed(3)} mH`;
};

const formatComponentValue = (c: MatchComponent): string => {
  if (c.kind === 'L') return formatInductance(c.value);
  if (c.kind === 'C') return formatCapacitance(c.value);
  if (c.kind === 'R') return `${c.value.toFixed(2)} Ω`;
  return c.value.toString();
};

/** Compute SWR from complex impedance and reference Z0. */
function computeSWR(r: number, x: number, z0: number): number {
  const denom = Math.sqrt((r + z0) ** 2 + x ** 2);
  if (denom < 1e-30) return 999;
  const gamma = Math.sqrt((r - z0) ** 2 + x ** 2) / denom;
  if (gamma >= 1) return 999;
  return (1 + gamma) / (1 - gamma);
}

const ComponentRow: React.FC<{ c: MatchComponent }> = ({ c }) => (
  <tr>
    <td><strong>{c.label}</strong></td>
    <td>{c.kind}</td>
    <td>{c.position}</td>
    <td>{formatComponentValue(c)}</td>
    <td className="muted">
      {c.reactance >= 0 ? '+' : '−'}j{Math.abs(c.reactance).toFixed(2)} Ω
    </td>
  </tr>
);

const SolutionCard: React.FC<{ s: MatchSolution }> = ({ s }) => {
  const label = TOPOLOGY_LABELS[s.topology] ?? s.topology;
  const skipped = !s.components || s.components.length === 0;
  return (
    <div className={`match-solution ${skipped ? 'match-solution-skipped' : ''}`}>
      <h4>{label}</h4>
      {skipped ? (
        <p className="muted small">{s.notes || 'Not applicable for this load.'}</p>
      ) : (
        <div className="match-solution-body">
          <div className="match-solution-schematic">
            <MatchSchematic components={s.components} />
          </div>
          <div className="match-solution-data">
            <table className="match-table">
              <thead>
                <tr>
                  <th>Element</th><th>Kind</th><th>Position</th><th>Value</th><th>Reactance</th>
                </tr>
              </thead>
              <tbody>
                {s.components.map((c, i) => <ComponentRow key={i} c={c} />)}
              </tbody>
            </table>
            {s.cores && s.cores.length > 0 && (
              <table className="match-table match-cores-table">
                <thead>
                  <tr>
                    <th>Core</th><th>Material</th><th>Range</th><th>AL (nH/T²)</th>
                    <th>N pri</th><th>N sec</th><th>L pri (µH)</th>
                  </tr>
                </thead>
                <tbody>
                  {s.cores.map((core, i) => (
                    <tr key={i}>
                      <td><strong>{core.name}</strong></td>
                      <td>{core.material}</td>
                      <td className="muted">{core.freq_range}</td>
                      <td>{core.al_nh_per_t2}</td>
                      <td>{core.primary_turns}</td>
                      <td>{core.secondary_turns}</td>
                      <td>{core.primary_inductance_uh.toFixed(1)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
            {s.notes && <p className="muted small match-notes">{s.notes}</p>}
          </div>
        </div>
      )}
    </div>
  );
};

const MatchingNetwork: React.FC = () => {
  const {
    simulationResult, referenceImpedance, frequency,
    wires, source, loads, transmissionLines, ground, weather,
  } = useAntennaStore();

  const [qFactor, setQFactor] = useState(10);
  const [targetFreqMHz, setTargetFreqMHz] = useState(() => frequency.frequencyMhz);
  const [targetImpedance, setTargetImpedance] = useState<{ r: number; x: number } | null>(null);
  const [result, setResult] = useState<MatchResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const z0 = referenceImpedance;
  const simFreqMHz = frequency.frequencyMhz;

  useEffect(() => {
    if (!simulationResult || targetFreqMHz <= 0) {
      setResult(null);
      setTargetImpedance(null);
      return;
    }

    let cancelled = false;
    setLoading(true);
    setError(null);

    const runMatch = (r: number, x: number) =>
      designMatch({ loadR: r, loadX: x, sourceZ0: z0, freqMHz: targetFreqMHz, qFactor })
        .then((res) => {
          if (!cancelled) {
            setTargetImpedance({ r, x });
            setResult(res);
            setLoading(false);
          }
        })
        .catch((err: unknown) => {
          if (!cancelled) {
            setError(err instanceof Error ? err.message : String(err));
            setResult(null);
            setLoading(false);
          }
        });

    if (Math.abs(targetFreqMHz - simFreqMHz) < 1e-6) {
      // Target matches simulation frequency — use cached impedance directly.
      const { r, x } = simulationResult.impedance;
      runMatch(r, x);
    } else {
      // Target differs — background simulate at the target frequency to get Z_ant.
      const req = buildSimulateRequest(
        wires, source, loads, transmissionLines, ground,
        { ...frequency, frequencyMhz: targetFreqMHz },
        referenceImpedance, weather,
      );
      simulate(req)
        .then((res) => runMatch(res.impedance.r, res.impedance.x))
        .catch((err: unknown) => {
          if (!cancelled) {
            setError(err instanceof Error ? err.message : String(err));
            setResult(null);
            setLoading(false);
          }
        });
    }

    return () => { cancelled = true; };
  }, [simulationResult, targetFreqMHz, simFreqMHz, z0, qFactor,
      wires, source, loads, transmissionLines, ground, weather,
      referenceImpedance, frequency]);

  if (!simulationResult) {
    return (
      <div className="match-panel placeholder">
        <p className="muted">Run a simulation first to design a matching network.</p>
      </div>
    );
  }

  const isOffFreq = Math.abs(targetFreqMHz - simFreqMHz) >= 1e-6;
  const antSWR = targetImpedance
    ? computeSWR(targetImpedance.r, targetImpedance.x, z0)
    : null;

  return (
    <div className="match-panel">
      <div className="match-header">
        <div className="match-header-left">
          <label className="match-freq-label">
            Design at:
            <input
              type="number"
              className="match-freq-input"
              min={0.1}
              max={30000}
              step={0.001}
              value={targetFreqMHz}
              onChange={(e) => {
                const v = parseFloat(e.target.value);
                if (v > 0) setTargetFreqMHz(v);
              }}
            />
            MHz
          </label>
          {isOffFreq && (
            <button
              className="match-reset-btn"
              onClick={() => setTargetFreqMHz(simFreqMHz)}
              title="Reset to simulation frequency"
            >
              Reset to {simFreqMHz.toFixed(3)} MHz
            </button>
          )}
        </div>
        <label className="match-q">
          Q (π / T):
          <input
            type="number"
            min={2}
            max={50}
            step={1}
            value={qFactor}
            onChange={(e) => setQFactor(Math.max(2, parseFloat(e.target.value) || 10))}
          />
        </label>
      </div>

      {targetImpedance && (
        <div className="match-antenna-z">
          <span>
            Antenna at {targetFreqMHz.toFixed(3)} MHz:{' '}
            <strong>{formatImpedance(targetImpedance.r, targetImpedance.x)}</strong>
          </span>
          {antSWR !== null && (
            <span className="muted small" style={{ marginLeft: 8 }}>
              SWR {antSWR > 100 ? '>100' : antSWR.toFixed(1)}:1
              {' '}→ matched to 1:1
            </span>
          )}
        </div>
      )}

      {loading && <p className="muted">{isOffFreq ? 'Simulating at target frequency…' : 'Designing…'}</p>}
      {error && <p className="status-error">{error}</p>}

      {result && (
        <div className="match-solutions">
          {result.solutions.map((s) => (
            <SolutionCard key={s.topology} s={s} />
          ))}
        </div>
      )}
    </div>
  );
};

export default MatchingNetwork;
