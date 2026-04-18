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
              <th>Coating</th>
              <th>εr</th>
              <th>Coat t ({unitLabel})</th>
              <th>tan δ</th>
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
