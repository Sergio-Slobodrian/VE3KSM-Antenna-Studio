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
 * Lumped-load editor.
 *
 * Manages an array of R/L/C lumped loads attached to specific
 * (wire, segment) pairs.  Each load is a series_rlc or parallel_rlc
 * combination; zero-valued components are simply omitted by the
 * backend (so {R: 50} alone is a 50 Ω terminator).
 *
 * Useful for traps, loading coils, terminations, hat capacitors,
 * folded-dipole stubs, and lumped baluns.
 */
import React from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import type { Load } from '@/types';

const LoadEditor: React.FC = () => {
  const { loads, wires, addLoad, updateLoad, removeLoad } = useAntennaStore();

  return (
    <div className="config-section">
      <div className="section-header">
        <h3>Lumped Loads</h3>
        <button
          className="btn btn-primary btn-small"
          onClick={() =>
            addLoad({
              wireIndex: 0,
              segmentIndex: Math.max(0, Math.floor((wires[0]?.segments ?? 11) / 2)),
              topology: 'series_rlc',
              r: 50,
              l: 0,
              c: 0,
            })
          }
        >
          + Add Load
        </button>
      </div>
      {loads.length === 0 ? (
        <p className="muted small">
          No loads defined.  Add one to model a trap, loading coil, terminator, or lumped balun.
        </p>
      ) : (
        <div className="table-scroll">
          <table className="wire-table compact">
            <thead>
              <tr>
                <th>Wire</th>
                <th>Seg</th>
                <th>Topology</th>
                <th>R (Ω)</th>
                <th>L (µH)</th>
                <th>C (pF)</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {loads.map((ld) => (
                <LoadRow key={ld.id} load={ld} wires={wires} updateLoad={updateLoad} removeLoad={removeLoad} />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};

interface LoadRowProps {
  load: Load;
  wires: { id: string; segments: number }[];
  updateLoad: (id: string, updates: Partial<Load>) => void;
  removeLoad: (id: string) => void;
}

/** Single editable row inside the LoadEditor table. */
const LoadRow: React.FC<LoadRowProps> = ({ load, wires, updateLoad, removeLoad }) => {
  const targetWire = wires[load.wireIndex];
  const maxSeg = targetWire ? targetWire.segments - 1 : 0;
  // L is shown in microhenries and C in picofarads for HF-friendly defaults.
  // Internal storage stays in SI units (H and F).
  const lMicro = load.l * 1e6;
  const cPico = load.c * 1e12;

  return (
    <tr className="wire-row">
      <td>
        <select
          value={load.wireIndex}
          onChange={(e) => updateLoad(load.id, {
            wireIndex: parseInt(e.target.value, 10),
            segmentIndex: 0,
          })}
        >
          {wires.map((_, i) => (
            <option key={i} value={i}>{i + 1}</option>
          ))}
        </select>
      </td>
      <td>
        <select
          value={load.segmentIndex}
          onChange={(e) => updateLoad(load.id, { segmentIndex: parseInt(e.target.value, 10) })}
        >
          {Array.from({ length: maxSeg + 1 }, (_, i) => (
            <option key={i} value={i}>{i}</option>
          ))}
        </select>
      </td>
      <td>
        <select
          value={load.topology}
          onChange={(e) => updateLoad(load.id, { topology: e.target.value as Load['topology'] })}
        >
          <option value="series_rlc">Series</option>
          <option value="parallel_rlc">Parallel</option>
        </select>
      </td>
      <td>
        <input
          type="number"
          className="wire-input"
          value={load.r}
          step={1}
          min={0}
          onChange={(e) => updateLoad(load.id, { r: parseFloat(e.target.value) || 0 })}
        />
      </td>
      <td>
        <input
          type="number"
          className="wire-input"
          value={parseFloat(lMicro.toPrecision(6))}
          step={0.1}
          min={0}
          onChange={(e) => updateLoad(load.id, { l: (parseFloat(e.target.value) || 0) * 1e-6 })}
        />
      </td>
      <td>
        <input
          type="number"
          className="wire-input"
          value={parseFloat(cPico.toPrecision(6))}
          step={1}
          min={0}
          onChange={(e) => updateLoad(load.id, { c: (parseFloat(e.target.value) || 0) * 1e-12 })}
        />
      </td>
      <td>
        <button className="btn-small btn-danger" onClick={() => removeLoad(load.id)}>Del</button>
      </td>
    </tr>
  );
};

export default LoadEditor;
