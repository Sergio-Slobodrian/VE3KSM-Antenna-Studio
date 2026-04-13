/**
 * Impedance matching network calculator.
 *
 * Computes component values for L-networks, Pi-networks, and toroidal
 * transformers to match a complex antenna impedance to a real source
 * impedance (typically 50 Ω). All component values are returned in
 * standard SI units (henries, farads, ohms) and also as formatted
 * human-readable strings with nearest standard values.
 */

// ── Standard component value series ──────────────────────────────────────────

/** E12 preferred values (capacitors and some inductors). */
const E12 = [1.0, 1.2, 1.5, 1.8, 2.2, 2.7, 3.3, 3.9, 4.7, 5.6, 6.8, 8.2];

/**
 * Find the nearest standard E12-series value to a given value.
 * Works across decades (pF, nF, µF, nH, µH, etc.).
 */
export function nearestStandard(value: number): number {
  if (value <= 0) return 0;
  const decade = Math.pow(10, Math.floor(Math.log10(value)));
  const normalized = value / decade;
  let best = E12[0];
  let bestDist = Math.abs(normalized - best);
  for (const v of E12) {
    const dist = Math.abs(normalized - v);
    if (dist < bestDist) {
      best = v;
      bestDist = dist;
    }
  }
  // Also check one decade up (10.0)
  if (Math.abs(normalized - 10.0) < bestDist) {
    return 10.0 * decade;
  }
  return best * decade;
}

// ── Formatting helpers ───────────────────────────────────────────────────────

/** Format capacitance in appropriate units (pF, nF, µF). */
export function formatCapacitance(farads: number): string {
  if (farads <= 0) return '—';
  if (farads < 1e-9) return `${(farads * 1e12).toFixed(1)} pF`;
  if (farads < 1e-6) return `${(farads * 1e9).toFixed(2)} nF`;
  return `${(farads * 1e6).toFixed(3)} µF`;
}

/** Format inductance in appropriate units (nH, µH, mH). */
export function formatInductance(henries: number): string {
  if (henries <= 0) return '—';
  if (henries < 1e-6) return `${(henries * 1e9).toFixed(1)} nH`;
  if (henries < 1e-3) return `${(henries * 1e6).toFixed(2)} µH`;
  return `${(henries * 1e3).toFixed(3)} mH`;
}

/** Format reactance as component type + value. */
export function formatReactance(X: number, freqHz: number): string {
  if (Math.abs(X) < 0.01) return '—';
  const omega = 2 * Math.PI * freqHz;
  if (X > 0) {
    // Inductor: X = ωL → L = X/ω
    const L = X / omega;
    return `L = ${formatInductance(L)}`;
  } else {
    // Capacitor: X = -1/(ωC) → C = -1/(ωX)
    const C = -1 / (omega * X);
    return `C = ${formatCapacitance(C)}`;
  }
}

// ── Component descriptor ─────────────────────────────────────────────────────

export interface Component {
  type: 'inductor' | 'capacitor';
  /** Reactance in ohms (positive for L, negative for C). */
  reactance: number;
  /** Component value in SI units (henries or farads). */
  value: number;
  /** Nearest standard E12 value. */
  standardValue: number;
  /** Human-readable string. */
  label: string;
  /** Human-readable standard value. */
  standardLabel: string;
  /** Position in the network. */
  position: 'series' | 'shunt';
}

function makeComponent(X: number, freqHz: number, position: 'series' | 'shunt'): Component {
  const omega = 2 * Math.PI * freqHz;
  if (X >= 0) {
    const L = X / omega;
    const stdL = nearestStandard(L);
    return {
      type: 'inductor',
      reactance: X,
      value: L,
      standardValue: stdL,
      label: formatInductance(L),
      standardLabel: formatInductance(stdL),
      position,
    };
  } else {
    const C = -1 / (omega * X);
    const stdC = nearestStandard(C);
    return {
      type: 'capacitor',
      reactance: X,
      value: C,
      standardValue: stdC,
      label: formatCapacitance(C),
      standardLabel: formatCapacitance(stdC),
      position,
    };
  }
}

// ── L-Network ────────────────────────────────────────────────────────────────

export interface LNetworkResult {
  type: 'L-network';
  /** Two solutions: "low-pass" and "high-pass" configurations. */
  solutions: LNetworkSolution[];
  sourceZ: number;
  loadR: number;
  loadX: number;
}

export interface LNetworkSolution {
  name: string;
  /** Component closer to source. */
  comp1: Component;
  /** Component closer to load. */
  comp2: Component;
  /** Loaded Q factor of the network. */
  Q: number;
  /** Bandwidth in Hz (approx BW = f/Q). */
  bandwidthHz: number;
}

/**
 * Design an L-network to match Z_load = R + jX to Z_source (real).
 * Returns two solutions (low-pass and high-pass configurations).
 */
export function designLNetwork(
  loadR: number,
  loadX: number,
  sourceR: number,
  freqHz: number,
): LNetworkResult {
  const solutions: LNetworkSolution[] = [];

  // We need to match R_load (after cancelling X_load) to R_source.
  // Two topologies depending on whether R_load > R_source or not.

  // Effective load resistance (the reactive part will be absorbed into the network)
  const Rl = loadR;
  const Rs = sourceR;

  if (Rl <= 0) {
    return { type: 'L-network', solutions: [], sourceZ: sourceR, loadR, loadX };
  }

  // For both cases, compute the network Q
  // If Rl > Rs: shunt element on load side, series element on source side
  // If Rl < Rs: shunt element on source side, series element on load side

  const Rhigh = Math.max(Rl, Rs);
  const Rlow = Math.min(Rl, Rs);

  if (Rhigh / Rlow < 1.001) {
    // Already matched (or very close) — just need to cancel reactance
    if (Math.abs(loadX) > 0.1) {
      const cancelX = -loadX;
      const comp = makeComponent(cancelX, freqHz, 'series');
      solutions.push({
        name: 'Series reactance cancellation',
        comp1: comp,
        comp2: comp, // single component
        Q: 0,
        bandwidthHz: freqHz,
      });
    }
    return { type: 'L-network', solutions, sourceZ: sourceR, loadR, loadX };
  }

  const Q = Math.sqrt(Rhigh / Rlow - 1);

  // Solution 1: Low-pass (series L, shunt C — or vice versa)
  // Solution 2: High-pass (series C, shunt L — or vice versa)
  for (const sign of [1, -1]) {
    let seriesX: number;
    let shuntX: number;

    if (Rl > Rs) {
      // Shunt element on load side, series element on source side
      shuntX = sign * Rl / Q;         // shunt reactance (parallel with load)
      seriesX = sign * Q * Rs;         // series reactance
      // Absorb load reactance into the shunt element
      // The shunt element must also cancel the load reactance contribution
      // Adjusted: the series element gets the extra -loadX
      seriesX = seriesX - loadX;       // cancel load reactance in series path
    } else {
      // Shunt element on source side, series element on load side
      shuntX = sign * Rs / Q;
      seriesX = sign * Q * Rl;
      seriesX = seriesX - loadX;
    }

    const seriesComp = makeComponent(seriesX, freqHz, 'series');
    const shuntComp = makeComponent(shuntX, freqHz, 'shunt');

    const name = sign > 0 ? 'Low-pass L-network' : 'High-pass L-network';

    solutions.push({
      name,
      comp1: Rl > Rs ? seriesComp : shuntComp,
      comp2: Rl > Rs ? shuntComp : seriesComp,
      Q,
      bandwidthHz: freqHz / Q,
    });
  }

  return { type: 'L-network', solutions, sourceZ: sourceR, loadR, loadX };
}

// ── Pi-Network ───────────────────────────────────────────────────────────────

export interface PiNetworkResult {
  type: 'Pi-network';
  shuntInput: Component;
  series: Component;
  shuntOutput: Component;
  Q: number;
  bandwidthHz: number;
  sourceZ: number;
  loadR: number;
  loadX: number;
}

/**
 * Design a Pi-network for a user-specified Q (controls bandwidth).
 * Good for matching high impedances down to low impedances.
 */
export function designPiNetwork(
  loadR: number,
  loadX: number,
  sourceR: number,
  freqHz: number,
  desiredQ?: number,
): PiNetworkResult {
  const Rl = loadR;
  const Rs = sourceR;
  const Rhigh = Math.max(Rl, Rs);

  // Minimum Q for the Pi-network
  const Qmin = Math.sqrt(Rhigh / Math.min(Rl, Rs) - 1);
  const Q = Math.max(desiredQ ?? Qmin + 1, Qmin + 0.1);

  // Virtual resistance at the center of the Pi
  const Rv = Rs / (1 + Q * Q);

  // Input shunt: transforms Rs down to Rv
  const Q1 = Q;
  const Xp1 = Rs / Q1;

  // Output shunt: transforms Rl down to Rv
  const Q2 = Math.sqrt(Rl / Rv - 1);
  const Xp2 = Rl / Q2;

  // Series element
  const Xs = Rv * (Q1 + Q2) - loadX;

  const shuntInput = makeComponent(-Xp1, freqHz, 'shunt');  // capacitive shunt
  const series = makeComponent(Xs, freqHz, 'series');
  const shuntOutput = makeComponent(-Xp2, freqHz, 'shunt');  // capacitive shunt

  return {
    type: 'Pi-network',
    shuntInput,
    series,
    shuntOutput,
    Q,
    bandwidthHz: freqHz / Q,
    sourceZ: sourceR,
    loadR,
    loadX,
  };
}

// ── Toroidal Transformer ─────────────────────────────────────────────────────

/** Common toroid cores with their AL values (nH/turn²) and frequency ranges. */
export const TOROID_CORES = [
  { name: 'T-37-2', al: 4.0, freqRange: '1–30 MHz', material: 'Iron powder #2' },
  { name: 'T-37-6', al: 3.0, freqRange: '10–50 MHz', material: 'Iron powder #6' },
  { name: 'T-50-2', al: 4.9, freqRange: '1–30 MHz', material: 'Iron powder #2' },
  { name: 'T-50-6', al: 4.0, freqRange: '10–50 MHz', material: 'Iron powder #6' },
  { name: 'T-68-2', al: 5.7, freqRange: '1–30 MHz', material: 'Iron powder #2' },
  { name: 'T-68-6', al: 4.7, freqRange: '10–50 MHz', material: 'Iron powder #6' },
  { name: 'T-80-2', al: 5.5, freqRange: '1–30 MHz', material: 'Iron powder #2' },
  { name: 'T-106-2', al: 13.5, freqRange: '1–30 MHz', material: 'Iron powder #2' },
  { name: 'FT-37-43', al: 420, freqRange: '0.01–1 MHz', material: 'Ferrite #43' },
  { name: 'FT-50-43', al: 523, freqRange: '0.01–1 MHz', material: 'Ferrite #43' },
  { name: 'FT-82-43', al: 557, freqRange: '0.01–1 MHz', material: 'Ferrite #43' },
];

export interface ToroidResult {
  type: 'Toroidal transformer';
  turnsRatio: number;
  impedanceRatio: number;
  primaryTurns: number;
  secondaryTurns: number;
  /** Recommended cores with computed turns. */
  coreOptions: {
    core: typeof TOROID_CORES[0];
    primaryTurns: number;
    secondaryTurns: number;
    primaryInductance_uH: number;
  }[];
  sourceZ: number;
  loadR: number;
  loadX: number;
  note: string;
}

/**
 * Design a toroidal impedance transformer.
 * Transforms the resistive part of the load to the source impedance.
 * The reactive part should be cancelled with a series component.
 */
export function designToroidalTransformer(
  loadR: number,
  loadX: number,
  sourceR: number,
  freqHz: number,
): ToroidResult {
  const impedanceRatio = loadR / sourceR;
  const turnsRatio = Math.sqrt(impedanceRatio);

  // The primary inductance should be at least 4× the source impedance
  // at the operating frequency for good low-frequency performance:
  // XL_primary >= 4 × R_source → L_primary >= 4 × R_source / (2πf)
  const omega = 2 * Math.PI * freqHz;
  const minPrimaryL = (4 * sourceR) / omega; // henries

  const coreOptions: ToroidResult['coreOptions'] = [];

  for (const core of TOROID_CORES) {
    // N_primary = sqrt(L / AL)  where AL is in nH/turn²
    const alHenries = core.al * 1e-9; // convert nH to H
    const nPrimary = Math.sqrt(minPrimaryL / alHenries);
    const nPrimaryRound = Math.max(Math.round(nPrimary), 2);
    const nSecondary = Math.max(Math.round(nPrimaryRound * turnsRatio), 1);
    const actualPrimaryL = alHenries * nPrimaryRound * nPrimaryRound;

    // Only include cores where the turns count is practical (2–80 turns)
    if (nPrimaryRound >= 2 && nPrimaryRound <= 80 && nSecondary >= 1 && nSecondary <= 80) {
      coreOptions.push({
        core,
        primaryTurns: nPrimaryRound,
        secondaryTurns: nSecondary,
        primaryInductance_uH: actualPrimaryL * 1e6,
      });
    }
  }

  let note = '';
  if (Math.abs(loadX) > 5) {
    const cancelX = -loadX;
    note = `Add a ${formatReactance(cancelX, freqHz)} in series with the antenna to cancel ${loadX.toFixed(1)} Ω reactance before the transformer.`;
  }

  return {
    type: 'Toroidal transformer',
    turnsRatio,
    impedanceRatio,
    primaryTurns: Math.round(Math.sqrt(minPrimaryL / (5e-9)) ), // default for T-50-2
    secondaryTurns: Math.round(Math.sqrt(minPrimaryL / (5e-9)) * turnsRatio),
    coreOptions,
    sourceZ: sourceR,
    loadR,
    loadX,
    note,
  };
}
