// Copyright 2026 Sergio Slobodrian
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import React from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import { WEATHER_PRESETS, METERS_TO_UNIT } from '@/types';

const WeatherPanel: React.FC = () => {
  const { weather, setWeather, displayUnit } = useAntennaStore();
  const factor = METERS_TO_UNIT[displayUnit];
  const dry = weather.preset === 'dry';
  const isActive = !dry && weather.thickness > 0;

  const handlePreset = (key: string) => {
    const preset = WEATHER_PRESETS.find((p) => p.key === key);
    if (!preset) return;
    setWeather({
      preset: preset.key,
      thickness: preset.defaultThicknessMm / 1000,
      epsR: preset.epsR,
      lossTan: preset.lossTan,
    });
  };

  const inactive = (extra?: string) =>
    dry ? `${extra ?? ''} wire-input-inactive`.trim() : extra ?? '';

  return (
    <div className="config-section">
      <h3 style={isActive ? { color: 'var(--accent)' } : undefined}>
        Environment{isActive ? ` — ${WEATHER_PRESETS.find((p) => p.key === weather.preset)?.label ?? ''}` : ''}
      </h3>

      <div className="config-row">
        <label>Weather</label>
        <select value={weather.preset} onChange={(e) => handlePreset(e.target.value)}>
          {WEATHER_PRESETS.map((p) => (
            <option key={p.key} value={p.key}>{p.label}</option>
          ))}
        </select>
      </div>

      <div className="config-row">
        <label>Film ({displayUnit})</label>
        <input
          type="number"
          value={parseFloat((weather.thickness * factor).toPrecision(4))}
          onChange={(e) => {
            const n = parseFloat(e.target.value);
            if (!isNaN(n)) setWeather({ thickness: n / factor });
          }}
          min={0}
          step={0.0001}
          disabled={dry}
          className={inactive()}
          title="Weather film thickness"
        />
      </div>

      <div className="config-row">
        <label>εr</label>
        <input
          type="number"
          value={weather.epsR}
          onChange={(e) => {
            const n = parseFloat(e.target.value);
            if (!isNaN(n)) setWeather({ epsR: n });
          }}
          min={1}
          step={0.1}
          disabled={dry}
          className={inactive()}
          title="Relative permittivity of weather film"
        />
      </div>

      <div className="config-row">
        <label>tanδ</label>
        <input
          type="number"
          value={weather.lossTan}
          onChange={(e) => {
            const n = parseFloat(e.target.value);
            if (!isNaN(n)) setWeather({ lossTan: n });
          }}
          min={0}
          step={0.001}
          disabled={dry}
          className={inactive()}
          title="Loss tangent of weather film"
        />
      </div>
    </div>
  );
};

export default WeatherPanel;
