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
  loads?: Omit<import('@/types').Load, 'id'>[];
  transmissionLines?: Omit<import('@/types').TransmissionLine, 'id'>[];
  referenceImpedance?: number;
  ground: GroundConfig;
  frequency: FrequencyConfig;
}

const Header: React.FC = () => {
  const {
    wires,
    source,
    loads,
    transmissionLines,
    ground,
    frequency,
    referenceImpedance,
    weather,
    isSimulating,
    setSimulationResult,
    setSweepResult,
    setSimulating,
    setError,
  } = useAntennaStore();

  const fileInputRef = useRef<HTMLInputElement>(null);
  const necInputRef = useRef<HTMLInputElement>(null);

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
      const request = buildSimulateRequest(wires, source, loads, transmissionLines, ground, frequency, referenceImpedance, weather);
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
      const request = buildSweepRequest(wires, source, loads, transmissionLines, ground, frequency, referenceImpedance, weather);
      const result = await sweep(request);
      setSweepResult(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Sweep failed');
    } finally {
      setSimulating(false);
    }
  };

  /** Export the current design as a NEC-2 .nec file via the backend. */
  const handleNECExport = async () => {
    try {
      const body = {
        wires: wires.map(({ id, ...rest }) => rest),
        source,
        loads: loads.map(({ id, ...rest }) => ({
          wire_index: rest.wireIndex,
          segment_index: rest.segmentIndex,
          topology: rest.topology,
          r: rest.r,
          l: rest.l,
          c: rest.c,
        })),
        transmission_lines: transmissionLines.map(({ id, ...rest }) => ({
          a: { wire_index: rest.a.wireIndex, segment_index: rest.a.segmentIndex },
          b: { wire_index: rest.b.wireIndex, segment_index: rest.b.segmentIndex },
          z0: rest.z0,
          length: rest.length,
          velocity_factor: rest.velocityFactor,
          loss_db_per_m: rest.lossDbPerM,
        })),
        ground,
        freq_start: frequency.freqStart,
        freq_end: frequency.freqEnd,
        freq_steps: frequency.freqSteps,
        reference_impedance: referenceImpedance,
      };
      const resp = await fetch('/api/nec2/export', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (!resp.ok) throw new Error(`HTTP ${resp.status}: ${await resp.text()}`);
      const text = await resp.text();
      const blob = new Blob([text], { type: 'text/plain' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'antenna.nec';
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'NEC export failed');
    }
  };

  /** Import a NEC-2 .nec file via the backend; replace the current model. */
  const handleNECImport = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = async () => {
      try {
        const resp = await fetch('/api/nec2/import', {
          method: 'POST',
          headers: { 'Content-Type': 'text/plain' },
          body: reader.result as string,
        });
        if (!resp.ok) throw new Error(`HTTP ${resp.status}: ${await resp.text()}`);
        const data = await resp.json() as {
          wires: Array<Record<string, unknown>>;
          loads: Array<Record<string, unknown>>;
          transmission_lines: Array<Record<string, unknown>>;
          source: Record<string, unknown>;
          ground: Record<string, unknown>;
          frequency: Record<string, number>;
        };
        const loadedWires: Wire[] = data.wires.map((w) => ({
          id: uuidv4(),
          x1: w.x1 as number, y1: w.y1 as number, z1: w.z1 as number,
          x2: w.x2 as number, y2: w.y2 as number, z2: w.z2 as number,
          radius: w.radius as number,
          segments: w.segments as number,
          material: ((w.material as string) || '') as Wire['material'],
        }));
        useAntennaStore.getState().loadTemplate({
          wires: loadedWires,
          source: {
            wireIndex: (data.source.wire_index as number) ?? 0,
            segmentIndex: (data.source.segment_index as number) ?? 0,
            voltage: (data.source.voltage as number) ?? 1,
          },
          ground: {
            type: ((data.ground.type as string) || 'free_space') as 'free_space' | 'perfect' | 'real',
            conductivity: (data.ground.conductivity as number) || 0.005,
            permittivity: (data.ground.permittivity as number) || 13,
          },
        });
        // Restore loads
        for (const ld of data.loads ?? []) {
          useAntennaStore.getState().addLoad({
            wireIndex: ld.wire_index as number,
            segmentIndex: ld.segment_index as number,
            topology: ((ld.topology as string) || 'series_rlc') as 'series_rlc' | 'parallel_rlc',
            r: (ld.r as number) || 0,
            l: (ld.l as number) || 0,
            c: (ld.c as number) || 0,
          });
        }
        // Restore TLs
        for (const tl of data.transmission_lines ?? []) {
          useAntennaStore.getState().addTransmissionLine({
            a: {
              wireIndex: ((tl.a as Record<string, number>).wire_index) ?? 0,
              segmentIndex: ((tl.a as Record<string, number>).segment_index) ?? 0,
            },
            b: {
              wireIndex: ((tl.b as Record<string, number>).wire_index) ?? -1,
              segmentIndex: ((tl.b as Record<string, number>).segment_index) ?? 0,
            },
            z0: (tl.z0 as number) || 50,
            length: (tl.length as number) || 0,
            velocityFactor: (tl.velocity_factor as number) || 1,
            lossDbPerM: (tl.loss_db_per_m as number) || 0,
          });
        }
        // Restore frequency
        if (data.frequency) {
          const fStart = (data.frequency.freq_start_mhz as number) || 0;
          const fEnd = (data.frequency.freq_end_mhz as number) || 0;
          const fSteps = (data.frequency.freq_steps as number) || 0;
          if (fSteps > 1) {
            useAntennaStore.getState().setFrequency({
              mode: 'sweep',
              freqStart: fStart, freqEnd: fEnd, freqSteps: fSteps,
              frequencyMhz: fStart,
            });
          } else if (data.frequency.frequency_mhz) {
            useAntennaStore.getState().setFrequency({
              mode: 'single',
              frequencyMhz: data.frequency.frequency_mhz as number,
            });
          }
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : 'NEC import failed');
      }
    };
    reader.readAsText(file);
    e.target.value = '';
  };

  /** Save the current design (wires, source, ground, frequency) to a JSON file. */
  const handleSave = () => {
    const design: DesignFile = {
      version: 1,
      wires: wires.map(({ id, ...rest }) => rest),
      source,
      loads: loads.map(({ id, ...rest }) => rest),
      transmissionLines: transmissionLines.map(({ id, ...rest }) => rest),
      referenceImpedance,
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
          material: ((w as Partial<Wire>).material ?? '') as Wire['material'],
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
        if (typeof design.referenceImpedance === 'number') {
          useAntennaStore.getState().setReferenceImpedance(design.referenceImpedance);
        }
        if (Array.isArray(design.loads)) {
          // loadTemplate above wiped existing loads; re-add from file.
          design.loads.forEach((l) => useAntennaStore.getState().addLoad(l));
        }
        if (Array.isArray(design.transmissionLines)) {
          design.transmissionLines.forEach((t) =>
            useAntennaStore.getState().addTransmissionLine(t),
          );
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
      <div className="header-title">VE3KSM Antenna Studio</div>
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
        <button
          className="btn btn-outline"
          onClick={handleNECExport}
          title="Export NEC-2 .nec deck"
        >
          .nec ⬇
        </button>
        <button
          className="btn btn-outline"
          onClick={() => necInputRef.current?.click()}
          title="Import NEC-2 .nec deck"
        >
          .nec ⬆
        </button>
        <input
          ref={necInputRef}
          type="file"
          accept=".nec,.nec2,.txt"
          style={{ display: 'none' }}
          onChange={handleNECImport}
        />
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
