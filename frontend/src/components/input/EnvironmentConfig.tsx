import React from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import { ENV_PRESETS } from '@/types';
import { METERS_TO_UNIT } from '@/types';

/** Return the preset label that matches current values, or 'Custom' if none match. */
function envPresetLabel(permittivity: number, thickness: number, lossTangent: number): string {
  if (permittivity <= 0 || thickness <= 0) return 'None';
  const match = ENV_PRESETS.find(
    (p) => p.permittivity > 0 &&
           p.permittivity === permittivity &&
           p.thickness === thickness &&
           p.lossTangent === lossTangent,
  );
  return match ? match.label : 'Custom';
}

const EnvironmentConfig: React.FC = () => {
  const envLayer = useAntennaStore((s) => s.envLayer);
  const setEnvLayer = useAntennaStore((s) => s.setEnvLayer);
  const displayUnit = useAntennaStore((s) => s.displayUnit);
  const factor = METERS_TO_UNIT[displayUnit];

  const currentLabel = envPresetLabel(envLayer.permittivity, envLayer.thickness, envLayer.lossTangent);

  const handlePreset = (label: string) => {
    if (label === 'None') {
      setEnvLayer({ permittivity: 0, thickness: 0, lossTangent: 0 });
    } else if (label !== 'Custom') {
      const preset = ENV_PRESETS.find((p) => p.label === label);
      if (preset) setEnvLayer({ permittivity: preset.permittivity, thickness: preset.thickness, lossTangent: preset.lossTangent });
    }
    // 'Custom' selected: leave current values intact
  };

  const active = envLayer.permittivity >= 1 && envLayer.thickness > 0;

  return (
    <div className="config-section">
      <h3>Environment</h3>
      <div className="config-row">
        <label>Condition</label>
        <select
          value={currentLabel}
          onChange={(e) => handlePreset(e.target.value)}
          className="wire-input wire-material-select"
        >
          {ENV_PRESETS.map((p) => (
            <option key={p.label} value={p.label}>{p.label}</option>
          ))}
        </select>
      </div>
      <div className="config-row">
        <label>εr</label>
        <input
          type="number"
          value={active ? envLayer.permittivity : ''}
          placeholder="—"
          min={1}
          step={0.1}
          disabled={!active && currentLabel === 'None'}
          onChange={(e) => {
            const v = parseFloat(e.target.value);
            setEnvLayer({ permittivity: isNaN(v) ? 0 : v });
          }}
          className="wire-input"
          title="Relative permittivity εr of the film"
        />
      </div>
      <div className="config-row">
        <label>Thickness ({displayUnit === 'meters' ? 'mm' : displayUnit})</label>
        <input
          type="number"
          value={active ? parseFloat((envLayer.thickness * (displayUnit === 'meters' ? 1000 : factor)).toPrecision(5)) : ''}
          placeholder="—"
          min={0}
          step={0.1}
          disabled={!active && currentLabel === 'None'}
          onChange={(e) => {
            const v = parseFloat(e.target.value);
            const inMeters = isNaN(v) ? 0 : v / (displayUnit === 'meters' ? 1000 : factor);
            setEnvLayer({ thickness: inMeters });
          }}
          className="wire-input"
          title="Film thickness in display units (meters shown as mm)"
        />
      </div>
      <div className="config-row">
        <label>tan δ</label>
        <input
          type="number"
          value={active ? envLayer.lossTangent : ''}
          placeholder="—"
          min={0}
          step={0.01}
          disabled={!active && currentLabel === 'None'}
          onChange={(e) => {
            const v = parseFloat(e.target.value);
            setEnvLayer({ lossTangent: isNaN(v) ? 0 : v });
          }}
          className="wire-input"
          title="Dielectric loss tangent (0 = lossless)"
        />
      </div>
    </div>
  );
};

export default EnvironmentConfig;
