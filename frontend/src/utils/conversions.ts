/**
 * Convert physics spherical coordinates to Three.js Cartesian coordinates.
 *
 * Physics convention: theta from +Z (zenith), phi in XY ground plane.
 * Three.js convention: Y is up. So we swap Y↔Z:
 *   three_x = physics_x, three_y = physics_z (up), three_z = physics_y
 */
export function sphericalToCartesian(
  r: number,
  thetaDeg: number,
  phiDeg: number
): { x: number; y: number; z: number } {
  const thetaRad = (thetaDeg * Math.PI) / 180;
  const phiRad = (phiDeg * Math.PI) / 180;
  // Physics: x = r sinθ cosφ, y = r sinθ sinφ, z = r cosθ
  // Three.js (Y-up): swap y↔z
  return {
    x: r * Math.sin(thetaRad) * Math.cos(phiRad),
    y: r * Math.cos(thetaRad),
    z: r * Math.sin(thetaRad) * Math.sin(phiRad),
  };
}

/**
 * Convert physics (x, y, z) with Z-up to Three.js (x, y, z) with Y-up.
 */
export function physicsToThree(px: number, py: number, pz: number): [number, number, number] {
  return [px, pz, py];
}

/** Convert a decibel power value to a linear scale factor. */
export function dbToLinear(db: number): number {
  return Math.pow(10, db / 10);
}

/** Convert a linear power ratio to decibels; clamps non-positive inputs to -100 dB. */
export function linearToDb(linear: number): number {
  if (linear <= 0) return -100;
  return 10 * Math.log10(linear);
}

/** Format complex impedance R + jX as a human-readable string with Ohm symbol. */
export function formatImpedance(r: number, x: number): string {
  const sign = x >= 0 ? '+' : '-';
  return `${r.toFixed(1)} ${sign} j${Math.abs(x).toFixed(1)} \u03A9`;
}
