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
import React, { useState } from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import NumericInput from '@/components/common/NumericInput';
import RegionMapPicker from '@/components/input/RegionMapPicker';
import {
  SOIL_MOISTURE_PRESETS,
  ITU_P832_ZONES,
  type GroundConfig as GroundConfigType,
  type SoilMoisturePreset,
} from '@/types';

const GroundConfig: React.FC = () => {
  const { ground, setGround } = useAntennaStore();
  const [mapOpen, setMapOpen] = useState(false);

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
  const moistureActive = isRealGround && ground.moisturePreset !== 'custom';
  const regionActive = isRealGround && !!ground.regionPreset;
  const headerActive = moistureActive || regionActive;

  // Look up the human-readable label for whatever region preset is set.
  const regionLabel = (() => {
    const rp = ground.regionPreset;
    if (!rp) return '';
    if (rp.startsWith('itu:')) {
      const zone = Number(rp.slice(4));
      return ITU_P832_ZONES.find((z) => z.zone === zone)?.label ?? `ITU zone ${zone}`;
    }
    // user:<uuid> — we don't have the friendly name cached here; show a
    // neutral label so the user can tell that *something* is applied.
    return 'User region';
  })();

  const moistureLabel = moistureActive
    ? SOIL_MOISTURE_PRESETS.find((p) => p.key === ground.moisturePreset)?.label ?? ''
    : '';

  const headerSuffix = [regionLabel, moistureLabel].filter(Boolean).join(' · ');

  return (
    <div className="config-section">
      <h3 style={headerActive ? { color: 'var(--accent)' } : undefined}>
        Ground{headerSuffix ? ` — ${headerSuffix}` : ''}
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
            <label>Region</label>
            <button
              type="button"
              className="ground-map-btn"
              onClick={() => setMapOpen(true)}
              title="Open the interactive world-map picker"
            >
              {regionActive ? `${regionLabel} — change…` : 'Pick on map…'}
            </button>
            {regionActive && (
              <button
                type="button"
                className="ground-map-btn"
                onClick={() => setGround({ regionPreset: '' })}
                title="Clear the region label (leaves εr/σ untouched)"
              >
                Clear
              </button>
            )}
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
      <RegionMapPicker
        open={mapOpen}
        onClose={() => setMapOpen(false)}
        onApply={({ epsR, sigma, regionPreset }) => {
          // Picking a region sets εr/σ directly; moisturePreset collapses to
          // 'custom' so the header only shows the region label (avoids
          // confusing double-labeling).
          setGround({
            type: 'real',
            permittivity: epsR,
            conductivity: sigma,
            regionPreset,
            moisturePreset: 'custom',
          });
        }}
      />
    </div>
  );
};

export default GroundConfig;
