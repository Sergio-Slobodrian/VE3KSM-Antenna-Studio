/**
 * Editable row for a single wire in the WireTable.
 *
 * Handles unit conversion between the internal meters representation and the
 * user-selected display unit.  Length fields (coordinates + radius) are
 * multiplied by METERS_TO_UNIT[displayUnit] for display and divided back on
 * input.  The 'segments' field is unitless and passed through directly.
 *
 * Adds a Material dropdown for the new conductor-loss feature.
 */
import React from 'react';
import type { Wire, Material } from '@/types';
import { METERS_TO_UNIT, MATERIAL_LABELS } from '@/types';
import { useAntennaStore } from '@/store/antennaStore';

interface WireRowProps {
  wire: Wire;
  index: number;
}

/** Fields that represent lengths/positions and should be unit-converted. */
const lengthFields = new Set<keyof Wire>(['x1', 'y1', 'z1', 'x2', 'y2', 'z2', 'radius']);

const numericFields: (keyof Wire)[] = ['x1', 'y1', 'z1', 'x2', 'y2', 'z2', 'radius', 'segments'];
const materialOptions: Material[] = ['', 'copper', 'aluminum', 'brass', 'steel', 'stainless', 'silver', 'gold'];

const WireRow: React.FC<WireRowProps> = ({ wire, index }) => {
  const { updateWire, removeWire, selectWire, selectedWireId, displayUnit } = useAntennaStore();
  const isSelected = selectedWireId === wire.id;
  const factor = METERS_TO_UNIT[displayUnit];

  const handleNumChange = (field: keyof Wire, value: string) => {
    const num = parseFloat(value);
    if (!isNaN(num)) {
      const stored = lengthFields.has(field) ? num / factor : num;
      updateWire(wire.id, { [field]: stored } as Partial<Wire>);
    }
  };

  const displayValue = (field: keyof Wire): number => {
    const raw = wire[field] as number;
    return lengthFields.has(field) ? raw * factor : raw;
  };

  return (
    <tr
      className={`wire-row ${isSelected ? 'wire-row-selected' : ''}`}
      onClick={() => selectWire(wire.id)}
    >
      <td>{index + 1}</td>
      {numericFields.map((field) => (
        <td key={field}>
          <input
            type="number"
            value={parseFloat(displayValue(field).toPrecision(6))}
            onChange={(e) => handleNumChange(field, e.target.value)}
            step={field === 'radius' ? 0.0001 : field === 'segments' ? 1 : 0.1}
            className="wire-input"
          />
        </td>
      ))}
      <td>
        <select
          value={wire.material}
          onChange={(e) => updateWire(wire.id, { material: e.target.value as Material })}
          onClick={(e) => e.stopPropagation()}
          className="wire-input wire-material-select"
          title="Conductor material (skin-effect loss)"
        >
          {materialOptions.map((m) => (
            <option key={m || 'pec'} value={m}>{MATERIAL_LABELS[m]}</option>
          ))}
        </select>
      </td>
      <td>
        <button
          className="btn-small btn-danger"
          onClick={(e) => {
            e.stopPropagation();
            removeWire(wire.id);
          }}
        >
          Del
        </button>
      </td>
    </tr>
  );
};

export default WireRow;
