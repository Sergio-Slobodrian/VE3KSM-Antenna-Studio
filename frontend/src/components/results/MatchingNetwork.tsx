/**
 * Impedance matching network designer.
 *
 * Takes the simulated antenna impedance and designs L-network, Pi-network,
 * and toroidal transformer matching solutions to transform the antenna
 * impedance to the transmitter impedance (default 50 Ω). Shows component
 * values, nearest standard values, ASCII schematics, and toroid core
 * recommendations.
 */
import React, { useState, useMemo } from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import { formatImpedance } from '@/utils/conversions';
import {
  designLNetwork,
  designPiNetwork,
  designToroidalTransformer,
  type LNetworkResult,
  type PiNetworkResult,
  type ToroidResult,
  type Component,
} from '@/utils/matching';

/** Renders a single component (inductor or capacitor) with exact and standard values. */
const ComponentInfo: React.FC<{ comp: Component; label: string }> = ({ comp, label }) => (
  <div className="match-component">
    <strong>{label}</strong> ({comp.position}):{' '}
    <span className="match-type">{comp.type === 'inductor' ? 'L' : 'C'}</span>
    {' = '}
    <span className="match-value">{comp.label}</span>
    {' '}
    <span className="match-std">(std: {comp.standardLabel})</span>
    {' — '}
    <span className="match-reactance">
      {comp.reactance >= 0 ? '+' : ''}
      {comp.reactance.toFixed(1)} Ω
    </span>
  </div>
);

/** ASCII schematic for an L-network. */
const LNetworkSchematic: React.FC<{ sol: LNetworkResult['solutions'][0]; loadR: number; sourceR: number }> = ({ sol, loadR, sourceR }) => {
  const c1 = sol.comp1;
  const c2 = sol.comp2;
  const c1Label = c1.type === 'inductor' ? 'L' : 'C';
  const c2Label = c2.type === 'inductor' ? 'L' : 'C';

  // Determine topology
  const seriesComp = c1.position === 'series' ? c1 : c2;
  const shuntComp = c1.position === 'shunt' ? c1 : c2;
  const sLabel = seriesComp.type === 'inductor' ? 'L' : 'C';
  const pLabel = shuntComp.type === 'inductor' ? 'L' : 'C';

  return (
    <pre className="match-schematic">
{`  ${sourceR}Ω        ${sLabel}(${seriesComp.standardLabel})        Z_ant
  TX ────┤────[${sLabel}]────┤──── Antenna
         │                   │    ${loadR.toFixed(0)}Ω
         ┴ ${pLabel}              ┴
        (${shuntComp.standardLabel})
         │                   │
        GND                 GND`}
    </pre>
  );
};

/** L-Network section. */
const LNetworkSection: React.FC<{ result: LNetworkResult; freqMHz: number }> = ({ result, freqMHz }) => {
  if (result.solutions.length === 0) {
    return <p className="match-note">Impedance already matched — no L-network needed.</p>;
  }

  return (
    <div>
      {result.solutions.map((sol, i) => (
        <div key={i} className="match-solution">
          <h4>{sol.name}</h4>
          <div className="match-meta">
            Q = {sol.Q.toFixed(1)} | BW ≈ {(sol.bandwidthHz / 1e6).toFixed(2)} MHz
            ({(freqMHz / sol.Q).toFixed(1)} MHz @ {freqMHz.toFixed(1)} MHz)
          </div>
          <ComponentInfo comp={sol.comp1} label="Element 1" />
          <ComponentInfo comp={sol.comp2} label="Element 2" />
          <LNetworkSchematic sol={sol} loadR={result.loadR} sourceR={result.sourceZ} />
        </div>
      ))}
    </div>
  );
};

/** Pi-Network section. */
const PiNetworkSection: React.FC<{ result: PiNetworkResult; freqMHz: number }> = ({ result, freqMHz }) => (
  <div className="match-solution">
    <div className="match-meta">
      Q = {result.Q.toFixed(1)} | BW ≈ {(result.bandwidthHz / 1e6).toFixed(2)} MHz
    </div>
    <ComponentInfo comp={result.shuntInput} label="Input shunt" />
    <ComponentInfo comp={result.series} label="Series" />
    <ComponentInfo comp={result.shuntOutput} label="Output shunt" />
    <pre className="match-schematic">
{`  ${result.sourceZ}Ω    C1       L/C       C2     Z_ant
  TX ──┤──┤──[series]──┤──┤── Antenna
       │  ┴            ┴  │   ${result.loadR.toFixed(0)}Ω
       │ (${result.shuntInput.standardLabel})  (${result.shuntOutput.standardLabel}) │
       │  │            │  │
      GND GND         GND GND`}
    </pre>
  </div>
);

/** Toroidal transformer section. */
const ToroidSection: React.FC<{ result: ToroidResult }> = ({ result }) => (
  <div className="match-solution">
    <div className="match-meta">
      Turns ratio: {result.turnsRatio.toFixed(2)}:1 |
      Impedance ratio: {result.impedanceRatio.toFixed(1)}:1
    </div>
    {result.note && <p className="match-note">{result.note}</p>}
    <pre className="match-schematic">
{`  ${result.sourceZ}Ω                          Z_ant
  TX ────┤  ╔══════╗  ┤──── Antenna
         │  ║ n:1  ║  │     ${result.loadR.toFixed(0)}Ω
         │  ║Toroid║  │
         ┤  ╚══════╝  ┤
        GND           GND`}
    </pre>
    {result.coreOptions.length > 0 && (
      <div className="match-cores">
        <h5>Recommended Cores</h5>
        <table className="match-core-table">
          <thead>
            <tr>
              <th>Core</th>
              <th>Material</th>
              <th>Primary</th>
              <th>Secondary</th>
              <th>L_primary</th>
              <th>Freq Range</th>
            </tr>
          </thead>
          <tbody>
            {result.coreOptions.slice(0, 6).map((opt, i) => (
              <tr key={i}>
                <td>{opt.core.name}</td>
                <td>{opt.core.material}</td>
                <td>{opt.primaryTurns}T</td>
                <td>{opt.secondaryTurns}T</td>
                <td>{opt.primaryInductance_uH.toFixed(2)} µH</td>
                <td>{opt.core.freqRange}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    )}
  </div>
);

/** Main matching network component. */
const MatchingNetwork: React.FC = () => {
  const simulationResult = useAntennaStore((s) => s.simulationResult);
  const frequency = useAntennaStore((s) => s.frequency);
  const [sourceZ, setSourceZ] = useState(50);
  const [activeTab, setActiveTab] = useState<'l' | 'pi' | 'toroid'>('l');

  const freqHz = frequency.frequencyMhz * 1e6;

  const { lNetwork, piNetwork, toroid } = useMemo(() => {
    if (!simulationResult) return { lNetwork: null, piNetwork: null, toroid: null };

    const R = simulationResult.impedance.r;
    const X = simulationResult.impedance.x;

    return {
      lNetwork: designLNetwork(R, X, sourceZ, freqHz),
      piNetwork: designPiNetwork(R, X, sourceZ, freqHz),
      toroid: designToroidalTransformer(R, X, sourceZ, freqHz),
    };
  }, [simulationResult, sourceZ, freqHz]);

  if (!simulationResult) {
    return (
      <div className="no-data-message">
        <p>No impedance data.</p>
        <p>Run a simulation first, then design a matching network.</p>
      </div>
    );
  }

  const R = simulationResult.impedance.r;
  const X = simulationResult.impedance.x;

  return (
    <div className="matching-container">
      <h3>Impedance Matching Network</h3>

      <div className="match-header">
        <div className="match-impedances">
          <div>
            <label>Antenna Z:</label>
            <span className="match-value">{formatImpedance(R, X)}</span>
            <span className="match-swr">SWR {simulationResult.swr.toFixed(2)}:1</span>
          </div>
          <div>
            <label>Target Z:</label>
            <input
              type="number"
              value={sourceZ}
              onChange={(e) => setSourceZ(Math.max(1, parseFloat(e.target.value) || 50))}
              min={1}
              step={1}
              className="wire-input"
              style={{ width: 60 }}
            />
            <span> Ω (transmitter)</span>
          </div>
          <div>
            <label>Frequency:</label>
            <span>{frequency.frequencyMhz.toFixed(3)} MHz</span>
          </div>
        </div>
      </div>

      <div className="match-tabs">
        <button
          className={`tab-btn ${activeTab === 'l' ? 'tab-active' : ''}`}
          onClick={() => setActiveTab('l')}
        >
          L-Network
        </button>
        <button
          className={`tab-btn ${activeTab === 'pi' ? 'tab-active' : ''}`}
          onClick={() => setActiveTab('pi')}
        >
          Pi-Network
        </button>
        <button
          className={`tab-btn ${activeTab === 'toroid' ? 'tab-active' : ''}`}
          onClick={() => setActiveTab('toroid')}
        >
          Toroidal Transformer
        </button>
      </div>

      <div className="match-content">
        {activeTab === 'l' && lNetwork && (
          <LNetworkSection result={lNetwork} freqMHz={frequency.frequencyMhz} />
        )}
        {activeTab === 'pi' && piNetwork && (
          <PiNetworkSection result={piNetwork} freqMHz={frequency.frequencyMhz} />
        )}
        {activeTab === 'toroid' && toroid && (
          <ToroidSection result={toroid} />
        )}
      </div>
    </div>
  );
};

export default MatchingNetwork;
