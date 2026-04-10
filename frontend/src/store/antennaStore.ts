import { create } from 'zustand';
import { v4 as uuidv4 } from 'uuid';
import type {
  Wire,
  Source,
  GroundConfig,
  FrequencyConfig,
  SimulationResult,
  SweepResult,
} from '@/types';

interface AntennaState {
  wires: Wire[];
  source: Source;
  ground: GroundConfig;
  frequency: FrequencyConfig;
  simulationResult: SimulationResult | null;
  sweepResult: SweepResult | null;
  selectedWireId: string | null;
  isSimulating: boolean;
  error: string | null;

  addWire: (wire?: Partial<Wire>) => void;
  updateWire: (id: string, updates: Partial<Wire>) => void;
  removeWire: (id: string) => void;
  setSource: (source: Partial<Source>) => void;
  setGround: (ground: Partial<GroundConfig>) => void;
  setFrequency: (freq: Partial<FrequencyConfig>) => void;
  selectWire: (id: string | null) => void;
  loadTemplate: (data: { wires: Wire[]; source: Source; ground: GroundConfig }) => void;
  setSimulationResult: (result: SimulationResult | null) => void;
  setSweepResult: (result: SweepResult | null) => void;
  setSimulating: (value: boolean) => void;
  setError: (error: string | null) => void;
}

const defaultWireId = uuidv4();

// Half-wave dipole for 14 MHz: wavelength = 300/14 ~= 21.43m, half-wave ~= 10.71m
// Vertical along Z axis, centered at origin
const DEFAULT_DIPOLE_LENGTH = 10.71;

export const useAntennaStore = create<AntennaState>((set) => ({
  wires: [
    {
      id: defaultWireId,
      x1: 0,
      y1: 0,
      z1: -DEFAULT_DIPOLE_LENGTH / 2,
      x2: 0,
      y2: 0,
      z2: DEFAULT_DIPOLE_LENGTH / 2,
      radius: 0.001,
      segments: 21,
    },
  ],
  source: {
    wireIndex: 0,
    segmentIndex: 10,
    voltage: 1.0,
  },
  ground: {
    type: 'free_space',
    conductivity: 0.005,
    permittivity: 13,
  },
  frequency: {
    mode: 'single',
    frequencyMhz: 14.0,
    freqStart: 13.0,
    freqEnd: 15.0,
    freqSteps: 50,
  },
  simulationResult: null,
  sweepResult: null,
  selectedWireId: defaultWireId,
  isSimulating: false,
  error: null,

  addWire: (wire) =>
    set((state) => ({
      wires: [
        ...state.wires,
        {
          id: uuidv4(),
          x1: 0,
          y1: 0,
          z1: 0,
          x2: 1,
          y2: 0,
          z2: 0,
          radius: 0.001,
          segments: 11,
          ...wire,
        },
      ],
    })),

  updateWire: (id, updates) =>
    set((state) => ({
      wires: state.wires.map((w) => (w.id === id ? { ...w, ...updates } : w)),
    })),

  removeWire: (id) =>
    set((state) => ({
      wires: state.wires.filter((w) => w.id !== id),
      selectedWireId: state.selectedWireId === id ? null : state.selectedWireId,
    })),

  setSource: (source) =>
    set((state) => ({ source: { ...state.source, ...source } })),

  setGround: (ground) =>
    set((state) => ({ ground: { ...state.ground, ...ground } })),

  setFrequency: (freq) =>
    set((state) => ({ frequency: { ...state.frequency, ...freq } })),

  selectWire: (id) => set({ selectedWireId: id }),

  loadTemplate: (data) =>
    set({
      wires: data.wires,
      source: data.source,
      ground: data.ground,
      selectedWireId: data.wires.length > 0 ? data.wires[0].id : null,
      simulationResult: null,
      sweepResult: null,
      error: null,
    }),

  setSimulationResult: (result) => set({ simulationResult: result }),
  setSweepResult: (result) => set({ sweepResult: result }),
  setSimulating: (value) => set({ isSimulating: value }),
  setError: (error) => set({ error }),
}));
