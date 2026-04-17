/**
 * Polarization analysis viewer.
 *
 * Displays:
 *  - Headline polarisation at peak-gain direction (AR, tilt, type, sense)
 *  - Axial-ratio vs azimuth and vs elevation SVG line charts
 *  - Tilt-angle vs azimuth and vs elevation SVG line charts
 *  - Polarisation ellipse visualisation at the peak direction
 *
 * Data comes from the SimulationResult.polarization field, which is
 * computed from complex Eθ/Eφ via Stokes parameters in the backend.
 */
import React from 'react';
import { useAntennaStore } from '@/store/antennaStore';
import type { PolarizationMetrics } from '@/types';

/** Colour for polarisation type classification. */
function polColor(polType: string): string {
  switch (polType) {
    case 'circular': return '#4fc3f7';
    case 'elliptical': return '#ffa726';
    default: return '#66bb6a';
  }
}

/** Format AR with appropriate precision. */
function fmtAR(ar: number): string {
  if (ar >= 99) return 'Linear';
  return ar.toFixed(1) + ' dB';
}

/** Compact SVG line chart for a principal-plane cut. */
const CutChart: React.FC<{
  title: string;
  xLabel: string;
  yLabel: string;
  xData: number[];
  yData: number[];
  yMin?: number;
  yMax?: number;
  color?: string;
  refLine?: number;
}> = ({ title, xLabel, yLabel, xData, yData, yMin, yMax, color = '#4fc3f7', refLine }) => {
  if (!xData || xData.length === 0) return null;

  const W = 460, H = 180;
  const PAD = { top: 28, right: 20, bottom: 36, left: 50 };
  const plotW = W - PAD.left - PAD.right;
  const plotH = H - PAD.top - PAD.bottom;

  const xMin = Math.min(...xData);
  const xMax = Math.max(...xData);
  const xRange = xMax - xMin || 1;

  // Clamp y values for display
  const clampedY = yData.map((v) => Math.min(v, 60));
  const dataYMin = yMin ?? Math.min(...clampedY);
  const dataYMax = yMax ?? Math.max(...clampedY);
  const yRange = dataYMax - dataYMin || 1;

  const toX = (v: number) => PAD.left + ((v - xMin) / xRange) * plotW;
  const toY = (v: number) => PAD.top + (1 - (v - dataYMin) / yRange) * plotH;

  const pathD = clampedY
    .map((y, i) => `${i === 0 ? 'M' : 'L'}${toX(xData[i]).toFixed(1)},${toY(y).toFixed(1)}`)
    .join(' ');

  // Tick marks
  const xTicks = [xMin, xMin + xRange / 4, xMin + xRange / 2, xMin + (3 * xRange) / 4, xMax];
  const yTicks = [dataYMin, dataYMin + yRange / 3, dataYMin + (2 * yRange) / 3, dataYMax];

  return (
    <div style={{ marginBottom: 16 }}>
      <div style={{ fontSize: 12, fontWeight: 600, color: '#ccc', marginBottom: 4 }}>{title}</div>
      <svg width={W} height={H} style={{ display: 'block', background: '#1a1a2e', borderRadius: 4 }}>
        {/* Grid */}
        {yTicks.map((y, i) => (
          <line key={`yg${i}`} x1={PAD.left} x2={W - PAD.right} y1={toY(y)} y2={toY(y)}
                stroke="#333" strokeWidth={0.5} />
        ))}
        {/* Reference line (e.g. 3 dB for AR) */}
        {refLine !== undefined && refLine >= dataYMin && refLine <= dataYMax && (
          <line x1={PAD.left} x2={W - PAD.right} y1={toY(refLine)} y2={toY(refLine)}
                stroke="#ff5252" strokeWidth={1} strokeDasharray="4,3" />
        )}
        {/* Data line */}
        <path d={pathD} fill="none" stroke={color} strokeWidth={1.5} />
        {/* Axes */}
        <line x1={PAD.left} x2={PAD.left} y1={PAD.top} y2={H - PAD.bottom} stroke="#666" />
        <line x1={PAD.left} x2={W - PAD.right} y1={H - PAD.bottom} y2={H - PAD.bottom} stroke="#666" />
        {/* X ticks */}
        {xTicks.map((x, i) => (
          <text key={`xt${i}`} x={toX(x)} y={H - PAD.bottom + 14} textAnchor="middle" fontSize={9} fill="#888">
            {x.toFixed(0)}
          </text>
        ))}
        {/* Y ticks */}
        {yTicks.map((y, i) => (
          <text key={`yt${i}`} x={PAD.left - 4} y={toY(y) + 3} textAnchor="end" fontSize={9} fill="#888">
            {y.toFixed(1)}
          </text>
        ))}
        {/* Labels */}
        <text x={W / 2} y={H - 4} textAnchor="middle" fontSize={10} fill="#aaa">{xLabel}</text>
        <text x={12} y={PAD.top + plotH / 2} textAnchor="middle" fontSize={10} fill="#aaa"
              transform={`rotate(-90,12,${PAD.top + plotH / 2})`}>{yLabel}</text>
        {/* 3dB label */}
        {refLine !== undefined && refLine >= dataYMin && refLine <= dataYMax && (
          <text x={W - PAD.right - 2} y={toY(refLine) - 3} textAnchor="end" fontSize={9} fill="#ff5252">
            {refLine} dB
          </text>
        )}
      </svg>
    </div>
  );
};

/** Polarisation ellipse SVG for a single direction. */
const PolarizationEllipse: React.FC<{
  arDb: number;
  tiltDeg: number;
  sense: string;
  polType: string;
}> = ({ arDb, tiltDeg, sense, polType }) => {
  const SIZE = 120;
  const cx = SIZE / 2;
  const cy = SIZE / 2;
  const R = SIZE / 2 - 10;

  // AR in linear scale: AR_linear = 10^(AR_dB/20)
  // For an ellipse: semi-major = R, semi-minor = R / AR_linear
  const arLinear = arDb >= 99 ? 1000 : Math.pow(10, arDb / 20);
  const semiMajor = R;
  const semiMinor = R / Math.min(arLinear, 100);
  const tiltRad = (tiltDeg * Math.PI) / 180;

  // Arrow direction for sense
  const arrowAngle = sense === 'RHCP' ? -Math.PI / 4 : Math.PI / 4;

  return (
    <div style={{ textAlign: 'center' }}>
      <svg width={SIZE} height={SIZE} style={{ display: 'block', margin: '0 auto' }}>
        {/* Reference circle */}
        <circle cx={cx} cy={cy} r={R} fill="none" stroke="#333" strokeWidth={0.5} />
        <line x1={cx - R} x2={cx + R} y1={cy} y2={cy} stroke="#333" strokeWidth={0.5} />
        <line x1={cx} x2={cx} y1={cy - R} y2={cy + R} stroke="#333" strokeWidth={0.5} />
        {/* Ellipse rotated by tilt angle */}
        <ellipse
          cx={cx} cy={cy}
          rx={semiMajor} ry={semiMinor}
          fill="none"
          stroke={polColor(polType)}
          strokeWidth={2}
          transform={`rotate(${-tiltDeg},${cx},${cy})`}
        />
        {/* Tilt axis line */}
        <line
          x1={cx - R * Math.cos(tiltRad)} y1={cy + R * Math.sin(tiltRad)}
          x2={cx + R * Math.cos(tiltRad)} y2={cy - R * Math.sin(tiltRad)}
          stroke={polColor(polType)} strokeWidth={0.5} strokeDasharray="3,3" opacity={0.5}
        />
        {/* Rotation sense arrow */}
        {sense && (
          <text x={cx + 30 * Math.cos(arrowAngle)} y={cy - 30 * Math.sin(arrowAngle)}
                textAnchor="middle" fontSize={14} fill={polColor(polType)}>
            {sense === 'RHCP' ? '\u21bb' : '\u21ba'}
          </text>
        )}
      </svg>
      <div style={{ fontSize: 10, color: '#888', marginTop: 2 }}>
        Polarisation ellipse at peak
      </div>
    </div>
  );
};

const PolarizationViewer: React.FC = () => {
  const simResult = useAntennaStore((s) => s.simulationResult);
  const pol: PolarizationMetrics | undefined = simResult?.polarization;

  if (!simResult) {
    return (
      <div style={{ padding: 12, color: '#999' }}>
        Run a simulation first to see polarisation analysis.
      </div>
    );
  }

  if (!pol || !pol.points || pol.points.length === 0) {
    return (
      <div style={{ padding: 12, color: '#999' }}>
        No polarisation data available. Run a simulation to compute Stokes-parameter-based polarisation analysis.
      </div>
    );
  }

  return (
    <div style={{ padding: 12 }}>
      {/* Headline metrics at peak-gain direction */}
      <div style={{ display: 'flex', gap: 24, alignItems: 'flex-start', marginBottom: 20, flexWrap: 'wrap' }}>
        <div>
          <h4 style={{ margin: '0 0 8px', fontSize: 13, color: '#ddd' }}>
            Polarisation at Peak Gain
          </h4>
          <table style={{ borderCollapse: 'collapse', fontSize: 12 }}>
            <tbody>
              <tr>
                <td style={labelStyle}>Type</td>
                <td style={{ ...valueStyle, color: polColor(pol.peakPolType) }}>
                  {pol.peakPolType.charAt(0).toUpperCase() + pol.peakPolType.slice(1)}
                  {pol.peakSense ? ` (${pol.peakSense})` : ''}
                </td>
              </tr>
              <tr>
                <td style={labelStyle}>Axial Ratio</td>
                <td style={valueStyle}>{fmtAR(pol.peakAxialRatioDb)}</td>
              </tr>
              <tr>
                <td style={labelStyle}>Tilt Angle</td>
                <td style={valueStyle}>{pol.peakTiltDeg.toFixed(1)}&deg;</td>
              </tr>
            </tbody>
          </table>
        </div>

        <PolarizationEllipse
          arDb={pol.peakAxialRatioDb}
          tiltDeg={pol.peakTiltDeg}
          sense={pol.peakSense}
          polType={pol.peakPolType}
        />
      </div>

      {/* Axial Ratio cuts */}
      <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap' }}>
        <CutChart
          title="Axial Ratio vs Azimuth"
          xLabel="Azimuth (deg)"
          yLabel="AR (dB)"
          xData={pol.azimuthDeg}
          yData={pol.azimuthArDb}
          yMin={0}
          yMax={Math.max(40, ...pol.azimuthArDb.map((v) => Math.min(v, 60)))}
          color="#4fc3f7"
          refLine={3}
        />
        <CutChart
          title="Axial Ratio vs Elevation"
          xLabel="Elevation (deg)"
          yLabel="AR (dB)"
          xData={pol.elevationDeg}
          yData={pol.elevationArDb}
          yMin={0}
          yMax={Math.max(40, ...pol.elevationArDb.map((v) => Math.min(v, 60)))}
          color="#4fc3f7"
          refLine={3}
        />
      </div>

      {/* Tilt angle cuts */}
      <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap' }}>
        <CutChart
          title="Tilt Angle vs Azimuth"
          xLabel="Azimuth (deg)"
          yLabel="Tilt (deg)"
          xData={pol.azimuthDeg}
          yData={pol.azimuthTiltDeg}
          yMin={-90}
          yMax={90}
          color="#ffa726"
        />
        <CutChart
          title="Tilt Angle vs Elevation"
          xLabel="Elevation (deg)"
          yLabel="Tilt (deg)"
          xData={pol.elevationDeg}
          yData={pol.elevationTiltDeg}
          yMin={-90}
          yMax={90}
          color="#ffa726"
        />
      </div>
    </div>
  );
};

const labelStyle: React.CSSProperties = {
  padding: '3px 12px 3px 0',
  color: '#aaa',
  fontWeight: 500,
};

const valueStyle: React.CSSProperties = {
  padding: '3px 0',
  color: '#eee',
  fontWeight: 600,
};

export default PolarizationViewer;
