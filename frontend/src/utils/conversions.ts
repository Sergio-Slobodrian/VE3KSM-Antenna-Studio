export function sphericalToCartesian(
  r: number,
  thetaDeg: number,
  phiDeg: number
): { x: number; y: number; z: number } {
  const thetaRad = (thetaDeg * Math.PI) / 180;
  const phiRad = (phiDeg * Math.PI) / 180;
  return {
    x: r * Math.sin(thetaRad) * Math.cos(phiRad),
    y: r * Math.sin(thetaRad) * Math.sin(phiRad),
    z: r * Math.cos(thetaRad),
  };
}

export function dbToLinear(db: number): number {
  return Math.pow(10, db / 10);
}

export function linearToDb(linear: number): number {
  if (linear <= 0) return -100;
  return 10 * Math.log10(linear);
}

export function formatImpedance(r: number, x: number): string {
  const sign = x >= 0 ? '+' : '-';
  return `${r.toFixed(1)} ${sign} j${Math.abs(x).toFixed(1)} \u03A9`;
}
