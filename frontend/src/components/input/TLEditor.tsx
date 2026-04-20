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
 * Transmission-line editor.
 *
 * Manages an array of NEC-style "TL" elements.  Each line connects an
 * A end (always a real wire/segment) to a B end which may be either
 * another wire/segment or a special termination (shorted / open) for
 * stub modelling.
 *
 * Stubs are the most common use case (matching stubs, λ/4 transformers,
 * traps), so the default added line is a 1 m shorted stub at 50 Ω.
 */
import React from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import type { TransmissionLine } from '@/types';
import { TLEndShorted, TLEndOpen, METERS_TO_UNIT, UNIT_LABELS } from '@/types';

const TLEditor: React.FC = () => {
  const {
    transmissionLines,
    wires,
    addTransmissionLine,
    updateTransmissionLine,
    removeTransmissionLine,
    displayUnit,
  } = useAntennaStore();
  const factor = METERS_TO_UNIT[displayUnit];
  const unitLabel = UNIT_LABELS[displayUnit];

  return (
    <div className="config-section">
      <div className="section-header">
        <h3>Transmission Lines</h3>
        <button
          className="btn btn-primary btn-small"
          onClick={() => addTransmissionLine()}
        >
          + Add TL
        </button>
      </div>
      {transmissionLines.length === 0 ? (
        <p className="muted small">
          No TLs defined.  Add a shorted/open stub or a 2-port line for matching, traps, or feed networks.
        </p>
      ) : (
        <div className="table-scroll">
          <table className="wire-table compact">
            <thead>
              <tr>
                <th>A: wire/seg</th>
                <th>B: end</th>
                <th>Z₀ (Ω)</th>
                <th>Length ({unitLabel})</th>
                <th>VF</th>
                <th>Loss (dB/m)</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {transmissionLines.map((tl) => (
                <TLRow
                  key={tl.id}
                  tl={tl}
                  wires={wires}
                  factor={factor}
                  update={updateTransmissionLine}
                  remove={removeTransmissionLine}
                />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};

interface TLRowProps {
  tl: TransmissionLine;
  wires: { id: string; segments: number }[];
  factor: number;
  update: (id: string, updates: Partial<TransmissionLine>) => void;
  remove: (id: string) => void;
}

const TLRow: React.FC<TLRowProps> = ({ tl, wires, factor, update, remove }) => {
  const aWire = wires[tl.a.wireIndex];
  const bWire = tl.b.wireIndex >= 0 ? wires[tl.b.wireIndex] : undefined;
  const aMaxSeg = aWire ? aWire.segments - 1 : 0;
  const bMaxSeg = bWire ? bWire.segments - 1 : 0;

  return (
    <tr className="wire-row">
      <td>
        <select
          value={tl.a.wireIndex}
          onChange={(e) => update(tl.id, {
            a: { wireIndex: parseInt(e.target.value, 10), segmentIndex: 0 },
          })}
        >
          {wires.map((_, i) => (
            <option key={i} value={i}>W{i + 1}</option>
          ))}
        </select>
        <select
          value={tl.a.segmentIndex}
          onChange={(e) => update(tl.id, {
            a: { ...tl.a, segmentIndex: parseInt(e.target.value, 10) },
          })}
          style={{ marginLeft: 4 }}
        >
          {Array.from({ length: aMaxSeg + 1 }, (_, i) => (
            <option key={i} value={i}>{i}</option>
          ))}
        </select>
      </td>
      <td>
        <select
          value={tl.b.wireIndex}
          onChange={(e) => {
            const wi = parseInt(e.target.value, 10);
            update(tl.id, { b: { wireIndex: wi, segmentIndex: 0 } });
          }}
        >
          <option value={TLEndShorted}>Short</option>
          <option value={TLEndOpen}>Open</option>
          {wires.map((_, i) => (
            <option key={i} value={i}>W{i + 1}</option>
          ))}
        </select>
        {tl.b.wireIndex >= 0 && (
          <select
            value={tl.b.segmentIndex}
            onChange={(e) => update(tl.id, {
              b: { ...tl.b, segmentIndex: parseInt(e.target.value, 10) },
            })}
            style={{ marginLeft: 4 }}
          >
            {Array.from({ length: bMaxSeg + 1 }, (_, i) => (
              <option key={i} value={i}>{i}</option>
            ))}
          </select>
        )}
      </td>
      <td>
        <input
          type="number"
          className="wire-input"
          value={tl.z0}
          step={1}
          min={1}
          onChange={(e) => update(tl.id, { z0: parseFloat(e.target.value) || 50 })}
        />
      </td>
      <td>
        <input
          type="number"
          className="wire-input"
          value={parseFloat((tl.length * factor).toPrecision(6))}
          step={0.01}
          min={0}
          onChange={(e) => update(tl.id, { length: (parseFloat(e.target.value) || 0) / factor })}
        />
      </td>
      <td>
        <input
          type="number"
          className="wire-input"
          value={tl.velocityFactor}
          step={0.01}
          min={0}
          max={1}
          onChange={(e) => update(tl.id, { velocityFactor: Math.min(1, Math.max(0, parseFloat(e.target.value) || 0)) })}
        />
      </td>
      <td>
        <input
          type="number"
          className="wire-input"
          value={tl.lossDbPerM}
          step={0.01}
          min={0}
          onChange={(e) => update(tl.id, { lossDbPerM: parseFloat(e.target.value) || 0 })}
        />
      </td>
      <td>
        <button className="btn-small btn-danger" onClick={() => remove(tl.id)}>Del</button>
      </td>
    </tr>
  );
};

export default TLEditor;
