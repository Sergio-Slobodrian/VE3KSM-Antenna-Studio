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
        </>
      )}
    </div>
  );
};

export default FrequencyInput;
