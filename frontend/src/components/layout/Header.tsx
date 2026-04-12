/**
 * Header toolbar.
 *
 * Contains the app title, template selector dropdown, and the Simulate / Sweep
 * action buttons.  Orchestrates the full request lifecycle: validation, API
 * call, result storage, and error handling.
 */
import React from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import {
  simulate,
  sweep,
  buildSimulateRequest,
  buildSweepRequest,
} from '@/api/client';
import { validateFrequency } from '@/utils/validation';
import TemplateSelector from '@/components/input/TemplateSelector';

const Header: React.FC = () => {
  const {
    wires,
    source,
    ground,
    frequency,
    isSimulating,
    setSimulationResult,
    setSweepResult,
    setSimulating,
    setError,
  } = useAntennaStore();

  /** Run a single-frequency simulation after validating inputs. */
  const handleSimulate = async () => {
    const freqError = validateFrequency(frequency);
    if (freqError) {
      setError(freqError);
      return;
    }
    if (wires.length === 0) {
      setError('No wires defined');
      return;
    }

    setSimulating(true);
    setError(null);
    try {
      const request = buildSimulateRequest(wires, source, ground, frequency);
      const result = await simulate(request);
      setSimulationResult(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Simulation failed');
    } finally {
      setSimulating(false);
    }
  };

  /** Run a frequency sweep; forces sweep-mode validation regardless of UI mode. */
  const handleSweep = async () => {
    const freqError = validateFrequency({ ...frequency, mode: 'sweep' });
    if (freqError) {
      setError(freqError);
      return;
    }
    if (wires.length === 0) {
      setError('No wires defined');
      return;
    }

    setSimulating(true);
    setError(null);
    try {
      const request = buildSweepRequest(wires, source, ground, frequency);
      const result = await sweep(request);
      setSweepResult(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Sweep failed');
    } finally {
      setSimulating(false);
    }
  };

  return (
    <header className="header">
      <div className="header-title">Antenna Studio</div>
      <div className="header-actions">
        <TemplateSelector />
        <button
          className="btn btn-primary"
          onClick={handleSimulate}
          disabled={isSimulating}
        >
          {isSimulating ? (
            <>
              <span className="spinner" /> Simulating...
            </>
          ) : (
            'Simulate'
          )}
        </button>
        <button
          className="btn btn-secondary"
          onClick={handleSweep}
          disabled={isSimulating}
        >
          {isSimulating ? (
            <>
              <span className="spinner" /> Sweeping...
            </>
          ) : (
            'Sweep'
          )}
        </button>
      </div>
    </header>
  );
};

export default Header;
