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
 * Source excitation + reference impedance configuration.
 *
 * Picks which wire and segment receives the voltage source, the source
 * voltage, and the reference impedance Z0 used for SWR / Smith-chart
 * reflection-coefficient calculations.
 */
import React from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import NumericInput from '@/components/common/NumericInput';

const SourceConfig: React.FC = () => {
  const { source, wires, setSource, referenceImpedance, setReferenceImpedance } = useAntennaStore();

  const selectedWire = wires[source.wireIndex];
  const maxSegment = selectedWire ? selectedWire.segments - 1 : 0;

  return (
    <div className="config-section">
      <h3>Source &amp; Reference</h3>
      <div className="config-row">
        <label>Wire</label>
        <select
          value={source.wireIndex}
          onChange={(e) =>
            setSource({ wireIndex: parseInt(e.target.value, 10), segmentIndex: 0 })
          }
        >
          {wires.map((_, i) => (
            <option key={i} value={i}>
              Wire {i + 1}
            </option>
          ))}
        </select>
      </div>
      <div className="config-row">
        <label>Segment</label>
        <select
          value={source.segmentIndex}
          onChange={(e) => setSource({ segmentIndex: parseInt(e.target.value, 10) })}
        >
          {Array.from({ length: maxSegment + 1 }, (_, i) => (
            <option key={i} value={i}>
              {i}
            </option>
          ))}
        </select>
      </div>
      <div className="config-row">
        <NumericInput
          label="Voltage (V)"
          value={source.voltage}
          onChange={(v) => setSource({ voltage: v })}
          min={0}
          step={0.1}
        />
      </div>
      <div className="config-row">
        <NumericInput
          label="Z0 (Ω)"
          value={referenceImpedance}
          onChange={(v) => setReferenceImpedance(v)}
          min={1}
          step={1}
        />
      </div>
    </div>
  );
};

export default SourceConfig;
