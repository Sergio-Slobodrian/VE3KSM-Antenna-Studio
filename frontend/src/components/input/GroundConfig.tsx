import React from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import NumericInput from '@/components/common/NumericInput';
import type { GroundConfig as GroundConfigType } from '@/types';

const GroundConfig: React.FC = () => {
  const { ground, setGround } = useAntennaStore();

  const groundTypes: { value: GroundConfigType['type']; label: string }[] = [
    { value: 'free_space', label: 'Free Space' },
    { value: 'perfect', label: 'Perfect Ground' },
    { value: 'real', label: 'Real Ground' },
  ];

  return (
    <div className="config-section">
      <h3>Ground</h3>
      <div className="config-row">
        <label>Type</label>
        <select
          value={ground.type}
          onChange={(e) =>
            setGround({ type: e.target.value as GroundConfigType['type'] })
          }
        >
          {groundTypes.map((gt) => (
            <option key={gt.value} value={gt.value}>
              {gt.label}
            </option>
          ))}
        </select>
      </div>
      {ground.type === 'real' && (
        <>
          <div className="config-row">
            <NumericInput
              label="Conductivity (S/m)"
              value={ground.conductivity}
              onChange={(v) => setGround({ conductivity: v })}
              min={0}
              step={0.001}
            />
          </div>
          <div className="config-row">
            <NumericInput
              label="Permittivity"
              value={ground.permittivity}
              onChange={(v) => setGround({ permittivity: v })}
              min={1}
              step={0.1}
            />
          </div>
        </>
      )}
    </div>
  );
};

export default GroundConfig;
