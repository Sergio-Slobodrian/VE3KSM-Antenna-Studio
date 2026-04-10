import React, { useMemo } from 'react';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ReferenceLine,
  ResponsiveContainer,
  Legend,
} from 'recharts';
import { useAntennaStore } from '@/store/antennaStore';
import { formatImpedance } from '@/utils/conversions';

const ImpedanceChart: React.FC = () => {
  const sweepResult = useAntennaStore((s) => s.sweepResult);

  const data = useMemo(() => {
    if (!sweepResult) return [];
    return sweepResult.frequencies.map((freq, i) => ({
      frequency: freq,
      R: sweepResult.impedance[i].r,
      X: sweepResult.impedance[i].x,
    }));
  }, [sweepResult]);

  if (!sweepResult || data.length === 0) {
    return (
      <div className="no-data-message">
        <p>No impedance data.</p>
        <p>Run a frequency sweep to see impedance vs. frequency.</p>
      </div>
    );
  }

  return (
    <div className="chart-container">
      <h3>Impedance vs Frequency</h3>
      <ResponsiveContainer width="100%" height={400}>
        <LineChart data={data} margin={{ top: 10, right: 30, left: 10, bottom: 10 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#333" />
          <XAxis
            dataKey="frequency"
            stroke="#aaa"
            label={{ value: 'Frequency (MHz)', position: 'insideBottom', offset: -5, fill: '#aaa' }}
          />
          <YAxis
            stroke="#aaa"
            label={{ value: 'Ohms', angle: -90, position: 'insideLeft', fill: '#aaa' }}
          />
          <Tooltip
            contentStyle={{ backgroundColor: '#2a2a3e', border: '1px solid #555', color: '#eee' }}
            formatter={(_value: number, name: string, props: { payload: { R: number; X: number } }) => {
              if (name === 'R') return [props.payload.R.toFixed(1) + ' \u03A9', 'R'];
              return [props.payload.X.toFixed(1) + ' \u03A9', 'X'];
            }}
            labelFormatter={(label: number) => {
              const point = data.find((d) => d.frequency === label);
              if (point) return `${label.toFixed(3)} MHz  |  ${formatImpedance(point.R, point.X)}`;
              return `${label.toFixed(3)} MHz`;
            }}
          />
          <Legend />
          <ReferenceLine y={0} stroke="#666" strokeDasharray="3 3" />
          <Line
            type="monotone"
            dataKey="R"
            stroke="#ff6644"
            strokeWidth={2}
            dot={false}
            name="R (Resistance)"
          />
          <Line
            type="monotone"
            dataKey="X"
            stroke="#44ccff"
            strokeWidth={2}
            strokeDasharray="5 5"
            dot={false}
            name="X (Reactance)"
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
};

export default ImpedanceChart;
