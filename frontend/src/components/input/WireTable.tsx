import React from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import WireRow from './WireRow';

const WireTable: React.FC = () => {
  const { wires, addWire } = useAntennaStore();

  return (
    <div className="wire-table-container">
      <h3>Wires</h3>
      <div className="table-scroll">
        <table className="wire-table">
          <thead>
            <tr>
              <th>#</th>
              <th>X1</th>
              <th>Y1</th>
              <th>Z1</th>
              <th>X2</th>
              <th>Y2</th>
              <th>Z2</th>
              <th>Radius</th>
              <th>Segs</th>
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
