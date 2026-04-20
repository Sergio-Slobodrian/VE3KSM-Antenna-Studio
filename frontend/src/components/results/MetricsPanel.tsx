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
 * Headline far-field metrics panel.
 *
 * Displays the scalar antenna-design numbers users actually care about:
 * peak gain, peak direction, F/B, beamwidths, sidelobe level, radiation
 * efficiency, and input/radiated power.  Sourced from the backend
 * post-processor (mom.FarFieldMetrics).
 */
import React from 'react';
import { useAntennaStore } from '@/store/antennaStore';

const fmt = (v: number, digits = 2) =>
  Number.isFinite(v) ? v.toFixed(digits) : 'n/a';

const fmtPower = (w: number) => {
  if (!Number.isFinite(w)) return 'n/a';
  if (Math.abs(w) >= 1) return `${w.toFixed(3)} W`;
  if (Math.abs(w) >= 1e-3) return `${(w * 1e3).toFixed(3)} mW`;
  if (Math.abs(w) >= 1e-6) return `${(w * 1e6).toFixed(3)} µW`;
  return `${(w * 1e9).toFixed(3)} nW`;
};

const MetricsPanel: React.FC = () => {
  const { simulationResult } = useAntennaStore();

  if (!simulationResult) {
    return (
      <div className="metrics-panel placeholder">
        <p className="muted">Run a simulation to see headline metrics.</p>
      </div>
    );
  }

  const m = simulationResult.metrics;
  const z = simulationResult.impedance;
  const efficiencyPct = m.radiationEfficiency * 100;

  const tiles: { label: string; value: string; hint?: string }[] = [
    { label: 'Peak Gain', value: `${fmt(m.peakGainDb)} dBi`, hint: 'Maximum directivity' },
    {
      label: 'Peak Direction',
      value: `θ=${fmt(m.peakThetaDeg, 1)}° φ=${fmt(m.peakPhiDeg, 1)}°`,
      hint: 'Polar / azimuth of main lobe',
    },
    {
      label: 'Front / Back',
      value: `${fmt(m.frontToBackDb)} dB`,
      hint: 'Peak − antipode gain',
    },
    {
      label: '−3 dB Az',
      value: m.beamwidthAzDeg > 0 ? `${fmt(m.beamwidthAzDeg, 1)}°` : 'omni',
      hint: 'Azimuthal beamwidth',
    },
    {
      label: '−3 dB El',
      value: m.beamwidthElDeg > 0 ? `${fmt(m.beamwidthElDeg, 1)}°` : 'wide',
      hint: 'Elevation beamwidth',
    },
    {
      label: 'Sidelobe',
      value: m.sidelobeLevelDb < 0 ? `${fmt(m.sidelobeLevelDb)} dB` : 'none',
      hint: 'Strongest sidelobe vs main lobe',
    },
    {
      label: 'Efficiency',
      value: `${fmt(efficiencyPct, 1)}%`,
      hint: '(P_in − P_loss) / P_in',
    },
    {
      label: 'Z_in',
      value: `${fmt(z.r, 1)} ${z.x >= 0 ? '+' : '−'} j${fmt(Math.abs(z.x), 1)} Ω`,
      hint: `Reference Z₀ = ${fmt(simulationResult.referenceImpedance, 0)} Ω`,
    },
    {
      label: 'VSWR',
      value: simulationResult.swr >= 999 ? '∞' : fmt(simulationResult.swr, 2),
      hint: `at ${fmt(simulationResult.referenceImpedance, 0)} Ω`,
    },
    {
      label: 'P_in',
      value: fmtPower(m.inputPowerW),
      hint: 'Re(V·I*) at feed',
    },
    {
      label: 'P_radiated',
      value: fmtPower(m.totalRadiatedPowerW),
      hint: 'P_in × η',
    },
  ];

  return (
    <div className="metrics-panel">
      {tiles.map((t) => (
        <div key={t.label} className="metric-tile" title={t.hint || ''}>
          <div className="metric-label">{t.label}</div>
          <div className="metric-value">{t.value}</div>
        </div>
      ))}
    </div>
  );
};

export default MetricsPanel;
