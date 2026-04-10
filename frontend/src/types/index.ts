export interface Wire {
  id: string;
  x1: number;
  y1: number;
  z1: number;
  x2: number;
  y2: number;
  z2: number;
  radius: number;
  segments: number;
}

export interface Source {
  wireIndex: number;
  segmentIndex: number;
  voltage: number;
}

export interface GroundConfig {
  type: 'free_space' | 'perfect' | 'real';
  conductivity: number;
  permittivity: number;
}

export interface FrequencyConfig {
  mode: 'single' | 'sweep';
  frequencyMhz: number;
  freqStart: number;
  freqEnd: number;
  freqSteps: number;
}

export interface PatternPoint {
  theta: number;
  phi: number;
  gainDb: number;
}

export interface CurrentEntry {
  segment: number;
  magnitude: number;
  phase: number;
}

export interface SimulationResult {
  impedance: { r: number; x: number };
  swr: number;
  gainDbi: number;
  pattern: PatternPoint[];
  currents: CurrentEntry[];
}

export interface SweepResult {
  frequencies: number[];
  swr: number[];
  impedance: { r: number; x: number }[];
}

export interface Template {
  name: string;
  description: string;
  parameters: TemplateParam[];
}

export interface TemplateParam {
  name: string;
  type: string;
  default: number;
}
