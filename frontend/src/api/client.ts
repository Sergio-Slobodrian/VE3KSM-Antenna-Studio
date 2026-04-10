import type {
  Wire,
  Source,
  GroundConfig,
  FrequencyConfig,
  SimulationResult,
  SweepResult,
  Template,
} from '@/types';

const API_BASE = import.meta.env.VITE_API_BASE || '';

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

function buildWires(wires: Wire[]) {
  return wires.map((w) => ({
    x1: w.x1, y1: w.y1, z1: w.z1,
    x2: w.x2, y2: w.y2, z2: w.z2,
    radius: w.radius, segments: w.segments,
  }));
}

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

async function fetchGet<T>(url: string): Promise<T> {
  const response = await fetch(`${API_BASE}${url}`);
  if (!response.ok) {
    const text = await response.text();
    throw new Error(`API error ${response.status}: ${text}`);
  }
  return response.json() as Promise<T>;
}

interface RawSimulateResponse {
  impedance: { r: number; x: number };
  swr: number;
  gain_dbi: number;
  pattern: { theta: number; phi: number; gain_db: number }[];
  currents: { segment: number; magnitude: number; phase: number }[];
}

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

export async function sweep(request: SweepRequest): Promise<SweepResult> {
  const raw = await fetchJson<Record<string, unknown>>('/api/sweep', request);
  return {
    frequencies: raw.frequencies as number[],
    swr: raw.swr as number[],
    impedance: raw.impedance as { r: number; x: number }[],
  };
}

export async function getTemplates(): Promise<Template[]> {
  return fetchGet<Template[]>('/api/templates');
}

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
