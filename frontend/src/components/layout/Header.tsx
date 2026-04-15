/**
 * Header toolbar.
 *
 * Contains the app title, template selector, Save/Load design buttons,
 * and the Simulate / Sweep action buttons.
 */
import React, { useRef } from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import {
  simulate,
  sweep,
  buildSimulateRequest,
  buildSweepRequest,
} from '@/api/client';
import { validateFrequency } from '@/utils/validation';
import TemplateSelector from '@/components/input/TemplateSelector';
import type { Wire, Source, GroundConfig, FrequencyConfig } from '@/types';
import { v4 as uuidv4 } from 'uuid';

/** Shape of the saved design JSON file. */
interface DesignFile {
  version: 1;
  wires: Omit<Wire, 'id'>[];
  source: Source;
  ground: GroundConfig;
  frequency: FrequencyConfig;
}

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

  const fileInputRef = useRef<HTMLInputElement>(null);

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

  /** Run a frequency sweep. */
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

  /** Save the current design (wires, source, ground, frequency) to a JSON file. */
  const handleSave = () => {
    const design: DesignFile = {
      version: 1,
      wires: wires.map(({ id, ...rest }) => rest),
      source,
      ground,
      frequency,
    };

    const blob = new Blob([JSON.stringify(design, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'antenna-design.json';
    a.click();
    URL.revokeObjectURL(url);
  };

  /** Load a design from a JSON file, replacing all current inputs. */
  const handleLoad = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = () => {
      try {
        const design = JSON.parse(reader.result as string) as DesignFile;

        if (!design.wires || !Array.isArray(design.wires) || design.wires.length === 0) {
          setError('Invalid design file: no wires found');
          return;
        }

        // Assign fresh IDs to loaded wires
        const loadedWires: Wire[] = design.wires.map((w) => ({
          ...w,
          id: uuidv4(),
        }));

        useAntennaStore.getState().loadTemplate({
          wires: loadedWires,
          source: design.source || { wireIndex: 0, segmentIndex: 0, voltage: 1.0 },
          ground: design.ground || { type: 'free_space', conductivity: 0.005, permittivity: 13 },
        });

        // Restore frequency settings if present
        if (design.frequency) {
          useAntennaStore.getState().setFrequency(design.frequency);
        }
      } catch {
        setError('Failed to parse design file');
      }
    };
    reader.readAsText(file);

    // Reset input so the same file can be re-loaded
    e.target.value = '';
  };

  return (
    <header className="header">
      <div className="header-title">Antenna Studio</div>
      <div className="header-actions">
        <TemplateSelector />
        <button className="btn btn-outline" onClick={handleSave} title="Save design to file">
          Save
        </button>
        <button
          className="btn btn-outline"
          onClick={() => fileInputRef.current?.click()}
          title="Load design from file"
        >
          Load
        </button>
        <input
          ref={fileInputRef}
          type="file"
          accept=".json"
          style={{ display: 'none' }}
          onChange={handleLoad}
        />
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
