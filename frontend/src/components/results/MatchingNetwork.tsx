/**
 * Impedance matching network designer.
 *
 * Takes the simulated antenna impedance and designs L-network, Pi-network,
 * and toroidal transformer matching solutions. Uses SVG schematics with
 * proper component symbols (inductor coils, capacitor plates, transformer).
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

// ── SVG Component Symbols ────────────────────────────────────────────────────

const WIRE_COLOR = '#aabbcc';
const LABEL_COLOR = '#ccdde8';
const VALUE_COLOR = '#66ddaa';
const COMP_COLOR = '#ffaa44';
const GND_COLOR = '#888899';

/** SVG inductor symbol (horizontal coil) centered at (cx, cy), width w. */
const InductorH: React.FC<{ cx: number; cy: number; w?: number }> = ({ cx, cy, w = 40 }) => {
  const x0 = cx - w / 2;
  const humps = 4;
  const humpW = w / humps;
  let d = `M ${x0} ${cy}`;
  for (let i = 0; i < humps; i++) {
    const sx = x0 + i * humpW;
    d += ` A ${humpW / 2} 6 0 0 1 ${sx + humpW} ${cy}`;
  }
  return <path d={d} fill="none" stroke={COMP_COLOR} strokeWidth="2" />;
};

/** SVG capacitor symbol (horizontal, two plates) centered at (cx, cy). */
const CapacitorH: React.FC<{ cx: number; cy: number }> = ({ cx, cy }) => (
  <g>
    <line x1={cx - 4} y1={cy - 8} x2={cx - 4} y2={cy + 8} stroke={COMP_COLOR} strokeWidth="2" />
    <line x1={cx + 4} y1={cy - 8} x2={cx + 4} y2={cy + 8} stroke={COMP_COLOR} strokeWidth="2" />
  </g>
);

/** SVG inductor symbol (vertical coil) centered at (cx, cy), height h. */
const InductorV: React.FC<{ cx: number; cy: number; h?: number }> = ({ cx, cy, h = 36 }) => {
  const y0 = cy - h / 2;
  const humps = 4;
  const humpH = h / humps;
  let d = `M ${cx} ${y0}`;
  for (let i = 0; i < humps; i++) {
    const sy = y0 + i * humpH;
    d += ` A 6 ${humpH / 2} 0 0 1 ${cx} ${sy + humpH}`;
  }
  return <path d={d} fill="none" stroke={COMP_COLOR} strokeWidth="2" />;
};

/** SVG capacitor symbol (vertical, two plates) centered at (cx, cy). */
const CapacitorV: React.FC<{ cx: number; cy: number }> = ({ cx, cy }) => (
  <g>
    <line x1={cx - 8} y1={cy - 4} x2={cx + 8} y2={cy - 4} stroke={COMP_COLOR} strokeWidth="2" />
    <line x1={cx - 8} y1={cy + 4} x2={cx + 8} y2={cy + 4} stroke={COMP_COLOR} strokeWidth="2" />
  </g>
);

/** SVG ground symbol at (cx, cy). */
const Ground: React.FC<{ cx: number; cy: number }> = ({ cx, cy }) => (
  <g>
    <line x1={cx - 10} y1={cy} x2={cx + 10} y2={cy} stroke={GND_COLOR} strokeWidth="1.5" />
    <line x1={cx - 6} y1={cy + 4} x2={cx + 6} y2={cy + 4} stroke={GND_COLOR} strokeWidth="1.5" />
    <line x1={cx - 3} y1={cy + 8} x2={cx + 3} y2={cy + 8} stroke={GND_COLOR} strokeWidth="1.5" />
  </g>
);

/** Helper: horizontal or vertical component based on type. */
const CompH: React.FC<{ comp: Component; cx: number; cy: number }> = ({ comp, cx, cy }) =>
  comp.type === 'inductor' ? <InductorH cx={cx} cy={cy} /> : <CapacitorH cx={cx} cy={cy} />;

const CompV: React.FC<{ comp: Component; cx: number; cy: number }> = ({ comp, cx, cy }) =>
  comp.type === 'inductor' ? <InductorV cx={cx} cy={cy} /> : <CapacitorV cx={cx} cy={cy} />;

// ── Component Info Text ──────────────────────────────────────────────────────

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
      {comp.reactance >= 0 ? '+' : ''}{comp.reactance.toFixed(1)} Ω
    </span>
  </div>
);

// ── L-Network SVG Schematic ──────────────────────────────────────────────────

const LNetworkSchematic: React.FC<{ sol: LNetworkResult['solutions'][0]; loadR: number; sourceR: number }> = ({ sol, sourceR, loadR }) => {
  const seriesComp = sol.comp1.position === 'series' ? sol.comp1 : sol.comp2;
  const shuntComp = sol.comp1.position === 'shunt' ? sol.comp1 : sol.comp2;

  return (
    <svg viewBox="0 0 400 160" width="100%" height="140" className="match-svg">
      {/* TX terminal */}
      <text x="10" y="42" fill={LABEL_COLOR} fontSize="12" fontWeight="bold">TX</text>
      <text x="10" y="56" fill={GND_COLOR} fontSize="10">{sourceR}Ω</text>

      {/* Wire from TX to junction 1 */}
      <line x1="40" y1="45" x2="100" y2="45" stroke={WIRE_COLOR} strokeWidth="1.5" />

      {/* Junction 1 dot */}
      <circle cx="100" cy="45" r="3" fill={WIRE_COLOR} />

      {/* Series component (horizontal) */}
      <line x1="100" y1="45" x2="160" y2="45" stroke={WIRE_COLOR} strokeWidth="1.5" />
      <CompH comp={seriesComp} cx={200} cy={45} />
      <line x1="160" y1="45" x2="175" y2="45" stroke={WIRE_COLOR} strokeWidth="1.5" />
      <line x1="225" y1="45" x2="300" y2="45" stroke={WIRE_COLOR} strokeWidth="1.5" />
      {/* Series label */}
      <text x="200" y="32" fill={VALUE_COLOR} fontSize="10" textAnchor="middle">
        {seriesComp.type === 'inductor' ? 'L' : 'C'} = {seriesComp.standardLabel}
      </text>

      {/* Junction 2 dot */}
      <circle cx="300" cy="45" r="3" fill={WIRE_COLOR} />

      {/* Wire to antenna */}
      <line x1="300" y1="45" x2="360" y2="45" stroke={WIRE_COLOR} strokeWidth="1.5" />
      <text x="365" y="42" fill={LABEL_COLOR} fontSize="12" fontWeight="bold">ANT</text>
      <text x="365" y="56" fill={GND_COLOR} fontSize="10">{loadR.toFixed(0)}Ω</text>

      {/* Shunt component (vertical) from junction 1 */}
      <line x1="100" y1="45" x2="100" y2="70" stroke={WIRE_COLOR} strokeWidth="1.5" />
      <CompV comp={shuntComp} cx={100} cy={95} />
      <line x1="100" y1="118" x2="100" y2="130" stroke={WIRE_COLOR} strokeWidth="1.5" />
      <Ground cx={100} cy={132} />
      {/* Shunt label */}
      <text x="125" y="100" fill={VALUE_COLOR} fontSize="10">
        {shuntComp.type === 'inductor' ? 'L' : 'C'} = {shuntComp.standardLabel}
      </text>
    </svg>
  );
};

// ── Pi-Network SVG Schematic ─────────────────────────────────────────────────

const PiNetworkSchematic: React.FC<{ result: PiNetworkResult }> = ({ result }) => (
  <svg viewBox="0 0 440 170" width="100%" height="150" className="match-svg">
    {/* TX */}
    <text x="5" y="42" fill={LABEL_COLOR} fontSize="12" fontWeight="bold">TX</text>
    <text x="5" y="56" fill={GND_COLOR} fontSize="10">{result.sourceZ}Ω</text>
    <line x1="35" y1="45" x2="80" y2="45" stroke={WIRE_COLOR} strokeWidth="1.5" />

    {/* Junction A */}
    <circle cx="80" cy="45" r="3" fill={WIRE_COLOR} />

    {/* Input shunt (vertical) */}
    <line x1="80" y1="45" x2="80" y2="68" stroke={WIRE_COLOR} strokeWidth="1.5" />
    <CompV comp={result.shuntInput} cx={80} cy={90} />
    <line x1="80" y1="112" x2="80" y2="130" stroke={WIRE_COLOR} strokeWidth="1.5" />
    <Ground cx={80} cy={132} />
    <text x="100" y="88" fill={VALUE_COLOR} fontSize="9">
      {result.shuntInput.type === 'inductor' ? 'L' : 'C'}₁ = {result.shuntInput.standardLabel}
    </text>

    {/* Series component (horizontal) */}
    <line x1="80" y1="45" x2="175" y2="45" stroke={WIRE_COLOR} strokeWidth="1.5" />
    <CompH comp={result.series} cx={220} cy={45} />
    <line x1="265" y1="45" x2="350" y2="45" stroke={WIRE_COLOR} strokeWidth="1.5" />
    <text x="220" y="32" fill={VALUE_COLOR} fontSize="9" textAnchor="middle">
      {result.series.type === 'inductor' ? 'L' : 'C'} = {result.series.standardLabel}
    </text>

    {/* Junction B */}
    <circle cx="350" cy="45" r="3" fill={WIRE_COLOR} />

    {/* Output shunt (vertical) */}
    <line x1="350" y1="45" x2="350" y2="68" stroke={WIRE_COLOR} strokeWidth="1.5" />
    <CompV comp={result.shuntOutput} cx={350} cy={90} />
    <line x1="350" y1="112" x2="350" y2="130" stroke={WIRE_COLOR} strokeWidth="1.5" />
    <Ground cx={350} cy={132} />
    <text x="370" y="88" fill={VALUE_COLOR} fontSize="9">
      {result.shuntOutput.type === 'inductor' ? 'L' : 'C'}₂ = {result.shuntOutput.standardLabel}
    </text>

    {/* Wire to antenna */}
    <line x1="350" y1="45" x2="395" y2="45" stroke={WIRE_COLOR} strokeWidth="1.5" />
    <text x="400" y="42" fill={LABEL_COLOR} fontSize="12" fontWeight="bold">ANT</text>
    <text x="400" y="56" fill={GND_COLOR} fontSize="10">{result.loadR.toFixed(0)}Ω</text>
  </svg>
);

// ── Toroidal Transformer SVG Schematic ───────────────────────────────────────

const ToroidSchematic: React.FC<{ result: ToroidResult }> = ({ result }) => (
  <svg viewBox="0 0 400 150" width="100%" height="130" className="match-svg">
    {/* TX */}
    <text x="10" y="47" fill={LABEL_COLOR} fontSize="12" fontWeight="bold">TX</text>
    <text x="10" y="61" fill={GND_COLOR} fontSize="10">{result.sourceZ}Ω</text>
    <line x1="40" y1="50" x2="120" y2="50" stroke={WIRE_COLOR} strokeWidth="1.5" />
    <line x1="40" y1="100" x2="120" y2="100" stroke={WIRE_COLOR} strokeWidth="1.5" />

    {/* Primary winding (left coil) */}
    {[0, 1, 2, 3, 4].map(i => (
      <path
        key={`pri-${i}`}
        d={`M ${120} ${50 + i * 10} A 8 5 0 0 1 ${120} ${60 + i * 10}`}
        fill="none" stroke={COMP_COLOR} strokeWidth="2"
      />
    ))}
    <text x="95" y="80" fill={VALUE_COLOR} fontSize="9" textAnchor="middle">
      {result.primaryTurns}T
    </text>

    {/* Core lines */}
    <line x1="130" y1="45" x2="130" y2="105" stroke={GND_COLOR} strokeWidth="1.5" />
    <line x1="135" y1="45" x2="135" y2="105" stroke={GND_COLOR} strokeWidth="1.5" />

    {/* Secondary winding (right coil) */}
    {[0, 1, 2, 3, 4].map(i => (
      <path
        key={`sec-${i}`}
        d={`M ${145} ${50 + i * 10} A 8 5 0 0 0 ${145} ${60 + i * 10}`}
        fill="none" stroke={COMP_COLOR} strokeWidth="2"
      />
    ))}
    <text x="170" y="80" fill={VALUE_COLOR} fontSize="9" textAnchor="middle">
      {result.secondaryTurns}T
    </text>

    {/* Wires to antenna */}
    <line x1="145" y1="50" x2="360" y2="50" stroke={WIRE_COLOR} strokeWidth="1.5" />
    <line x1="145" y1="100" x2="360" y2="100" stroke={WIRE_COLOR} strokeWidth="1.5" />

    <text x="365" y="47" fill={LABEL_COLOR} fontSize="12" fontWeight="bold">ANT</text>
    <text x="365" y="61" fill={GND_COLOR} fontSize="10">{result.loadR.toFixed(0)}Ω</text>

    {/* Ground symbols */}
    <Ground cx={40} cy={104} />
    <Ground cx={360} cy={104} />

    {/* Ratio label */}
    <text x="132" y="120" fill={LABEL_COLOR} fontSize="10" textAnchor="middle">
      {result.turnsRatio.toFixed(2)} : 1
    </text>
  </svg>
);

// ── Sections ─────────────────────────────────────────────────────────────────

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

const PiNetworkSection: React.FC<{ result: PiNetworkResult; freqMHz: number }> = ({ result }) => (
  <div className="match-solution">
    <div className="match-meta">
      Q = {result.Q.toFixed(1)} | BW ≈ {(result.bandwidthHz / 1e6).toFixed(2)} MHz
    </div>
    <ComponentInfo comp={result.shuntInput} label="Input shunt" />
    <ComponentInfo comp={result.series} label="Series" />
    <ComponentInfo comp={result.shuntOutput} label="Output shunt" />
    <PiNetworkSchematic result={result} />
  </div>
);

const ToroidSection: React.FC<{ result: ToroidResult }> = ({ result }) => (
  <div className="match-solution">
    <div className="match-meta">
      Turns ratio: {result.turnsRatio.toFixed(2)}:1 |
      Impedance ratio: {result.impedanceRatio.toFixed(1)}:1
    </div>
    {result.note && <p className="match-note">{result.note}</p>}
    <ToroidSchematic result={result} />
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

// ── Main Component ───────────────────────────────────────────────────────────

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
              min={1} step={1}
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
        <button className={`tab-btn ${activeTab === 'l' ? 'tab-active' : ''}`} onClick={() => setActiveTab('l')}>
          L-Network
        </button>
        <button className={`tab-btn ${activeTab === 'pi' ? 'tab-active' : ''}`} onClick={() => setActiveTab('pi')}>
          Pi-Network
        </button>
        <button className={`tab-btn ${activeTab === 'toroid' ? 'tab-active' : ''}`} onClick={() => setActiveTab('toroid')}>
          Toroidal Transformer
        </button>
      </div>

      <div className="match-content">
        {activeTab === 'l' && lNetwork && <LNetworkSection result={lNetwork} freqMHz={frequency.frequencyMhz} />}
        {activeTab === 'pi' && piNetwork && <PiNetworkSection result={piNetwork} freqMHz={frequency.frequencyMhz} />}
        {activeTab === 'toroid' && toroid && <ToroidSection result={toroid} />}
      </div>
    </div>
  );
};

export default MatchingNetwork;
