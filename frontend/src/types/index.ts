/**
 * Shared type definitions and constants for VE3KSM Antenna Studio.
 *
 * All spatial coordinates are stored internally in meters using the physics
 * convention (Z-up). Display-unit conversion is handled at the UI layer via
 * METERS_TO_UNIT factors.
 */

/** Conductor material name; an empty string means perfect conductor (loss-free). */
export type Material =
  | ''
  | 'copper'
  | 'aluminum'
  | 'brass'
  | 'steel'
  | 'stainless'
  | 'silver'
  | 'gold';

/** Human-friendly labels for the material dropdown. */
export const MATERIAL_LABELS: Record<Material, string> = {
  '': 'Perfect (lossless)',
  copper: 'Copper',
  aluminum: 'Aluminum',
  brass: 'Brass',
  steel: 'Steel (μr~1000)',
  stainless: 'Stainless',
  silver: 'Silver',
  gold: 'Gold',
};

/** A single wire element in the antenna geometry.
 *  Endpoints (x1,y1,z1)-(x2,y2,z2) are in meters, physics Z-up frame.
 *  `radius` is wire radius in meters; `segments` is the MoM discretisation count.
 *  `material` selects the conductor for skin-effect loss; '' = perfect conductor.
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
  material: Material;
  /** Dielectric coating (IS-card model). Zero or omitted = bare wire. */
  coatingThickness: number; // outer shell thickness (m); 0 = bare
  coatingEpsR: number;      // relative permittivity εr; ≤1 = bare
  coatingLossTan: number;   // loss tangent tanδ
}

/** Voltage source placement: which wire and segment to excite, plus voltage magnitude. */
export interface Source {
  wireIndex: number;
  segmentIndex: number;
  voltage: number;
}

/** Lumped R / L / C load attached to a single segment.
 *  series_rlc: Z = R + jωL + 1/(jωC) (omitting any zero component).
 *  parallel_rlc: Y = 1/R + 1/(jωL) + jωC, then Z = 1/Y.
 */
export interface Load {
  id: string;
  wireIndex: number;
  segmentIndex: number;
  topology: 'series_rlc' | 'parallel_rlc';
  r: number; // Ohms
  l: number; // Henries
  c: number; // Farads
}

/** One end of a transmission-line element.
 *  wireIndex >= 0: attaches to a (wire, segment) on the antenna model.
 *  wireIndex == -1: shorted termination.
 *  wireIndex == -2: open termination.
 */
export interface TLEnd {
  wireIndex: number;
  segmentIndex: number;
}

export const TLEndShorted = -1;
export const TLEndOpen = -2;

/** NEC-style 2-port transmission line.  Stubs use a real A end with B
 *  set to TLEndShorted or TLEndOpen.
 */
export interface TransmissionLine {
  id: string;
  a: TLEnd;
  b: TLEnd;
  z0: number;
  length: number; // metres
  velocityFactor: number; // 0..1
  lossDbPerM: number;
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

/** Sweep solver mode: 'auto' picks interpolated when steps > 32. */
export type SweepMode = 'auto' | 'exact' | 'interpolated';

/** Basis function order for MoM current expansion. */
export type BasisOrderType = '' | 'triangle' | 'sinusoidal' | 'quadratic';

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
  /** Sweep solver mode: auto (default), exact, or interpolated. */
  sweepMode: SweepMode;
  /** Basis function order: '' = triangle (default), 'sinusoidal', 'quadratic'. */
  basisOrder: BasisOrderType;
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

/** Complex reflection coefficient Γ = (Z - Z0)/(Z + Z0). */
export interface Reflection {
  re: number;
  im: number;
}

/** Headline far-field metrics surfaced by the backend post-processor. */
export interface FarFieldMetrics {
  peakGainDb: number;
  peakThetaDeg: number;
  peakPhiDeg: number;
  frontToBackDb: number;
  beamwidthAzDeg: number;
  beamwidthElDeg: number;
  sidelobeLevelDb: number;
  radiationEfficiency: number;
  totalRadiatedPowerW: number;
  inputPowerW: number;
}

/** 2D principal-plane cuts through the 3D pattern. */
export interface PolarCuts {
  azimuthDeg: number[];
  azimuthGainDb: number[];
  elevationDeg: number[];
  elevationGainDb: number[];
  fixedElevationDeg: number;
  fixedAzimuthDeg: number;
}

/** Polarisation state at a single far-field observation direction. */
export interface PolarizationPoint {
  theta: number;
  phi: number;
  axialRatioDb: number;
  tiltDeg: number;
  polType: 'linear' | 'circular' | 'elliptical';
  sense: 'RHCP' | 'LHCP' | '';
}

/** Aggregate polarisation metrics + principal-plane AR cuts. */
export interface PolarizationMetrics {
  peakAxialRatioDb: number;
  peakTiltDeg: number;
  peakPolType: 'linear' | 'circular' | 'elliptical';
  peakSense: 'RHCP' | 'LHCP' | '';
  azimuthDeg: number[];
  azimuthArDb: number[];
  azimuthTiltDeg: number[];
  elevationDeg: number[];
  elevationArDb: number[];
  elevationTiltDeg: number[];
  points: PolarizationPoint[];
}

/** Non-blocking accuracy warnings from the MoM segmentation validator. */
export interface Warning {
  code: string;
  severity: 'info' | 'warn' | 'error';
  message: string;
  wireIndex?: number;
  segmentIndex?: number;
}

/** Complete result from a single-frequency MoM simulation. */
export interface SimulationResult {
  /** Feed-point impedance: R (resistance) + jX (reactance) in Ohms. */
  impedance: { r: number; x: number };
  /** Standing Wave Ratio at the user-supplied reference impedance. */
  swr: number;
  /** Complex reflection coefficient at Z0 (Smith-chart input). */
  reflection: Reflection;
  /** Reference impedance used for SWR / reflection (Ohms). */
  referenceImpedance: number;
  /** Peak gain in dBi (alias for metrics.peakGainDb, kept for back-compat). */
  gainDbi: number;
  /** Headline far-field metrics. */
  metrics: FarFieldMetrics;
  /** Azimuth + elevation 2D cuts for polar plotting. */
  polarCuts: PolarCuts;
  /** Polarisation analysis: axial ratio, tilt, sense per direction. */
  polarization: PolarizationMetrics;
  /** 3D far-field radiation pattern sampled on a theta/phi grid. */
  pattern: PatternPoint[];
  /** Current distribution across all wire segments. */
  currents: CurrentEntry[];
  /** Non-blocking accuracy warnings from the MoM validator. */
  warnings: Warning[];
}

/** Result of a frequency-sweep simulation: arrays indexed in lockstep. */
export interface SweepResult {
  frequencies: number[];
  swr: number[];
  impedance: { r: number; x: number }[];
  reflections: Reflection[];
  referenceImpedance: number;
  /** Non-blocking accuracy warnings for the sweep range (validated at start + end frequency). */
  warnings: Warning[];
}

/** A single observation point in a near-field grid. */
export interface NearFieldPoint {
  x: number;
  y: number;
  z: number;
  e_mag: number;
  h_mag: number;
  e_mag_db: number;
  h_mag_db: number;
}

/** Result of a near-field E/H computation on a 2D observation grid. */
export interface NearFieldResult {
  points: NearFieldPoint[];
  plane: string;
  axis1_label: string;
  axis2_label: string;
  axis1_vals: number[];
  axis2_vals: number[];
  steps1: number;
  steps2: number;
  e_max_db: number;
  e_min_db: number;
  h_max_db: number;
  h_min_db: number;
}

/** A single characteristic mode from CMA eigendecomposition. */
export interface CMAMode {
  index: number;
  eigenvalue: number;
  modal_significance: number;
  characteristic_angle: number;
  current_magnitudes: number[];
}

/** Full CMA result at one frequency. */
export interface CMAResult {
  modes: CMAMode[];
  num_modes: number;
  freq_mhz: number;
}

/** One tuneable variable for the PSO optimizer. */
export interface OptimVariable {
  name: string;
  wire_index: number;
  field: string; // x1,y1,z1,x2,y2,z2,radius
  min: number;
  max: number;
}

/** One term of the composite objective function. */
export interface OptimGoal {
  metric: string; // swr, gain, front_to_back, impedance_r, impedance_x, efficiency
  target: number;
  weight: number;
}

/** Result of a PSO optimisation run. */
export interface OptimResult {
  best_params: Record<string, number>;
  best_cost: number;
  best_metrics: Record<string, number>;
  convergence: number[];
  iterations: number;
  optimized_wires: {
    X1: number; Y1: number; Z1: number;
    X2: number; Y2: number; Z2: number;
    Radius: number; Segments: number;
    Material: string;
  }[];
}

/** A single sample in a time-domain transient waveform. */
export interface TransientPoint {
  time_ns: number;
  amplitude: number;
}

/** Result of a time-domain transient analysis via IFFT. */
export interface TransientResult {
  waveform: TransientPoint[];
  excitation: TransientPoint[];
  frequencies: number[];
  mag_response: number[];
  phase_response: number[];
  peak_amplitude: number;
  peak_time_ns: number;
  ringdown_time_ns: number;
  pulse_fwhm_ns: number;
  response_type: string;
}

/** One objective for Pareto multi-objective optimization. */
export interface ParetoObjective {
  metric: string;
  direction: 'minimize' | 'maximize';
}

/** One non-dominated design on the Pareto front. */
export interface ParetoSolution {
  params: Record<string, number>;
  metrics: Record<string, number>;
  rank: number;
  wires: {
    X1: number; Y1: number; Z1: number;
    X2: number; Y2: number; Z2: number;
    Radius: number; Segments: number;
    Material: string;
  }[];
}

/** Result of a Pareto (NSGA-II) optimization run. */
export interface ParetoResult {
  front: ParetoSolution[];
  all_fronts: ParetoSolution[];
  generations: number;
  objectives: string[];
}

/** A standard wire-coating preset with typical dielectric properties. */
export interface CoatingPreset {
  key: string;
  label: string;
  thickness: number; // meters
  epsR: number;
  lossTan: number;
}

/** Standard coating presets keyed by material. Thickness is a typical value;
 *  users can edit the fields after applying a preset. */
export const COATING_PRESETS: CoatingPreset[] = [
  { key: 'bare',   label: 'Bare wire',       thickness: 0,       epsR: 1.0,  lossTan: 0      },
  { key: 'pvc',    label: 'PVC',             thickness: 0.0015,  epsR: 3.8,  lossTan: 0.05   },
  { key: 'pe',     label: 'PE',              thickness: 0.002,   epsR: 2.3,  lossTan: 0.0002  },
  { key: 'ptfe',   label: 'PTFE (Teflon)',   thickness: 0.001,   epsR: 2.1,  lossTan: 0.0002  },
  { key: 'fep',    label: 'FEP',             thickness: 0.001,   epsR: 2.1,  lossTan: 0.0003  },
  { key: 'xlpe',   label: 'XLPE',            thickness: 0.002,   epsR: 2.3,  lossTan: 0.0003  },
  { key: 'nylon',  label: 'Nylon (PA)',       thickness: 0.001,   epsR: 3.5,  lossTan: 0.04   },
  { key: 'rubber', label: 'Rubber (EPDM)',    thickness: 0.002,   epsR: 3.0,  lossTan: 0.02   },
  { key: 'enamel', label: 'Enamel/varnish',   thickness: 0.00008, epsR: 3.5,  lossTan: 0.04   },
  { key: 'ice',    label: 'Ice (weather)',    thickness: 0.001,   epsR: 3.17, lossTan: 0.002   },
  { key: 'water',  label: 'Water film (wet)', thickness: 0.0001,  epsR: 80.0, lossTan: 0.2    },
];

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

/** Result of a convergence check: 1x vs 2x segmentation comparison. */
export interface ConvergenceResult {
  impedance_r_1x: number;
  impedance_x_1x: number;
  swr_1x: number;
  gain_dbi_1x: number;
  total_segments_1x: number;
  impedance_r_2x: number;
  impedance_x_2x: number;
  swr_2x: number;
  gain_dbi_2x: number;
  total_segments_2x: number;
  delta_r_pct: number;
  delta_x_pct: number;
  delta_z_mag_pct: number;
  delta_swr_pct: number;
  delta_gain_db: number;
  converged: boolean;
  verdict: string;
}
