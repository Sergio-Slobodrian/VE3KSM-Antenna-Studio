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

export type DisplayUnit = 'meters' | 'feet' | 'inches' | 'cm' | 'mm';

export const UNIT_LABELS: Record<DisplayUnit, string> = {
  meters: 'm',
  feet: 'ft',
  inches: 'in',
  cm: 'cm',
  mm: 'mm',
};

/** Multiply meters by this factor to get the display unit. */
export const METERS_TO_UNIT: Record<DisplayUnit, number> = {
  meters: 1,
  feet: 3.28084,
  inches: 39.3701,
  cm: 100,
  mm: 1000,
};

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
