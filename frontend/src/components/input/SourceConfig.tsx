import React from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import NumericInput from '@/components/common/NumericInput';

const SourceConfig: React.FC = () => {
  const { source, wires, setSource } = useAntennaStore();

  const selectedWire = wires[source.wireIndex];
  const maxSegment = selectedWire ? selectedWire.segments - 1 : 0;

  return (
    <div className="config-section">
      <h3>Source</h3>
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
    </div>
  );
};

export default SourceConfig;
