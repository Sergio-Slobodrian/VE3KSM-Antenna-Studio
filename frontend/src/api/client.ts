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
  GroundConfig,
  FrequencyConfig,
  SimulationResult,
  SweepResult,
  Template,
} from '@/types';

/**
 * Base URL for API requests.  The Go backend serves both the bundled
 * frontend and the JSON API from the same origin, so an empty string
 * (→ relative URL) is the right default.  esbuild's Define option in
 * backend/internal/assets replaces `import.meta.env.VITE_API_BASE` with
 * a literal "" at bundle time, so nothing Vite-specific remains in the
 * emitted JavaScript.
 */
const API_BASE: string = import.meta.env.VITE_API_BASE || '';

/** POST body for /api/simulate (snake_case keys matching Go backend). */
interface SimulateRequest {
  wires: {
    x1: number; y1: number; z1: number;
    x2: number; y2: number; z2: number;
    radius: number; segments: number;
  }[];
  source: { wire_index: number; segment_index: number; voltage: number };
  ground: { type: string; conductivity: number; permittivity: number };
  frequency_mhz: number;
}

/** POST body for /api/sweep (snake_case keys matching Go backend). */
interface SweepRequest {
  wires: {
    x1: number; y1: number; z1: number;
    x2: number; y2: number; z2: number;
    radius: number; segments: number;
  }[];
  source: { wire_index: number; segment_index: number; voltage: number };
  ground: { type: string; conductivity: number; permittivity: number };
  freq_start: number;
  freq_end: number;
  freq_steps: number;
}

// --- Request builders: strip client-only fields (id) and remap key casing ---

/** Strip the client-side `id` field from wires for the API payload. */
function buildWires(wires: Wire[]) {
  return wires.map((w) => ({
    x1: w.x1, y1: w.y1, z1: w.z1,
    x2: w.x2, y2: w.y2, z2: w.z2,
    radius: w.radius, segments: w.segments,
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

/** Assemble a complete single-frequency simulation request. */
export function buildSimulateRequest(
  wires: Wire[],
  source: Source,
  ground: GroundConfig,
  frequency: FrequencyConfig
): SimulateRequest {
  return {
    wires: buildWires(wires),
    source: buildSource(source),
    ground: buildGround(ground),
    frequency_mhz: frequency.frequencyMhz,
  };
}

/** Assemble a frequency-sweep request. */
export function buildSweepRequest(
  wires: Wire[],
  source: Source,
  ground: GroundConfig,
  frequency: FrequencyConfig
): SweepRequest {
  return {
    wires: buildWires(wires),
    source: buildSource(source),
    ground: buildGround(ground),
    freq_start: frequency.freqStart,
    freq_end: frequency.freqEnd,
    freq_steps: frequency.freqSteps,
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

interface RawSimulateResponse {
  impedance: { r: number; x: number };
  swr: number;
  gain_dbi: number;
  pattern: { theta: number; phi: number; gain_db: number }[];
  currents: { segment: number; magnitude: number; phase: number }[];
}

/** Run a single-frequency simulation; maps snake_case response to camelCase types. */
export async function simulate(request: SimulateRequest): Promise<SimulationResult> {
  const raw = await fetchJson<RawSimulateResponse>('/api/simulate', request);
  return {
    impedance: raw.impedance,
    swr: raw.swr,
    gainDbi: raw.gain_dbi,
    pattern: (raw.pattern || []).map((p) => ({
      theta: p.theta,
      phi: p.phi,
      gainDb: p.gain_db,
    })),
    currents: raw.currents || [],
  };
}

/** Run a frequency sweep; returns arrays of SWR and impedance per frequency step. */
export async function sweep(request: SweepRequest): Promise<SweepResult> {
  const raw = await fetchJson<Record<string, unknown>>('/api/sweep', request);
  return {
    frequencies: raw.frequencies as number[],
    swr: raw.swr as number[],
    impedance: raw.impedance as { r: number; x: number }[],
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
    id: crypto.randomUUID ? crypto.randomUUID() : `${Date.now()}-${Math.random()}`,
    x1: w.x1 as number,
    y1: w.y1 as number,
    z1: w.z1 as number,
    x2: w.x2 as number,
    y2: w.y2 as number,
    z2: w.z2 as number,
    radius: (w.radius as number) || 0.001,
    segments: (w.segments as number) || 11,
  }));

  const src = raw.source as Record<string, number>;
  const source: Source = {
    wireIndex: src.wire_index ?? src.wireIndex ?? 0,
    segmentIndex: src.segment_index ?? src.segmentIndex ?? 0,
    voltage: src.voltage ?? 1.0,
  };

  const gnd = raw.ground as Record<string, unknown>;
  const ground: GroundConfig = {
    type: (gnd.type as GroundConfig['type']) || 'free_space',
    conductivity: (gnd.conductivity as number) || 0.005,
    permittivity: (gnd.permittivity as number) || 13,
  };

  return { wires, source, ground };
}
