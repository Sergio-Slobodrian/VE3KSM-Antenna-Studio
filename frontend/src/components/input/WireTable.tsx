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
 * Wire geometry table.
 *
 * Displays all wires in a tabular form with editable endpoint coordinates,
 * radius, segment count, and conductor material.  Includes a unit-selector
 * dropdown that controls the display unit for all length fields.
 */
import React from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import WireRow from './WireRow';
import type { DisplayUnit } from '@/types';
import { UNIT_LABELS } from '@/types';

const unitOptions: DisplayUnit[] = ['meters', 'feet', 'inches', 'cm', 'mm'];

const WireTable: React.FC = () => {
  const { wires, addWire, displayUnit, setDisplayUnit } = useAntennaStore();
  const unitLabel = UNIT_LABELS[displayUnit];

  return (
    <div className="wire-table-container">
      <div className="section-header">
        <h3>Wires</h3>
        <div className="unit-selector">
          <label>Units:</label>
          <select
            value={displayUnit}
            onChange={(e) => setDisplayUnit(e.target.value as DisplayUnit)}
          >
            {unitOptions.map((u) => (
              <option key={u} value={u}>{u}</option>
            ))}
          </select>
        </div>
      </div>
      <div className="table-scroll">
        <table className="wire-table">
          <thead>
            <tr>
              <th>#</th>
              <th>X1 ({unitLabel})</th>
              <th>Y1 ({unitLabel})</th>
              <th>Z1 ({unitLabel})</th>
              <th>X2 ({unitLabel})</th>
              <th>Y2 ({unitLabel})</th>
              <th>Z2 ({unitLabel})</th>
              <th>Radius ({unitLabel})</th>
              <th>Segs</th>
              <th>Material</th>
              <th>Coating Preset</th>
              <th>Coat-t ({unitLabel})</th>
              <th>εr</th>
              <th>tanδ</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {wires.map((wire, i) => (
              <WireRow key={wire.id} wire={wire} index={i} />
            ))}
          </tbody>
        </table>
      </div>
      <button className="btn btn-primary" onClick={() => addWire()}>
        + Add Wire
      </button>
    </div>
  );
};

export default WireTable;
