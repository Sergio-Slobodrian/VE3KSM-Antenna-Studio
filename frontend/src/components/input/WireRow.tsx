import React from 'react';
import type { Wire } from '@/types';
import { useAntennaStore } from '@/store/antennaStore';

interface WireRowProps {
  wire: Wire;
  index: number;
}

const WireRow: React.FC<WireRowProps> = ({ wire, index }) => {
  const { updateWire, removeWire, selectWire, selectedWireId } = useAntennaStore();
  const isSelected = selectedWireId === wire.id;

  const handleChange = (field: keyof Wire, value: string) => {
    const num = parseFloat(value);
    if (!isNaN(num)) {
      updateWire(wire.id, { [field]: num });
    }
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
            value={wire[field] as number}
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
