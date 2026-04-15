/**
 * Current distribution display.
 *
 * Shows the MoM-computed current on each wire segment in two forms:
 *  - A bar chart of current magnitude (Amps) vs segment index.
 *  - A scrollable table with magnitude and phase columns.
 *
 * Data comes from a single-frequency simulation (not sweep).
 */
import React, { useMemo } from 'react';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import { useAntennaStore } from '@/store/antennaStore';

const CurrentDisplay: React.FC = () => {
  const simulationResult = useAntennaStore((s) => s.simulationResult);

  const data = useMemo(() => {
    if (!simulationResult) return [];
    return simulationResult.currents.map((c) => ({
      segment: c.segment,
      magnitude: c.magnitude,
      phase: c.phase,
    }));
  }, [simulationResult]);

  if (!simulationResult || data.length === 0) {
    return (
      <div className="no-data-message">
        <p>No current distribution data.</p>
        <p>Run a simulation to see current distribution.</p>
      </div>
    );
  }

  return (
    <div className="chart-container">
      <h3>Current Distribution</h3>
      <ResponsiveContainer width="100%" height={300}>
        <BarChart data={data} margin={{ top: 10, right: 30, left: 10, bottom: 10 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#333" />
          <XAxis
            dataKey="segment"
            stroke="#aaa"
            label={{ value: 'Segment', position: 'insideBottom', offset: -5, fill: '#aaa' }}
          />
          <YAxis
            stroke="#aaa"
            label={{ value: 'Magnitude (A)', angle: -90, position: 'insideLeft', fill: '#aaa' }}
          />
          <Tooltip
            contentStyle={{ backgroundColor: '#2a2a3e', border: '1px solid #555', color: '#eee' }}
            formatter={(value: number, name: string) => {
              if (name === 'magnitude') return [value.toFixed(4) + ' A', 'Magnitude'];
              return [value.toFixed(1) + '\u00b0', 'Phase'];
            }}
          />
          <Bar dataKey="magnitude" fill="#44aaff" />
        </BarChart>
      </ResponsiveContainer>

      <div className="table-scroll" style={{ marginTop: '16px', maxHeight: '200px' }}>
        <table className="wire-table">
          <thead>
            <tr>
              <th>Segment</th>
              <th>Magnitude (A)</th>
              <th>Phase (deg)</th>
            </tr>
          </thead>
          <tbody>
            {data.map((c) => (
              <tr key={c.segment}>
                <td>{c.segment}</td>
                <td>{c.magnitude.toFixed(4)}</td>
                <td>{c.phase.toFixed(1)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
};

export default CurrentDisplay;
