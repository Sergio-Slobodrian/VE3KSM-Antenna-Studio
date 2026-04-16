/**
 * Export helpers for sweep data.
 *
 * Two formats:
 *   - CSV: frequency, R, X, |Z|, SWR, Re(Γ), Im(Γ), |Γ|, return loss
 *   - Touchstone .s1p (S-parameters, single-port): the canonical
 *     interchange format for VNAs and RF design tools.
 */
import type { SweepResult } from '@/types';

const formatNum = (v: number, digits = 6) => {
  if (!Number.isFinite(v)) return '0';
  return Number(v.toFixed(digits)).toString();
};

/** Trigger a browser file download of the given text payload. */
export function downloadText(filename: string, contents: string, mime: string): void {
  const blob = new Blob([contents], { type: mime });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

/** CSV with one row per frequency point.  Header row is included. */
export function sweepToCSV(sweep: SweepResult): string {
  const z0 = sweep.referenceImpedance || 50;
  const header = [
    'frequency_mhz',
    'r_ohm',
    'x_ohm',
    'mag_z_ohm',
    'swr',
    'gamma_re',
    'gamma_im',
    'mag_gamma',
    'return_loss_db',
  ].join(',');

  const rows: string[] = [header];
  for (let i = 0; i < sweep.frequencies.length; i++) {
    const f = sweep.frequencies[i];
    const z = sweep.impedance[i] ?? { r: 0, x: 0 };
    const swr = sweep.swr[i] ?? 0;
    const g = sweep.reflections[i] ?? { re: 0, im: 0 };
    const magG = Math.sqrt(g.re * g.re + g.im * g.im);
    const magZ = Math.sqrt(z.r * z.r + z.x * z.x);
    const rl = magG > 0 ? -20 * Math.log10(magG) : Infinity;
    rows.push([
      formatNum(f),
      formatNum(z.r),
      formatNum(z.x),
      formatNum(magZ),
      formatNum(swr, 4),
      formatNum(g.re),
      formatNum(g.im),
      formatNum(magG),
      Number.isFinite(rl) ? formatNum(rl, 3) : 'inf',
    ].join(','));
  }
  // Trailing comment with provenance (CSV-friendly).
  rows.push(`# z0_ohm=${z0}`);
  return rows.join('\n') + '\n';
}

/** Touchstone .s1p (S-parameters, single port).
 *
 * Format spec: Touchstone v1.1.  One header line "# Hz S RI R Z0",
 * then one data line per frequency containing freq, Re(S11), Im(S11).
 *
 * The frequencies in our SweepResult are in MHz, so we convert to Hz
 * to match the most common Touchstone convention.
 */
export function sweepToTouchstone(sweep: SweepResult): string {
  const z0 = sweep.referenceImpedance || 50;
  const lines: string[] = [];
  lines.push('!VE3KSM Antenna Studio sweep export');
  lines.push(`!Generated ${new Date().toISOString()}`);
  lines.push(`# Hz S RI R ${z0}`);

  for (let i = 0; i < sweep.frequencies.length; i++) {
    const fHz = (sweep.frequencies[i] ?? 0) * 1e6;
    const g = sweep.reflections[i] ?? { re: 0, im: 0 };
    lines.push(`${formatNum(fHz, 3)} ${formatNum(g.re, 8)} ${formatNum(g.im, 8)}`);
  }
  return lines.join('\n') + '\n';
}
