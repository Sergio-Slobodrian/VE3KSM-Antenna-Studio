/**
 * Ground-plane configuration panel.
 *
 * Offers three ground types: Free Space (no ground), Perfect (PEC at Z=0),
 * and Real (lossy ground with user-specified conductivity and permittivity).
 * Conductivity/permittivity fields are only shown when "Real Ground" is selected.
 *
 * Under Real Ground a Moisture preset dropdown pre-fills εr/σ from standard
 * ARRL/ITU-R soil categories. Picking "Custom" leaves εr/σ alone (legacy
 * behaviour).  Users may hand-edit εr/σ after picking a preset — the preset
 * label sticks, matching the WeatherPanel preset-with-override pattern.
 */
import React from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import NumericInput from '@/components/common/NumericInput';
import {
  SOIL_MOISTURE_PRESETS,
  type GroundConfig as GroundConfigType,
  type SoilMoisturePreset,
} from '@/types';

const GroundConfig: React.FC = () => {
  const { ground, setGround } = useAntennaStore();

  const groundTypes: { value: GroundConfigType['type']; label: string }[] = [
    { value: 'free_space', label: 'Free Space' },
    { value: 'perfect', label: 'Perfect Ground' },
    { value: 'real', label: 'Real Ground' },
  ];

  const handleMoisturePreset = (key: SoilMoisturePreset) => {
    const preset = SOIL_MOISTURE_PRESETS.find((p) => p.key === key);
    if (!preset) return;
    if (preset.key === 'custom') {
      setGround({ moisturePreset: 'custom' });
      return;
    }
    setGround({
      moisturePreset: preset.key,
      permittivity: preset.epsR,
      conductivity: preset.sigma,
    });
  };

  const isRealGround = ground.type === 'real';
  const presetActive = isRealGround && ground.moisturePreset !== 'custom';

  return (
    <div className="config-section">
      <h3 style={presetActive ? { color: 'var(--accent)' } : undefined}>
        Ground
        {presetActive
          ? ` — ${SOIL_MOISTURE_PRESETS.find((p) => p.key === ground.moisturePreset)?.label ?? ''}`
          : ''}
      </h3>
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
      {isRealGround && (
        <>
          <div className="config-row">
            <label>Moisture</label>
            <select
              value={ground.moisturePreset}
              onChange={(e) => handleMoisturePreset(e.target.value as SoilMoisturePreset)}
              title="Soil moisture preset — fills εr and conductivity from standard soil categories"
            >
              {SOIL_MOISTURE_PRESETS.map((p) => (
                <option key={p.key} value={p.key}>
                  {p.label}
                </option>
              ))}
            </select>
          </div>
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
