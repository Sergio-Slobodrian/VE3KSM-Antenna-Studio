/**
 * Shared type definitions and constants for Antenna Studio.
 *
 * All spatial coordinates are stored internally in meters using the physics
 * convention (Z-up). Display-unit conversion is handled at the UI layer via
 * METERS_TO_UNIT factors.
 */

/** A single wire element in the antenna geometry.
 *  Endpoints (x1,y1,z1)-(x2,y2,z2) are in meters, physics Z-up frame.
 *  `radius` is wire radius in meters; `segments` is the MoM discretisation count.
 */
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

/** Voltage source placement: which wire and segment to excite, plus voltage magnitude. */
export interface Source {
  wireIndex: number;
  segmentIndex: number;
  voltage: number;
}

/** Ground-plane configuration.
 *  'free_space' = no ground; 'perfect' = PEC ground at Z=0;
 *  'real' = lossy ground characterised by conductivity (S/m) and relative permittivity.
 */
export interface GroundConfig {
  type: 'free_space' | 'perfect' | 'real';
  conductivity: number;
  permittivity: number;
}

/** Frequency settings for single-point simulation or multi-point sweep. */
export interface FrequencyConfig {
  mode: 'single' | 'sweep';
  /** Used in 'single' mode. */
  frequencyMhz: number;
  /** Sweep start frequency in MHz. */
  freqStart: number;
  /** Sweep end frequency in MHz. */
  freqEnd: number;
  /** Number of discrete frequency steps in a sweep. */
  freqSteps: number;
}

/** A single point in the 3D far-field radiation pattern (spherical coords, dB gain). */
export interface PatternPoint {
  theta: number;
  phi: number;
  gainDb: number;
}

/** Current distribution on a single segment: magnitude (A) and phase (degrees). */
export interface CurrentEntry {
  segment: number;
  magnitude: number;
  phase: number;
}

/** Complete result from a single-frequency MoM simulation. */
export interface SimulationResult {
  /** Feed-point impedance: R (resistance) + jX (reactance) in Ohms. */
  impedance: { r: number; x: number };
  /** Standing Wave Ratio relative to 50 Ohms. */
  swr: number;
  /** Peak gain in dBi. */
  gainDbi: number;
  /** 3D far-field radiation pattern sampled on a theta/phi grid. */
  pattern: PatternPoint[];
  /** Current distribution across all wire segments. */
  currents: CurrentEntry[];
}

/** Result of a frequency-sweep simulation: arrays indexed in lockstep. */
export interface SweepResult {
  frequencies: number[];
  swr: number[];
  impedance: { r: number; x: number }[];
}

/** Supported display units for the UI; internal storage is always meters. */
export type DisplayUnit = 'meters' | 'feet' | 'inches' | 'cm' | 'mm';

/** Short labels for each display unit, used in table headers and input labels. */
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

/** A predefined antenna template served by the backend (e.g. dipole, yagi). */
export interface Template {
  name: string;
  description: string;
  parameters: TemplateParam[];
}

/** A single tuneable parameter for a template (e.g. frequency, element length). */
export interface TemplateParam {
  name: string;
  type: string;
  default: number;
}
