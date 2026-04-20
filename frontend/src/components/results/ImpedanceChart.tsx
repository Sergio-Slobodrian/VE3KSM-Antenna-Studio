// Copyright 2026 Sergio Slobodrian
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/**
 * Impedance vs Frequency chart (Recharts line chart).
 *
 * Uses dual Y-axes so resistance R and reactance X can scale independently.
 * R (solid orange) is on the left axis, X (dashed cyan) on the right.
 * This prevents large X values from squashing the R trace to the zero line.
 * A zero-reference line on the right axis helps identify resonance (X = 0).
 */
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
        <LineChart data={data} margin={{ top: 10, right: 60, left: 10, bottom: 10 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#333" />
          <XAxis
            dataKey="frequency"
            stroke="#aaa"
            label={{ value: 'Frequency (MHz)', position: 'insideBottom', offset: -5, fill: '#aaa' }}
          />
          {/* Left Y-axis for Resistance (R) */}
          <YAxis
            yAxisId="left"
            stroke="#ff6644"
            label={{ value: 'R (Ω)', angle: -90, position: 'insideLeft', fill: '#ff6644' }}
          />
          {/* Right Y-axis for Reactance (X) */}
          <YAxis
            yAxisId="right"
            orientation="right"
            stroke="#44ccff"
            label={{ value: 'X (Ω)', angle: 90, position: 'insideRight', fill: '#44ccff' }}
          />
          <Tooltip
            contentStyle={{ backgroundColor: '#2a2a3e', border: '1px solid #555', color: '#eee' }}
            formatter={(_value: number, name: string, props: { payload: { R: number; X: number } }) => {
              if (name === 'R (Resistance)') return [props.payload.R.toFixed(1) + ' \u03A9', 'R'];
              return [props.payload.X.toFixed(1) + ' \u03A9', 'X'];
            }}
            labelFormatter={(label: number) => {
              const point = data.find((d) => d.frequency === label);
              if (point) return `${label.toFixed(3)} MHz  |  ${formatImpedance(point.R, point.X)}`;
              return `${label.toFixed(3)} MHz`;
            }}
          />
          <Legend />
          {/* Zero reference on the reactance axis to show resonance crossing */}
          <ReferenceLine yAxisId="right" y={0} stroke="#44ccff" strokeDasharray="3 3" strokeOpacity={0.5} />
          <Line
            yAxisId="left"
            type="monotone"
            dataKey="R"
            stroke="#ff6644"
            strokeWidth={2}
            dot={false}
            name="R (Resistance)"
          />
          <Line
            yAxisId="right"
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
