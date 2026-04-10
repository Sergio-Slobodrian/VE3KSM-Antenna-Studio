import React from 'react';
import type { Wire } from '@/types';
import { METERS_TO_UNIT } from '@/types';
import { useAntennaStore } from '@/store/antennaStore';

interface WireRowProps {
  wire: Wire;
  index: number;
}

/** Fields that represent lengths/positions and should be unit-converted. */
const lengthFields: (keyof Wire)[] = ['x1', 'y1', 'z1', 'x2', 'y2', 'z2', 'radius'];

const WireRow: React.FC<WireRowProps> = ({ wire, index }) => {
  const { updateWire, removeWire, selectWire, selectedWireId, displayUnit } = useAntennaStore();
  const isSelected = selectedWireId === wire.id;
  const factor = METERS_TO_UNIT[displayUnit];

  const handleChange = (field: keyof Wire, value: string) => {
    const num = parseFloat(value);
    if (!isNaN(num)) {
      // Convert display value back to meters for storage
      const stored = lengthFields.includes(field) ? num / factor : num;
      updateWire(wire.id, { [field]: stored });
    }
  };

  const displayValue = (field: keyof Wire): number => {
    const raw = wire[field] as number;
    return lengthFields.includes(field) ? raw * factor : raw;
  };

  const fields: (keyof Wire)[] = ['x1', 'y1', 'z1', 'x2', 'y2', 'z2', 'radius', 'segments'];

  return (
    <tr
      className={`wire-row ${isSelected ? 'wire-row-selected' : ''}`}
      onClick={() => selectWire(wire.id)}
    >
      <td>{index + 1}</td>
      {fields.map((field) => (
        <td key={field}>
          <input
            type="number"
            value={parseFloat(displayValue(field).toPrecision(6))}
            onChange={(e) => handleChange(field, e.target.value)}
            step={field === 'radius' ? 0.0001 : field === 'segments' ? 1 : 0.1}
            className="wire-input"
          />
        </td>
      ))}
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
