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
} from 'recharts';
import { useAntennaStore } from '@/store/antennaStore';

const SWRChart: React.FC = () => {
  const sweepResult = useAntennaStore((s) => s.sweepResult);

  const data = useMemo(() => {
    if (!sweepResult) return [];
    return sweepResult.frequencies.map((freq, i) => ({
      frequency: freq,
      swr: sweepResult.swr[i],
    }));
  }, [sweepResult]);

  if (!sweepResult || data.length === 0) {
    return (
      <div className="no-data-message">
        <p>No SWR data.</p>
        <p>Run a frequency sweep to see the SWR chart.</p>
      </div>
    );
  }

  return (
    <div className="chart-container">
      <h3>SWR vs Frequency</h3>
      <ResponsiveContainer width="100%" height={400}>
        <LineChart data={data} margin={{ top: 10, right: 30, left: 10, bottom: 10 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#333" />
          <XAxis
            dataKey="frequency"
            stroke="#aaa"
            label={{ value: 'Frequency (MHz)', position: 'insideBottom', offset: -5, fill: '#aaa' }}
          />
          <YAxis
            domain={[1, 'auto']}
            stroke="#aaa"
            label={{ value: 'SWR', angle: -90, position: 'insideLeft', fill: '#aaa' }}
          />
          <Tooltip
            contentStyle={{ backgroundColor: '#2a2a3e', border: '1px solid #555', color: '#eee' }}
            formatter={(value: number) => [value.toFixed(2), 'SWR']}
            labelFormatter={(label: number) => `${label.toFixed(3)} MHz`}
          />
          <ReferenceLine y={2} stroke="#ff8844" strokeDasharray="5 5" label={{ value: 'SWR 2.0', fill: '#ff8844' }} />
          <Line
            type="monotone"
            dataKey="swr"
            stroke="#44aaff"
            strokeWidth={2}
            dot={false}
            activeDot={{ r: 4 }}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
};

export default SWRChart;
