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

/**
 * Frequency configuration panel.
 *
 * Supports two modes via a toggle:
 *  - Single: one frequency in MHz for a point simulation.
 *  - Sweep: start/end frequencies and step count for a frequency sweep.
 * The step count is rounded to an integer on change.
 */
import React from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import NumericInput from '@/components/common/NumericInput';

const FrequencyInput: React.FC = () => {
  const { frequency, setFrequency } = useAntennaStore();

  return (
    <div className="config-section">
      <h3>Frequency</h3>
      <div className="config-row">
        <label>Mode</label>
        <div className="toggle-group">
          <button
            className={`toggle-btn ${frequency.mode === 'single' ? 'active' : ''}`}
            onClick={() => setFrequency({ mode: 'single' })}
          >
            Single
          </button>
          <button
            className={`toggle-btn ${frequency.mode === 'sweep' ? 'active' : ''}`}
            onClick={() => setFrequency({ mode: 'sweep' })}
          >
            Sweep
          </button>
        </div>
      </div>
      {frequency.mode === 'single' ? (
        <div className="config-row">
          <NumericInput
            label="Frequency (MHz)"
            value={frequency.frequencyMhz}
            onChange={(v) => setFrequency({ frequencyMhz: v })}
            min={0.01}
            step={0.1}
          />
        </div>
      ) : (
        <>
          <div className="config-row">
            <NumericInput
              label="Start (MHz)"
              value={frequency.freqStart}
              onChange={(v) => setFrequency({ freqStart: v })}
              min={0.01}
              step={0.1}
            />
          </div>
          <div className="config-row">
            <NumericInput
              label="End (MHz)"
              value={frequency.freqEnd}
              onChange={(v) => setFrequency({ freqEnd: v })}
              min={0.01}
              step={0.1}
            />
          </div>
          <div className="config-row">
            <NumericInput
              label="Steps"
              value={frequency.freqSteps}
              onChange={(v) => setFrequency({ freqSteps: Math.round(v) })}
              min={2}
              max={1000}
              step={1}
            />
          </div>
          <div className="config-row">
            <label>Sweep mode</label>
            <select
              value={frequency.sweepMode || 'auto'}
              onChange={(e) =>
                setFrequency({ sweepMode: e.target.value as 'auto' | 'exact' | 'interpolated' })
              }
              style={{ flex: 1 }}
            >
              <option value="auto">Auto</option>
              <option value="exact">Exact (full solve every point)</option>
              <option value="interpolated">Interpolated (PCHIP)</option>
            </select>
          </div>
        </>
      )}
      <div className="config-row">
        <label>Basis functions</label>
        <select
          value={frequency.basisOrder || ''}
          onChange={(e) =>
            setFrequency({ basisOrder: e.target.value as '' | 'triangle' | 'sinusoidal' | 'quadratic' })
          }
          style={{ flex: 1 }}
        >
          <option value="">Triangle (default)</option>
          <option value="sinusoidal">Sinusoidal (King-type)</option>
          <option value="quadratic">Quadratic (Hermite)</option>
        </select>
      </div>
    </div>
  );
};

export default FrequencyInput;
