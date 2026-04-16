/**
 * Backend API client.
 *
 * Translates between the frontend's camelCase TypeScript types and the
 * backend's snake_case JSON wire format.  All HTTP communication with the
 * Go MoM solver is consolidated here.
 */
import type {
  Wire,
  Source,
  Load,
  TransmissionLine,
  GroundConfig,
  FrequencyConfig,
  SimulationResult,
  SweepResult,
  Template,
} from '@/types';

const API_BASE: string = import.meta.env.VITE_API_BASE || '';

/** POST body for /api/simulate (snake_case keys matching Go backend). */
interface SimulateRequest {
  wires: ReturnType<typeof buildWires>;
  source: ReturnType<typeof buildSource>;
  loads: ReturnType<typeof buildLoads>;
  transmission_lines: ReturnType<typeof buildTLs>;
  ground: ReturnType<typeof buildGround>;
  frequency_mhz: number;
  reference_impedance: number;
}

/** POST body for /api/sweep (snake_case keys matching Go backend). */
interface SweepRequest {
  wires: ReturnType<typeof buildWires>;
  source: ReturnType<typeof buildSource>;
  loads: ReturnType<typeof buildLoads>;
  transmission_lines: ReturnType<typeof buildTLs>;
  ground: ReturnType<typeof buildGround>;
  freq_start: number;
  freq_end: number;
  freq_steps: number;
  reference_impedance: number;
}

// --- Request builders: strip client-only fields (id) and remap key casing ---

/** Strip the client-side `id` field from wires and forward material. */
function buildWires(wires: Wire[]) {
  return wires.map((w) => ({
    x1: w.x1, y1: w.y1, z1: w.z1,
    x2: w.x2, y2: w.y2, z2: w.z2,
    radius: w.radius, segments: w.segments,
    material: w.material || undefined,
  }));
}

/** Convert camelCase Source to snake_case for the backend. */
function buildSource(source: Source) {
  return {
    wire_index: source.wireIndex,
    segment_index: source.segmentIndex,
    voltage: source.voltage,
  };
}

function buildGround(ground: GroundConfig) {
  return {
    type: ground.type,
    conductivity: ground.conductivity,
    permittivity: ground.permittivity,
  };
}

/** Strip client-side `id` field from loads and forward to the backend. */
function buildLoads(loads: Load[]) {
  return loads.map((l) => ({
    wire_index: l.wireIndex,
    segment_index: l.segmentIndex,
    topology: l.topology,
    r: l.r,
    l: l.l,
    c: l.c,
  }));
}

/** Strip client-side `id` field from TLs and remap to snake_case. */
function buildTLs(tls: TransmissionLine[]) {
  return tls.map((t) => ({
    a: { wire_index: t.a.wireIndex, segment_index: t.a.segmentIndex },
    b: { wire_index: t.b.wireIndex, segment_index: t.b.segmentIndex },
    z0: t.z0,
    length: t.length,
    velocity_factor: t.velocityFactor,
    loss_db_per_m: t.lossDbPerM,
  }));
}

/** Assemble a complete single-frequency simulation request. */
export function buildSimulateRequest(
  wires: Wire[],
  source: Source,
  loads: Load[],
  transmissionLines: TransmissionLine[],
  ground: GroundConfig,
  frequency: FrequencyConfig,
  referenceImpedance: number
): SimulateRequest {
  return {
    wires: buildWires(wires),
    source: buildSource(source),
    loads: buildLoads(loads),
    transmission_lines: buildTLs(transmissionLines),
    ground: buildGround(ground),
    frequency_mhz: frequency.frequencyMhz,
    reference_impedance: referenceImpedance,
  };
}

/** Assemble a frequency-sweep request. */
export function buildSweepRequest(
  wires: Wire[],
  source: Source,
  loads: Load[],
  transmissionLines: TransmissionLine[],
  ground: GroundConfig,
  frequency: FrequencyConfig,
  referenceImpedance: number
): SweepRequest {
  return {
    wires: buildWires(wires),
    source: buildSource(source),
    loads: buildLoads(loads),
    transmission_lines: buildTLs(transmissionLines),
    ground: buildGround(ground),
    freq_start: frequency.freqStart,
    freq_end: frequency.freqEnd,
    freq_steps: frequency.freqSteps,
    reference_impedance: referenceImpedance,
  };
}

/** Generic POST helper; throws on non-2xx status with the response body as message. */
async function fetchJson<T>(url: string, body: unknown): Promise<T> {
  const response = await fetch(`${API_BASE}${url}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(`API error ${response.status}: ${text}`);
  }
  return response.json() as Promise<T>;
}

/** Generic GET helper; throws on non-2xx status. */
async function fetchGet<T>(url: string): Promise<T> {
  const response = await fetch(`${API_BASE}${url}`);
  if (!response.ok) {
    const text = await response.text();
    throw new Error(`API error ${response.status}: ${text}`);
  }
  return response.json() as Promise<T>;
}

// --- Response types: raw snake_case shapes from the backend ---

interface RawMetrics {
  peak_gain_db: number;
  peak_theta_deg: number;
  peak_phi_deg: number;
  front_to_back_db: number;
  beamwidth_az_deg: number;
  beamwidth_el_deg: number;
  sidelobe_level_db: number;
  radiation_efficiency: number;
  total_radiated_power_w: number;
  input_power_w: number;
}

interface RawCuts {
  azimuth_deg: number[];
  azimuth_gain_db: number[];
  elevation_deg: number[];
  elevation_gain_db: number[];
  fixed_elevation_deg: number;
  fixed_azimuth_deg: number;
}

interface RawWarning {
  code: string;
  severity: 'info' | 'warn' | 'error';
  message: string;
  wire_index?: number;
  segment_index?: number;
}

interface RawSimulateResponse {
  impedance: { r: number; x: number };
  swr: number;
  reflection: { re: number; im: number };
  reference_impedance: number;
  gain_dbi: number;
  metrics: RawMetrics;
  polar_cuts: RawCuts;
  pattern: { theta: number; phi: number; gain_db: number }[];
  currents: { segment: number; magnitude: number; phase: number }[];
  warnings?: RawWarning[];
}

interface RawSweepResponse {
  frequencies: number[];
  swr: number[];
  impedance: { r: number; x: number }[];
  reflections: { re: number; im: number }[];
  reference_impedance: number;
  warnings?: RawWarning[];
}

/** Run a single-frequency simulation; maps snake_case response to camelCase types. */
export async function simulate(request: SimulateRequest): Promise<SimulationResult> {
  const raw = await fetchJson<RawSimulateResponse>('/api/simulate', request);
  return {
    impedance: raw.impedance,
    swr: raw.swr,
    reflection: raw.reflection || { re: 0, im: 0 },
    referenceImpedance: raw.reference_impedance ?? 50,
    gainDbi: raw.gain_dbi,
    metrics: {
      peakGainDb: raw.metrics?.peak_gain_db ?? raw.gain_dbi,
      peakThetaDeg: raw.metrics?.peak_theta_deg ?? 0,
      peakPhiDeg: raw.metrics?.peak_phi_deg ?? 0,
      frontToBackDb: raw.metrics?.front_to_back_db ?? 0,
      beamwidthAzDeg: raw.metrics?.beamwidth_az_deg ?? 0,
      beamwidthElDeg: raw.metrics?.beamwidth_el_deg ?? 0,
      sidelobeLevelDb: raw.metrics?.sidelobe_level_db ?? 0,
      radiationEfficiency: raw.metrics?.radiation_efficiency ?? 1,
      totalRadiatedPowerW: raw.metrics?.total_radiated_power_w ?? 0,
      inputPowerW: raw.metrics?.input_power_w ?? 0,
    },
    polarCuts: {
      azimuthDeg: raw.polar_cuts?.azimuth_deg ?? [],
      azimuthGainDb: raw.polar_cuts?.azimuth_gain_db ?? [],
      elevationDeg: raw.polar_cuts?.elevation_deg ?? [],
      elevationGainDb: raw.polar_cuts?.elevation_gain_db ?? [],
      fixedElevationDeg: raw.polar_cuts?.fixed_elevation_deg ?? 0,
      fixedAzimuthDeg: raw.polar_cuts?.fixed_azimuth_deg ?? 0,
    },
    pattern: (raw.pattern || []).map((p) => ({
      theta: p.theta,
      phi: p.phi,
      gainDb: p.gain_db,
    })),
    currents: raw.currents || [],
    warnings: (raw.warnings || []).map((w) => ({
      code: w.code,
      severity: w.severity,
      message: w.message,
      wireIndex: w.wire_index,
      segmentIndex: w.segment_index,
    })),
  };
}

/** Run a frequency sweep; returns arrays of SWR, impedance and Γ per frequency step. */
export async function sweep(request: SweepRequest): Promise<SweepResult> {
  const raw = await fetchJson<RawSweepResponse>('/api/sweep', request);
  return {
    frequencies: raw.frequencies,
    swr: raw.swr,
    impedance: raw.impedance,
    reflections: raw.reflections || [],
    referenceImpedance: raw.reference_impedance ?? 50,
    warnings: (raw.warnings || []).map((w) => ({
      code: w.code,
      severity: w.severity,
      message: w.message,
      wireIndex: w.wire_index,
      segmentIndex: w.segment_index,
    })),
  };
}

/** Fetch the list of available antenna templates from the backend. */
export async function getTemplates(): Promise<Template[]> {
  return fetchGet<Template[]>('/api/templates');
}

/** Generate antenna geometry from a named template with user-supplied parameters.
 *  Assigns client-side UUIDs to wires and normalises snake_case keys.
 */
export async function generateTemplate(
  name: string,
  params: Record<string, number>
): Promise<{ wires: Wire[]; source: Source; ground: GroundConfig }> {
  const raw = await fetchJson<Record<string, unknown>>(
    `/api/templates/${encodeURIComponent(name)}`,
    params,
  );

  const wires = ((raw.wires as Array<Record<string, unknown>>) || []).map((w) => ({
    id: typeof crypto !== 'undefined' && crypto.randomUUID
      ? crypto.randomUUID()
      : `${Date.now()}-${Math.random()}`,
    x1: w.x1 as number,
    y1: w.y1 as number,
    z1: w.z1 as number,
    x2: w.x2 as number,
    y2: w.y2 as number,
    z2: w.z2 as number,
    radius: (w.radius as number) || 0.001,
    segments: (w.segments as number) || 11,
    material: ((w.material as string) || '') as Wire['material'],
  }));

  const src = raw.source as Record<string, number>;
  const source: Source = {
    wireIndex: src.wire_index ?? src.wireIndex ?? 0,
    segmentIndex: src.segment_index ?? src.segmentIndex ?? 0,
    voltage: src.voltage ?? 1,
  };

  const grnd = raw.ground as Record<string, unknown>;
  const ground: GroundConfig = {
    type: ((grnd?.type as GroundConfig['type']) || 'free_space'),
    conductivity: (grnd?.conductivity as number) || 0.005,
    permittivity: (grnd?.permittivity as number) || 13,
  };

  return { wires, source, ground };
}
