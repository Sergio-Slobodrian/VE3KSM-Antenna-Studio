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
 */
import React, { useEffect, useState } from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import { designMatch, type MatchResult, type MatchSolution, type MatchComponent } from '@/api/client';
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
  const { simulationResult, referenceImpedance, frequency } = useAntennaStore();
  const [qFactor, setQFactor] = useState(10);
  const [result, setResult] = useState<MatchResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const r = simulationResult?.impedance.r;
  const x = simulationResult?.impedance.x;
  const freqMHz = frequency.frequencyMhz;
  const z0 = referenceImpedance;

  useEffect(() => {
    if (r === undefined || x === undefined || freqMHz <= 0) {
      setResult(null);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    designMatch({ loadR: r, loadX: x, sourceZ0: z0, freqMHz, qFactor })
      .then((res) => {
        if (!cancelled) {
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
    return () => { cancelled = true; };
  }, [r, x, z0, freqMHz, qFactor]);

  if (!simulationResult) {
    return (
      <div className="match-panel placeholder">
        <p className="muted">Run a simulation first to design a matching network.</p>
      </div>
    );
  }

  return (
    <div className="match-panel">
      <div className="match-header">
        <div>
          <strong>Match {formatImpedance(r ?? 0, x ?? 0)} → {z0.toFixed(0)} Ω</strong>
          <span className="muted small" style={{ marginLeft: 8 }}>at {freqMHz.toFixed(3)} MHz</span>
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

      {loading && <p className="muted">Designing...</p>}
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
