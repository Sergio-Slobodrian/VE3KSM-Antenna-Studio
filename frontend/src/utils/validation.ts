/**
 * Client-side validation for antenna geometry and frequency settings.
 * Returns null when valid, or an error message string when invalid.
 */
import type { Wire, FrequencyConfig } from '@/types';

/** Validate a single wire: checks for zero-length, bad radius, and segment count. */
export function validateWire(wire: Wire): string | null {
  if (
    wire.x1 === wire.x2 &&
    wire.y1 === wire.y2 &&
    wire.z1 === wire.z2
  ) {
    return 'Wire endpoints cannot be identical (zero length)';
  }
  if (wire.radius <= 0) {
    return 'Wire radius must be positive';
  }
  if (wire.segments < 1 || !Number.isInteger(wire.segments)) {
    return 'Segments must be a positive integer';
  }
  if (wire.radius > 1) {
    return 'Wire radius seems too large (> 1 m)';
  }

  // Dielectric coating: mirror the backend checks so users see the error
  // inline instead of waiting for a simulate round-trip.
  if (wire.coatingThickness < 0) {
    return 'Coating thickness must be non-negative';
  }
  if (wire.coatingLossTan < 0) {
    return 'Coating loss tangent must be non-negative';
  }
  if (wire.coatingThickness > 0) {
    if (wire.coatingEpsR < 1) {
      return 'Coating εr must be ≥ 1 when coating thickness > 0';
    }
    const dx = wire.x2 - wire.x1;
    const dy = wire.y2 - wire.y1;
    const dz = wire.z2 - wire.z1;
    const length = Math.sqrt(dx * dx + dy * dy + dz * dz);
    const segLen = length / wire.segments;
    const coatedRadius = wire.radius + wire.coatingThickness;
    if (coatedRadius > segLen / 2) {
      return 'Coated outer radius is too large for the segment length; thin-wire kernel becomes invalid';
    }
  }
  return null;
}

/** Validate frequency config: positive values, sane ranges, sweep ordering. */
export function validateFrequency(freq: FrequencyConfig): string | null {
  if (freq.mode === 'single') {
    if (freq.frequencyMhz <= 0) {
      return 'Frequency must be positive';
    }
    if (freq.frequencyMhz > 100000) {
      return 'Frequency exceeds 100 GHz';
    }
  } else {
    if (freq.freqStart <= 0 || freq.freqEnd <= 0) {
      return 'Start and end frequencies must be positive';
    }
    if (freq.freqStart >= freq.freqEnd) {
      return 'Start frequency must be less than end frequency';
    }
    if (freq.freqSteps < 2) {
      return 'At least 2 sweep steps are required';
    }
    if (freq.freqSteps > 1000) {
      return 'Maximum 1000 sweep steps';
    }
  }
  return null;
}
